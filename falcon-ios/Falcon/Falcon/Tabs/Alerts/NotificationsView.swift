import SwiftUI

struct NotificationsView: View {
    @Environment(NotificationManager.self) var nm
    @Environment(LanguageManager.self) var lm
    @Environment(SessionManager.self) var session
    @Environment(CVUploadViewModel.self) var cvVM
    @Binding var selectedTab: AppTab

    var body: some View {
        NavigationStack {
            Group {
                if let n = nm.lastNotification {
                    List {
                        heroBanner
                            .listRowInsets(EdgeInsets(top: 12, leading: 16, bottom: 4, trailing: 16))
                            .listRowBackground(Color.clear)
                            .listRowSeparator(.hidden)
                        notificationCard(n)
                    }
                    .listStyle(.insetGrouped)
                } else {
                    VStack(spacing: 0) {
                        heroBanner
                            .padding(.horizontal, 16)
                            .padding(.top, 8)
                        Spacer()
                        if isCVFailed {
                            cvFailedView
                        } else if nm.authStatus != .authorized {
                            notificationsDisabledView
                        } else if session.userID.isEmpty {
                            VStack(spacing: 16) {
                                ContentUnavailableView(
                                    lm.t(.noCVWarningTitle),
                                    systemImage: "exclamationmark.triangle.fill",
                                    description: Text(lm.t(.noCVWarningBody))
                                )
                                Button(lm.t(.profileCVUpload)) {
                                    withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) {
                                        selectedTab = .profile
                                    }
                                }
                                .buttonStyle(.borderedProminent)
                            }
                        } else {
                            ContentUnavailableView(
                                lm.t(.alertsEmpty),
                                systemImage: "bell.slash",
                                description: Text(lm.t(.alertsEmptyDescription))
                            )
                        }
                        Spacer()
                    }
                }
            }
            .navigationTitle(lm.t(.tabAlerts))
            .withLoginToolbar()
            .safeAreaInset(edge: .bottom) { Color.clear.frame(height: 90) }
        }
    }

    // MARK: - CV failed

    private var isCVFailed: Bool {
        if case .failed = cvVM.state { return true }
        return false
    }

    private var cvFailedView: some View {
        VStack(spacing: 16) {
            ContentUnavailableView(
                lm.t(.cvFailedAlertTitle),
                systemImage: "exclamationmark.triangle.fill",
                description: Text(lm.t(.cvFailedAlertBody))
            )
            Button(lm.t(.cvFailedAlertButton)) {
                withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) {
                    selectedTab = .profile
                }
            }
            .buttonStyle(.borderedProminent)
            .tint(.red)
        }
    }

    // MARK: - Notifications disabled state

    private var notificationsDisabledView: some View {
        VStack(spacing: 16) {
            ContentUnavailableView(
                lm.t(.noNotifPermissionTitle),
                systemImage: "bell.slash.fill",
                description: Text(lm.t(.noNotifPermissionBody))
            )
            Button(lm.t(.noNotifPermissionButton)) {
                nm.requestPermission()
            }
            .buttonStyle(.borderedProminent)
        }
    }

    // MARK: - Hero banner

    private var heroBanner: some View {
        HStack(spacing: 14) {
            FalconIconView(size: 48, cornerRadius: 11)

            VStack(alignment: .leading, spacing: 3) {
                Text("Falcon")
                    .font(.system(size: 18, weight: .bold, design: .rounded))
                Text(lm.t(.alertsBannerTagline))
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(.secondary)
            }

            Spacer()

            VStack(alignment: .trailing, spacing: 2) {
                Image(systemName: nm.authStatus == .authorized ? "bell.fill" : "bell.slash.fill")
                    .font(.system(size: 20))
                    .foregroundStyle(nm.authStatus == .authorized ? .green : .secondary)
                Text(nm.authStatus == .authorized ? lm.t(.notifStatusActive) : lm.t(.notifStatusPending))
                    .font(.system(size: 10, weight: .medium))
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 14)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.07), radius: 14, x: 0, y: 4)
        )
    }

    // MARK: - Notification card

    private func notificationCard(_ n: NotificationManager.ReceivedNotification) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack {
                Text(n.title)
                    .fontWeight(.semibold)
                Spacer()
                Text(n.receivedAt.formatted(date: .omitted, time: .shortened))
                    .font(.caption2)
                    .foregroundStyle(.tertiary)
            }
            Text(n.body)
                .font(.subheadline)
                .foregroundStyle(.secondary)
        }
        .padding(.vertical, 4)
    }
}
