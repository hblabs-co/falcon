import Foundation
import ActivityKit

@Observable
final class JobsViewModel {
    private(set) var projects:     [ProjectItem] = []
    private(set) var isLoading     = false
    private(set) var isLoadingMore = false
    private(set) var error:        String?
    private(set) var currentPage   = 0
    private(set) var totalPages    = 1
    private(set) var total         = 0

    var hasMore: Bool { currentPage < totalPages }

    private var liveActivity: Activity<FalconJobsAttributes>?

    // MARK: - Public

    func loadInitial() async {
        guard !isLoading else { return }
        isLoading = true
        error     = nil
        projects  = []
        currentPage = 0
        await fetch(page: 1)
        isLoading = false
    }

    func loadMore() async {
        guard !isLoadingMore, hasMore else { return }
        isLoadingMore = true
        await fetch(page: currentPage + 1)
        isLoadingMore = false
    }

    func endLiveActivity() async {
        let state = FalconJobsAttributes.ContentState(
            projectCount: total,
            latestTitle: projects.first?.displayTitle ?? ""
        )
        await liveActivity?.end(ActivityContent(state: state, staleDate: nil), dismissalPolicy: .immediate)
        liveActivity = nil
    }

    // MARK: - Private

    private func fetch(page: Int) async {
        do {
            let response = try await ProjectsAPI.fetch(page: page)
            if page == 1 {
                projects = response.data
            } else {
                projects.append(contentsOf: response.data)
            }
            currentPage = response.pagination.page
            totalPages  = response.pagination.totalPages
            total       = response.pagination.total
            await updateLiveActivity()
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func updateLiveActivity() async {
        guard ActivityAuthorizationInfo().areActivitiesEnabled else { return }
        let state = FalconJobsAttributes.ContentState(
            projectCount: total,
            latestTitle: projects.first?.displayTitle ?? ""
        )
        if let activity = liveActivity {
            await activity.update(ActivityContent(state: state, staleDate: nil))
        } else {
            liveActivity = try? Activity.request(
                attributes: FalconJobsAttributes(),
                content: ActivityContent(state: state, staleDate: nil),
                pushType: nil
            )
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
