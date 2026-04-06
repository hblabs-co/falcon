import ActivityKit
import WidgetKit
import SwiftUI

// MARK: - Reusable logo view (widget-safe, no UIKit dependency)

private struct AppLogo: View {
    var size: CGFloat

    var body: some View {
        if let _ = UIImage(named: "FalconLogo") {
            Image("FalconLogo")
                .resizable()
                .scaledToFit()
                .frame(width: size, height: size)
                .clipShape(RoundedRectangle(cornerRadius: size * 0.225, style: .continuous))
        } else {
            RoundedRectangle(cornerRadius: size * 0.225, style: .continuous)
                .fill(LinearGradient(
                    colors: [Color.accentColor, Color.accentColor.opacity(0.7)],
                    startPoint: .topLeading, endPoint: .bottomTrailing
                ))
                .frame(width: size, height: size)
                .overlay {
                    Image(systemName: "bird.fill")
                        .font(.system(size: size * 0.42, weight: .semibold))
                        .foregroundStyle(.white)
                }
        }
    }
}

// MARK: - Count badge — the separate circle next to the pill

private struct CountBadge: View {
    var count: Int
    var size: CGFloat = 22

    var body: some View {
        Text("\(count)")
            .font(.system(size: size * 0.55, weight: .bold, design: .rounded))
            .foregroundStyle(.white)
            .frame(minWidth: size, minHeight: size)
            .padding(.horizontal, size * 0.2)
            .background(Capsule().fill(Color.accentColor))
            .contentTransition(.numericText())
    }
}

// MARK: - Live Activity widget

struct FalconWidgetLiveActivity: Widget {
    var body: some WidgetConfiguration {
        ActivityConfiguration(for: FalconJobsAttributes.self) { context in
            lockScreenView(context)
        } dynamicIsland: { context in
            DynamicIsland {
                // Expanded — tapped state
                DynamicIslandExpandedRegion(.leading) {
                    HStack(spacing: 8) {
                        AppLogo(size: 28)
                        Text("Falcon")
                            .font(.system(size: 16, weight: .bold, design: .rounded))
                            .foregroundStyle(.white)
                    }
                    .padding(.leading, 4)
                }
                DynamicIslandExpandedRegion(.trailing) {
                    VStack(alignment: .trailing, spacing: 2) {
                        Text("\(context.state.projectCount)")
                            .font(.system(size: 22, weight: .bold, design: .rounded))
                            .foregroundStyle(.white)
                            .contentTransition(.numericText())
                        Text("jobs")
                            .font(.system(size: 11, weight: .medium))
                            .foregroundStyle(.white.opacity(0.7))
                    }
                    .padding(.trailing, 4)
                }
                DynamicIslandExpandedRegion(.bottom) {
                    if !context.state.latestTitle.isEmpty {
                        HStack(spacing: 6) {
                            Image(systemName: "sparkles")
                                .font(.system(size: 11))
                                .foregroundStyle(.white.opacity(0.6))
                            Text(context.state.latestTitle)
                                .font(.system(size: 13, weight: .medium))
                                .foregroundStyle(.white.opacity(0.9))
                                .lineLimit(1)
                        }
                        .padding(.horizontal, 4)
                        .padding(.bottom, 2)
                    }
                }
            } compactLeading: {
                CountBadge(count: context.state.projectCount, size: 22)
                    .padding(.leading, 2)
            } compactTrailing: {
                AppLogo(size: 20)
                    .padding(.trailing, 2)
            } minimal: {
                // Detached circle on the right when two Live Activities compete
                AppLogo(size: 20)
            }
            .keylineTint(Color.accentColor)
        }
    }

    @ViewBuilder
    private func lockScreenView(_ context: ActivityViewContext<FalconJobsAttributes>) -> some View {
        HStack(spacing: 14) {
            AppLogo(size: 40)

            VStack(alignment: .leading, spacing: 3) {
                Text("Falcon Jobs")
                    .font(.system(size: 15, weight: .bold, design: .rounded))
                if !context.state.latestTitle.isEmpty {
                    Text(context.state.latestTitle)
                        .font(.system(size: 12))
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }

            Spacer()

            VStack(alignment: .trailing, spacing: 2) {
                Text("\(context.state.projectCount)")
                    .font(.system(size: 24, weight: .bold, design: .rounded))
                    .contentTransition(.numericText())
                Text("projects")
                    .font(.system(size: 10))
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 14)
        .activityBackgroundTint(Color(.systemBackground))
        .activitySystemActionForegroundColor(Color.accentColor)
    }
}

// MARK: - Preview

#Preview("Compact", as: .dynamicIsland(.compact), using: FalconJobsAttributes()) {
    FalconWidgetLiveActivity()
} contentStates: {
    FalconJobsAttributes.ContentState(projectCount: 42, latestTitle: "Fullstack Java/Kotlin — Frankfurt")
    FalconJobsAttributes.ContentState(projectCount: 87, latestTitle: "Senior Cloud Engineer AWS")
}

#Preview("Minimal", as: .dynamicIsland(.minimal), using: FalconJobsAttributes()) {
    FalconWidgetLiveActivity()
} contentStates: {
    FalconJobsAttributes.ContentState(projectCount: 42, latestTitle: "")
}

#Preview("Expanded", as: .dynamicIsland(.expanded), using: FalconJobsAttributes()) {
    FalconWidgetLiveActivity()
} contentStates: {
    FalconJobsAttributes.ContentState(projectCount: 42, latestTitle: "Fullstack Java/Kotlin — Frankfurt")
}
