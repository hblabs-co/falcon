import SwiftUI

struct SplashView: View {
    var onComplete: () -> Void

    @State private var scale: CGFloat = 0.75
    @State private var iconOpacity: Double = 0
    @State private var titleOpacity: Double = 0
    @State private var subtitleOpacity: Double = 0

    var body: some View {
        ZStack {
            Color(.systemBackground).ignoresSafeArea()

            VStack(spacing: 16) {
                FalconIconView(size: 96)
                    .scaleEffect(scale)
                    .opacity(iconOpacity)

                Text("Falcon")
                    .font(.system(size: 34, weight: .bold, design: .rounded))
                    .opacity(titleOpacity)

                Text("Job Intelligence")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(.secondary)
                    .opacity(subtitleOpacity)

                Text("v\(Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0")")
                    .font(.system(size: 11, weight: .regular, design: .monospaced))
                    .foregroundStyle(.tertiary)
                    .opacity(subtitleOpacity)
            }
        }
        .onAppear {
            withAnimation(.spring(response: 0.5, dampingFraction: 0.72)) {
                scale = 1.0
                iconOpacity = 1.0
            }
            withAnimation(.easeOut(duration: 0.3).delay(0.3)) {
                titleOpacity = 1.0
            }
            withAnimation(.easeOut(duration: 0.3).delay(0.45)) {
                subtitleOpacity = 1.0
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
                onComplete()
            }
        }
    }
}
