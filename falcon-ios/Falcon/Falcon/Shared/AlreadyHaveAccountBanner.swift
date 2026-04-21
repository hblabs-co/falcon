import SwiftUI

/// Nudges unauthenticated users toward the login flow. The "Anmelden"
/// button in the nav bar is easy to miss, so we also surface a banner
/// inside the content area of every no-session screen (Matches, Jobs,
/// Profile). Tap opens the same LoginSheet the toolbar button does.
/// Rendered only when the user is NOT authenticated — call sites should
/// guard on `session.isAuthenticated`.
struct AlreadyHaveAccountBanner: View {
    @Environment(LanguageManager.self) var lm
    @State private var showLogin = false

    var body: some View {
        Button {
            showLogin = true
        } label: {
            HStack(spacing: 12) {
                Image(systemName: "person.badge.key.fill")
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(Color.accentColor)
                    .frame(width: 30, height: 30)
                    .background(Circle().fill(Color.accentColor.opacity(0.15)))

                VStack(alignment: .leading, spacing: 2) {
                    Text(lm.t(.alreadyHaveAccountTitle))
                        .font(.system(size: 13, weight: .bold))
                        .foregroundStyle(Color.accentColor)
                    Text(lm.t(.alreadyHaveAccountBody))
                        .font(.system(size: 10, weight: .medium))
                        .foregroundStyle(Color.accentColor.opacity(0.8))
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
                    .fill(Color.accentColor.opacity(0.1))
            )
        }
        .buttonStyle(.plain)
        .sheet(isPresented: $showLogin) {
            LoginSheet()
                .presentationDetents([.medium])
                .presentationCornerRadius(22)
                .presentationDragIndicator(.visible)
        }
    }
}
