import SwiftUI

/// Compact warning banner shown at the top of feeds (Jobs, Matches) when the
/// user has disabled notifications. Taps request permission (or open Settings
/// if already denied). Use the full-screen `ContentUnavailableView` flavor in
/// the empty-state branch instead.
struct NotificationsDisabledBanner: View {
    @Environment(LanguageManager.self) var lm
    @Environment(NotificationManager.self) var nm

    var body: some View {
        Button {
            nm.requestPermission()
        } label: {
            HStack(spacing: 12) {
                Image(systemName: "bell.slash.fill")
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(Color.pastelRed)
                    .frame(width: 30, height: 30)
                    .background(Circle().fill(Color.pastelRed.opacity(0.15)))

                VStack(alignment: .leading, spacing: 2) {
                    Text(lm.t(.noNotifPermissionTitle))
                        .font(.system(size: 13, weight: .bold))
                        .foregroundStyle(Color.pastelRed)
                    Text(lm.t(.noNotifPermissionBody))
                        .font(.system(size: 10, weight: .medium))
                        .foregroundStyle(Color.pastelRed.opacity(0.8))
                        .lineLimit(2)
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
