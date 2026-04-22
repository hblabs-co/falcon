import UIKit
import OSLog

/// Logger visible in Console.app AND Xcode even when the scheme is
/// "Wait for the executable to be launched" — print()/NSLog() go silent
/// in that mode. All interpolations must be `privacy: .public` or the
/// value redacts to "<private>".
private let log = FalconLog.make(category: "launch")

class AppDelegate: NSObject, UIApplicationDelegate {

    func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]? = nil
    ) -> Bool {
        UNUserNotificationCenter.current().delegate = NotificationManager.shared
        application.registerForRemoteNotifications()
        registerSettingsBundle()

        // Cold-launch entry detection. For warm resumes, didReceive fires
        // via the UNUserNotificationCenter delegate on a separate pathway
        // (not observable from here). For cold launches, the OS stashes
        // the launch trigger in launchOptions and this runs BEFORE any
        // SwiftUI scenePhase transition — so by the time scheduleAppOpened
        // ticks, the source flags are already set.
        let keys = (launchOptions?.keys.map { $0.rawValue } ?? [])
        log.error("launchOptions keys: \(keys, privacy: .public)")
        if launchOptions?[.remoteNotification] != nil {
            log.error("cold-launched via push notification")
            RealtimeClient.shared.noteAppOpenSource("push_notification")
            RealtimeClient.shared.noteSessionSource("push_notification")
        } else if let url = launchOptions?[.url] as? URL {
            log.error("cold-launched via URL: \(url, privacy: .public)")
            switch url.host {
            case "auth":
                RealtimeClient.shared.noteAppOpenSource("magic_link")
                RealtimeClient.shared.noteSessionSource("magic_link")
            case "match":
                RealtimeClient.shared.noteAppOpenSource("live_activity")
                RealtimeClient.shared.noteSessionSource("live_activity")
            default:
                break
            }
        } else {
            log.error("cold-launched with no trigger (plain foreground)")
        }
        return true
    }

    private func registerSettingsBundle() {
        let version = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "?"
        let build = Bundle.main.infoDictionary?["CFBundleVersion"] as? String ?? "?"
        UserDefaults.standard.set(version, forKey: "version_preference")
        UserDefaults.standard.set(build, forKey: "build_preference")
    }

    func application(_ application: UIApplication, didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
        let token = deviceToken.map { String(format: "%02.2hhx", $0) }.joined()
        NotificationManager.shared.onTokenReceived(token)
    }

    func application(_ application: UIApplication, didFailToRegisterForRemoteNotificationsWithError error: Error) {
        NotificationManager.shared.onRegistrationFailed(error)
    }
}
