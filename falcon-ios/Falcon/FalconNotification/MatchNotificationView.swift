import SwiftUI

struct MatchNotificationView: View {
    let data: MatchData

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            scoreHeader
            Divider()
            dimensionScores
            if !data.matchedSkills.isEmpty || !data.missingSkills.isEmpty {
                Divider()
                skillsRow
            }
        }
        .padding(16)
        .background(Color(.systemBackground))
    }

    // MARK: - Score header

    private var scoreHeader: some View {
        HStack(alignment: .firstTextBaseline, spacing: 6) {
            Text(String(format: "%.1f", data.score))
                .font(.system(size: 48, weight: .bold, design: .rounded))
                .foregroundStyle(scoreColor)
            Text("/ 10")
                .font(.title3)
                .foregroundStyle(.secondary)
            Spacer()
            Text(labelText)
                .font(.caption)
                .fontWeight(.semibold)
                .padding(.horizontal, 8)
                .padding(.vertical, 4)
                .background(scoreColor.opacity(0.15))
                .foregroundStyle(scoreColor)
                .clipShape(Capsule())
        }
    }

    // MARK: - Dimension scores

    private var dimensionScores: some View {
        VStack(spacing: 6) {
            ScoreRow(label: "Skills Match",    value: data.skillsMatch)
            ScoreRow(label: "Tech Stack",      value: data.techStackOverlap)
            ScoreRow(label: "Seniority",       value: data.seniorityFit)
            ScoreRow(label: "Projekt Relevanz",value: data.projectRelevance)
            ScoreRow(label: "Domain",          value: data.domainExperience)
            ScoreRow(label: "Kommunikation",   value: data.communicationClarity)
        }
    }

    // MARK: - Skills

    private var skillsRow: some View {
        VStack(alignment: .leading, spacing: 6) {
            if !data.matchedSkills.isEmpty {
                skillBadges(skills: data.matchedSkills, color: .green, symbol: "checkmark")
            }
            if !data.missingSkills.isEmpty {
                skillBadges(skills: data.missingSkills, color: .red, symbol: "xmark")
            }
        }
    }

    private func skillBadges(skills: [String], color: Color, symbol: String) -> some View {
        HStack(spacing: 6) {
            Image(systemName: symbol)
                .font(.caption2)
                .foregroundStyle(color)
            FlowRow(items: skills) { skill in
                Text(skill)
                    .font(.caption2)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(color.opacity(0.1))
                    .foregroundStyle(color)
                    .clipShape(Capsule())
            }
        }
    }

    // MARK: - Helpers

    private var scoreColor: Color {
        switch data.score {
        case 8.5...: return .green
        case 7.0...: return .blue
        case 5.0...: return .orange
        default:     return .red
        }
    }

    private var labelText: String {
        switch data.label {
        case "apply_immediately": return "Sofort bewerben"
        case "top_candidate":    return "Top Kandidat"
        case "acceptable":       return "Akzeptabel"
        default:                 return ""
        }
    }
}

// MARK: - ScoreRow

struct ScoreRow: View {
    let label: String
    let value: Double

    var body: some View {
        HStack(spacing: 8) {
            Text(label)
                .font(.caption)
                .foregroundStyle(.secondary)
                .frame(width: 110, alignment: .leading)
            GeometryReader { geo in
                ZStack(alignment: .leading) {
                    Capsule().fill(Color(.systemFill))
                    Capsule()
                        .fill(barColor)
                        .frame(width: geo.size.width * value / 10)
                }
            }
            .frame(height: 6)
            Text(String(format: "%.1f", value))
                .font(.caption)
                .fontWeight(.medium)
                .frame(width: 28, alignment: .trailing)
        }
    }

    private var barColor: Color {
        switch value {
        case 8.5...: return .green
        case 7.0...: return .blue
        case 5.0...: return .orange
        default:     return .red
        }
    }
}

// MARK: - FlowRow (wrapping tag layout)

struct FlowRow<Item: Hashable, Content: View>: View {
    let items: [Item]
    let content: (Item) -> Content

    var body: some View {
        // Simple horizontal scroll for notification — full flow layout needs iOS 16 Layout protocol
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 4) {
                ForEach(items, id: \.self) { content($0) }
            }
        }
    }
}

#Preview {
    MatchNotificationView(data: .preview)
        .frame(width: 375)
}
