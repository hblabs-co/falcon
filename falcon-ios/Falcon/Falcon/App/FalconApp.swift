import SwiftUI

@main
struct FalconApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) var appDelegate
    @Environment(\.scenePhase) private var scenePhase
    @State private var authHandler = AuthLinkHandler()
    @State private var showAuthError = false

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(NotificationManager.shared)
                .environment(LanguageManager.shared)
                .environment(SessionManager.shared)
                .environment(RealtimeClient.shared)
                .onChange(of: scenePhase) { _, phase in
                    // App lifecycle → realtime:
                    //  .active     = foreground and receiving input (first
                    //                launch + every resume). Emit order is
                    //                semantic: the user opened the app
                    //                FIRST, then the server session begins.
                    //  .background = off-screen (home, app switcher, locked):
                    //                iOS suspends networking in seconds, so
                    //                we emit app_backgrounded and close
                    //                cleanly before it happens. If the app
                    //                is KILLED (swipe up, crash) this never
                    //                runs and the server reaps the socket
                    //                via its 90s read deadline.
                    switch phase {
                    case .active:
                        RealtimeClient.shared.connect()
                        // Defer the emit by a microtask so .onOpenURL (which
                        // fires during the same SwiftUI update) has a chance
                        // to call noteAppOpenSource("magic_link") first.
                        RealtimeClient.shared.scheduleAppOpened(defaultSource: "foreground")
                        RealtimeClient.shared.emitSessionStartedOnce()
                    case .background:
                        Task { await RealtimeClient.shared.emitAppBackgroundedAndClose() }
                    default:
                        break
                    }
                }
                .onOpenURL { url in
                    // Route deep links by host:
                    //   falcon://auth?token=...         → magic-link auth
                    //   falcon://match?project_id&cv_id → Live Activity tap → open match detail
                    //
                    // Tag the pending app_opened with the source BEFORE the
                    // scheduleAppOpened microtask runs. This fires during
                    // the same view-update pass as scenePhase .active, so
                    // the flag is set in time.
                    switch url.host {
                    case "match":
                        RealtimeClient.shared.noteAppOpenSource("live_activity")
                        RealtimeClient.shared.noteSessionSource("live_activity")
                        NotificationManager.shared.handleMatchDeepLink(url)
                    case "auth":
                        RealtimeClient.shared.noteAppOpenSource("magic_link")
                        RealtimeClient.shared.noteSessionSource("magic_link")
                        authHandler.handle(url)
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
