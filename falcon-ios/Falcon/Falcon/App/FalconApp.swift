import SwiftUI

@main
struct FalconApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(NotificationManager.shared)
                .environment(LanguageManager.shared)
                .environment(SessionManager.shared)
        }
    }
}
