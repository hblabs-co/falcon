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

    /// Clears all loaded state. Called by MainTabView on logout so the
    /// tab badge drops to 0 and the hero counter in MatchesView goes
    /// back to "—" instead of lingering with the previous user's data.
    func reset() {
        matches       = []
        currentPage   = 0
        totalPages    = 1
        total         = 0
        unreadCount   = 0
        isLoading     = false
        isLoadingMore = false
        error         = nil
    }

    // MARK: - Private

    private func fetch(page: Int) async {
        do {
            let response = try await MatchesAPI.fetch(page: page, jwt: session.cachedJWT)
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
    static func fetch(page: Int, jwt: String?) async throws -> MatchesResponse {
        guard let jwt, !jwt.isEmpty else {
            throw URLError(.userAuthenticationRequired)
        }
        guard let url = URL(string: "\(NotificationManager.shared.apiURL)/matches?page=\(page)") else {
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
