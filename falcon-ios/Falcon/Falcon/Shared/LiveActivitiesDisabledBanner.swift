import SwiftUI

/// Compact warning banner shown at the top of the matches feed when BOTH
/// system-level Live Activities toggles are disabled (master + frequent
/// updates). Tap opens the app's Settings page so the user can re-enable.
struct LiveActivitiesDisabledBanner: View {
    @Environment(LanguageManager.self) var lm
    @Environment(NotificationManager.self) var nm

    var body: some View {
        Button {
            nm.openAppSettings()
        } label: {
            HStack(spacing: 12) {
                Image(systemName: "rectangle.on.rectangle.slash.fill")
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(Color.pastelRed)
                    .frame(width: 30, height: 30)
                    .background(Circle().fill(Color.pastelRed.opacity(0.15)))

                VStack(alignment: .leading, spacing: 2) {
                    Text(lm.t(.liveActivitiesDisabledTitle))
                        .font(.system(size: 13, weight: .bold))
                        .foregroundStyle(Color.pastelRed)
                    Text(lm.t(.liveActivitiesDisabledBody))
                        .font(.system(size: 10, weight: .medium))
                        .foregroundStyle(Color.pastelRed.opacity(0.8))
                        .lineLimit(3)
                        .fixedSize(horizontal: false, vertical: true)
                }

                Spacer(minLength: 0)

                Image(systemName: "chevron.right")
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(.tertiary)
            }
            .padding(12)
            .background(
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .fill(Color.pastelRed.opacity(0.12))
            )
        }
        .buttonStyle(.plain)
    }
}

extension Color {
    /// Warm pastel red/coral used by warning banners (notifications off,
    /// live activities off). Less harsh than `.red`, more readable than
    /// `.pink`. Keep this a single shared definition so all banners match.
    static let pastelRed = Color(red: 0.90, green: 0.40, blue: 0.42)
}
