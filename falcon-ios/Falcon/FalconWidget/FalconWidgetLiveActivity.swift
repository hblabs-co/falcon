import ActivityKit
import WidgetKit
import SwiftUI

// MARK: - Reusable logo view (widget-safe, no UIKit dependency)

private struct AppLogo: View {
    var size: CGFloat
    /// Circle shape matches the Dynamic Island's rounded edges → cleaner visual
    /// on the compact leading/trailing slots. Rounded square is default for
    /// the Lock Screen where there's no such edge-hugging constraint.
    var circular: Bool = false

    var body: some View {
        Group {
            if let _ = UIImage(named: "FalconLogo") {
                Image("FalconLogo")
                    .resizable()
                    .scaledToFit()
                    .frame(width: size, height: size)
            } else {
                Rectangle()
                    .fill(LinearGradient(
                        colors: [Color.accentColor, Color.accentColor.opacity(0.7)],
                        startPoint: .topLeading, endPoint: .bottomTrailing
                    ))
                    .frame(width: size, height: size)
                    .overlay {
                        Image(systemName: "bird.fill")
                            .font(.system(size: size * 0.42, weight: .semibold))
                            .foregroundStyle(.white)
                    }
            }
        }
        .clipShape(circular
            ? AnyShape(Circle())
            : AnyShape(RoundedRectangle(cornerRadius: size * 0.225, style: .continuous)))
    }
}

// MARK: - Shared helpers

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

private func labelText(_ raw: String, lang: String) -> String {
    switch lang {
    case "de":
        switch raw {
        case "apply_immediately": return "Jetzt bewerben"
        case "top_candidate":     return "Top-Kandidat"
        case "acceptable":        return "Akzeptabel"
        case "not_suitable":      return "Ungeeignet"
        default:                  return "Neuer Treffer"
        }
    case "es":
        switch raw {
        case "apply_immediately": return "Aplica ya"
        case "top_candidate":     return "Top candidato"
        case "acceptable":        return "Aceptable"
        case "not_suitable":      return "No apto"
        default:                  return "Nueva coincidencia"
        }
    default: // en + fallback
        switch raw {
        case "apply_immediately": return "Apply now"
        case "top_candidate":     return "Top candidate"
        case "acceptable":        return "Acceptable"
        case "not_suitable":      return "Not suitable"
        default:                  return "New match"
        }
    }
}

private func ctaText(lang: String) -> String {
    switch lang {
    case "de": return "Treffer ansehen"
    case "es": return "Ver coincidencia"
    default:   return "View match"
    }
}

// Localized plural for the total match count in the Live Activity header.
private func totalMatchesWord(lang: String) -> String {
    switch lang {
    case "de": return "Treffer"
    case "es": return "coincidencias"
    default:   return "matches"
    }
}

private func scoreDimensionLabel(_ key: String, lang: String) -> String {
    // Short compact labels for the progress-bar rows.
    switch (key, lang) {
    case ("skills",        "de"): return "Fähigkeiten"
    case ("seniority",     "de"): return "Seniorität"
    case ("domain",        "de"): return "Branche"
    case ("communication", "de"): return "Kommunikation"
    case ("relevance",     "de"): return "Relevanz"
    case ("techstack",     "de"): return "Tech-Stack"

    case ("skills",        "es"): return "Habilidades"
    case ("seniority",     "es"): return "Seniority"
    case ("domain",        "es"): return "Sector"
    case ("communication", "es"): return "Comunicación"
    case ("relevance",     "es"): return "Relevancia"
    case ("techstack",     "es"): return "Tech stack"

    case ("skills",        _): return "Skills"
    case ("seniority",     _): return "Seniority"
    case ("domain",        _): return "Domain"
    case ("communication", _): return "Communication"
    case ("relevance",     _): return "Relevance"
    case ("techstack",     _): return "Tech stack"
    default:                   return key
    }
}

// MARK: - Company initials icon (fallback when no logo URL is available).

private struct CompanyInitialsIcon: View {
    var name: String
    var size: CGFloat

    private var initials: String {
        let words = name.split(separator: " ").prefix(2)
        return words.compactMap { $0.first }.map(String.init).joined().uppercased()
    }

    var body: some View {
        ZStack {
            Circle().fill(Color.accentColor.opacity(0.18))
            Text(initials)
                .font(.system(size: size * 0.48, weight: .bold, design: .rounded))
                .foregroundStyle(Color.accentColor)
        }
        .frame(width: size, height: size)
    }
}

// MARK: - Score badge — compact circular indicator

/// Score "gauge" shared by every surface: Lock Screen, Dynamic Island
/// expanded, compact leading, and minimal. The ring's arc length is
/// `score / 10` and its color tracks the score (red → orange → green).
/// `.animation(value: score)` means a push that changes the score from
/// e.g. 6.4 → 8.1 visually inflates the ring on the Lock Screen — iOS
/// runs the tween for us, matching the user's "sube o baja" mental model.
/// Same component scales for 20pt (minimal) up to 60pt (Lock Screen) by
/// keeping every internal dimension proportional to `size`.
private struct ScoreBadge: View {
    var score: Double
    var size: CGFloat

    private var progress: Double { min(1.0, max(0, score / 10)) }
    private var lineWidth: CGFloat { max(2, size * 0.08) }

    var body: some View {
        ZStack {
            Circle()
                .fill(scoreColor(score).opacity(0.18))
            Circle()
                .trim(from: 0, to: progress)
                .stroke(
                    scoreColor(score),
                    style: StrokeStyle(lineWidth: lineWidth, lineCap: .round)
                )
                .rotationEffect(.degrees(-90))
                .padding(lineWidth / 2)
                .animation(.easeOut(duration: 0.7), value: score)
            Text(String(format: "%.1f", score))
                .font(.system(size: size * 0.42, weight: .bold, design: .rounded))
                .foregroundStyle(scoreColor(score))
        }
        .frame(width: size, height: size)
    }
}

// MARK: - Live Activity widget

// MARK: - Deep link

private func matchDeepLink(_ state: FalconMatchAttributes.ContentState) -> URL? {
    guard !state.projectID.isEmpty, !state.cvID.isEmpty else { return nil }
    var c = URLComponents()
    c.scheme = "falcon"
    c.host = "match"
    c.queryItems = [
        URLQueryItem(name: "project_id", value: state.projectID),
        URLQueryItem(name: "cv_id", value: state.cvID)
    ]
    return c.url
}

struct FalconWidgetLiveActivity: Widget {
    var body: some WidgetConfiguration {
        ActivityConfiguration(for: FalconMatchAttributes.self) { context in
            lockScreenView(context)
                .widgetURL(matchDeepLink(context.state))
        } dynamicIsland: { context in
            DynamicIsland {
                // Expanded layout:
                //   row 1: "Falcon" (leading) ............. [Score] (trailing)
                //   row 2: [Logo] Project title
                //   row 3: summary (description)
                DynamicIslandExpandedRegion(.leading) {
                    VStack(alignment: .leading, spacing: 1) {
                        Text("Falcon")
                            .font(.system(size: 18, weight: .bold, design: .rounded))
                            .foregroundStyle(.white)
                        Text("\(context.state.totalMatches) \(totalMatchesWord(lang: context.state.lang))")
                            .font(.system(size: 10, weight: .medium))
                            .foregroundStyle(.white.opacity(0.6))
                            .contentTransition(.numericText())
                    }
                    .padding(.leading, 4)
                    .widgetURL(matchDeepLink(context.state))
                }
                DynamicIslandExpandedRegion(.trailing) {
                    ScoreBadge(score: context.state.score, size: 34)
                        .padding(.trailing, 4)
                        .widgetURL(matchDeepLink(context.state))
                }
                DynamicIslandExpandedRegion(.bottom) {
                    VStack(alignment: .leading, spacing: 6) {
                        HStack(alignment: .center, spacing: 10) {
                            AppLogo(size: 32)
                            VStack(alignment: .leading, spacing: 2) {
                                if !context.state.projectTitle.isEmpty {
                                    Text(context.state.projectTitle)
                                        .font(.system(size: 13, weight: .semibold))
                                        .foregroundStyle(.white)
                                        .lineLimit(1)
                                        .truncationMode(.tail)
                                }
                                HStack(spacing: 6) {
                                    if !context.state.companyName.isEmpty {
                                        HStack(spacing: 5) {
                                            CompanyInitialsIcon(name: context.state.companyName, size: 14)
                                            Text(context.state.companyName)
                                                .font(.system(size: 11, weight: .medium))
                                                .foregroundStyle(.white.opacity(0.7))
                                                .lineLimit(1)
                                                .truncationMode(.tail)
                                        }
                                    }
                                    Text(labelText(context.state.label, lang: context.state.lang))
                                        .font(.system(size: 9, weight: .semibold))
                                        .foregroundStyle(scoreColor(context.state.score))
                                        .padding(.horizontal, 6)
                                        .padding(.vertical, 2)
                                        .background(Capsule().fill(scoreColor(context.state.score).opacity(0.2)))
                                        .lineLimit(1)
                                }
                            }
                            Spacer(minLength: 0)
                        }
                        if !context.state.summary.isEmpty {
                            Text(context.state.summary)
                                .font(.system(size: 11))
                                .foregroundStyle(.white.opacity(0.75))
                                .lineLimit(2)
                                .fixedSize(horizontal: false, vertical: true)
                        }
                    }
                    .padding(.horizontal, 4)
                    .padding(.bottom, 2)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .widgetURL(matchDeepLink(context.state))
                }
            } compactLeading: {
                ScoreBadge(score: context.state.score, size: 22)
                    .widgetURL(matchDeepLink(context.state))
            } compactTrailing: {
                AppLogo(size: 20, circular: true)
                    .widgetURL(matchDeepLink(context.state))
            } minimal: {
                ScoreBadge(score: context.state.score, size: 20)
                    .widgetURL(matchDeepLink(context.state))
            }
            .keylineTint(scoreColor(context.state.score))
        }
    }

    @ViewBuilder
    private func lockScreenView(_ context: ActivityViewContext<FalconMatchAttributes>) -> some View {
        let state = context.state
        VStack(alignment: .leading, spacing: 7) {
            // ─────── UPPER SECTION ───────
            // Score on the left (full height of the upper), header + label
            // row on the right.
            HStack(alignment: .top, spacing: 10) {
                ScoreBadge(score: state.score, size: 60)

                VStack(alignment: .leading, spacing: 5) {
                    // Row 1: [Logo] [Falcon + ✨ count] [Spacer] [CTA bottom-right].
                    HStack(alignment: .bottom, spacing: 8) {
                        AppLogo(size: 30)
                        VStack(alignment: .leading, spacing: 1) {
                            Text("Falcon")
                                .font(.system(size: 15, weight: .bold, design: .rounded))
                            HStack(spacing: 3) {
                                Image(systemName: "sparkles")
                                    .font(.system(size: 9, weight: .semibold))
                                    .foregroundStyle(.tertiary)
                                Text("\(state.totalMatches) \(totalMatchesWord(lang: state.lang))")
                                    .font(.system(size: 10, weight: .medium))
                                    .foregroundStyle(.secondary)
                                    .contentTransition(.numericText())
                            }
                        }
                        Spacer(minLength: 0)
                        ctaCompact(lang: state.lang)
                    }

                    // Row 2: label pill + company icon + company name.
                    HStack(spacing: 7) {
                        Text(labelText(state.label, lang: state.lang))
                            .font(.system(size: 10, weight: .semibold))
                            .foregroundStyle(scoreColor(state.score))
                            .padding(.horizontal, 7)
                            .padding(.vertical, 2)
                            .background(Capsule().fill(scoreColor(state.score).opacity(0.15)))
                        if !state.companyName.isEmpty {
                            HStack(spacing: 4) {
                                CompanyInitialsIcon(name: state.companyName, size: 16)
                                Text(state.companyName)
                                    .font(.system(size: 11, weight: .medium))
                                    .foregroundStyle(.secondary)
                                    .lineLimit(1)
                            }
                        }
                        Spacer(minLength: 0)
                    }
                }
            }

            // ─────── LOWER SECTION (full width) ───────
            // Title + bars use the whole Lock Screen width — no competencia
            // de ancho con el score badge ni el CTA.
            if !state.projectTitle.isEmpty {
                Text(state.projectTitle)
                    .font(.system(size: 13, weight: .semibold, design: .rounded))
                    .lineLimit(1)
                    .truncationMode(.tail)
            }

            VStack(spacing: 3) {
                HStack(spacing: 10) {
                    scoreRow(scoreDimensionLabel("skills",    lang: state.lang), value: state.skillsMatch)
                    scoreRow(scoreDimensionLabel("domain",    lang: state.lang), value: state.domainExperience)
                }
                HStack(spacing: 10) {
                    scoreRow(scoreDimensionLabel("seniority",     lang: state.lang), value: state.seniorityFit)
                    scoreRow(scoreDimensionLabel("communication", lang: state.lang), value: state.communicationClarity)
                }
                HStack(spacing: 10) {
                    scoreRow(scoreDimensionLabel("relevance", lang: state.lang), value: state.projectRelevance)
                    scoreRow(scoreDimensionLabel("techstack", lang: state.lang), value: state.techStackOverlap)
                }
            }
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 24)
        // No activityBackgroundTint: iOS picks the Lock Screen background
        // based on the user's wallpaper (dark tint over light wallpaper,
        // vice versa). Forcing `systemBackground` here resolves against
        // the system mode, not iOS's widget mode — creating white-on-
        // white or black-on-black when the two disagree. Letting iOS
        // choose means semantic colors (.primary/.secondary/.tertiary)
        // auto-adapt to whatever tint Apple picked.
        .activitySystemActionForegroundColor(Color.accentColor)
    }

    // Compact CTA capsule placed in the top-right of the Lock Screen presentation.
    //
    // Uses semantic colors so iOS (not us) picks the contrast: `.primary`
    // flips between black and white based on whatever tint the Live
    // Activity got assigned. A 15% opacity fill gives the capsule
    // enough shape to read as interactive without forcing an
    // accent-color rectangle that clashes with Apple's chosen
    // background (e.g. blue button over a blue-tinted Live Activity).
    @ViewBuilder
    private func ctaCompact(lang: String) -> some View {
        HStack(spacing: 4) {
            Text(ctaText(lang: lang))
                .font(.system(size: 11, weight: .semibold))
            Image(systemName: "chevron.right")
                .font(.system(size: 9, weight: .bold))
        }
        .foregroundStyle(.primary)
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
        .background(Capsule().fill(Color.primary.opacity(0.15)))
        .fixedSize()
    }

    // MARK: - Compact progress bar for each score dimension.
    // .animation(_, value:) animates between state updates. On first render
    // there's no previous value so it shows static bars immediately, but any
    // subsequent update pushes from the server animate the fill smoothly.
    @ViewBuilder
    private func scoreRow(_ label: String, value: Double) -> some View {
        HStack(spacing: 6) {
            Text(label)
                .font(.system(size: 9, weight: .medium))
                .foregroundStyle(.secondary)
                .frame(width: 70, alignment: .leading)
                .lineLimit(1)
                .truncationMode(.tail)
            GeometryReader { geo in
                ZStack(alignment: .leading) {
                    Capsule()
                        .fill(Color.secondary.opacity(0.12))
                        .frame(height: 4)
                    Capsule()
                        .fill(scoreColor(value))
                        .frame(width: max(3, geo.size.width * CGFloat(min(10, max(0, value)) / 10)), height: 4)
                        .animation(.easeOut(duration: 0.6), value: value)
                }
                .frame(maxHeight: .infinity, alignment: .center)
            }
            .frame(height: 8)
            Text("\(Int((value * 10).rounded()))%")
                .font(.system(size: 9, weight: .semibold, design: .rounded))
                .foregroundStyle(scoreColor(value))
                .frame(width: 28, alignment: .trailing)
                .contentTransition(.numericText())
        }
    }

    // MARK: - CTA at the bottom ("View match" localized).
    @ViewBuilder
    private func cta(lang: String) -> some View {
        HStack(spacing: 6) {
            Text(ctaText(lang: lang))
                .font(.system(size: 13, weight: .semibold))
            Image(systemName: "chevron.right")
                .font(.system(size: 10, weight: .bold))
        }
        .foregroundStyle(.white)
        .frame(maxWidth: .infinity)
        .padding(.vertical, 10)
        .background(
            RoundedRectangle(cornerRadius: 12, style: .continuous)
                .fill(Color.accentColor)
        )
    }
}

// MARK: - Preview

#Preview("Compact", as: .dynamicIsland(.compact), using: FalconMatchAttributes()) {
    FalconWidgetLiveActivity()
} contentStates: {
    FalconMatchAttributes.ContentState(score: 7.8, label: "top_candidate", lang: "de", projectTitle: "Senior React Dev Kotlin, Kuberntes, React, Java, Golang, C++ MongoDB", companyName: "ACME GmbH", totalMatches: 12, summary: "Score 7.8 · React/TypeScript stark, fehlendes AWS.", projectID: "p1", cvID: "c1", skillsMatch: 8.5, seniorityFit: 7.2, domainExperience: 6.0, communicationClarity: 9.0, projectRelevance: 7.5, techStackOverlap: 8.0)
    FalconMatchAttributes.ContentState(score: 9.1, label: "apply_immediately", lang: "de", projectTitle: "Cloud Architect AWS", companyName: "Bosch", totalMatches: 23, summary: "Score 9.1 · perfekt passend.", projectID: "p2", cvID: "c1", skillsMatch: 9.5, seniorityFit: 9.0, domainExperience: 8.8, communicationClarity: 9.2, projectRelevance: 9.0, techStackOverlap: 9.5)
}

#Preview("Minimal", as: .dynamicIsland(.minimal), using: FalconMatchAttributes()) {
    FalconWidgetLiveActivity()
} contentStates: {
    FalconMatchAttributes.ContentState(score: 7.8, label: "top_candidate", lang: "de", projectTitle: "", companyName: "", totalMatches: 5, summary: "", projectID: "p1", cvID: "c1", skillsMatch: 8.5, seniorityFit: 7.2, domainExperience: 6.0, communicationClarity: 9.0, projectRelevance: 7.5, techStackOverlap: 8.0)
}

#Preview("Expanded", as: .dynamicIsland(.expanded), using: FalconMatchAttributes()) {
    FalconWidgetLiveActivity()
} contentStates: {
    FalconMatchAttributes.ContentState(score: 7.8, label: "top_candidate", lang: "de", projectTitle: "Fullstack Java/Kotlin — Frankfurt", companyName: "ACME GmbH", totalMatches: 12, summary: "Score 7.8 · React/TypeScript stark, fehlendes AWS und Docker.", projectID: "p1", cvID: "c1", skillsMatch: 8.5, seniorityFit: 7.2, domainExperience: 6.0, communicationClarity: 9.0, projectRelevance: 7.5, techStackOverlap: 8.0)
}
