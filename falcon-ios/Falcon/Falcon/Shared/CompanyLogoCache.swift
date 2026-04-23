import Foundation
import CryptoKit

/// App Group identifier shared between the main Falcon target and
/// FalconWidget. Configured in both targets' entitlements. Everything
/// the cache writes to disk lives inside this container so the widget
/// process (which runs the Live Activity view) can read it
/// synchronously — widgets can't do `AsyncImage` reliably because
/// iOS snapshots the view before any network request resolves.
enum FalconAppGroup {
    static let identifier = "group.co.hblabs.falcon"

    /// Root directory for all shared files. Returns nil if the App
    /// Group entitlement isn't configured yet (Xcode capability still
    /// pending) — callers treat that as "no cache" and fall back to
    /// initials.
    static var containerURL: URL? {
        FileManager.default.containerURL(forSecurityApplicationGroupIdentifier: identifier)
    }

    /// Subdirectory for company logo PNG/JPG payloads. Kept next to
    /// `companies.json` (the manifest) under a single "companies"
    /// folder so a nuke-the-cache operation is one recursive delete.
    static var companyLogosDir: URL? {
        guard let base = containerURL else { return nil }
        return base.appendingPathComponent("companies/logos", isDirectory: true)
    }

    /// Path to the cached manifest produced by the last successful
    /// `GET /companies` pull. Reading this synchronously is what lets
    /// the widget resolve "companyLogoUrl → local filename".
    static var companiesManifestURL: URL? {
        containerURL?.appendingPathComponent("companies/companies.json")
    }
}

/// Entry in the cached companies manifest. Stays on disk, not in the
/// Live Activity ContentState — passing this through APNs would blow
/// the 4KB payload limit with even a handful of logos.
struct CompanyManifestEntry: Codable, Hashable {
    let id:       String
    let name:     String
    let platform: String
    /// Full MinIO URL — the canonical key. Matches the
    /// `companyLogoUrl` that arrives in Live Activity ContentState, so
    /// the widget can look up the local file by hashing the URL.
    let logoUrl:  String

    enum CodingKeys: String, CodingKey {
        case id
        case name
        case platform
        case logoUrl = "logo_url"
    }
}

struct CompaniesManifest: Codable {
    let companies: [CompanyManifestEntry]
    let updatedAt: Date
    let count:     Int

    enum CodingKeys: String, CodingKey {
        case companies
        case updatedAt = "updated_at"
        case count
    }
}

/// Maps a logo URL to a deterministic on-disk filename. Same URL →
/// same filename across devices, without the server having to stamp
/// and persist filenames. Using SHA-256 means the probability of
/// collision across the entire company catalogue is ignorable and we
/// don't care about cryptographic strength here — this is a naming
/// scheme, not a security primitive.
enum CompanyLogoFilename {
    static func from(url: String) -> String {
        let digest = SHA256.hash(data: Data(url.utf8))
        let hex = digest.map { String(format: "%02x", $0) }.joined()
        // Keep the original extension when we can spot it in the URL
        // so QuickLook / Previews show a proper thumbnail if anyone
        // inspects the App Group container. Default to .jpg because
        // that's what MinIO emits today for company logos.
        let ext = url.lowercased().hasSuffix(".png") ? "png" : "jpg"
        return "\(hex).\(ext)"
    }

    /// Absolute URL of the cached logo for a given remote URL, or nil
    /// if the App Group isn't configured. Callers must check
    /// `FileManager.fileExists(atPath:)` before loading.
    static func localURL(for remoteURL: String) -> URL? {
        guard let dir = FalconAppGroup.companyLogosDir else { return nil }
        return dir.appendingPathComponent(from(url: remoteURL))
    }
}
