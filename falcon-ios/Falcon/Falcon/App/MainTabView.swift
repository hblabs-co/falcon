import SwiftUI

enum AppTab {
    case jobs, matches, stats, profile, settings
}

struct MainTabView: View {
    @Environment(LanguageManager.self) var lm
    @Environment(NotificationManager.self) var nm
    @Environment(SessionManager.self) var session
    @Environment(\.scenePhase) var scenePhase
    @State private var selectedTab: AppTab = .jobs
    @State private var contactDrawer: ContactDrawerInfo? = nil
    @State private var cvUploadVM = CVUploadViewModel()
    @State private var jobsScrollToTop = false
    @State private var matchesScrollToTop = false

    var body: some View {
        ZStack(alignment: .bottom) {
            ZStack {
                // Jobs is always kept alive so scroll position, pagination and
                // the loaded list survive switching to another tab.
                JobsView(scrollToTop: $jobsScrollToTop)
                    .opacity(selectedTab == .jobs ? 1 : 0)
                    .allowsHitTesting(selectedTab == .jobs)

                // Other tabs are created on demand and torn down when leaving.
                Group {
                    switch selectedTab {
                    case .matches:     MatchesView(selectedTab: $selectedTab, scrollToTop: $matchesScrollToTop)
                    case .profile:     ProfileView()
                    case .settings:    SettingsView(contactDrawer: $contactDrawer)
                    case .jobs, .stats: EmptyView()
                    }
                }
            }
            .environment(cvUploadVM)
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
        .onChange(of: scenePhase) { _, phase in
            if phase == .active {
                Task { await nm.refreshStatus() }
                // Retry CV restore if it failed (e.g. local network prompt not yet accepted).
                if session.isAuthenticated, case .idle = cvUploadVM.state {
                    Task { await cvUploadVM.restoreFromServer() }
                }
            }
        }
        .task(id: session.isAuthenticated) {
            if session.isAuthenticated {
                await cvUploadVM.restoreFromServer()
            } else {
                cvUploadVM.reset()
            }
        }
    }

    private var floatingTabBar: some View {
        HStack(spacing: 0) {
            tabItem(icon: "sparkles",             label: lm.t(.tabMatches),   tab: .matches)
            // tabItem(icon: "chart.bar.fill",      label: lm.t(.tabStats),     tab: .stats)
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
            if isActive {
                if tab == .jobs { jobsScrollToTop.toggle() }
                if tab == .matches { matchesScrollToTop.toggle() }
            } else {
                withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) {
                    selectedTab = tab
                }
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
