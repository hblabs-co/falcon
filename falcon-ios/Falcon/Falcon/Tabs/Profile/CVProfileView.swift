import SwiftUI

// MARK: - Root CV profile card (shown once normalized)

struct CVProfileView: View {
    let cv: NormalizedCVData.Lang
    let onReplace: () -> Void
    @Environment(LanguageManager.self) var lm
    @State private var selectedEntry: NormalizedCVData.ExperienceEntry? = nil

    var body: some View {
        VStack(spacing: 20) {
            headerCard
            if !cv.experience.isEmpty {
                experienceSection
            }
            technologiesSection
            replaceButton
        }
        .sheet(item: $selectedEntry) { entry in
            ExperienceDetailSheet(entry: entry)
                .presentationDetents([.medium, .large])
                .presentationCornerRadius(22)
                .presentationDragIndicator(.visible)
        }
    }

    // MARK: - Header

    private var headerCard: some View {
        VStack(spacing: 16) {
            // Initials avatar
            ZStack {
                Circle()
                    .fill(Color.accentColor.opacity(0.12))
                    .frame(width: 72, height: 72)
                Text(cv.initials)
                    .font(.system(size: 26, weight: .semibold, design: .rounded))
                    .foregroundStyle(Color.accentColor)
            }

            if let name = cv.fullName {
                Text(name)
                    .font(.system(size: 20, weight: .semibold, design: .rounded))
                    .multilineTextAlignment(.center)
            }

            if let summary = cv.summary?.nilIfEmpty {
                Text(summary)
                    .font(.system(size: 14))
                    .foregroundStyle(.secondary)
                    .multilineTextAlignment(.center)
                    .fixedSize(horizontal: false, vertical: true)
                    .padding(.horizontal, 4)
            }
        }
        .frame(maxWidth: .infinity)
        .padding(20)
        .background(cardBackground)
    }

    // MARK: - Experience

    private var experienceSection: some View {
        VStack(alignment: .leading, spacing: 0) {
            sectionHeader(title: lm.t(.profileCVSectionExperience), icon: "briefcase.fill")
                .padding(.bottom, 16)

            VStack(spacing: 1) {
                ForEach(Array(cv.experience.enumerated()), id: \.element.id) { index, entry in
                    experienceEntry(entry, isLast: index == cv.experience.count - 1)
                }
            }
        }
        .padding(16)
        .background(cardBackground)
    }

    private func experienceEntry(_ entry: NormalizedCVData.ExperienceEntry, isLast: Bool) -> some View {
        Button { selectedEntry = entry } label: {
            HStack(alignment: .top, spacing: 12) {
                // Timeline indicator
                VStack(spacing: 0) {
                    Circle()
                        .fill(Color.accentColor)
                        .frame(width: 9, height: 9)
                        .padding(.top, 5)
                    if !isLast {
                        Rectangle()
                            .fill(Color.accentColor.opacity(0.2))
                            .frame(width: 1.5)
                            .frame(maxHeight: .infinity)
                    }
                }
                .frame(width: 9)

                // Summary row
                HStack(alignment: .top) {
                    VStack(alignment: .leading, spacing: 4) {
                        if let role = entry.role?.nilIfEmpty {
                            Text(role)
                                .font(.system(size: 14, weight: .semibold))
                                .foregroundStyle(.primary)
                        }
                        HStack(spacing: 4) {
                            if let company = entry.company?.nilIfEmpty {
                                Text(company)
                                    .font(.system(size: 12, weight: .medium))
                                    .foregroundStyle(.secondary)
                            }
                            if let duration = entry.duration?.nilIfEmpty {
                                Text("·")
                                    .foregroundStyle(.tertiary)
                                Text(duration)
                                    .font(.system(size: 12))
                                    .foregroundStyle(.tertiary)
                            }
                        }
                        if let desc = entry.shortDescription?.nilIfEmpty {
                            Text(desc)
                                .font(.system(size: 12))
                                .foregroundStyle(.secondary)
                                .lineLimit(2)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                    }
                    Spacer()
                    Image(systemName: "chevron.right")
                        .font(.system(size: 11, weight: .medium))
                        .foregroundStyle(.tertiary)
                        .padding(.top, 4)
                }
                .padding(.bottom, isLast ? 0 : 16)
            }
        }
        .buttonStyle(.plain)
    }

    // MARK: - Technologies

    private var technologiesSection: some View {
        VStack(alignment: .leading, spacing: 14) {
            sectionHeader(title: lm.t(.profileCVSectionTechnologies), icon: "cpu.fill")

            let tech = cv.technologies
            let categories: [(String, [String], Color)] = [
                ("Frontend",              tech.frontend,  .blue),
                ("Backend",               tech.backend,   .purple),
                ("Databases",             tech.databases,  .orange),
                ("DevOps",                tech.devops,    .green),
                ("Tools",                 tech.tools,     .pink),
                (lm.t(.profileCVSectionOthers), tech.others, .gray),
            ].filter { !$0.1.isEmpty }

            if categories.isEmpty {
                Text("—")
                    .font(.subheadline)
                    .foregroundStyle(.tertiary)
                    .frame(maxWidth: .infinity, alignment: .center)
                    .padding(.vertical, 8)
            } else {
                VStack(alignment: .leading, spacing: 12) {
                    ForEach(categories, id: \.0) { label, items, color in
                        VStack(alignment: .leading, spacing: 6) {
                            Text(label)
                                .font(.system(size: 11, weight: .semibold))
                                .foregroundStyle(.tertiary)
                                .textCase(.uppercase)
                                .tracking(0.5)
                            chipFlow(items: items, color: color)
                        }
                    }
                }
            }
        }
        .padding(16)
        .background(cardBackground)
    }

    // MARK: - Replace button

    private var replaceButton: some View {
        Button(action: onReplace) {
            Text(lm.t(.profileCVReplace))
                .font(.system(size: 14, weight: .medium))
                .foregroundStyle(.secondary)
                .frame(maxWidth: .infinity)
                .padding(.vertical, 14)
                .background(
                    RoundedRectangle(cornerRadius: 14, style: .continuous)
                        .strokeBorder(Color.secondary.opacity(0.25), lineWidth: 1)
                )
        }
        .buttonStyle(.plain)
    }

    // MARK: - Helpers

    private func sectionHeader(title: String, icon: String) -> some View {
        Label(title, systemImage: icon)
            .font(.system(size: 15, weight: .semibold))
    }

    private func chipFlow(items: [String], color: Color = .accentColor) -> some View {
        FlowLayout(spacing: 6) {
            ForEach(items, id: \.self) { item in
                chip(item, color: color)
            }
        }
    }

    private func chip(_ text: String, color: Color = .accentColor) -> some View {
        Text(text)
            .font(.system(size: 12, weight: .medium))
            .fixedSize()
            .padding(.horizontal, 10)
            .padding(.vertical, 5)
            .background(
                Capsule()
                    .fill(color.opacity(0.09))
            )
            .foregroundStyle(color)
    }

    private var cardBackground: some View {
        RoundedRectangle(cornerRadius: 18, style: .continuous)
            .fill(.background)
            .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
    }
}

// MARK: - Experience Detail Sheet

private struct ExperienceDetailSheet: View {
    let entry: NormalizedCVData.ExperienceEntry
    @State private var descriptionExpanded = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                // Header
                VStack(alignment: .leading, spacing: 6) {
                    if let role = entry.role?.nilIfEmpty {
                        Text(role)
                            .font(.system(size: 20, weight: .semibold, design: .rounded))
                    }
                    HStack(spacing: 6) {
                        if let company = entry.company?.nilIfEmpty {
                            Text(company)
                                .font(.system(size: 14, weight: .medium))
                                .foregroundStyle(.secondary)
                        }
                        if let duration = entry.duration?.nilIfEmpty {
                            Text("·")
                                .foregroundStyle(.tertiary)
                            Text(duration)
                                .font(.system(size: 14))
                                .foregroundStyle(.tertiary)
                        }
                    }
                    let period = [entry.start, entry.end].compactMap { $0?.nilIfEmpty }.joined(separator: " – ")
                    if !period.isEmpty {
                        Text(period)
                            .font(.system(size: 12))
                            .foregroundStyle(.tertiary)
                    }
                }

                // Highlights
                if !entry.highlights.isEmpty {
                    VStack(alignment: .leading, spacing: 8) {
                        ForEach(entry.highlights, id: \.self) { highlight in
                            HStack(alignment: .top, spacing: 10) {
                                Circle()
                                    .fill(.green)
                                    .frame(width: 7, height: 7)
                                    .padding(.top, 5)
                                Text(highlight)
                                    .font(.system(size: 14, weight: .medium))
                                    .foregroundStyle(.primary)
                                    .fixedSize(horizontal: false, vertical: true)
                            }
                        }
                    }
                    .padding(14)
                    .background(
                        RoundedRectangle(cornerRadius: 12, style: .continuous)
                            .fill(Color.green.opacity(0.06))
                    )
                }

                // Description
                if let desc = entry.longDescription?.nilIfEmpty {
                    VStack(alignment: .leading, spacing: 4) {
                        Text(desc)
                            .font(.system(size: 14))
                            .foregroundStyle(.secondary)
                            .lineLimit(descriptionExpanded ? nil : 2)
                            .fixedSize(horizontal: false, vertical: descriptionExpanded)

                        Button {
                            withAnimation(.easeInOut(duration: 0.25)) {
                                descriptionExpanded.toggle()
                            }
                        } label: {
                            Text(descriptionExpanded ? "Show less" : "Show more...")
                                .font(.system(size: 13, weight: .medium))
                                .foregroundStyle(Color.accentColor)
                        }
                        .buttonStyle(.plain)
                    }
                }

                // Tasks
                if !entry.tasks.isEmpty {
                    VStack(alignment: .leading, spacing: 10) {
                        Label("Tasks", systemImage: "list.bullet")
                            .font(.system(size: 13, weight: .semibold))
                            .foregroundStyle(.secondary)
                            .textCase(.uppercase)

                        VStack(alignment: .leading, spacing: 8) {
                            ForEach(entry.tasks, id: \.self) { task in
                                HStack(alignment: .top, spacing: 10) {
                                    Circle()
                                        .fill(Color.accentColor)
                                        .frame(width: 6, height: 6)
                                        .padding(.top, 5)
                                    Text(task)
                                        .font(.system(size: 14))
                                        .foregroundStyle(.primary)
                                        .fixedSize(horizontal: false, vertical: true)
                                }
                            }
                        }
                    }
                }

                // Technologies
                if !entry.technologies.isEmpty {
                    VStack(alignment: .leading, spacing: 10) {
                        Label("Technologies", systemImage: "cpu.fill")
                            .font(.system(size: 13, weight: .semibold))
                            .foregroundStyle(.secondary)
                            .textCase(.uppercase)

                        FlowLayout(spacing: 6) {
                            ForEach(entry.technologies, id: \.self) { tech in
                                Text(tech)
                                    .font(.system(size: 12, weight: .medium))
                                    .fixedSize()
                                    .padding(.horizontal, 10)
                                    .padding(.vertical, 5)
                                    .background(Capsule().fill(Color.accentColor.opacity(0.09)))
                                    .foregroundStyle(Color.accentColor)
                            }
                        }
                    }
                }
            }
            .padding(24)
        }
    }
}
