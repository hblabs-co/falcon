import SwiftUI

enum AppTab {
    case jobs, alerts, stats, profile, settings
}

struct MainTabView: View {
    @Environment(LanguageManager.self) var lm
    @Environment(NotificationManager.self) var nm
    @State private var selectedTab: AppTab = .jobs
    @State private var contactDrawer: ContactDrawerInfo? = nil

    var body: some View {
        ZStack(alignment: .bottom) {
            Group {
                switch selectedTab {
                case .jobs:        JobsView()
                case .alerts:      NotificationsView()
                case .stats:       StatsView()
                case .profile:     ProfileView()
                case .settings:    SettingsView(contactDrawer: $contactDrawer)
                }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)

            floatingTabBar
                .padding(.bottom, 28)

            if contactDrawer != nil {
                ContactDrawer(info: contactDrawer!, lm: lm) {
                    contactDrawer = nil
                }
                .zIndex(100)
            }
        }
        .ignoresSafeArea(edges: .bottom)
        .task { await nm.refreshStatus() }
    }

    private var floatingTabBar: some View {
        HStack(spacing: 0) {
            tabItem(icon: "bell.fill",           label: lm.t(.tabAlerts),    tab: .alerts)
            tabItem(icon: "chart.bar.fill",      label: lm.t(.tabStats),     tab: .stats)
            tabItem(icon: "briefcase.fill",      label: lm.t(.tabJobs),      tab: .jobs)
            tabItem(icon: "person.fill",         label: lm.t(.tabProfile),   tab: .profile)
            tabItem(icon: "gearshape.fill",      label: lm.t(.tabSettings),  tab: .settings)
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 12)
        .background(
            Capsule()
                .fill(.ultraThinMaterial)
                .shadow(color: .black.opacity(0.12), radius: 24, x: 0, y: 8)
        )
        .padding(.horizontal, 20)
    }

    private func tabItem(icon: String, label: String, tab: AppTab) -> some View {
        let isActive = selectedTab == tab
        return Button {
            withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) {
                selectedTab = tab
            }
        } label: {
            VStack(spacing: 3) {
                Image(systemName: icon)
                    .font(.system(size: 18, weight: isActive ? .semibold : .regular))
                Text(label)
                    .font(.system(size: 9, weight: .medium))
            }
            .foregroundStyle(isActive ? .primary : .tertiary)
            .frame(maxWidth: .infinity)
            .scaleEffect(isActive ? 1.08 : 1.0)
            .animation(.spring(response: 0.3, dampingFraction: 0.7), value: isActive)
        }
        .buttonStyle(.plain)
    }
}
