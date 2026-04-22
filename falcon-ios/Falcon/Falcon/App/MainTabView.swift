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
    /// Hoisted from MatchesView so the tab bar can read unreadCount for
    /// the badge regardless of which tab is currently active. Created
    /// once in .task once SessionManager is available.
    @State private var matchesVM: MatchesViewModel?
    /// Hoisted from JobsView for symmetry with matchesVM: the realtime
    /// listener at this level routes project.normalized pushes into the
    /// VM counter so state survives if JobsView ever stops being kept
    /// alive via opacity(0).
    @State private var jobsVM = JobsViewModel()
    @State private var jobsScrollToTop = false
    @State private var matchesScrollToTop = false

    var body: some View {
        ZStack(alignment: .bottom) {
            ZStack {
                // Jobs is always kept alive so scroll position, pagination and
                // the loaded list survive switching to another tab.
                JobsView(vm: jobsVM, scrollToTop: $jobsScrollToTop)
                    .opacity(selectedTab == .jobs ? 1 : 0)
                    .allowsHitTesting(selectedTab == .jobs)

                // Other tabs are created on demand and torn down when leaving.
                Group {
                    switch selectedTab {
                    case .matches:
                        if let matchesVM {
                            MatchesView(vm: matchesVM, selectedTab: $selectedTab, scrollToTop: $matchesScrollToTop)
                        }
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
        .overlay(alignment: .top) {
            // Global realtime health banner — appears on top of whatever
            // tab is active. Debounces a 10s dropout before showing the
            // "server offline" pill so brief reconnects don't flash.
            ServerStatusBanner()
        }
        .task {
            await nm.refreshStatus()
            // Bootstrap the shared matches VM so the tab badge has
            // data even before the user opens the Matches tab.
            if matchesVM == nil {
                let vm = MatchesViewModel(session: session)
                matchesVM = vm
                if session.isAuthenticated {
                    await vm.loadInitial()
                }
            }
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
                await matchesVM?.loadInitial()
            } else {
                cvUploadVM.reset()
                matchesVM?.reset()
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
        // Realtime fan-out lives here (not inside MatchesView) because
        // MatchesView is destroyed when the user is on another tab —
        // any listener there would miss match.result / project.normalized
        // pushes that arrive while the user is browsing Jobs. The VM
        // persists across tab switches, so state lands there and
        // MatchesView just reflects it on next appear.
        .onReceive(NotificationCenter.default.publisher(for: .realtimeMessage)) { note in
            guard let type = note.userInfo?["type"] as? String,
                  let payload = note.userInfo?["payload"] as? [String: Any] else { return }
            switch type {
            case "project.normalized":
                // Jobs: bump the banner counter + hero "heute" count.
                // Matches: clear the "Zum Job" spinner for any loaded
                // match that was waiting on this project to normalize.
                withAnimation(.spring(response: 0.35, dampingFraction: 0.75)) {
                    jobsVM.bumpOnNewProject()
                }
                if let projectId = payload["project_id"] as? String {
                    withAnimation { matchesVM?.markProjectNormalized(projectId: projectId) }
                }
            case "match.flipped":
                // Safety-net broadcast from match-engine's periodic
                // sweep: the original project.normalized may have been
                // missed (socket down, app closed). Same handling as
                // project.normalized but without the jobs-side bump —
                // this event doesn't represent a new project, only a
                // stale flag being corrected.
                if let projectId = payload["project_id"] as? String {
                    withAnimation { matchesVM?.markProjectNormalized(projectId: projectId) }
                }
            case "match.result":
                withAnimation(.spring(response: 0.35, dampingFraction: 0.75)) {
                    matchesVM?.bumpOnNewMatch()
                }
            default:
                break
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
            tabItem(icon: "sparkles",             label: lm.t(.tabMatches),   tab: .matches, badge: matchesVM?.unreadCount ?? 0)
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
