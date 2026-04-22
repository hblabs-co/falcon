import Foundation
import OSLog

private let log = FalconLog.make(category: "projects")

@Observable
final class ProjectsViewModel {
    private(set) var projects:     [ProjectItem] = []
    private(set) var isLoading     = false
    private(set) var isLoadingMore = false
    private(set) var error:        String?
    private(set) var currentPage   = 0
    private(set) var totalPages    = 1
    private(set) var total         = 0
    private(set) var todayCount    = 0
    /// Counts `project.normalized` pushes that arrived since the user
    /// last tapped the "new projects" banner. Lives on the VM (not in
    /// the View) so the state survives any future restructuring of
    /// MainTabView — if ProjectsView ever stops being kept-alive via
    /// opacity(0), the counter keeps working.
    private(set) var newProjectCount = 0

    var hasMore: Bool { currentPage < totalPages }

    // MARK: - Public

    func loadInitial() async {
        guard !isLoading else {
            log.info("loadInitial skipped — already loading")
            return
        }
        log.info("loadInitial starting")
        isLoading = true
        error     = nil
        await fetch(page: 1)
        isLoading = false
        log.info("loadInitial done — \(self.projects.count, privacy: .public) projects, error=\((self.error ?? "nil"), privacy: .public)")
    }

    func refresh() async {
        log.info("refresh starting")
        error = nil
        await fetch(page: 1)
        log.info("refresh done — \(self.projects.count, privacy: .public) projects")
    }

    func loadMore() async {
        guard !isLoadingMore, hasMore else { return }
        isLoadingMore = true
        await fetch(page: currentPage + 1)
        isLoadingMore = false
    }

    /// Optimistic bump for a realtime `project.normalized` push. Updates
    /// the hero-banner "today" count, the total, and the floating-banner
    /// counter so the UI shows live growth without waiting for the next
    /// refresh.
    func bumpOnNewProject() {
        todayCount      += 1
        total           += 1
        newProjectCount += 1
    }

    /// Resets the banner counter after the user taps the floating pill
    /// (which also kicks off a refetch).
    func clearNewProjectCount() {
        newProjectCount = 0
    }

    // MARK: - Private

    private func fetch(page: Int) async {
        log.info("fetch page=\(page, privacy: .public)")
        do {
            let response = try await ProjectsAPI.fetch(page: page)
            log.info("fetch page=\(page, privacy: .public) got \(response.data.count, privacy: .public) items")
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
            log.error("fetch page=\(page, privacy: .public) cancelled (CancellationError)")
        } catch let urlError as URLError where urlError.code == .cancelled {
            log.error("fetch page=\(page, privacy: .public) cancelled (URLError)")
        } catch {
            log.error("fetch page=\(page, privacy: .public) error: \(error.localizedDescription, privacy: .public)")
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
