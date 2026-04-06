import SwiftUI

struct RootView: View {
    @Environment(LanguageManager.self) var lm
    @State private var showSplash = true

    var body: some View {
        Group {
            if showSplash {
                SplashView {
                    withAnimation(.easeOut(duration: 0.35)) {
                        showSplash = false
                    }
                }
                .transition(.opacity)
            } else {
                MainTabView()
                    .transition(.opacity)
            }
        }
        // Loads persisted language preference from the API once the app is ready.
        // Skipped silently when there is no userID (anonymous session).
        .task { await lm.loadFromAPI() }
    }
}
