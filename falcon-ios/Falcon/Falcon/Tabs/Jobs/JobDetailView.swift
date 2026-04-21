import SwiftUI

struct JobDetailView: View {
    let project: ProjectItem
    @Environment(LanguageManager.self) var lm

    var body: some View {
        VStack(spacing: 0) {
            // Fixed zone: header + contact CTAs (call/email)
            VStack(alignment: .leading, spacing: 16) {
                header
                contactCTAs
            }
            .padding(.horizontal, 20)
            .padding(.top, 24)
            .padding(.bottom, 20)
            .background(Color(UIColor.systemBackground))

            Divider()

            // Scrollable zone
            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    if let facts = project.data.ui?.heroFacts, !facts.isEmpty {
                        heroFactsSection(facts)
                    }
                    if let highlights = project.data.summary?.highlights, !highlights.isEmpty {
                        highlightsSection(highlights)
                    }
                    if let must = project.data.requirements?.mustHave, !must.isEmpty {
                        requirementsSection(lm.t(.detailMustHave), items: must, accent: .red)
                    }
                    if let should = project.data.requirements?.shouldHave, !should.isEmpty {
                        requirementsSection(lm.t(.detailShouldHave), items: should, accent: .orange)
                    }
                    if let nice = project.data.requirements?.niceToHave, !nice.isEmpty {
                        requirementsSection(lm.t(.detailNiceToHave), items: nice, accent: .blue)
                    }
                    if let tasks = project.data.responsibilities, !tasks.isEmpty {
                        responsibilitiesSection(tasks)
                    }
                    if let contact = project.data.contact {
                        contactSection(contact)
                    }
                    if let urlString = project.data.source?.url?.nilIfEmpty,
                       let url = URL(string: urlString) {
                        viewOriginalSection(url: url)
                    }
                }
                .padding(.horizontal, 20)
                .padding(.top, 20)
                .padding(.bottom, 48)
            }
            .background(Color(UIColor.systemGroupedBackground))
        }
        .presentationDragIndicator(.visible)
        .presentationCornerRadius(22)
    }

    @ViewBuilder
    private var contactCTAs: some View {
        let phone = project.data.contact?.phone?.nilIfEmpty
        let email = project.data.contact?.email?.nilIfEmpty
        let phoneURL = phone.flatMap { URL(string: "tel:\($0)") }
        let emailURL = email.flatMap { URL(string: "mailto:\($0)") }

        if let phoneURL, let emailURL {
            HStack(spacing: 10) {
                ctaButton(icon: "phone.fill", label: lm.t(.detailCallCTA), url: phoneURL)
                ctaIconButton(icon: "envelope.fill", url: emailURL)
            }
        } else if let phoneURL {
            ctaButton(icon: "phone.fill", label: lm.t(.detailCallCTA), url: phoneURL)
        } else if let emailURL {
            ctaButton(icon: "envelope.fill", label: lm.t(.detailEmailCTA), url: emailURL)
        }
    }

    private func ctaButton(icon: String, label: String, url: URL) -> some View {
        Link(destination: url) {
            HStack(spacing: 8) {
                Image(systemName: icon)
                    .font(.system(size: 14, weight: .semibold))
                Text(label)
                    .font(.system(size: 15, weight: .semibold))
                    .lineLimit(1)
                    .minimumScaleFactor(0.8)
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 13)
            .background(RoundedRectangle(cornerRadius: 12, style: .continuous).fill(Color.accentColor))
            .foregroundStyle(.white)
        }
    }

    private func ctaIconButton(icon: String, url: URL) -> some View {
        Link(destination: url) {
            Image(systemName: icon)
                .font(.system(size: 16, weight: .semibold))
                .frame(width: 52, height: 46)
                .background(RoundedRectangle(cornerRadius: 12, style: .continuous).fill(Color.accentColor))
                .foregroundStyle(.white)
        }
    }

    // MARK: - Header

    private var header: some View {
        HStack(alignment: .top, spacing: 14) {
            if let url = project.resolvedLogoURL {
                AsyncImage(url: url) { phase in
                    switch phase {
                    case .success(let img): img.resizable().scaledToFit()
                    default: logoPlaceholder
                    }
                }
                .frame(width: 60, height: 60)
                .clipShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
            } else {
                logoPlaceholder
                    .frame(width: 60, height: 60)
            }

            VStack(alignment: .leading, spacing: 5) {
                Text(project.displayTitle)
                    .font(.system(size: 18, weight: .bold, design: .rounded))
                    .fixedSize(horizontal: false, vertical: true)
                Label(project.displayCompany, systemImage: "building.2")
                    .font(.system(size: 13, weight: .medium))
                    .foregroundStyle(.secondary)
                if let loc = project.displayLocation {
                    Label(loc, systemImage: "mappin.and.ellipse")
                        .font(.system(size: 13))
                        .foregroundStyle(.secondary)
                }
                if let stats = project.recruiterRodeoStats {
                    recruiterStatsRow(stats)
                }
            }
            Spacer()
        }
    }

    private func recruiterStatsRow(_ stats: RecruiterRodeoStats) -> some View {
        HStack(spacing: 10) {
            // Star rating
            HStack(spacing: 2) {
                ForEach(0..<3, id: \.self) { i in
                    let filled = stats.overallRating >= Double(i + 1) * (5.0 / 3.0) - 0.5
                    Image(systemName: filled ? "star.fill" : "star")
                        .font(.system(size: 10, weight: .medium))
                        .foregroundStyle(filled ? ratingColor(stats.overallRating) : AnyShapeStyle(.quaternary))
                }
                Text(String(format: "%.1f", stats.overallRating))
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(ratingColor(stats.overallRating))
            }

            Text("·")
                .font(.system(size: 11))
                .foregroundStyle(.tertiary)

            // Recommendation rate
            HStack(spacing: 3) {
                Image(systemName: "hand.thumbsup.fill")
                    .font(.system(size: 10))
                    .foregroundStyle(.green)
                Text(stats.recommendationRate)
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(.secondary)
            }

            Text("·")
                .font(.system(size: 11))
                .foregroundStyle(.tertiary)

            // Review count
            HStack(spacing: 3) {
                Image(systemName: "person.2.fill")
                    .font(.system(size: 10))
                    .foregroundStyle(.secondary)
                Text("\(stats.reviewCount)")
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.top, 2)
    }

    private func ratingColor(_ rating: Double) -> AnyShapeStyle {
        switch rating {
        case ..<2.5:  return AnyShapeStyle(.red)
        case 2.5..<3.5: return AnyShapeStyle(.orange)
        default:      return AnyShapeStyle(.green)
        }
    }

    private var logoPlaceholder: some View {
        RoundedRectangle(cornerRadius: 14, style: .continuous)
            .fill(.quaternary)
            .overlay {
                Text(project.displayCompany.prefix(1).uppercased())
                    .font(.system(size: 24, weight: .semibold, design: .rounded))
                    .foregroundStyle(.secondary)
            }
    }

    // MARK: - Hero facts grid

    private func heroFactsSection(_ facts: [HeroFact]) -> some View {
        LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible())], spacing: 12) {
            ForEach(Array(facts.prefix(6).enumerated()), id: \.offset) { _, fact in
                VStack(alignment: .leading, spacing: 3) {
                    Text(fact.label ?? "")
                        .font(.system(size: 10, weight: .medium))
                        .foregroundStyle(.tertiary)
                        .textCase(.uppercase)
                    Text(fact.value ?? "—")
                        .font(.system(size: 14, weight: .semibold, design: .rounded))
                        .foregroundStyle(.primary)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(12)
                .background(
                    RoundedRectangle(cornerRadius: 12, style: .continuous)
                        .fill(.background)
                        .shadow(color: .black.opacity(0.05), radius: 6, x: 0, y: 2)
                )
            }
        }
    }

    // MARK: - Highlights

    private func highlightsSection(_ highlights: [String]) -> some View {
        sectionBox(title: lm.t(.detailHighlights)) {
            VStack(alignment: .leading, spacing: 10) {
                ForEach(highlights, id: \.self) { h in
                    HStack(alignment: .top, spacing: 10) {
                        Image(systemName: "checkmark.circle.fill")
                            .font(.system(size: 14))
                            .foregroundStyle(.green)
                        Text(h)
                            .font(.system(size: 13))
                            .foregroundStyle(.primary)
                            .fixedSize(horizontal: false, vertical: true)
                        Spacer(minLength: 0)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
    }

    // MARK: - Requirements

    private func requirementsSection(_ title: String, items: [RequirementItem], accent: Color) -> some View {
        sectionBox(title: title) {
            VStack(alignment: .leading, spacing: 12) {
                ForEach(Array(items.enumerated()), id: \.offset) { _, item in
                    VStack(alignment: .leading, spacing: 4) {
                        HStack(spacing: 6) {
                            Circle().fill(accent).frame(width: 7, height: 7)
                            Text(item.name ?? "")
                                .font(.system(size: 13, weight: .semibold))
                            if let years = item.minYears {
                                Text("\(years)+ yrs")
                                    .font(.system(size: 11, weight: .medium))
                                    .padding(.horizontal, 7)
                                    .padding(.vertical, 2)
                                    .background(Capsule().fill(accent.opacity(0.12)))
                                    .foregroundStyle(accent)
                            }
                            Spacer(minLength: 0)
                        }
                        if let tools = item.relatedTools, !tools.isEmpty {
                            Text(tools.joined(separator: " · "))
                                .font(.system(size: 11))
                                .foregroundStyle(.secondary)
                                .padding(.leading, 13)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
    }

    // MARK: - Responsibilities

    private func responsibilitiesSection(_ tasks: [String]) -> some View {
        sectionBox(title: lm.t(.detailResponsibilities)) {
            VStack(alignment: .leading, spacing: 10) {
                ForEach(Array(tasks.enumerated()), id: \.offset) { i, task in
                    HStack(alignment: .top, spacing: 10) {
                        Text("\(i + 1)")
                            .font(.system(size: 11, weight: .bold, design: .rounded))
                            .foregroundStyle(.white)
                            .frame(width: 20, height: 20)
                            .background(Circle().fill(.secondary))
                        Text(task)
                            .font(.system(size: 13))
                            .foregroundStyle(.primary)
                            .fixedSize(horizontal: false, vertical: true)
                        Spacer(minLength: 0)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
        }
    }

    // MARK: - Contact

    @ViewBuilder
    private func contactSection(_ contact: ProjectContact) -> some View {
        let hasContent = [contact.name, contact.company, contact.email, contact.phone, contact.image]
            .compactMap { $0?.nilIfEmpty }.count > 0
        if hasContent {
            sectionBox(title: lm.t(.detailContact)) {
                VStack(alignment: .leading, spacing: 12) {
                    HStack(alignment: .top, spacing: 12) {
                        if contact.name?.nilIfEmpty != nil {
                            recruiterAvatar(contact)
                        }
                        VStack(alignment: .leading, spacing: 8) {
                            if let name = contact.name?.nilIfEmpty {
                                contactRow(icon: "person", text: name, primary: true)
                            }
                            if let company = contact.company?.nilIfEmpty {
                                contactRow(icon: "building.2", text: company)
                            }
                            if let email = contact.email?.nilIfEmpty {
                                contactRow(icon: "envelope", text: email)
                            }
                        }
                    }

                    if let phone = contact.phone?.nilIfEmpty,
                       let url = URL(string: "tel:\(phone)") {
                        contactActionButton(
                            icon: "phone.fill",
                            label: lm.t(.detailCallContact),
                            url: url
                        )
                    }
                    if let email = contact.email?.nilIfEmpty,
                       let url = URL(string: "mailto:\(email)") {
                        contactActionButton(
                            icon: "envelope.fill",
                            label: lm.t(.detailEmailContact),
                            url: url
                        )
                    }
                }
            }
        }
    }

    private func contactRow(icon: String, text: String, primary: Bool = false) -> some View {
        HStack(spacing: 8) {
            Image(systemName: icon)
                .font(.system(size: 12))
                .foregroundStyle(.tertiary)
                .frame(width: 16)
            Text(text)
                .font(.system(size: 13, weight: primary ? .medium : .regular))
                .foregroundStyle(primary ? .primary : .secondary)
            Spacer(minLength: 0)
        }
    }

    private func contactActionButton(icon: String, label: String, url: URL) -> some View {
        Link(destination: url) {
            HStack(spacing: 8) {
                Image(systemName: icon)
                    .font(.system(size: 14, weight: .semibold))
                Text(label)
                    .font(.system(size: 15, weight: .semibold))
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 13)
            .background(RoundedRectangle(cornerRadius: 12, style: .continuous).fill(Color.accentColor))
            .foregroundStyle(.white)
        }
        .padding(.top, 4)
    }

    @ViewBuilder
    private func viewOriginalSection(url: URL) -> some View {
        sectionBox(title: lm.t(.detailViewOriginal)) {
            Link(destination: url) {
                HStack(spacing: 8) {
                    Image(systemName: "safari")
                        .font(.system(size: 14, weight: .semibold))
                    Text(lm.t(.detailViewOriginal))
                        .font(.system(size: 15, weight: .semibold))
                }
                .frame(maxWidth: .infinity)
                .padding(.vertical, 12)
                .background(
                    RoundedRectangle(cornerRadius: 12, style: .continuous)
                        .stroke(Color.accentColor, lineWidth: 1.5)
                )
                .foregroundStyle(Color.accentColor)
            }
        }
    }

    @ViewBuilder
    private func recruiterAvatar(_ contact: ProjectContact) -> some View {
        let initials = (contact.name?.prefix(1).uppercased())
            ?? (contact.company?.prefix(1).uppercased())
            ?? "·"
        ZStack {
            if let urlString = contact.image?.nilIfEmpty,
               let url = URL(string: urlString) {
                AsyncImage(url: url) { phase in
                    switch phase {
                    case .success(let img): img.resizable().scaledToFill()
                    default: avatarPlaceholder(initials: initials)
                    }
                }
            } else {
                avatarPlaceholder(initials: initials)
            }
        }
        .frame(width: 56, height: 56)
        .clipShape(Circle())
    }

    private func avatarPlaceholder(initials: String) -> some View {
        Circle()
            .fill(.quaternary)
            .overlay {
                Text(initials)
                    .font(.system(size: 20, weight: .semibold, design: .rounded))
                    .foregroundStyle(.secondary)
            }
    }

    // MARK: - Section box helper

    private func sectionBox<Content: View>(title: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(title)
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(.secondary)
                .textCase(.uppercase)
                .tracking(0.6)
            content()
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.05), radius: 8, x: 0, y: 2)
        )
    }
}
