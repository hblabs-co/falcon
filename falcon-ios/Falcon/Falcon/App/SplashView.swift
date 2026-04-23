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

                Text("Project Intelligence")
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(.secondary)
                    .opacity(subtitleOpacity)

                // "v1.0.5" style: marketing version (CFBundleShortVersionString)
                // + build number (CFBundleVersion) joined by a dot. Lets us
                // bump the build every TestFlight iteration and have the
                // splash reflect it without editing MARKETING_VERSION each
                // time. Reads "1.0.1" to a user instead of "1.0 (1)".
                Text("v\(appVersionLabel)")
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

    /// "1.0.1" — marketing version + build joined by a dot. Empty
    /// fallbacks ("—") surface the problem instead of silently showing
    /// "1.0" when the Info.plist keys are missing.
    private var appVersionLabel: String {
        let short = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "—"
        let build = Bundle.main.infoDictionary?["CFBundleVersion"] as? String ?? "—"
        return "\(short).\(build)"
    }
}
