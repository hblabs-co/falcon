import SwiftUI

@main
struct FalconApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) var appDelegate
    @State private var authHandler = AuthLinkHandler()
    @State private var showAuthError = false

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(NotificationManager.shared)
                .environment(LanguageManager.shared)
                .environment(SessionManager.shared)
                .onOpenURL { url in
                    // Route deep links by host:
                    //   falcon://auth?token=...         → magic-link auth
                    //   falcon://match?project_id&cv_id → Live Activity tap → open match detail
                    switch url.host {
                    case "match":
                        NotificationManager.shared.handleMatchDeepLink(url)
                    default:
                        authHandler.handle(url)
                    }
                }
                .onChange(of: authHandler.errorKey) { _, key in
                    if key != nil { showAuthError = true }
                }
                .alert(
                    LanguageManager.shared.t(.authErrorTitle),
                    isPresented: $showAuthError
                ) {
                    Button("OK", role: .cancel) {
                        authHandler.errorKey = nil
                    }
                } message: {
                    Text(LanguageManager.shared.t(authHandler.errorKey ?? .authErrorGeneric))
                }
        }
    }
}
