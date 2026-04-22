import SwiftUI

/// Floating "N new X — tap to refresh" pill that slides in from the top
/// whenever realtime pushes arrive while the user is in the tab. Kept
/// generic so both Jobs (`project.normalized` stream) and Matches
/// (`match.result` stream) can reuse it — each tab owns its own counter
/// and passes the localized singular/plural strings.
///
/// Rendered as an overlay above the scroll view so it floats over the
/// content without pushing anything down.
struct LiveNewItemsBanner: View {
    @Environment(LanguageManager.self) var lm
    var count: Int
    /// String keys used for the pluralization. Both must contain `%d`.
    var singularKey: StringKey
    var pluralKey: StringKey
    var onTap: () -> Void

    var body: some View {
        if count > 0 {
            Button(action: onTap) {
                HStack(spacing: 8) {
                    Image(systemName: "sparkles")
                        .font(.system(size: 12, weight: .bold))
                    VStack(alignment: .leading, spacing: 1) {
                        Text(headline)
                            .font(.system(size: 13, weight: .bold, design: .rounded))
                        Text(lm.t(.liveTapToRefresh))
                            .font(.system(size: 10, weight: .medium))
                            .foregroundStyle(.white.opacity(0.8))
                    }
                    Image(systemName: "arrow.clockwise")
                        .font(.system(size: 11, weight: .bold))
                }
                .foregroundStyle(.white)
                .padding(.horizontal, 14)
                .padding(.vertical, 9)
                .background(
                    Capsule()
                        .fill(Color.accentColor)
                        .shadow(color: .black.opacity(0.2), radius: 10, x: 0, y: 4)
                )
            }
            .buttonStyle(.plain)
            .contentTransition(.numericText())
            // Entrance from below with a spring: the pill nudges up into
            // its final position while fading in. Feels "alive" compared
            // to a flat slide from the top. asymmetric exit keeps the
            // dismiss quick and unobtrusive.
            .transition(.asymmetric(
                insertion: .move(edge: .bottom).combined(with: .opacity).animation(.spring(response: 0.45, dampingFraction: 0.7)),
                removal:   .opacity.animation(.easeOut(duration: 0.2))
            ))
            // Offset below the navigation bar. The overlay host sits at
            // the top of the NavigationStack so we add clearance for the
            // inline nav bar (~44pt) plus a little air. Using padding
            // rather than safeAreaInset to avoid pushing scroll content.
            .padding(.top, 54)
        }
    }

    private var headline: String {
        let template = lm.t(count == 1 ? singularKey : pluralKey)
        return String(format: template, count)
    }
}
