import SwiftUI

struct RootView: View {
    @Environment(LanguageManager.self) var lm
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
    }
}
