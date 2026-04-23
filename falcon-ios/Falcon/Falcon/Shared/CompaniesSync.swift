import Foundation
import OSLog

private let log = FalconLog.make(category: "companies-sync")

/// Owns the periodic refresh of `/companies` and the on-disk logo
/// cache in the App Group container. Stateless singleton; persistence
/// lives in the container (manifest JSON + logo files) so cold launches
/// always see the last good cache.
///
/// Refresh cadence: 2 hours. Chosen over a longer window because
/// iOS can't run background fetches via WebSocket — the only way a
/// brand-new company's logo reaches a device without a silent push
/// path is "on the next foreground resume after TTL expires". 2h
/// keeps the "user sees initials for a new company" window tight
/// without turning app launches into chatty sync bursts (most
/// refreshes inside this window will be no-ops anyway — manifest
/// unchanged, zero downloads). App version changes still bypass this
/// and force a fresh pull.
@MainActor
final class CompaniesSync {
    static let shared = CompaniesSync()
    private init() {}

    /// How often to re-pull the manifest. Stored in UserDefaults so a
    /// relaunch within the window skips the network entirely.
    private let refreshInterval: TimeInterval = 2 * 60 * 60
    private let lastFetchKey   = "CompaniesSync.lastFetchAt"
    private let lastVersionKey = "CompaniesSync.lastFetchAppVersion"

    private var lastFetchAt: Date? {
        get {
            let ts = UserDefaults.standard.double(forKey: lastFetchKey)
            return ts > 0 ? Date(timeIntervalSince1970: ts) : nil
        }
        set {
            UserDefaults.standard.set(newValue?.timeIntervalSince1970 ?? 0, forKey: lastFetchKey)
        }
    }

    private var lastFetchAppVersion: String? {
        get { UserDefaults.standard.string(forKey: lastVersionKey) }
        set { UserDefaults.standard.set(newValue, forKey: lastVersionKey) }
    }

    /// Current app version+build, e.g. "1.0.1-42". A change in either
    /// component (new TestFlight build, new App Store release) is
    /// treated as a cache-invalidation signal regardless of the TTL.
    /// Reason: a new build may ship schema changes or expect fresh
    /// logos, and forcing one refetch on first launch of a new binary
    /// costs nothing but avoids the "old logos for up to 7d after
    /// update" footgun.
    private var currentAppVersion: String {
        let info = Bundle.main.infoDictionary
        let short = info?["CFBundleShortVersionString"] as? String ?? "?"
        let build = info?["CFBundleVersion"] as? String ?? "?"
        return "\(short)-\(build)"
    }

    /// Entry point called from app launch. Cheap no-op when the cache
    /// is still fresh AND the app version hasn't changed; otherwise
    /// fires a background Task so app launch isn't blocked on network.
    func refreshIfNeeded(force: Bool = false) {
        let version = currentAppVersion
        let versionChanged = lastFetchAppVersion != version

        if !force, !versionChanged,
           let last = lastFetchAt, Date().timeIntervalSince(last) < refreshInterval {
            log.debug("cache fresh (age=\(Int(Date().timeIntervalSince(last)), privacy: .public)s, version=\(version, privacy: .public)) — skipping")
            return
        }

        if versionChanged {
            log.info("app version changed \(self.lastFetchAppVersion ?? "nil", privacy: .public) → \(version, privacy: .public) — forcing refresh")
        }
        Task { await refresh() }
    }

    /// Full refresh: pull manifest, diff against on-disk copy, download
    /// any logos whose URL changed or are missing. Writes happen to
    /// the App Group container so the widget process picks them up on
    /// its next snapshot.
    func refresh() async {
        guard FalconAppGroup.containerURL != nil else {
            log.error("App Group not configured — skipping company sync")
            return
        }

        let manifest: CompaniesManifest
        do {
            manifest = try await fetchManifest()
        } catch {
            log.error("fetch manifest failed: \(error.localizedDescription, privacy: .public)")
            return
        }

        log.info("fetched manifest count=\(manifest.count, privacy: .public)")

        // Decide which logos need downloading. A logo is needed when
        // either (a) the file doesn't exist yet, or (b) the URL
        // changed vs the previous manifest (which implicitly means
        // the hash-derived filename changed too).
        let previous = readManifest()?.companies ?? []
        let previousByURL = Dictionary(uniqueKeysWithValues: previous.map { ($0.logoUrl, $0) })

        var toDownload: [(entry: CompanyManifestEntry, localURL: URL)] = []
        var skipped = 0
        for entry in manifest.companies where !entry.logoUrl.isEmpty {
            guard let localURL = CompanyLogoFilename.localURL(for: entry.logoUrl) else { continue }
            let fileExists = FileManager.default.fileExists(atPath: localURL.path)
            let urlChanged = previousByURL[entry.logoUrl] == nil
            if fileExists && !urlChanged {
                skipped += 1
                continue
            }
            toDownload.append((entry, localURL))
        }

        // Parallel download with bounded concurrency — see
        // Sequence+Concurrent.swift for the rationale behind the cap.
        // 8 in-flight is a safe HTTP default for a modest device +
        // our on-prem MinIO.
        let outcomes = await toDownload.concurrentMap(maxConcurrent: 8) { job in
            await Self.attemptDownload(entry: job.entry, to: job.localURL)
        }
        let downloaded = outcomes.filter { $0 }.count

        // Persist manifest LAST so if download failed we don't orphan
        // on-disk state. The widget reads the manifest to resolve
        // URL → filename; an out-of-sync manifest would point at a
        // logo that doesn't exist yet.
        writeManifest(manifest)
        lastFetchAt = Date()
        // Stamp the version after the manifest write so a crash
        // mid-refresh doesn't convince the next launch that this
        // version's cache is already up-to-date.
        lastFetchAppVersion = currentAppVersion

        log.info("sync done downloaded=\(downloaded, privacy: .public) skipped=\(skipped, privacy: .public)")
    }

    // MARK: - Network

    private func fetchManifest() async throws -> CompaniesManifest {
        guard let url = URL(string: "\(NotificationManager.shared.apiURL)/companies") else {
            throw URLError(.badURL)
        }
        let (data, response) = try await URLSession.shared.data(from: url)
        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            throw URLError(.badServerResponse)
        }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try decoder.decode(CompaniesManifest.self, from: data)
    }

    /// Downloads a single logo to disk. Static + nonisolated on
    /// purpose — called from the concurrent TaskGroup, so pinning it
    /// to `@MainActor` would serialise every completion back on the
    /// main thread and defeat the parallelism. Returns true on
    /// success, false on any failure; logs the failure but doesn't
    /// throw so the caller can keep a clean `[Bool]` outcome list.
    nonisolated static func attemptDownload(entry: CompanyManifestEntry, to localURL: URL) async -> Bool {
        do {
            guard let remote = URL(string: entry.logoUrl) else { throw URLError(.badURL) }
            let (tmp, response) = try await URLSession.shared.download(from: remote)
            guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
                throw URLError(.badServerResponse)
            }
            try FileManager.default.createDirectory(
                at: localURL.deletingLastPathComponent(),
                withIntermediateDirectories: true
            )
            // Replace atomically — avoid the "widget reads half-written
            // bytes" race during a concurrent refresh.
            if FileManager.default.fileExists(atPath: localURL.path) {
                try FileManager.default.removeItem(at: localURL)
            }
            try FileManager.default.moveItem(at: tmp, to: localURL)
            return true
        } catch {
            log.error("download \(entry.id, privacy: .public) failed: \(error.localizedDescription, privacy: .public)")
            return false
        }
    }

    // MARK: - Manifest persistence

    private func readManifest() -> CompaniesManifest? {
        guard let url = FalconAppGroup.companiesManifestURL,
              let data = try? Data(contentsOf: url) else { return nil }
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try? decoder.decode(CompaniesManifest.self, from: data)
    }

    private func writeManifest(_ manifest: CompaniesManifest) {
        guard let url = FalconAppGroup.companiesManifestURL else { return }
        do {
            try FileManager.default.createDirectory(
                at: url.deletingLastPathComponent(),
                withIntermediateDirectories: true
            )
            let encoder = JSONEncoder()
            encoder.dateEncodingStrategy = .iso8601
            let data = try encoder.encode(manifest)
            try data.write(to: url, options: .atomic)
        } catch {
            log.error("write manifest failed: \(error.localizedDescription, privacy: .public)")
        }
    }
}
