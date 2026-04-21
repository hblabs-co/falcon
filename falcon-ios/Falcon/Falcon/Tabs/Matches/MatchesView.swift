import SwiftUI
import OSLog

private let log = Logger(subsystem: "co.hblabs.falcon", category: "matches")

struct MatchesView: View {
    @Environment(LanguageManager.self) var lm
    @Environment(NotificationManager.self) var nm
    @Environment(SessionManager.self) var session
    @Environment(CVUploadViewModel.self) var cvVM
    @Environment(\.scenePhase) var scenePhase
    @Binding var selectedTab: AppTab
    @Binding var scrollToTop: Bool

    @State private var vm: MatchesViewModel?
    @State private var openedProject: ProjectItem?
    @State private var openedMatch: MatchResult?
    @State private var loadingProjectID: String?
    @State private var bannerVisible = true
    // matchID to briefly highlight after scrolling to it (notification tap flow).
    @State private var highlightedMatchID: String?

    var body: some View {
        NavigationStack {
            ScrollViewReader { proxy in
                ScrollView {
                    VStack(spacing: 16) {
                        heroBanner.id("top")
                        if !session.isAuthenticated {
                            AlreadyHaveAccountBanner()
                        }
                        contentBody
                    }
                    .padding(.horizontal, 16)
                    .padding(.top, 8)
                    .padding(.bottom, 110)
                }
                .onScrollGeometryChange(for: CGFloat.self) { geo in
                    geo.contentOffset.y
                } action: { _, newOffset in
                    withAnimation(.easeInOut(duration: 0.2)) {
                        bannerVisible = newOffset < 30
                    }
                }
                .onChange(of: scrollToTop) { _, _ in
                    withAnimation { proxy.scrollTo("top", anchor: .top) }
                }
                // Consume pending notification tap. Fires on 3 triggers so
                // cold-launch and live-tap both work:
                //   1. payload changes (runtime tap while MatchesView is alive)
                //   2. matches list finishes loading (cold-launch — view
                //      mounted before loadInitial completed)
                //   3. view first appears (user navigates manually to tab after
                //      a tap that arrived earlier)
                .onChange(of: nm.pendingMatchNavigation) { _, payload in
                    guard let payload else { return }
                    handleMatchNavigation(payload: payload, proxy: proxy)
                }
                .onChange(of: vm?.matches.count ?? 0) { _, count in
                    guard count > 0, let payload = nm.pendingMatchNavigation else { return }
                    handleMatchNavigation(payload: payload, proxy: proxy)
                }
                .onAppear {
                    if let payload = nm.pendingMatchNavigation {
                        handleMatchNavigation(payload: payload, proxy: proxy)
                    }
                }
            }
            .background(Color(UIColor.systemGroupedBackground))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .principal) {
                    navTitle
                }
            }
            .withLoginToolbar()
            .task {
                if vm == nil {
                    vm = MatchesViewModel(session: session)
                }
                await vm?.loadInitial()
            }
            .refreshable { await vm?.refresh() }
            .onChange(of: scenePhase) { _, phase in
                if phase == .active, vm?.error != nil {
                    Task { await vm?.loadInitial() }
                }
            }
            .sheet(item: $openedProject) { project in
                JobDetailView(project: project, source: "matches").environment(lm)
            }
            .sheet(item: $openedMatch) { match in
                MatchDetailView(match: match).environment(lm)
            }
        }
        .overlay(alignment: .bottomTrailing) {
            scrollToTopButton
        }
    }

    // MARK: - Nav title (compact when scrolled, expanded "Treffer" centered when at top)

    @ViewBuilder
    private var navTitle: some View {
        if bannerVisible {
            Text(lm.t(.tabMatches))
                .font(.system(size: 17, weight: .semibold, design: .rounded))
                .transition(.move(edge: .bottom).combined(with: .opacity))
        } else {
            HStack(spacing: 8) {
                Button {
                    scrollToTop.toggle()
                    Task { await vm?.refresh() }
                } label: {
                    FalconIconView(size: 32, cornerRadius: 7)
                }
                .buttonStyle(.plain)
                VStack(alignment: .leading, spacing: 1) {
                    Text(lm.t(.tabMatches))
                        .font(.system(size: 15, weight: .semibold, design: .rounded))
                    if let total = vm?.total, total > 0 {
                        HStack(spacing: 3) {
                            Text("\(total)")
                                .font(.system(size: 11, weight: .bold, design: .rounded))
                                .foregroundStyle(.primary)
                            Text(lm.t(.matchesBannerTotal))
                                .font(.system(size: 11, weight: .regular))
                                .foregroundStyle(.secondary)
                        }
                    }
                }
            }
            .transition(.move(edge: .top).combined(with: .opacity))
        }
    }

    // MARK: - Hero banner

    private var heroBanner: some View {
        HStack(spacing: 14) {
            FalconIconView(size: 48, cornerRadius: 11)

            VStack(alignment: .leading, spacing: 3) {
                Text("Falcon")
                    .font(.system(size: 18, weight: .bold, design: .rounded))
                Text(lm.t(.matchesBannerTagline))
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(.secondary)
            }

            Spacer()

            VStack(alignment: .trailing, spacing: 2) {
                Text((vm?.total ?? 0) > 0 ? "\(vm?.total ?? 0)" : "—")
                    .font(.system(size: 22, weight: .bold, design: .rounded))
                Text(lm.t(.matchesBannerTotal))
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

    // MARK: - Floating scroll-to-top button

    @ViewBuilder
    private var scrollToTopButton: some View {
        if !bannerVisible {
            Button {
                scrollToTop.toggle()
            } label: {
                Image(systemName: "arrow.up")
                    .font(.system(size: 18, weight: .semibold))
                    .foregroundStyle(.white)
                    .frame(width: 44, height: 44)
                    .background(
                        Circle()
                            .fill(Color.accentColor)
                            .shadow(color: .black.opacity(0.2), radius: 10, x: 0, y: 4)
                    )
            }
            .buttonStyle(.plain)
            .padding(.trailing, 20)
            .padding(.bottom, 130)
            .transition(.scale.combined(with: .opacity))
        }
    }

    // MARK: - Content body (state branches)
    //
    // Priority:
    //   1. No CV             → upload first (hard blocker)
    //   2. CV failed         → retry (hard blocker)
    //   3. Empty + notifs off → full-screen CTA to enable notifications
    //   4. Empty             → empty state
    //   5. Has items / loading / error → list or status with a compact
    //      "notifs off" banner on top when applicable
    @ViewBuilder
    private var contentBody: some View {
        if session.userID.isEmpty {
            noSessionView
        } else if isCVFailed {
            cvFailedView
        } else if let vm, vm.matches.isEmpty, !vm.isLoading, vm.error == nil {
            if nm.authStatus != .authorized {
                notificationsDisabledView
            } else {
                emptyView
            }
        } else {
            VStack(spacing: 16) {
                if let vm, vm.isLoading {
                    loadingView
                } else if let vm, let err = vm.error {
                    errorView(err)
                } else if let vm {
                    matchListBody(vm)
                } else {
                    loadingView
                }
            }
        }
    }

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
        .padding(.top, 40)
    }

    // MARK: - Info badge shown at the top of the list (always visible).

    private var infoBadge: some View {
        HStack(spacing: 12) {
            Image(systemName: "sparkles")
                .font(.system(size: 14, weight: .semibold))
                .foregroundStyle(Color.accentColor)
                .frame(width: 30, height: 30)
                .background(Circle().fill(Color.accentColor.opacity(0.15)))

            VStack(alignment: .leading, spacing: 2) {
                Text(lm.t(.matchesInfoBadgeTitle))
                    .font(.system(size: 13, weight: .bold))
                    .foregroundStyle(Color.accentColor)
                Text(lm.t(.matchesInfoBadge))
                    .font(.system(size: 10, weight: .medium))
                    .foregroundStyle(Color.accentColor.opacity(0.75))
                    .lineLimit(3)
                    .fixedSize(horizontal: false, vertical: true)
            }

            Spacer(minLength: 0)
        }
        .padding(12)
        .background(
            RoundedRectangle(cornerRadius: 14, style: .continuous)
                .fill(Color.accentColor.opacity(0.1))
        )
    }

    // MARK: - List body (no wrapping ScrollView — parent handles scrolling)

    private func matchListBody(_ vm: MatchesViewModel) -> some View {
        LazyVStack(spacing: 14) {
            infoBadge
            ForEach(vm.matches) { match in
                matchCard(match)
                    .id(match.id)
                    .scaleEffect(highlightedMatchID == match.id ? 1.015 : 1.0)
                    .shadow(
                        color: highlightedMatchID == match.id ? Color.accentColor.opacity(0.35) : .clear,
                        radius: highlightedMatchID == match.id ? 18 : 0
                    )
                    .animation(.spring(response: 0.35, dampingFraction: 0.7), value: highlightedMatchID)
                    .onAppear {
                        if match.id == vm.matches.last?.id {
                            Task { await vm.loadMore() }
                        }
                    }
            }
            if vm.isLoadingMore {
                ProgressView().padding(.vertical, 12)
            }
        }
    }

    // MARK: - Card

    private func matchCard(_ match: MatchResult) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            topRow(match)
            titleBlock(match)
            scoreBreakdown(match)
            skillsSection(match)
            actionRow(match)
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
    }

    private func topRow(_ match: MatchResult) -> some View {
        HStack(alignment: .center, spacing: 10) {
            scoreBadge(match)
            Text(match.labelText(lm: lm))
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(match.labelColor)
                .padding(.horizontal, 8)
                .padding(.vertical, 3)
                .background(Capsule().fill(match.labelColor.opacity(0.12)))
            Spacer()
            if let date = match.relativeDate(for: lm.appLanguage) {
                Text(date)
                    .font(.system(size: 11))
                    .foregroundStyle(.tertiary)
            }
        }
    }

    private func scoreBadge(_ match: MatchResult) -> some View {
        Text(String(format: "%.1f", match.score))
            .font(.system(size: 20, weight: .bold, design: .rounded))
            .foregroundStyle(match.labelColor)
            .frame(width: 52, height: 52)
            .background(Circle().fill(match.labelColor.opacity(0.12)))
    }

    private func titleBlock(_ match: MatchResult) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(match.projectTitle)
                .font(.system(size: 15, weight: .semibold, design: .rounded))
                .lineLimit(2)
            Label {
                Text(match.companyName.nilIfEmpty ?? match.platform)
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(.secondary)
            } icon: {
                Image(systemName: "building.2")
                    .font(.system(size: 10, weight: .medium))
                    .foregroundStyle(.tertiary)
            }
        }
    }

    // MARK: - Score breakdown (six bars)

    private func scoreBreakdown(_ match: MatchResult) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(lm.t(.matchesScoreBreakdown))
                .font(.system(size: 10, weight: .semibold))
                .foregroundStyle(.tertiary)
                .textCase(.uppercase)
                .tracking(0.5)
                .padding(.bottom, 2)
            scoreRow(label: lm.t(.matchesScoreSkillsMatch),          value: match.scores.skillsMatch)
            scoreRow(label: lm.t(.matchesScoreSeniorityFit),         value: match.scores.seniorityFit)
            scoreRow(label: lm.t(.matchesScoreDomainExperience),     value: match.scores.domainExperience)
            scoreRow(label: lm.t(.matchesScoreCommunicationClarity), value: match.scores.communicationClarity)
            scoreRow(label: lm.t(.matchesScoreProjectRelevance),     value: match.scores.projectRelevance)
            scoreRow(label: lm.t(.matchesScoreTechStackOverlap),     value: match.scores.techStackOverlap)
        }
    }

    private func scoreRow(label: String, value: Double) -> some View {
        HStack(spacing: 8) {
            Text(label)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(.secondary)
                .frame(width: 96, alignment: .leading)
            GeometryReader { geo in
                ZStack(alignment: .leading) {
                    Capsule()
                        .fill(Color.secondary.opacity(0.12))
                        .frame(height: 6)
                    Capsule()
                        .fill(scoreColor(value))
                        .frame(width: max(4, geo.size.width * CGFloat(value / 10)), height: 6)
                }
                .frame(maxHeight: .infinity, alignment: .center)
            }
            .frame(height: 12)
            Text("\(Int((value * 10).rounded()))%")
                .font(.system(size: 10, weight: .semibold, design: .rounded))
                .foregroundStyle(scoreColor(value))
                .frame(width: 36, alignment: .trailing)
        }
    }

    // Color scale on red/orange/green only — pastel variants for the lower
    // end of each band so intensity rises with the score.
    private func scoreColor(_ value: Double) -> Color {
        switch value {
        case ..<3:   return .red
        case ..<5:   return .red.opacity(0.6)
        case ..<6:   return .orange.opacity(0.75)
        case ..<7:   return .orange
        case ..<9:   return .green.opacity(0.7)
        default:     return .green
        }
    }

    // MARK: - Skills (matched + missing)

    @ViewBuilder
    private func skillsSection(_ match: MatchResult) -> some View {
        if !match.matchedSkills.isEmpty {
            skillsRow(
                title: lm.t(.matchesSkillsYouHave),
                icon: "checkmark.circle.fill",
                color: .green,
                items: match.matchedSkills
            )
        }
        if !match.missingSkills.isEmpty {
            skillsRow(
                title: lm.t(.matchesMissingSkills),
                icon: "xmark.circle.fill",
                color: .red,
                items: match.missingSkills
            )
        }
    }

    private func skillsRow(title: String, icon: String, color: Color, items: [String]) -> some View {
        VStack(alignment: .leading, spacing: 6) {
            HStack(spacing: 5) {
                Image(systemName: icon)
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(color)
                Text(title)
                    .font(.system(size: 10, weight: .semibold))
                    .foregroundStyle(.tertiary)
                    .textCase(.uppercase)
                    .tracking(0.5)
            }
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: 6) {
                    ForEach(items, id: \.self) { skill in
                        Text(skill)
                            .font(.system(size: 11, weight: .medium))
                            .padding(.horizontal, 10)
                            .padding(.vertical, 5)
                            .background(Capsule().fill(color.opacity(0.12)))
                            .foregroundStyle(color)
                    }
                }
            }
        }
    }

    // MARK: - Action row

    private func actionRow(_ match: MatchResult) -> some View {
        HStack(spacing: 8) {
            Button {
                Task { await openProject(match) }
            } label: {
                HStack(spacing: 6) {
                    ZStack {
                        Image(systemName: "briefcase.fill")
                            .font(.system(size: 11, weight: .semibold))
                            .opacity(loadingProjectID == match.projectId ? 0 : 1)
                        if loadingProjectID == match.projectId {
                            ProgressView().scaleEffect(0.6).tint(.white)
                        }
                    }
                    .frame(width: 16, height: 16)
                    Text(lm.t(.matchesViewJob))
                        .font(.system(size: 13, weight: .semibold))
                        .lineLimit(1)
                        .minimumScaleFactor(0.8)
                }
                .frame(maxWidth: .infinity)
                .padding(.vertical, 10)
                .background(RoundedRectangle(cornerRadius: 10, style: .continuous).fill(Color.accentColor))
                .foregroundStyle(.white)
            }
            .buttonStyle(.plain)

            Button {
                openedMatch = match
            } label: {
                HStack(spacing: 6) {
                    Image(systemName: "sparkles")
                        .font(.system(size: 11, weight: .semibold))
                    Text(lm.t(.matchesDetails))
                        .font(.system(size: 13, weight: .semibold))
                        .lineLimit(1)
                        .minimumScaleFactor(0.8)
                }
                .frame(maxWidth: .infinity)
                .padding(.vertical, 10)
                .background(
                    RoundedRectangle(cornerRadius: 10, style: .continuous)
                        .stroke(Color.accentColor, lineWidth: 1.5)
                )
                .foregroundStyle(Color.accentColor)
            }
            .buttonStyle(.plain)
        }
    }

    // MARK: - States

    private var loadingView: some View {
        LazyVStack(spacing: 14) {
            ForEach(0..<4, id: \.self) { _ in
                RoundedRectangle(cornerRadius: 18, style: .continuous)
                    .fill(.quaternary)
                    .frame(height: 220)
            }
        }
    }

    private var emptyView: some View {
        ContentUnavailableView(
            lm.t(.matchesEmpty),
            systemImage: "sparkles",
            description: Text(lm.t(.matchesEmptyDescription))
        )
        .padding(.top, 40)
    }

    private var noSessionView: some View {
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
        .padding(.top, 40)
    }

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
        .padding(.top, 40)
    }

    private func errorView(_ message: String) -> some View {
        VStack(spacing: 12) {
            Image(systemName: "exclamationmark.triangle")
                .font(.system(size: 32))
                .foregroundStyle(.secondary)
            Text(message)
                .font(.system(size: 14))
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
                .padding(.horizontal, 32)
            Button("Retry") { Task { await vm?.loadInitial() } }
                .buttonStyle(.bordered)
        }
        .padding(.top, 40)
    }

    // MARK: - Notification tap navigation

    /// Scroll to the notification's match, open its detail sheet, and briefly
    /// highlight the card. Called from both .onChange (payload arrives while
    /// MatchesView is alive) and .onAppear (payload arrived before this tab
    /// was rendered).
    private func handleMatchNavigation(payload: NotificationManager.MatchNotificationPayload, proxy: ScrollViewProxy) {
        // Only consume the payload when we actually find the match. If matches
        // haven't loaded yet (cold-launch via notification), leave the payload
        // for the next trigger (onChange of matches.count) to retry.
        guard let match = vm?.matches.first(where: { $0.id == payload.matchID }) else {
            log.info("notification match \(payload.matchID, privacy: .public) not loaded yet — will retry when list fills")
            return
        }

        Task { @MainActor in
            // Let the tab-switch animation settle before scrolling.
            try? await Task.sleep(for: .milliseconds(350))

            withAnimation(.easeInOut(duration: 0.35)) {
                proxy.scrollTo(match.id, anchor: .center)
            }
            highlightedMatchID = match.id
            openedMatch = match

            // Consume the payload so a later tap re-triggers.
            nm.pendingMatchNavigation = nil

            // Fade out the highlight after a moment.
            try? await Task.sleep(for: .seconds(2))
            if highlightedMatchID == match.id {
                highlightedMatchID = nil
            }
        }
    }

    // MARK: - Open project

    private func openProject(_ match: MatchResult) async {
        loadingProjectID = match.projectId
        defer { loadingProjectID = nil }
        do {
            let project = try await MatchesAPI.fetchProject(
                id: match.projectId,
                lang: lm.appLanguage.rawValue,
                jwt: session.cachedJWT
            )
            openedProject = project
        } catch {
            log.error("open project \(match.projectId, privacy: .public) failed: \(error.localizedDescription, privacy: .public)")
        }
    }
}
