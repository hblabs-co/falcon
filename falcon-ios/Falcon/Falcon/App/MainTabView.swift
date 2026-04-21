import SwiftUI

enum AppTab {
    case jobs, matches, stats, profile, settings, actions
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
                    case .actions:     ActionsView()
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
        .task {
            await nm.refreshStatus()
            // Cold-launch via notification tap: didReceive fires BEFORE this
            // view mounts, so .onChange never sees the transition. Check the
            // initial value explicitly.
            if nm.pendingMatchNavigation != nil {
                withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) {
                    selectedTab = .matches
                }
            }
        }
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
        // When the user taps a MATCH_RESULT push, NotificationManager fills
        // pendingMatchNavigation. Switch to the matches tab here; MatchesView
        // picks up the same payload to scroll + open the detail sheet.
        .onChange(of: nm.pendingMatchNavigation) { _, payload in
            guard payload != nil else { return }
            withAnimation(.spring(response: 0.3, dampingFraction: 0.7)) {
                selectedTab = .matches
            }
        }
    }

    /// Number of pending action banners ActionsView would render. Drives
    /// the red counter badge on the tab icon so the user can see from any
    /// screen that something needs attention.
    private var actionsPendingCount: Int {
        var n = 0
        if !session.isAuthenticated { n += 1 }
        if nm.authStatus != .authorized { n += 1 }
        if nm.liveActivitiesRestricted { n += 1 }
        return n
    }

    private var floatingTabBar: some View {
        HStack(spacing: 0) {
            tabItem(icon: "bell.fill",           label: lm.t(.tabActions),   tab: .actions, badge: actionsPendingCount)
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

    private func tabItem(icon: String, label: String, tab: AppTab, badge: Int = 0) -> some View {
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
                ZStack(alignment: .topTrailing) {
                    Image(systemName: icon)
                        .font(.system(size: 18, weight: isActive ? .semibold : .regular))
                    if badge > 0 {
                        Text("\(badge)")
                            .font(.system(size: 10, weight: .bold, design: .rounded))
                            .foregroundStyle(.white)
                            .frame(minWidth: 16, minHeight: 16)
                            .padding(.horizontal, 3)
                            .background(Capsule().fill(Color.red))
                            .offset(x: 10, y: -6)
                    }
                }
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
