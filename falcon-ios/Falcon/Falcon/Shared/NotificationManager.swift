import Foundation
import UserNotifications
import UIKit
import ActivityKit

@Observable
final class NotificationManager: NSObject, UNUserNotificationCenterDelegate {

    static let shared = NotificationManager()

    var authStatus: UNAuthorizationStatus = .notDetermined

    /// Master Settings toggle: "Live Activities" per app. Refreshed on
    /// refreshStatus(). Banner nudges user to Settings when this is off.
    var liveActivitiesEnabled: Bool = true

    /// iOS 17.2+ "More Frequent Updates" toggle. Before 17.2 the concept
    /// doesn't exist — we default to `true` so the badge logic doesn't
    /// fire on older devices.
    var frequentPushesEnabled: Bool = true

    /// True when EITHER system toggle for Live Activities is disabled —
    /// master off OR (on 17.2+) frequent updates off. Both must be on for
    /// activities to deliver reliably; the badge hides only in that case.
    var liveActivitiesRestricted: Bool {
        if #available(iOS 17.2, *) {
            return !liveActivitiesEnabled || !frequentPushesEnabled
        }
        return !liveActivitiesEnabled
    }

    var deviceToken: String? {
        didSet { UserDefaults.standard.set(deviceToken, forKey: "apns_device_token") }
    }
    var registrationError: String?
    var signalStatus: SignalStatus = .idle
    var lastNotification: ReceivedNotification?

    /// Set when the user taps a MATCH_RESULT notification. Observers (MainTabView,
    /// MatchesView) react by switching to the matches tab, scrolling to the
    /// match, and opening its details. Reset to nil after the UI consumes it.
    var pendingMatchNavigation: MatchNotificationPayload?

    /// Live Activity started on foreground MATCH_RESULT pushes. nil when no
    /// activity is currently running.
    private var matchActivity: Activity<FalconMatchAttributes>?

    /// iOS 17.2+ push-to-start token. Re-sent to backend on each rotation so
    /// signal can trigger Live Activities even when the app is killed.
    private(set) var liveActivityPushToStartToken: String?
    private var liveActivityTokenObserver: Task<Void, Never>?
    private var liveActivityUpdatesObserver: Task<Void, Never>?

    // API URL source of truth:
    // - DEBUG   → UserDefaults (editable from Settings) with Config.apiURL as fallback
    // - Release → hardcoded production falcon-api URL, never user-editable
    private(set) var apiURL: String

    enum SignalStatus {
        case idle, registering, registered
        case failed(String)
    }

    struct ReceivedNotification {
        let title: String
        let body: String
        let receivedAt: Date
    }

    /// Payload extracted from a tapped MATCH_RESULT push. Equatable so SwiftUI's
    /// onChange fires only when the target match changes.
    struct MatchNotificationPayload: Equatable {
        let projectID: String
        let cvID: String
        /// Matches `MatchResult.id` (composite) so ScrollViewReader + sheet(item:)
        /// can target it directly without an extra lookup.
        var matchID: String { "\(projectID)-\(cvID)" }
    }

    private override init() {
        #if DEBUG
        apiURL = UserDefaults.standard.string(forKey: "api_url") ?? Config.apiURL
        #else
        apiURL = "https://api.falcon.hblabs.co" // TODO: confirm production falcon-api URL
        #endif
        super.init()
        deviceToken = UserDefaults.standard.string(forKey: "apns_device_token")
        SessionManager.shared.onUserIDAvailable = { [weak self] id in
            self?.registerWithSignal(userID: id)
        }
        startLiveActivityTokenObserver()
        startLiveActivityUpdatesObserver()
    }

    /// Observes push-to-start tokens for FalconMatchAttributes. On iOS 17.2+
    /// Apple assigns one token per activity type; rotated periodically or on
    /// reinstall. Each new token gets persisted and re-sent to signal so it
    /// can target push-to-start. On older iOS the async sequence doesn't
    /// exist, so this method is a no-op there.
    private func startLiveActivityTokenObserver() {
        guard #available(iOS 17.2, *) else { return }
        liveActivityTokenObserver = Task { [weak self] in
            for await data in Activity<FalconMatchAttributes>.pushToStartTokenUpdates {
                let token = data.map { String(format: "%02x", $0) }.joined()
                await MainActor.run {
                    self?.liveActivityPushToStartToken = token
                }
                print("[live-activity] pushToStart token: \(token.prefix(16))…")
                let userID = SessionManager.shared.userID
                if !userID.isEmpty {
                    self?.registerWithSignal(userID: userID)
                }
            }
        }
    }

    /// Observes each activity's lifecycle. When iOS starts an activity (via
    /// push-to-start or manual request), we grab its per-activity update
    /// token so signal can send event="update" pushes instead of spawning a
    /// new activity on every match. When the activity ends, we clear the
    /// token on backend so the next push falls back to start.
    private func startLiveActivityUpdatesObserver() {
        print("[live-activity] startLiveActivityUpdatesObserver() called")
        guard #available(iOS 16.2, *) else {
            print("[live-activity] iOS < 16.2 — observer skipped")
            return
        }
        print("[live-activity] scheduling activityUpdates observer task")
        liveActivityUpdatesObserver = Task { [weak self] in
            print("[live-activity] activityUpdates observer task RUNNING")
            // Log any activities that are ALREADY running when this observer
            // starts (e.g. app launched while a push-to-start activity is on
            // Lock Screen). activityUpdates only yields NEW arrivals.
            let existing = Activity<FalconMatchAttributes>.activities
            print("[live-activity] observer started — \(existing.count) already-running activity(ies)")

            // If there are no running activities, clear any stale update_token
            // that may still be in the backend (app was killed before we got
            // to observe the .ended/.dismissed state). Prevents signal from
            // trying to UPDATE an activity that no longer exists.
            if existing.isEmpty {
                await self?.sendLiveActivityUpdateToken("")
            } else {
                // For existing activities, subscribe to their token/state
                // streams so we can re-capture the update token (iOS may
                // replay the current one) and observe later lifecycle events.
                for a in existing {
                    print("[live-activity]  ↳ existing id=\(a.id) state=\(a.activityState) project=\(a.content.state.projectID) score=\(a.content.state.score) title=\(a.content.state.projectTitle)")
                    self?.observeActivity(a)
                }
            }

            for await activity in Activity<FalconMatchAttributes>.activityUpdates {
                print("[live-activity] ▶ NEW activity arrived id=\(activity.id) state=\(activity.activityState) project=\(activity.content.state.projectID) score=\(activity.content.state.score) title=\(activity.content.state.projectTitle)")
                self?.observeActivity(activity)
            }
        }
    }

    /// Subscribes to the three async streams of a Live Activity: per-activity
    /// update token, lifecycle state, and content updates. Used for both
    /// newly-arrived activities (via activityUpdates) and ones found running
    /// at observer start time.
    @available(iOS 16.2, *)
    private func observeActivity(_ activity: Activity<FalconMatchAttributes>) {
        Task { [weak self] in
            for await data in activity.pushTokenUpdates {
                let token = data.map { String(format: "%02x", $0) }.joined()
                print("[live-activity] update token for activity \(activity.id): \(token.prefix(16))… (len=\(token.count))")
                await self?.sendLiveActivityUpdateToken(token)
            }
        }
        Task { [weak self] in
            for await state in activity.activityStateUpdates {
                print("[live-activity] activity \(activity.id) → state=\(state)")
                switch state {
                case .ended, .dismissed:
                    await self?.sendLiveActivityUpdateToken("")
                default:
                    break
                }
            }
        }
        Task {
            for await content in activity.contentUpdates {
                print("[live-activity] activity \(activity.id) content updated — score=\(content.state.score) project=\(content.state.projectID) title=\(content.state.projectTitle)")
            }
        }
    }

    /// POSTs the per-activity update token (or empty string to clear) to
    /// /live-activity-update-token. Signal persists it per-device so the next
    /// match push can UPDATE the running activity instead of starting a new one.
    private func sendLiveActivityUpdateToken(_ token: String) async {
        guard let url = URL(string: "\(apiURL)/live-activity-update-token") else { return }
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try? JSONSerialization.data(withJSONObject: [
            "device_id": KeychainHelper.deviceID,
            "token":     token
        ])
        _ = try? await URLSession.shared.data(for: request)
    }

#if DEBUG
    func devSetAPIURL(_ url: String) {
        apiURL = url
        UserDefaults.standard.set(url, forKey: "api_url")
    }
#endif

    // MARK: - Permission

    func requestPermission() {
        if authStatus == .denied {
            // User previously denied — the OS won't show the prompt again.
            // Open the app's notification settings directly.
            if let url = URL(string: UIApplication.openSettingsURLString) {
                UIApplication.shared.open(url)
            }
            return
        }

        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge]) { granted, _ in
            Task { @MainActor in
                await self.refreshStatus()
                if granted {
                    UIApplication.shared.registerForRemoteNotifications()
                }
            }
        }
    }

    func refreshStatus() async {
        let settings = await UNUserNotificationCenter.current().notificationSettings()
        authStatus = settings.authorizationStatus

        let info = ActivityAuthorizationInfo()
        liveActivitiesEnabled = info.areActivitiesEnabled
        if #available(iOS 17.2, *) {
            frequentPushesEnabled = info.frequentPushesEnabled
        }
    }

    /// Opens the app's Settings page so the user can toggle Live Activities.
    func openAppSettings() {
        if let url = URL(string: UIApplication.openSettingsURLString) {
            UIApplication.shared.open(url)
        }
    }

    // MARK: - Token

    func onTokenReceived(_ token: String) {
        deviceToken = token
        registrationError = nil
        let userID = SessionManager.shared.userID
        if !userID.isEmpty {
            registerWithSignal(userID: userID)
        }
    }

    func onRegistrationFailed(_ error: Error) {
        registrationError = error.localizedDescription
    }

    // MARK: - Register with falcon-api

    func registerWithSignal(userID: String) {
        guard let token = deviceToken else { return }
        guard let url = URL(string: "\(apiURL)/device-token") else { return }

        signalStatus = .registering

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let deviceID = KeychainHelper.deviceID
        var body: [String: Any] = [
            "user_id": userID,
            "device_id": deviceID,
            "token": token
        ]
        // iOS 17.2+ only. On older devices the token is nil and the field is
        // omitted → signal persists an empty string and falls back to the
        // regular APNs push (no Live Activity).
        if let liveToken = liveActivityPushToStartToken {
            body["live_activity_token"] = liveToken
            print("[live-activity] registerWithSignal: including live_activity_token=\(liveToken.prefix(16))…")
        } else {
            print("[live-activity] registerWithSignal: no live_activity_token yet (iOS < 17.2 or not assigned yet)")
        }
        request.httpBody = try? JSONSerialization.data(withJSONObject: body)

        URLSession.shared.dataTask(with: request) { _, response, error in
            Task { @MainActor in
                if let error {
                    self.signalStatus = .failed(error.localizedDescription)
                    return
                }
                let code = (response as? HTTPURLResponse)?.statusCode ?? 0
                self.signalStatus = code == 200 ? .registered : .failed("HTTP \(code)")
            }
        }.resume()
    }

    // MARK: - Foreground notifications

    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        let content = notification.request.content
        print("[push] willPresent fired — category=\(content.categoryIdentifier) title=\(content.title)")
        print("[push] userInfo keys: \(Array(content.userInfo.keys))")
        Task { @MainActor in
            self.lastNotification = ReceivedNotification(
                title: content.title,
                body: content.body,
                receivedAt: Date()
            )
            // Mirror a match push as a Live Activity — the user can keep an
            // eye on the latest score on the Lock Screen / Dynamic Island
            // without leaving whatever they were doing in the app.
            if content.categoryIdentifier == "MATCH_RESULT" {
                self.startOrUpdateMatchActivity(content: content)
            } else {
                print("[push] category is \"\(content.categoryIdentifier)\" — not starting activity (expected \"MATCH_RESULT\")")
            }
        }
        completionHandler([.banner, .sound, .badge])
    }

    /// Handles notification taps (app launched or backgrounded). For MATCH_RESULT
    /// pushes we set `pendingMatchNavigation` so the UI can navigate to the
    /// matches tab and open the tapped match's detail sheet.
    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        defer { completionHandler() }

        let content = response.notification.request.content
        guard content.categoryIdentifier == "MATCH_RESULT" else { return }

        let info = content.userInfo
        guard let projectID = info["project_id"] as? String,
              let cvID      = info["cv_id"]      as? String else { return }

        Task { @MainActor in
            self.pendingMatchNavigation = MatchNotificationPayload(
                projectID: projectID, cvID: cvID
            )
        }
    }

    // MARK: - Deep link handling

    /// Routes a `falcon://match?project_id=...&cv_id=...` URL from either a
    /// Live Activity tap or a push tap into the existing pendingMatchNavigation
    /// signal. MainTabView + MatchesView already react to it.
    func handleMatchDeepLink(_ url: URL) {
        guard url.scheme == "falcon", url.host == "match",
              let components = URLComponents(url: url, resolvingAgainstBaseURL: false) else {
            return
        }
        let projectID = components.queryItems?.first { $0.name == "project_id" }?.value ?? ""
        let cvID = components.queryItems?.first { $0.name == "cv_id" }?.value ?? ""
        guard !projectID.isEmpty, !cvID.isEmpty else { return }

        print("[deep-link] match project_id=\(projectID) cv_id=\(cvID)")
        Task { @MainActor in
            self.pendingMatchNavigation = MatchNotificationPayload(
                projectID: projectID, cvID: cvID
            )
        }
    }

    // MARK: - Match Live Activity

    #if DEBUG
    /// Debug-only: start a Live Activity with fake data. Used to isolate the
    /// widget render path from the push delivery path.
    @MainActor
    func debugStartFakeMatchActivity() {
        let authInfo = ActivityAuthorizationInfo()
        print("[live-activity][debug] areActivitiesEnabled=\(authInfo.areActivitiesEnabled)")
        if #available(iOS 17.2, *) {
            if let t = liveActivityPushToStartToken {
                print("[live-activity][debug] pushToStartToken=\(t.prefix(16))… (len=\(t.count))")
            } else {
                print("[live-activity][debug] pushToStartToken=nil — iOS hasn't assigned one yet")
            }
        } else {
            print("[live-activity][debug] iOS < 17.2 — push-to-start not supported on this device")
        }
        guard authInfo.areActivitiesEnabled else { return }

        let state = FalconMatchAttributes.ContentState(
            score: 7.8,
            label: "top_candidate",
            lang: LanguageManager.shared.appLanguage.rawValue,
            projectTitle: "Senior React Dev — Frankfurt",
            companyName: "ACME GmbH",
            totalMatches: 12,
            summary: "Score 7.8 · React/TypeScript stark, fehlendes AWS und Docker.",
            projectID: "debug-project-id",
            cvID: "debug-cv-id",
            skillsMatch: 8.5,
            seniorityFit: 7.2,
            domainExperience: 6.0,
            communicationClarity: 9.0,
            projectRelevance: 7.5,
            techStackOverlap: 8.0
        )
        let staleDate = Date().addingTimeInterval(30 * 60)
        let content = ActivityContent(state: state, staleDate: staleDate)

        if let activity = matchActivity {
            Task { await activity.update(content) }
        } else {
            do {
                let activity = try Activity.request(
                    attributes: FalconMatchAttributes(),
                    content: content,
                    pushType: nil
                )
                matchActivity = activity
                print("[live-activity][debug] started id=\(activity.id)")
            } catch {
                print("[live-activity][debug] failed: \(error)")
            }
        }
    }
    #endif

    /// Starts (or updates) a Live Activity for a foreground MATCH_RESULT push.
    /// Dismisses automatically 30 minutes after arrival via staleDate so the
    /// user's lock screen doesn't accumulate stale activities.
    @MainActor
    private func startOrUpdateMatchActivity(content: UNNotificationContent) {
        let authInfo = ActivityAuthorizationInfo()
        guard authInfo.areActivitiesEnabled else {
            print("[live-activity] areActivitiesEnabled=false — check Settings → Falcon → Live Activities and Info.plist NSSupportsLiveActivities=YES")
            return
        }

        let info = content.userInfo
        let score = (info["score"] as? NSNumber)?.doubleValue ?? 0
        let label = (info["label"] as? String) ?? ""
        let projectID = (info["project_id"] as? String) ?? ""
        let cvID = (info["cv_id"] as? String) ?? ""
        let companyName = (info["company_name"] as? String) ?? ""
        let totalMatches = (info["total_matches"] as? NSNumber)?.intValue ?? 0
        let projectTitle = content.title
        let summary = content.body

        // Six dimension scores come nested under "scores" (set by signal's apns.go).
        let scores = (info["scores"] as? [String: Any]) ?? [:]
        func s(_ key: String) -> Double { (scores[key] as? NSNumber)?.doubleValue ?? 0 }

        print("[live-activity] starting/updating with score=\(score) label=\(label) title=\(projectTitle)")

        let state = FalconMatchAttributes.ContentState(
            score:        score,
            label:        label,
            lang:         LanguageManager.shared.appLanguage.rawValue,
            projectTitle: projectTitle,
            companyName:  companyName,
            totalMatches: totalMatches,
            summary:      summary,
            projectID:    projectID,
            cvID:         cvID,
            skillsMatch:          s("skills_match"),
            seniorityFit:         s("seniority_fit"),
            domainExperience:     s("domain_experience"),
            communicationClarity: s("communication_clarity"),
            projectRelevance:     s("project_relevance"),
            techStackOverlap:     s("tech_stack_overlap")
        )
        // 5min staleDate: Apple throttles push-to-start starts aggressively
        // (per-hour budget + concurrency limits). Short stales free up slots
        // faster and reduce "silent drop" occurrences during testing.
        let staleDate = Date().addingTimeInterval(5 * 60)
        let activityContent = ActivityContent(state: state, staleDate: staleDate)

        if let activity = matchActivity {
            Task {
                await activity.update(activityContent)
                print("[live-activity] updated existing activity id=\(activity.id)")
            }
        } else {
            do {
                let activity = try Activity.request(
                    attributes: FalconMatchAttributes(),
                    content: activityContent,
                    pushType: nil
                )
                matchActivity = activity
                print("[live-activity] requested new activity id=\(activity.id) state=\(activity.activityState)")
            } catch {
                print("[live-activity] Activity.request failed: \(error)")
            }
        }
    }
}
