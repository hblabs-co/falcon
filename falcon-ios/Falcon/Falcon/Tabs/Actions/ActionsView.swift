import SwiftUI

/// Action center for items the user should address — missing account,
/// disabled notifications, disabled Live Activities. Other tabs no longer
/// surface the red warnings; this view is the single home for them, plus
/// an empty state when everything is in order. The tab's icon badge (see
/// MainTabView.actionsPendingCount) mirrors the number of banners this
/// view would render, so users know at a glance whether anything needs
/// attention.
struct ActionsView: View {
    @Environment(LanguageManager.self) var lm
    @Environment(NotificationManager.self) var nm
    @Environment(SessionManager.self) var session

    private var hasAnyAction: Bool {
        !session.isAuthenticated
            || nm.authStatus != .authorized
            || nm.liveActivitiesRestricted
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 16) {
                    if hasAnyAction {
                        if !session.isAuthenticated {
                            AlreadyHaveAccountBanner()
                        }
                        if nm.authStatus != .authorized {
                            NotificationsDisabledBanner()
                        }
                        if nm.liveActivitiesRestricted {
                            LiveActivitiesDisabledBanner()
                        }
                    } else {
                        allSetCard
                    }
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
                .padding(.bottom, 110)
            }
            .background(Color(UIColor.systemGroupedBackground))
            .navigationTitle(lm.t(.tabActions))
            .navigationBarTitleDisplayMode(.inline)
            .withLoginToolbar()
        }
    }

    /// Friendly empty state shown when there are no pending actions.
    /// Matches the visual language of the other banners but with a
    /// green tint to signal "all clear".
    private var allSetCard: some View {
        HStack(spacing: 12) {
            Image(systemName: "checkmark.seal.fill")
                .font(.system(size: 14, weight: .semibold))
                .foregroundStyle(Color.green)
                .frame(width: 30, height: 30)
                .background(Circle().fill(Color.green.opacity(0.15)))

            VStack(alignment: .leading, spacing: 2) {
                Text(lm.t(.actionsEmptyTitle))
                    .font(.system(size: 13, weight: .bold))
                    .foregroundStyle(Color.green)
                Text(lm.t(.actionsEmptyBody))
                    .font(.system(size: 10, weight: .medium))
                    .foregroundStyle(Color.green.opacity(0.8))
                    .fixedSize(horizontal: false, vertical: true)
            }

            Spacer(minLength: 0)
        }
        .padding(12)
        .background(
            RoundedRectangle(cornerRadius: 14, style: .continuous)
                .fill(Color.green.opacity(0.1))
        )
        .padding(.top, 40)
    }
}
