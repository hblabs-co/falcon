import SwiftUI
import OSLog

private let log = Logger(subsystem: "co.hblabs.falcon", category: "matches")

/// Density modes for the match cards in the list.
/// - full:    current card — score, title, breakdown bars, skills, both CTAs.
/// - compact: header + CTAs only — no breakdown, no skills.
/// - minimal: 2-column grid, score + title + date + icon-only CTAs.
/// Raw-value-backed so @AppStorage can persist it across launches.
enum MatchCardMode: String {
    case full, compact, minimal
}

struct MatchesView: View {
    @Environment(LanguageManager.self) var lm
    @Environment(NotificationManager.self) var nm
    @Environment(SessionManager.self) var session
    @Environment(CVUploadViewModel.self) var cvVM
    @Environment(\.scenePhase) var scenePhase
    /// Owned by MainTabView so the tab-bar badge can read unreadCount
    /// regardless of which tab is currently visible.
    var vm: MatchesViewModel
    @Binding var selectedTab: AppTab
    @Binding var scrollToTop: Bool
    @State private var openedProject: ProjectItem?
    @State private var openedMatch: MatchResult?
    @State private var loadingProjectID: String?
    @State private var bannerVisible = true
    // matchID to briefly highlight after scrolling to it (notification tap flow).
    @State private var highlightedMatchID: String?
    /// 0 → 1 progress multiplier for the score breakdown bars. Animates
    /// from 0 to 1 every time the tab appears, giving each match card a
    /// "bars fill up" effect so the user notices the variance instead of
    /// seeing static numbers.
    @State private var barsProgress: Double = 0

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
                .onChange(of: vm.matches.count) { _, count in
                    guard count > 0, let payload = nm.pendingMatchNavigation else { return }
                    handleMatchNavigation(payload: payload, proxy: proxy)
                }
                .onAppear {
                    if let payload = nm.pendingMatchNavigation {
                        handleMatchNavigation(payload: payload, proxy: proxy)
                    }
                    // Replay the bar-fill animation every time the tab
                    // reappears. The first render must commit with
                    // barsProgress=0; if we flip to 1 in the same frame,
                    // SwiftUI collapses the change and no animation
                    // runs. Deferring to the next runloop lets the
                    // initial frame render at 0, then the animation
                    // drives the width from 0 → 1.
                    barsProgress = 0
                    DispatchQueue.main.async {
                        withAnimation(.easeOut(duration: 0.7)) {
                            barsProgress = 1
                        }
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
                // VM is owned by MainTabView; load if empty.
                if vm.matches.isEmpty { await vm.loadInitial() }
            }
            .refreshable { await vm.refresh() }
            .onChange(of: scenePhase) { _, phase in
                if phase == .active, vm.error != nil {
                    Task { await vm.loadInitial() }
                }
            }
            .sheet(item: $openedProject) { project in
                JobDetailView(project: project, source: "matches").environment(lm)
            }
            .sheet(item: $openedMatch) { match in
                MatchDetailView(match: match)
                    .environment(lm)
                    .onAppear {
                        // Mark the match as viewed: updates the dot on
                        // the card, decrements the tab badge, and PATCHes
                        // the server. Optimistic — the UI reflects the
                        // change immediately even if the network is slow.
                        vm.markViewed(projectId: match.projectId, cvId: match.cvId)
                    }
            }
        }
        .overlay(alignment: .top) {
            LiveNewItemsBanner(
                count: vm.newMatchCount,
                singularKey: .liveNewMatchesSingular,
                pluralKey:   .liveNewMatchesPlural
            ) {
                // Drop the filter so the user lands on the full list —
                // a new match might be unread but the surrounding
                // context (label pills, score ordering) reads better
                // when everything is visible after tapping "new".
                vm.clearFilter()
                vm.clearNewMatchCount()
                scrollToTop.toggle()
                Task { await vm.refresh() }
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
                    Task { await vm.refresh() }
                } label: {
                    FalconIconView(size: 32, cornerRadius: 7)
                }
                .buttonStyle(.plain)
                VStack(alignment: .leading, spacing: 1) {
                    Text(lm.t(.tabMatches))
                        .font(.system(size: 15, weight: .semibold, design: .rounded))
                    if vm.total > 0 {
                        HStack(spacing: 3) {
                            Text("\(vm.total)")
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
                Text(vm.total > 0 ? "\(vm.total)" : "—")
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
        } else if vm.matches.isEmpty, !vm.isLoading, vm.error == nil {
            if nm.authStatus != .authorized {
                notificationsDisabledView
            } else {
                emptyView
            }
        } else {
            VStack(spacing: 16) {
                if vm.isLoading {
                    loadingView
                } else if let err = vm.error {
                    errorView(err)
                } else {
                    matchListBody(vm)
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
            controlRow
            if vm.showOnlyUnread && vm.matches.isEmpty && !vm.isLoading {
                Text(lm.t(.matchesFilterEmptyUnread))
                    .font(.system(size: 13, weight: .medium))
                    .foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 40)
            }
            if vm.cardMode == .minimal {
                minimalGrid(vm)
            } else {
                ForEach(vm.matches) { match in
                    cardForMode(match)
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
            }
            if vm.isLoadingMore {
                ProgressView().padding(.vertical, 12)
            }
        }
    }

    /// 2-column grid for `.minimal` — dense at-a-glance scrolling.
    private func minimalGrid(_ vm: MatchesViewModel) -> some View {
        LazyVGrid(
            columns: [
                GridItem(.flexible(), spacing: 10),
                GridItem(.flexible(), spacing: 10),
            ],
            spacing: 10
        ) {
            ForEach(vm.matches) { match in
                minimalCard(match)
                    .id(match.id)
                    .onAppear {
                        if match.id == vm.matches.last?.id {
                            Task { await vm.loadMore() }
                        }
                    }
            }
        }
    }

    /// Filter chips + card-density switcher on the same row.
    private var controlRow: some View {
        HStack(spacing: 8) {
            filterOption(title: lm.t(.matchesFilterAll),    active: !vm.showOnlyUnread) {
                Task { await vm.setFilter(onlyUnread: false) }
            }
            filterOption(title: lm.t(.matchesFilterUnread), active:  vm.showOnlyUnread, badge: vm.unreadCount) {
                Task { await vm.setFilter(onlyUnread: true) }
            }
            Spacer(minLength: 0)
            modeSwitcher
        }
    }

    /// Three-segment density picker. Each segment shows an SF Symbol that
    /// conveys how dense the layout becomes — ascending from "lots of
    /// detail per row" to "many cells per screen".
    private var modeSwitcher: some View {
        HStack(spacing: 2) {
            modeButton(icon: "rectangle.grid.1x2.fill", mode: .full)
            modeButton(icon: "list.bullet",             mode: .compact)
            modeButton(icon: "square.grid.2x2.fill",    mode: .minimal)
        }
        .padding(2)
        .background(Capsule().fill(Color.accentColor.opacity(0.10)))
    }

    @ViewBuilder
    private func modeButton(icon: String, mode: MatchCardMode) -> some View {
        let active = vm.cardMode == mode
        Button {
            withAnimation(.spring(response: 0.3, dampingFraction: 0.75)) {
                vm.cardMode = mode
            }
        } label: {
            Image(systemName: icon)
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(active ? .white : Color.accentColor)
                .frame(width: 28, height: 24)
                .background(
                    Capsule().fill(active ? Color.accentColor : Color.clear)
                )
        }
        .buttonStyle(.plain)
    }

    @ViewBuilder
    private func cardForMode(_ match: MatchResult) -> some View {
        switch vm.cardMode {
        case .full:    matchCard(match)
        case .compact: compactCard(match)
        case .minimal: minimalCard(match)
        }
    }

    @ViewBuilder
    private func filterOption(title: String, active: Bool, badge: Int = 0, action: @escaping () -> Void) -> some View {
        Button(action: { withAnimation(.spring(response: 0.3, dampingFraction: 0.75)) { action() } }) {
            HStack(spacing: 6) {
                Text(title)
                    .font(.system(size: 12, weight: .semibold))
                if badge > 0 {
                    Text("\(badge)")
                        .font(.system(size: 10, weight: .bold, design: .rounded))
                        .padding(.horizontal, 5)
                        .padding(.vertical, 1)
                        .background(
                            Capsule().fill(active ? Color.white.opacity(0.25) : Color.accentColor.opacity(0.18))
                        )
                }
            }
            .foregroundStyle(active ? .white : Color.accentColor)
            .padding(.horizontal, 12)
            .padding(.vertical, 6)
            .background(
                Capsule().fill(active ? Color.accentColor : Color.accentColor.opacity(0.12))
            )
        }
        .buttonStyle(.plain)
    }

    // MARK: - Card

    private func matchCard(_ match: MatchResult) -> some View {
        VStack(alignment: .leading, spacing: 14) {
            headerRow(match)
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
        // Whole-card tap opens the match detail sheet. Inner buttons
        // (Zum Job, Treffer-Details) and the ellipsis menu consume their
        // own taps first — SwiftUI respects that hierarchy, so those
        // elements keep doing their specific jobs.
        .contentShape(RoundedRectangle(cornerRadius: 18, style: .continuous))
        .onTapGesture { openedMatch = match }
    }

    /// Compact card: header (score, label, date, title, company) + two
    /// CTAs. Omits the 6-row score breakdown and skills chips — good
    /// middle ground between at-a-glance and full detail.
    /// Padding/radius/shadow deliberately match `matchCard` so toggling
    /// Full ↔ Compact doesn't shift content horizontally; only the inner
    /// vertical sections change, which reads as density, not layout jitter.
    private func compactCard(_ match: MatchResult) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            headerRow(match)
            actionRow(match)
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
        .contentShape(RoundedRectangle(cornerRadius: 18, style: .continuous))
        .onTapGesture { openedMatch = match }
    }

    /// Minimal card (2-col grid cell). Layout top-down:
    ///   row 1 : score ring  | (date, label pill)  | spacer | ⋯ menu
    ///   row 2 : project title
    ///   row 3 : AI summary (if any)
    ///   row 4 : spacer pushing company to the bottom
    ///   row 5 : company name — whisper-small
    ///
    /// Replaces the previous dual-blue-button footer with a single ⋯
    /// menu (top-right) to cut visual noise. Tapping ⋯ reveals the
    /// same two actions as the Full/Compact card. iOS's Menu handles
    /// the "tap another card, close mine" behaviour out of the box:
    /// any tap outside the menu's hit area dismisses it.
    private func minimalCard(_ match: MatchResult) -> some View {
        let waiting = !match.isNormalized
        let summary = match.summaryAll[lm.appLanguage.rawValue] ?? ""

        return VStack(alignment: .leading, spacing: 6) {
            // Row 1: score ring | Spacer | [date+label pill] | ⋮ ghost.
            // The date/label pair sits right-aligned next to the ⋮ button
            // so all the "meta" lives in one corner of the card, letting
            // the title + summary use the full width below.
            HStack(alignment: .top, spacing: 6) {
                scoreRingBadge(score: match.score, color: match.labelColor, size: 34)
                Spacer(minLength: 0)
                VStack(alignment: .trailing, spacing: 3) {
                    HStack(spacing: 4) {
                        if !match.isViewed {
                            Circle()
                                .fill(Color.accentColor)
                                .frame(width: 6, height: 6)
                        }
                        if let date = match.relativeDate(for: lm.appLanguage) {
                            Text(date)
                                .font(.system(size: 9))
                                .foregroundStyle(.tertiary)
                                .lineLimit(1)
                        }
                    }
                    Text(match.labelText(lm: lm))
                        .font(.system(size: 9, weight: .semibold))
                        .foregroundStyle(match.labelColor)
                        .padding(.horizontal, 5)
                        .padding(.vertical, 1)
                        .background(Capsule().fill(match.labelColor.opacity(0.14)))
                        .lineLimit(1)
                }
                Menu {
                    Button {
                        Task { await openProject(match) }
                    } label: {
                        Label(lm.t(.matchesViewJob), systemImage: "briefcase.fill")
                    }
                    .disabled(waiting)

                    Button {
                        openedMatch = match
                    } label: {
                        Label(lm.t(.matchesDetails), systemImage: "sparkles")
                    }
                } label: {
                    // Vertical 3-dot glyph inside a "ghost" circle so the
                    // hit-area reads as a tappable control without the
                    // visual weight of a solid button. Rotating the plain
                    // `ellipsis` symbol 90° gives vertical dots without
                    // needing a special SF Symbol.
                    Image(systemName: "ellipsis")
                        .font(.system(size: 12, weight: .bold))
                        .rotationEffect(.degrees(90))
                        .foregroundStyle(.secondary)
                        .frame(width: 26, height: 26)
                        .background(Circle().stroke(Color.secondary.opacity(0.25), lineWidth: 1))
                        .contentShape(Circle())
                }
                .buttonStyle(.plain)
            }

            // Row 2: title.
            Text(match.projectTitle)
                .font(.system(size: 11, weight: .semibold, design: .rounded))
                .lineLimit(2)
                .multilineTextAlignment(.leading)
                .frame(maxWidth: .infinity, alignment: .leading)

            // Row 3: AI summary — up to 4 lines so the user gets real
            // context at a glance without the card growing unboundedly.
            if !summary.isEmpty {
                Text(summary)
                    .font(.system(size: 9))
                    .foregroundStyle(.secondary)
                    .lineLimit(4)
                    .multilineTextAlignment(.leading)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }

            Spacer(minLength: 0)

            // Row 5: initials avatar + company name — mirrors the Lock
            // Screen Live Activity layout so the two surfaces feel
            // visually of-a-kind.
            if !match.companyName.isEmpty {
                HStack(spacing: 5) {
                    companyInitialsAvatar(name: match.companyName, size: 16)
                    Text(match.companyName)
                        .font(.system(size: 9, weight: .medium))
                        .foregroundStyle(.tertiary)
                        .lineLimit(1)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .padding(10)
        .frame(maxWidth: .infinity, minHeight: 150, alignment: .topLeading)
        .background(
            RoundedRectangle(cornerRadius: 14, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.05), radius: 10, x: 0, y: 3)
        )
        .contentShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
        .onTapGesture { openedMatch = match }
    }

    /// Circular avatar with the company's two first-letter initials.
    /// Matches the `CompanyInitialsIcon` used in the Live Activity widget
    /// so a user who sees "AC" (ACME Corp) on the Lock Screen sees the
    /// same chip in the minimal match card.
    private func companyInitialsAvatar(name: String, size: CGFloat) -> some View {
        let words = name.split(separator: " ").prefix(2)
        let initials = words.compactMap { $0.first }.map(String.init).joined().uppercased()
        return ZStack {
            Circle().fill(Color.accentColor.opacity(0.18))
            Text(initials)
                .font(.system(size: size * 0.48, weight: .bold, design: .rounded))
                .foregroundStyle(Color.accentColor)
        }
        .frame(width: size, height: size)
    }

    /// Two-column header:
    ///  - Left  : score badge
    ///  - Right : 3 stacked rows — label pill + date, title, company.
    /// Keeps the vertical rhythm of the card predictable regardless of
    /// whether the title wraps.
    private func headerRow(_ match: MatchResult) -> some View {
        HStack(alignment: .top, spacing: 12) {
            scoreBadge(match)
            VStack(alignment: .leading, spacing: 4) {
                // Row 1: label pill on the left. Right side combines
                // "NEW" (when unread) with the relative date so the
                // user can scan freshness in one place. NEW disappears
                // after the user opens MatchDetailView (isViewed=true).
                HStack(spacing: 10) {
                    Text(match.labelText(lm: lm))
                        .font(.system(size: 11, weight: .semibold))
                        .foregroundStyle(match.labelColor)
                        .padding(.horizontal, 8)
                        .padding(.vertical, 3)
                        .background(Capsule().fill(match.labelColor.opacity(0.12)))
                    Spacer(minLength: 0)
                    HStack(spacing: 6) {
                        if !match.isViewed {
                            Text(lm.t(.matchesNewBadge))
                                .font(.system(size: 9, weight: .bold, design: .rounded))
                                .foregroundStyle(.white)
                                .padding(.horizontal, 6)
                                .padding(.vertical, 2)
                                .background(Capsule().fill(Color.accentColor))
                        }
                        if let date = match.relativeDate(for: lm.appLanguage) {
                            Text(date)
                                .font(.system(size: 11))
                                .foregroundStyle(.tertiary)
                        }
                    }
                }
                // Row 2: project title.
                Text(match.projectTitle)
                    .font(.system(size: 15, weight: .semibold, design: .rounded))
                    .lineLimit(2)
                // Row 3: initials avatar + company — same treatment the
                // minimal card and Live Activity Lock Screen use, so the
                // same company reads as the same "chip" wherever it
                // appears in the app.
                HStack(spacing: 6) {
                    companyInitialsAvatar(name: match.companyName.nilIfEmpty ?? match.platform, size: 18)
                    Text(match.companyName.nilIfEmpty ?? match.platform)
                        .font(.system(size: 12, weight: .medium))
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }
        }
    }

    private func scoreBadge(_ match: MatchResult) -> some View {
        scoreRingBadge(score: match.score, color: match.labelColor, size: 52)
    }

    /// Reusable ring-around-the-number badge. Arc length ∝ score/10.
    /// `barsProgress` drives the first-appear animation (same state that
    /// powers the score-breakdown bars) so the ring fills from 0 → full
    /// every time the user re-enters the tab. Any subsequent score update
    /// (e.g. realtime push) animates between the old and new arc thanks
    /// to `.animation(value: score)` — the ring visibly rises when the
    /// score improves, shrinks when it drops.
    private func scoreRingBadge(score: Double, color: Color, size: CGFloat) -> some View {
        let lineWidth: CGFloat = max(2, size * 0.08)
        let progress = min(1.0, max(0, score / 10))
        return ZStack {
            Circle().fill(color.opacity(0.12))
            Circle()
                .trim(from: 0, to: progress * barsProgress)
                .stroke(color, style: StrokeStyle(lineWidth: lineWidth, lineCap: .round))
                .rotationEffect(.degrees(-90))
                .padding(lineWidth / 2)
                .animation(.easeOut(duration: 0.8), value: barsProgress)
                .animation(.easeOut(duration: 0.5), value: score)
            Text(String(format: "%.1f", score))
                .font(.system(size: size * 0.38, weight: .bold, design: .rounded))
                .foregroundStyle(color)
        }
        .frame(width: size, height: size)
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
                        // barsProgress drives the fill width from 0 (on
                        // tab appear) to 1 (fully extended) via the
                        // withAnimation wrapper in .onAppear. Implicit
                        // animation picks up the width change.
                        .frame(width: max(4, geo.size.width * CGFloat(value / 10) * barsProgress), height: 6)
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
        // `waiting` = normalizer hasn't finished yet; tapping would 404.
        // `loading` = user tapped and we're fetching the project page.
        // Both show a spinner + disabled button, but waiting cares about
        // server-side normalization while loading is a transient in-flight.
        let waiting = !match.isNormalized
        let loading = loadingProjectID == match.projectId
        let showSpinner = waiting || loading

        return HStack(spacing: 8) {
            Button {
                Task { await openProject(match) }
            } label: {
                HStack(spacing: 6) {
                    ZStack {
                        Image(systemName: "briefcase.fill")
                            .font(.system(size: 11, weight: .semibold))
                            .opacity(showSpinner ? 0 : 1)
                        if showSpinner {
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
                .background(
                    RoundedRectangle(cornerRadius: 10, style: .continuous)
                        .fill(Color.accentColor.opacity(waiting ? 0.55 : 1))
                )
                .foregroundStyle(.white)
            }
            .buttonStyle(.plain)
            .disabled(waiting)

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
            Button("Retry") { Task { await vm.loadInitial() } }
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
        // Drop the filter if it's on — the user was sent here by a
        // push/URL, so the landing state should show everything, not
        // a filtered view that might omit context around the target
        // match. A refresh is triggered on the view's regular .task
        // / .onChange chain so we don't need to manually refetch.
        if vm.showOnlyUnread {
            vm.clearFilter()
            Task { await vm.refresh() }
        }

        // Only consume the payload when we actually find the match. If matches
        // haven't loaded yet (cold-launch via notification), leave the payload
        // for the next trigger (onChange of matches.count) to retry.
        guard let match = vm.matches.first(where: { $0.id == payload.matchID }) else {
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
