import SwiftUI

struct RootView: View {
    @State private var showSplash = true

    var body: some View {
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
}
