import SwiftUI

/// Sheet showing the LLM's full analysis for a match: summary + positive points,
/// concerns, and improvement tips. Opened from the "Match details" button on
/// each match card. Separate from ProjectDetailView (which shows the project itself).
struct MatchDetailView: View {
    let match: MatchResult
    @Environment(LanguageManager.self) var lm
    /// Drives the first-appear animation for the score ring AND the
    /// six breakdown bars. Starts at 0, animates to 1 via DispatchQueue
    /// so SwiftUI commits the initial frame at 0 before tweening —
    /// same pattern MatchesView uses for its list cards.
    @State private var barsProgress: Double = 0

    var body: some View {
        VStack(spacing: 0) {
            header
                .padding(.horizontal, 20)
                .padding(.top, 24)
                .padding(.bottom, 16)
                .background(Color(UIColor.systemBackground))

            Divider()

            ScrollView {
                VStack(alignment: .leading, spacing: 20) {
                    scoreBreakdownCard

                    let summary = match.summary(for: lm.appLanguage)
                    if !summary.isEmpty {
                        summaryCard(summary)
                    }

                    let positives = match.positivePoints(for: lm.appLanguage)
                    if !positives.isEmpty {
                        bulletSection(
                            title: lm.t(.matchesPositivePoints),
                            icon: "checkmark.seal.fill",
                            color: .green,
                            items: positives
                        )
                    }

                    let negatives = match.negativePoints(for: lm.appLanguage)
                    if !negatives.isEmpty {
                        bulletSection(
                            title: lm.t(.matchesNegativePoints),
                            icon: "exclamationmark.triangle.fill",
                            color: .orange,
                            items: negatives
                        )
                    }

                    let tips = match.improvementTips(for: lm.appLanguage)
                    if !tips.isEmpty {
                        bulletSection(
                            title: lm.t(.matchesImprovementTips),
                            icon: "lightbulb.fill",
                            color: .blue,
                            items: tips
                        )
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
        .onAppear {
            barsProgress = 0
            DispatchQueue.main.async {
                withAnimation(.easeOut(duration: 0.8)) {
                    barsProgress = 1
                }
            }
        }
        .task {
            RealtimeClient.shared.emitMatchViewed(
                projectID: match.projectId,
                cvID: match.cvId
            )
        }
    }

    // MARK: - Header

    private var header: some View {
        HStack(alignment: .center, spacing: 14) {
            scoreRingBadge(size: 64)

            VStack(alignment: .leading, spacing: 4) {
                Text(match.labelText(lm: lm))
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundStyle(match.labelColor)
                    .padding(.horizontal, 8)
                    .padding(.vertical, 3)
                    .background(Capsule().fill(match.labelColor.opacity(0.12)))
                Text(match.projectTitle)
                    .font(.system(size: 16, weight: .bold, design: .rounded))
                    .lineLimit(2)
                // Initials avatar + company name — mirrors the Live
                // Activity Lock Screen + minimal card treatment so the
                // surfaces feel consistent.
                HStack(spacing: 6) {
                    companyInitialsAvatar(name: match.companyName.nilIfEmpty ?? match.platform, size: 18)
                    Text(match.companyName.nilIfEmpty ?? match.platform)
                        .font(.system(size: 12))
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }
            Spacer(minLength: 0)
        }
    }

    // MARK: - Score ring + breakdown

    /// Same ring-around-the-number treatment the list uses so the detail
    /// header reads as a bigger version of the card the user just tapped.
    private func scoreRingBadge(size: CGFloat) -> some View {
        let lineWidth: CGFloat = max(2, size * 0.08)
        let progress = min(1.0, max(0, match.score / 10))
        return ZStack {
            Circle().fill(match.labelColor.opacity(0.12))
            Circle()
                .trim(from: 0, to: progress * barsProgress)
                .stroke(match.labelColor, style: StrokeStyle(lineWidth: lineWidth, lineCap: .round))
                .rotationEffect(.degrees(-90))
                .padding(lineWidth / 2)
                .animation(.easeOut(duration: 0.8), value: barsProgress)
            Text(String(format: "%.1f", match.score))
                .font(.system(size: size * 0.42, weight: .bold, design: .rounded))
                .foregroundStyle(match.labelColor)
        }
        .frame(width: size, height: size)
    }

    /// Six-dimension breakdown identical to the Full card's layout,
    /// wrapped in the same rounded card container as the other sections
    /// below. Bar widths multiply by `barsProgress` so they fill up on
    /// sheet appear.
    private var scoreBreakdownCard: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(lm.t(.matchesScoreBreakdown))
                .font(.system(size: 11, weight: .semibold))
                .foregroundStyle(.secondary)
                .textCase(.uppercase)
                .tracking(0.6)
            scoreRow(label: lm.t(.matchesScoreSkillsMatch),          value: match.scores.skillsMatch)
            scoreRow(label: lm.t(.matchesScoreSeniorityFit),         value: match.scores.seniorityFit)
            scoreRow(label: lm.t(.matchesScoreDomainExperience),     value: match.scores.domainExperience)
            scoreRow(label: lm.t(.matchesScoreCommunicationClarity), value: match.scores.communicationClarity)
            scoreRow(label: lm.t(.matchesScoreProjectRelevance),     value: match.scores.projectRelevance)
            scoreRow(label: lm.t(.matchesScoreTechStackOverlap),     value: match.scores.techStackOverlap)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.05), radius: 8, x: 0, y: 2)
        )
    }

    private func scoreRow(label: String, value: Double) -> some View {
        HStack(spacing: 8) {
            Text(label)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(.secondary)
                .frame(width: 110, alignment: .leading)
            GeometryReader { geo in
                ZStack(alignment: .leading) {
                    Capsule()
                        .fill(Color.secondary.opacity(0.12))
                        .frame(height: 6)
                    Capsule()
                        .fill(scoreColor(value))
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

    /// Color ramp for score dimensions — matches the scale used in the
    /// list so the bar colors don't "jump" between surfaces.
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

    /// Same initials-in-a-circle avatar the Live Activity and the
    /// minimal match card use — copy kept local to this view so the
    /// file stays drop-in without new shared-folder additions.
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

    // MARK: - Summary card

    private func summaryCard(_ text: String) -> some View {
        Text(text)
            .font(.system(size: 14))
            .foregroundStyle(.primary)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(16)
            .background(
                RoundedRectangle(cornerRadius: 16, style: .continuous)
                    .fill(.background)
                    .shadow(color: .black.opacity(0.05), radius: 8, x: 0, y: 2)
            )
    }

    // MARK: - Bullet section (positives / negatives / tips)

    private func bulletSection(title: String, icon: String, color: Color, items: [String]) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                Image(systemName: icon)
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(color)
                Text(title)
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(.secondary)
                    .textCase(.uppercase)
                    .tracking(0.6)
            }
            VStack(alignment: .leading, spacing: 10) {
                ForEach(Array(items.enumerated()), id: \.offset) { _, item in
                    HStack(alignment: .top, spacing: 10) {
                        Circle()
                            .fill(color)
                            .frame(width: 6, height: 6)
                            .padding(.top, 6)
                        Text(item)
                            .font(.system(size: 13))
                            .foregroundStyle(.primary)
                            .fixedSize(horizontal: false, vertical: true)
                        Spacer(minLength: 0)
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            }
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
