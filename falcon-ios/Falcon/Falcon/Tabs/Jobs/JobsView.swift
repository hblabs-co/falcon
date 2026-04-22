import SwiftUI

// MARK: - ProjectsView

struct ProjectsView: View {
    @Environment(LanguageManager.self) var lm
    @Environment(NotificationManager.self) var nm
    @Environment(SessionManager.self) var session
    @Environment(\.scenePhase) var scenePhase
    /// Owned by MainTabView so the realtime listener at that level can
    /// funnel project.normalized pushes into the VM — which keeps the
    /// banner counter working independent of how this view is mounted.
    var vm: ProjectsViewModel
    @State private var bannerVisible = true
    @Binding var scrollToTop: Bool

    var body: some View {
        NavigationStack {
            ScrollViewReader { proxy in
                ScrollView {
                    VStack(spacing: 16) {
                        heroBanner
                            .id("top")
                        if !session.isAuthenticated {
                            AlreadyHaveAccountBanner()
                        }
                        projectsList
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
            }
            .background(Color(UIColor.systemGroupedBackground))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .principal) {
                    navTitle
                }
            }
            .withLoginToolbar()
            .task { await vm.loadInitial() }
            .refreshable { await vm.refresh() }
            .onChange(of: lm.appLanguage) { Task { await vm.loadInitial() } }
            .onChange(of: scenePhase) { _, phase in
                if phase == .active, vm.error != nil {
                    Task { await vm.loadInitial() }
                }
            }
        }
        .overlay(alignment: .top) {
            LiveNewItemsBanner(
                count: vm.newProjectCount,
                singularKey: .liveNewProjectsSingular,
                pluralKey:   .liveNewProjectsPlural
            ) {
                vm.clearNewProjectCount()
                scrollToTop.toggle()
                Task { await vm.refresh() }
            }
        }
        .overlay(alignment: .bottomTrailing) {
            scrollToTopButton
        }
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

    // MARK: - Navigation title (collapses into near-Dynamic-Island area)

    @ViewBuilder
    private var navTitle: some View {
        if bannerVisible {
            Text(lm.t(.tabProjects))
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
                    Text(lm.t(.tabProjects))
                        .font(.system(size: 15, weight: .semibold, design: .rounded))
                    if vm.todayCount > 0 {
                        HStack(spacing: 3) {
                            Text("\(vm.todayCount)")
                                .font(.system(size: 11, weight: .bold, design: .rounded))
                                .foregroundStyle(.primary)
                            Text(lm.t(.projectsBannerMatchCount))
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
                Text(lm.t(.projectsBannerTagline))
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(.secondary)
            }

            Spacer()

            VStack(alignment: .trailing, spacing: 2) {
                Text(vm.todayCount > 0 ? "\(vm.todayCount)" : "—")
                    .font(.system(size: 22, weight: .bold, design: .rounded))
                Text(lm.t(.projectsBannerMatchCount))
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

    // MARK: - List

    @ViewBuilder
    private var projectsList: some View {
        if vm.isLoading {
            skeletonList
        } else if let error = vm.error {
            errorView(error)
        } else {
            LazyVStack(spacing: 14) {
                ForEach(vm.projects) { project in
                    ProjectCard(project: project)
                        .onAppear {
                            if project.id == vm.projects.last?.id {
                                Task { await vm.loadMore() }
                            }
                        }
                }
                if vm.isLoadingMore {
                    ProgressView()
                        .padding(.vertical, 12)
                }
            }
        }
    }

    private var skeletonList: some View {
        LazyVStack(spacing: 14) {
            ForEach(0..<5, id: \.self) { _ in
                ProjectCardSkeleton()
            }
        }
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
            Button("Retry") { Task { await vm.loadInitial() } }
                .buttonStyle(.bordered)
        }
        .padding(.top, 40)
    }
}

// MARK: - Project Card

struct ProjectCard: View {
    let project: ProjectItem
    @Environment(LanguageManager.self) var lm
    @State private var showDetail = false

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            if let badge = reviewBadge {
                badge
            }
            header
            if let summary = project.data.summary?.short {
                Text(summary)
                    .font(.system(size: 13))
                    .foregroundStyle(.secondary)
                    .lineLimit(4...)
            }
            if let chips = project.data.ui?.requirementChips, !chips.isEmpty {
                chipRow(chips)
            }
            footer
            showMoreButton
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
        .contentShape(RoundedRectangle(cornerRadius: 18, style: .continuous))
        .onTapGesture { showDetail = true }
        .sheet(isPresented: $showDetail) {
            ProjectDetailView(project: project, source: "projects")
                .environment(lm)
        }
    }

    private var showMoreButton: some View {
        HStack {
            Button { showDetail = true } label: {
                HStack(spacing: 4) {
                    Text(lm.t(.detailShowMore))
                        .font(.system(size: 12, weight: .medium))
                    Image(systemName: "chevron.right")
                        .font(.system(size: 10, weight: .semibold))
                }
                .foregroundStyle(Color.accentColor)
            }
            .buttonStyle(.plain)
            Spacer()
            if let date = project.relativeDate(for: lm.appLanguage) {
                Text(date)
                    .font(.system(size: 11))
                    .foregroundStyle(.tertiary)
            }
        }
    }

    // MARK: Review badge

    @ViewBuilder
    private var reviewBadge: (some View)? {
        if let stats = project.recruiterRodeoStats {
            let rating = stats.overallRating
            let (label, icon, color): (String, String, Color) = {
                if rating >= 3.5 { return (lm.t(.reviewGood), "hand.thumbsup.fill", .green) }
                if rating >= 2.5 { return (lm.t(.reviewAcceptable), "hand.point.up.fill", .orange) }
                return (lm.t(.reviewBad), "hand.thumbsdown.fill", .red)
            }()

            HStack(spacing: 6) {
                Image(systemName: icon)
                    .font(.system(size: 10, weight: .semibold))
                Text(label)
                    .font(.system(size: 11, weight: .semibold))
                if stats.reviewCount > 0 {
                    Text("·")
                        .foregroundStyle(color.opacity(0.5))
                    Text("\(stats.reviewCount) \(lm.t(.reviewCount))")
                        .font(.system(size: 10, weight: .medium))
                        .foregroundStyle(color.opacity(0.7))
                }
            }
            .foregroundStyle(color)
            .padding(.horizontal, 10)
            .padding(.vertical, 5)
            .background(
                Capsule().fill(color.opacity(0.1))
            )
        }
    }

    // MARK: Header

    private var header: some View {
        HStack(alignment: .top, spacing: 12) {
            companyLogo
            VStack(alignment: .leading, spacing: 4) {
                Text(project.displayTitle)
                    .font(.system(size: 15, weight: .semibold, design: .rounded))
                    .lineLimit(2)
                    .frame(maxWidth: .infinity, alignment: .leading)
                companyLocationLine
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }

    @ViewBuilder
    private var companyLocationLine: some View {
        VStack(alignment: .leading, spacing: 3) {
            companyLabel
            if let loc = project.displayLocation {
                locationLabel(loc)
            }
        }
    }

    private var companyLabel: some View {
        Label {
            Text(project.displayCompany)
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(.secondary)
        } icon: {
            Image(systemName: "building.2")
                .font(.system(size: 10, weight: .medium))
                .foregroundStyle(.tertiary)
        }
    }

    private func locationLabel(_ loc: String) -> some View {
        Label {
            Text(loc)
                .font(.system(size: 12))
                .foregroundStyle(.secondary)
        } icon: {
            Image(systemName: "mappin.and.ellipse")
                .font(.system(size: 10, weight: .medium))
                .foregroundStyle(.tertiary)
        }
    }

    // MARK: Company logo + rating column

    private let logoSize: CGFloat = 53

    private var companyLogo: some View {
        VStack(alignment: .center, spacing: 6) {
            if let stats = project.recruiterRodeoStats {
                starRating(for: stats.overallRating)
            }

            Group {
                if let url = project.resolvedLogoURL {
                    AsyncImage(url: url) { phase in
                        switch phase {
                        case .success(let img): img.resizable().scaledToFit()
                        default: logoPlaceholder
                        }
                    }
                } else {
                    logoPlaceholder
                }
            }
            .frame(width: logoSize, height: logoSize)
            .clipShape(RoundedRectangle(cornerRadius: 12, style: .continuous))
        }
        .frame(width: logoSize)
    }

    private func starRating(for rating: Double) -> some View {
        let count = rating >= 3.5 ? 3 : rating >= 2.5 ? 2 : 1
        let color = ratingColor(rating)
        return HStack(spacing: 2) {
            ForEach(0..<3, id: \.self) { i in
                Image(systemName: i < count ? "star.fill" : "star")
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(i < count ? AnyShapeStyle(color) : AnyShapeStyle(.quaternary))
            }
        }
    }

    private var logoPlaceholder: some View {
        RoundedRectangle(cornerRadius: 12, style: .continuous)
            .fill(.quaternary)
            .overlay {
                Text(project.displayCompany.prefix(1).uppercased())
                    .font(.system(size: 20, weight: .semibold, design: .rounded))
                    .foregroundStyle(.secondary)
            }
    }

    private func ratingColor(_ rating: Double) -> Color {
        switch rating {
        case ..<2.5:  return .red
        case 2.5..<3.5: return .orange
        default:      return .green
        }
    }

    // MARK: Chip row

    private func chipRow(_ chips: [String]) -> some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 6) {
                ForEach(chips.prefix(6), id: \.self) { chip in
                    Text(chip)
                        .font(.system(size: 11, weight: .medium))
                        .padding(.horizontal, 10)
                        .padding(.vertical, 5)
                        .background(Capsule().fill(Color.accentColor.opacity(0.1)))
                        .foregroundStyle(Color.accentColor)
                }
            }
        }
    }

    // MARK: Footer

    private var footer: some View {
        HStack {
            if let rate = project.displayRate {
                Label(rate, systemImage: "eurosign.circle")
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(.secondary)
            }
            if let badges = project.data.ui?.badges, let first = badges.first {
                Text(first.label ?? "")
                    .font(.system(size: 11, weight: .medium))
                    .padding(.horizontal, 10)
                    .padding(.vertical, 5)
                    .background(Capsule().fill(.quaternary))
                    .foregroundStyle(.secondary)
            }
        }
    }
}

// MARK: - Skeleton

struct ProjectCardSkeleton: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 12) {
                RoundedRectangle(cornerRadius: 10, style: .continuous)
                    .fill(.quaternary).frame(width: 46, height: 46)
                VStack(alignment: .leading, spacing: 5) {
                    RoundedRectangle(cornerRadius: 4)
                        .fill(.quaternary).frame(width: 170, height: 13)
                    RoundedRectangle(cornerRadius: 4)
                        .fill(.quaternary).frame(width: 110, height: 11)
                }
                Spacer()
            }
            VStack(alignment: .leading, spacing: 6) {
                RoundedRectangle(cornerRadius: 4)
                    .fill(.quaternary).frame(maxWidth: .infinity, minHeight: 11)
                RoundedRectangle(cornerRadius: 4)
                    .fill(.quaternary).frame(width: 220, height: 11)
            }
            HStack(spacing: 8) {
                ForEach(0..<3, id: \.self) { _ in
                    RoundedRectangle(cornerRadius: 20)
                        .fill(.quaternary).frame(width: 64, height: 24)
                }
            }
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
    }
}

