import SwiftUI

struct StatsView: View {
    @Environment(LanguageManager.self) var lm

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 16) {
                    summaryCards
                    matchHistorySection
                    topSkillsSection
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
                .padding(.bottom, 110)
            }
            .navigationTitle(lm.t(.tabStats))
            .safeAreaInset(edge: .bottom) { Color.clear.frame(height: 90) }
        }
    }

    // MARK: - Summary cards

    private var summaryCards: some View {
        HStack(spacing: 12) {
            StatCard(icon: "checkmark.seal.fill", color: .green,
                     label: lm.t(.statsMatchesTotal), value: "—")
            StatCard(icon: "chart.line.uptrend.xyaxis", color: .blue,
                     label: lm.t(.statsAvgScore), value: "—")
            StatCard(icon: "bell.fill", color: .orange,
                     label: lm.t(.statsAlertsTotal), value: "—")
        }
    }

    // MARK: - Match history

    private var matchHistorySection: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(lm.t(.statsMatchHistory))
                .font(.system(size: 15, weight: .semibold))

            ForEach(0..<4, id: \.self) { _ in
                MatchHistoryRow()
            }
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
    }

    // MARK: - Top skills

    private var topSkillsSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(lm.t(.statsTopSkills))
                .font(.system(size: 15, weight: .semibold))

            Text(lm.t(.statsTopSkillsEmpty))
                .font(.subheadline)
                .foregroundStyle(.tertiary)
                .frame(maxWidth: .infinity, alignment: .center)
                .padding(.vertical, 16)
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
    }
}

// MARK: - Sub-components

private struct StatCard: View {
    let icon: String
    let color: Color
    let label: String
    let value: String

    var body: some View {
        VStack(spacing: 8) {
            Image(systemName: icon)
                .font(.system(size: 22))
                .foregroundStyle(color)
            Text(value)
                .font(.system(size: 20, weight: .bold, design: .rounded))
            Text(label)
                .font(.system(size: 10, weight: .medium))
                .foregroundStyle(.secondary)
                .multilineTextAlignment(.center)
        }
        .frame(maxWidth: .infinity)
        .padding(.vertical, 14)
        .background(
            RoundedRectangle(cornerRadius: 14, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 10, x: 0, y: 3)
        )
    }
}

private struct MatchHistoryRow: View {
    var body: some View {
        HStack(spacing: 12) {
            RoundedRectangle(cornerRadius: 8)
                .fill(.quaternary)
                .frame(width: 38, height: 38)

            VStack(alignment: .leading, spacing: 4) {
                RoundedRectangle(cornerRadius: 4).fill(.quaternary).frame(width: 140, height: 12)
                RoundedRectangle(cornerRadius: 4).fill(.quaternary).frame(width: 90, height: 10)
            }

            Spacer()

            RoundedRectangle(cornerRadius: 6).fill(.quaternary).frame(width: 36, height: 28)
        }
    }
}
