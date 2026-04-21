import Foundation

@Observable
final class MatchesViewModel {
    private(set) var matches:      [MatchResult] = []
    private(set) var isLoading     = false
    private(set) var isLoadingMore = false
    private(set) var error:        String?
    private(set) var currentPage   = 0
    private(set) var totalPages    = 1
    private(set) var total         = 0

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
            self.error  = nil
        } catch is CancellationError {
            // ignore
        } catch let urlError as URLError where urlError.code == .cancelled {
            // ignore
        } catch {
            print("[matches] fetch page=\(page) error: \(error)")
            self.error = error.localizedDescription
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
