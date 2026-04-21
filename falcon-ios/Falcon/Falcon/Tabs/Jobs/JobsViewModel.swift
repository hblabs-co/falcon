import Foundation

@Observable
final class JobsViewModel {
    private(set) var projects:     [ProjectItem] = []
    private(set) var isLoading     = false
    private(set) var isLoadingMore = false
    private(set) var error:        String?
    private(set) var currentPage   = 0
    private(set) var totalPages    = 1
    private(set) var total         = 0
    private(set) var todayCount    = 0

    var hasMore: Bool { currentPage < totalPages }

    // MARK: - Public

    func loadInitial() async {
        guard !isLoading else {
            print("[jobs] loadInitial skipped — already loading")
            return
        }
        print("[jobs] loadInitial starting")
        isLoading = true
        error     = nil
        await fetch(page: 1)
        isLoading = false
        print("[jobs] loadInitial done — \(projects.count) projects, error=\(error ?? "nil")")
    }

    func refresh() async {
        print("[jobs] refresh starting")
        error = nil
        await fetch(page: 1)
        print("[jobs] refresh done — \(projects.count) projects")
    }

    func loadMore() async {
        guard !isLoadingMore, hasMore else { return }
        isLoadingMore = true
        await fetch(page: currentPage + 1)
        isLoadingMore = false
    }

    // MARK: - Private

    private func fetch(page: Int) async {
        print("[jobs] fetch page=\(page)")
        do {
            let response = try await ProjectsAPI.fetch(page: page)
            print("[jobs] fetch page=\(page) got \(response.data.count) items")
            if page == 1 {
                projects = response.data
            } else {
                let existingIDs = Set(projects.map(\.id))
                let newItems = response.data.filter { !existingIDs.contains($0.id) }
                projects.append(contentsOf: newItems)
            }
            currentPage = response.pagination.page
            totalPages  = response.pagination.totalPages
            total       = response.pagination.total
            todayCount  = response.todayCount
            self.error  = nil
        } catch is CancellationError {
            print("[jobs] fetch page=\(page) cancelled (CancellationError)")
        } catch let urlError as URLError where urlError.code == .cancelled {
            print("[jobs] fetch page=\(page) cancelled (URLError)")
        } catch {
            print("[jobs] fetch page=\(page) error: \(error)")
            self.error = error.localizedDescription
        }
    }

}

// MARK: - API

enum ProjectsAPI {
    static func fetch(page: Int) async throws -> ProjectsResponse {
        let lang = LanguageManager.shared.appLanguage.rawValue
        guard let url = URL(string: "\(NotificationManager.shared.apiURL)/projects?page=\(page)&lang=\(lang)") else {
            throw URLError(.badURL)
        }
        let (data, response) = try await URLSession.shared.data(from: url)
        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            throw URLError(.badServerResponse)
        }
        return try JSONDecoder().decode(ProjectsResponse.self, from: data)
    }
}
