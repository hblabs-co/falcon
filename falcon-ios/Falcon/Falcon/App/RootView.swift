import SwiftUI

struct RootView: View {
    @Environment(LanguageManager.self) var lm
    @Environment(SessionManager.self) var session
    @State private var splashDone = false
    @State private var sessionReady = false

    private var showMain: Bool { splashDone && sessionReady }

    var body: some View {
        Group {
            if showMain {
                MainTabView()
                    .transition(.opacity)
            } else {
                SplashView {
                    withAnimation(.easeOut(duration: 0.35)) {
                        splashDone = true
                    }
                }
                .transition(.opacity)
            }
        }
        .task {
            await SessionManager.shared.restore()
            sessionReady = true
        }
        // Realtime client is attached at launch with whatever user_id is
        // available (usually empty on first run). When SessionManager
        // settles on a user_id (login, restore), re-handshake so the
        // server can bucket the connection under the correct user.
        // session_started waits on this transition — we don't count
        // anonymous launches as sessions.
        .onChange(of: session.userID) { _, _ in
            RealtimeClient.shared.rebindIfUserChanged()
            RealtimeClient.shared.emitSessionStartedOnce()
        }
    }
}
