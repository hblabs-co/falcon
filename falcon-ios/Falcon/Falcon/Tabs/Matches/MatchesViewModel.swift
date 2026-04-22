import Foundation
import OSLog

private let log = Logger(subsystem: "co.hblabs.falcon", category: "matches")

@Observable
final class MatchesViewModel {
    private(set) var matches:      [MatchResult] = []
    private(set) var isLoading     = false
    private(set) var isLoadingMore = false
    private(set) var error:        String?
    private(set) var currentPage   = 0
    private(set) var totalPages    = 1
    private(set) var total         = 0
    /// Server-reported unread count (viewed=false) across all matches,
    /// not just loaded pages. MainTabView reads this to render the red
    /// badge on the matches tab icon.
    private(set) var unreadCount   = 0
    /// Counts `match.result` pushes that arrived since the user last
    /// tapped the "new matches" banner. Lives on the VM (not in the
    /// View) because MatchesView is created on demand — any @State
    /// counter inside it would reset on every tab re-entry and miss
    /// pushes that arrived while the user was on another tab.
    private(set) var newMatchCount = 0
    /// Global "only show unread" filter — persists across tab switches
    /// (so coming back to Matches keeps the user's choice) but resets
    /// on logout, notification nav, and banner taps. Sent as
    /// `?only_unread=true` to /matches so the actual DB query filters
    /// server-side — no wasted bandwidth returning rows the UI will hide.
    private(set) var showOnlyUnread = false
    /// Current card density (full / compact / minimal). Lives here so
    /// the choice survives tab switches and gets cleared on logout —
    /// a user who logged out was likely handing the phone off, so
    /// "reset to the most informative layout" is a safer default for
    /// the next login.
    var cardMode: MatchCardMode = .full

    var hasMore: Bool { currentPage < totalPages }

    private let session: SessionManager

    init(session: SessionManager) {
        self.session = session
    }

    // MARK: - Public

    func loadInitial() async {
        guard !isLoading else { return }
        isLoading = true
        error     = nil
        await fetch(page: 1)
        isLoading = false
    }

    func refresh() async {
        error = nil
        await fetch(page: 1)
    }

    func loadMore() async {
        guard !isLoadingMore, hasMore else { return }
        isLoadingMore = true
        await fetch(page: currentPage + 1)
        isLoadingMore = false
    }

    /// Optimistic bump for a realtime `match.result` push. Nudges the
    /// visible counters (hero total, tab unread badge, banner count)
    /// immediately so the user sees "live" feedback before the next
    /// /matches refetch. The actual match doc arrives on refresh — we
    /// don't fabricate one here.
    func bumpOnNewMatch() {
        total         += 1
        unreadCount   += 1
        newMatchCount += 1
    }

    /// Resets the banner counter after the user taps the floating pill
    /// (which also kicks off a refetch). Kept separate from bumpOnNewMatch
    /// so the banner's "0 → hidden" transition is driven explicitly.
    func clearNewMatchCount() {
        newMatchCount = 0
    }

    /// Flips the filter and re-fetches page 1 so the list reflects the
    /// new server-side filter. No-op if the flag doesn't actually change.
    func setFilter(onlyUnread: Bool) async {
        guard showOnlyUnread != onlyUnread else { return }
        showOnlyUnread = onlyUnread
        await fetch(page: 1)
    }

    /// Resets the filter to "All" without triggering a fetch — the caller
    /// is expected to follow up with `refresh()` if new data is needed.
    /// Used on notification nav, banner taps, and anywhere we want a
    /// clean "show everything" landing state.
    func clearFilter() {
        showOnlyUnread = false
    }

    /// Marks every local match with the given project_id as normalized.
    /// Called from the MatchesView listener that watches falcon-realtime
    /// `project.normalized` pushes so the "Zum Job" spinner clears the
    /// instant the normalizer finishes, without a refetch.
    func markProjectNormalized(projectId: String) {
        for i in matches.indices where matches[i].projectId == projectId && !matches[i].isNormalized {
            matches[i].isNormalized = true
        }
    }

    /// Clears all loaded state. Called by MainTabView on logout so the
    /// tab badge drops to 0 and the hero counter in MatchesView goes
    /// back to "—" instead of lingering with the previous user's data.
    func reset() {
        matches        = []
        currentPage    = 0
        totalPages     = 1
        total          = 0
        unreadCount    = 0
        newMatchCount  = 0
        showOnlyUnread = false
        cardMode       = .full
        isLoading      = false
        isLoadingMore  = false
        error          = nil
    }

    // MARK: - Private

    private func fetch(page: Int) async {
        do {
            let response = try await MatchesAPI.fetch(
                page: page,
                onlyUnread: showOnlyUnread,
                jwt: session.cachedJWT
            )
            if page == 1 {
                matches = response.data
            } else {
                let existingIDs = Set(matches.map(\.id))
                let newItems = response.data.filter { !existingIDs.contains($0.id) }
                matches.append(contentsOf: newItems)
            }
            currentPage = response.pagination.page
            totalPages  = response.pagination.totalPages
            total       = response.pagination.total
            unreadCount = response.unreadCount
            self.error  = nil
        } catch is CancellationError {
            // ignore
        } catch let urlError as URLError where urlError.code == .cancelled {
            // ignore
        } catch {
            log.error("fetch page=\(page, privacy: .public) error: \(error.localizedDescription, privacy: .public)")
            self.error = error.localizedDescription
        }
    }

    /// Optimistically flips the local match to viewed and decrements the
    /// unread counter, then fires a PATCH to the server. If the PATCH
    /// fails the state stays optimistic — worst case the server disagrees
    /// and the badge shows the "real" count on the next fetch.
    func markViewed(projectId: String, cvId: String) {
        if let idx = matches.firstIndex(where: { $0.projectId == projectId && $0.cvId == cvId }),
           !matches[idx].isViewed {
            matches[idx].isViewed = true
            if unreadCount > 0 { unreadCount -= 1 }
        }
        Task { [jwt = session.cachedJWT] in
            do {
                try await MatchesAPI.markViewed(projectId: projectId, cvId: cvId, jwt: jwt)
            } catch {
                log.error("markViewed project=\(projectId, privacy: .public) cv=\(cvId, privacy: .public) failed: \(error.localizedDescription, privacy: .public)")
            }
        }
    }
}

// MARK: - API

enum MatchesAPI {
    static func fetch(page: Int, onlyUnread: Bool, jwt: String?) async throws -> MatchesResponse {
        guard let jwt, !jwt.isEmpty else {
            throw URLError(.userAuthenticationRequired)
        }
        var comps = URLComponents(string: "\(NotificationManager.shared.apiURL)/matches")
        var items: [URLQueryItem] = [URLQueryItem(name: "page", value: "\(page)")]
        if onlyUnread { items.append(URLQueryItem(name: "only_unread", value: "true")) }
        comps?.queryItems = items
        guard let url = comps?.url else {
            throw URLError(.badURL)
        }
        var req = URLRequest(url: url)
        req.setValue("Bearer \(jwt)", forHTTPHeaderField: "Authorization")
        let (data, response) = try await URLSession.shared.data(for: req)
        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            throw URLError(.badServerResponse)
        }
        return try JSONDecoder().decode(MatchesResponse.self, from: data)
    }

    /// Fire-and-forget PATCH to mark a match as viewed. Idempotent
    /// server-side — retries or double-calls are safe.
    static func markViewed(projectId: String, cvId: String, jwt: String?) async throws {
        guard let jwt, !jwt.isEmpty else { throw URLError(.userAuthenticationRequired) }
        guard let url = URL(string: "\(NotificationManager.shared.apiURL)/matches/viewed") else {
            throw URLError(.badURL)
        }
        var req = URLRequest(url: url)
        req.httpMethod = "PATCH"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.setValue("Bearer \(jwt)",    forHTTPHeaderField: "Authorization")
        req.httpBody = try JSONSerialization.data(withJSONObject: [
            "project_id": projectId,
            "cv_id":      cvId
        ])
        let (_, response) = try await URLSession.shared.data(for: req)
        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            throw URLError(.badServerResponse)
        }
    }

    static func fetchProject(id: String, lang: String, jwt: String?) async throws -> ProjectItem {
        guard let url = URL(string: "\(NotificationManager.shared.apiURL)/projects/\(id)?lang=\(lang)") else {
            throw URLError(.badURL)
        }
        var req = URLRequest(url: url)
        if let jwt, !jwt.isEmpty {
            req.setValue("Bearer \(jwt)", forHTTPHeaderField: "Authorization")
        }
        let (data, response) = try await URLSession.shared.data(for: req)
        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            throw URLError(.badServerResponse)
        }
        return try JSONDecoder().decode(ProjectItem.self, from: data)
    }
}
