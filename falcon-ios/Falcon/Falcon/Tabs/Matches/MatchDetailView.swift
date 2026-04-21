import SwiftUI

/// Sheet showing the LLM's full analysis for a match: summary + positive points,
/// concerns, and improvement tips. Opened from the "Match details" button on
/// each match card. Separate from JobDetailView (which shows the project itself).
struct MatchDetailView: View {
    let match: MatchResult
    @Environment(LanguageManager.self) var lm

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
    }

    // MARK: - Header

    private var header: some View {
        HStack(alignment: .center, spacing: 14) {
            Text(String(format: "%.1f", match.score))
                .font(.system(size: 28, weight: .bold, design: .rounded))
                .foregroundStyle(match.labelColor)
                .frame(width: 64, height: 64)
                .background(Circle().fill(match.labelColor.opacity(0.12)))

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
                Text(match.companyName.nilIfEmpty ?? match.platform)
                    .font(.system(size: 12))
                    .foregroundStyle(.secondary)
            }
            Spacer(minLength: 0)
        }
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
