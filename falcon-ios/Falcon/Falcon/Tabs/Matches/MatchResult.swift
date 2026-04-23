import Foundation
import SwiftUI

// MARK: - API response

struct MatchesResponse: Decodable {
    let data: [MatchResult]
    let pagination: MatchesPagination
    /// Server-calculated count of matches with viewed=false for the user
    /// (across all pages, not just this one). Powers the tab-icon badge.
    let unreadCount: Int

    enum CodingKeys: String, CodingKey {
        case data
        case pagination
        case unreadCount = "unread_count"
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        // `data: null` and missing key both map to an empty list —
        // matches the semantic (no results) without surfacing a
        // "data could not be read" error on the empty-filter path.
        data        = (try? c.decodeIfPresent([MatchResult].self, forKey: .data)) ?? []
        pagination  = try c.decode(MatchesPagination.self, forKey: .pagination)
        unreadCount = (try? c.decodeIfPresent(Int.self, forKey: .unreadCount)) ?? 0
    }
}

struct MatchesPagination: Decodable {
    let page: Int
    let pageSize: Int
    let total: Int
    let totalPages: Int

    enum CodingKeys: String, CodingKey {
        case page
        case pageSize  = "page_size"
        case total
        case totalPages = "total_pages"
    }
}

// MARK: - Match result

struct MatchResult: Decodable, Identifiable {
    let cvId:             String
    let userId:           String
    let projectId:        String
    let projectTitle:     String
    let platform:         String
    let companyName:      String
    /// Public MinIO URL for the company's logo. Empty when the company
    /// has no logo — `resolvedLogoURL` returns nil and the UI falls back
    /// to the initials avatar.
    let companyLogoUrl:   String
    let score:            Double
    let label:            String
    let scores:           MatchScores
    let matchedSkills:    [String]
    let missingSkills:    [String]
    let positivePointsAll:  [String: [String]]
    let negativePointsAll:  [String: [String]]
    let improvementTipsAll: [String: [String]]
    let passedThreshold:  Bool
    let scoredAt:         String
    let summaryAll:       [String: String]
    /// Mirrors the server `viewed` flag — flips to true after the user
    /// opens MatchDetailView. Kept as `var` so we can optimistic-update
    /// locally without refetching the whole list.
    var isViewed:         Bool
    /// Mirrors the server `normalized` flag. `false` means
    /// falcon-normalizer hasn't produced the UI-ready project doc yet,
    /// so "Zum Projekt" must show a spinner instead of opening a
    /// placeholder sheet. Flipped to `true` either server-side
    /// (match-engine sweep / event) or client-side when we receive a
    /// realtime `project.normalized` push for the matching project_id.
    var isNormalized:     Bool

    // Composite id so SwiftUI's diffing handles re-matches of the same project
    // with a different CV (and re-matches of the same CV after a re-score).
    var id: String { "\(projectId)-\(cvId)" }

    enum CodingKeys: String, CodingKey {
        case cvId             = "cv_id"
        case userId           = "user_id"
        case projectId        = "project_id"
        case projectTitle     = "project_title"
        case platform
        case companyName      = "company_name"
        case companyLogoUrl   = "company_logo_url"
        case score
        case label
        case scores
        case matchedSkills    = "matched_skills"
        case missingSkills    = "missing_skills"
        case positivePointsAll  = "positive_points"
        case negativePointsAll  = "negative_points"
        case improvementTipsAll = "improvement_tips"
        case passedThreshold  = "passed_threshold"
        case scoredAt         = "scored_at"
        case summaryAll       = "summary"
        case isViewed         = "viewed"
        case isNormalized     = "normalized"
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        cvId               = (try? c.decodeIfPresent(String.self,              forKey: .cvId))               ?? ""
        userId             = (try? c.decodeIfPresent(String.self,              forKey: .userId))             ?? ""
        projectId          = (try? c.decodeIfPresent(String.self,              forKey: .projectId))          ?? ""
        projectTitle       = (try? c.decodeIfPresent(String.self,              forKey: .projectTitle))       ?? ""
        platform           = (try? c.decodeIfPresent(String.self,              forKey: .platform))           ?? ""
        companyName        = (try? c.decodeIfPresent(String.self,              forKey: .companyName))        ?? ""
        companyLogoUrl     = (try? c.decodeIfPresent(String.self,              forKey: .companyLogoUrl))     ?? ""
        score              = (try? c.decodeIfPresent(Double.self,              forKey: .score))              ?? 0
        label              = (try? c.decodeIfPresent(String.self,              forKey: .label))              ?? ""
        scores             = (try? c.decodeIfPresent(MatchScores.self,         forKey: .scores))             ?? MatchScores.empty
        matchedSkills      = (try? c.decodeIfPresent([String].self,            forKey: .matchedSkills))      ?? []
        missingSkills      = (try? c.decodeIfPresent([String].self,            forKey: .missingSkills))      ?? []
        positivePointsAll  = (try? c.decodeIfPresent([String: [String]].self,  forKey: .positivePointsAll))  ?? [:]
        negativePointsAll  = (try? c.decodeIfPresent([String: [String]].self,  forKey: .negativePointsAll))  ?? [:]
        improvementTipsAll = (try? c.decodeIfPresent([String: [String]].self,  forKey: .improvementTipsAll)) ?? [:]
        passedThreshold    = (try? c.decodeIfPresent(Bool.self,                forKey: .passedThreshold))    ?? false
        scoredAt           = (try? c.decodeIfPresent(String.self,              forKey: .scoredAt))           ?? ""
        summaryAll         = (try? c.decodeIfPresent([String: String].self,    forKey: .summaryAll))         ?? [:]
        isViewed           = (try? c.decodeIfPresent(Bool.self,                forKey: .isViewed))           ?? false
        isNormalized       = (try? c.decodeIfPresent(Bool.self,                forKey: .isNormalized))       ?? false
    }

    // MARK: - Language-aware accessors (fallback chain: requested → de → en → es)

    private func pickString(_ map: [String: String], lang: String) -> String {
        map[lang] ?? map["de"] ?? map["en"] ?? map["es"] ?? ""
    }

    private func pickList(_ map: [String: [String]], lang: String) -> [String] {
        map[lang] ?? map["de"] ?? map["en"] ?? map["es"] ?? []
    }

    func summary(for lang: AppLanguage)          -> String   { pickString(summaryAll,         lang: lang.rawValue) }
    func positivePoints(for lang: AppLanguage)   -> [String] { pickList(positivePointsAll,    lang: lang.rawValue) }
    func negativePoints(for lang: AppLanguage)   -> [String] { pickList(negativePointsAll,    lang: lang.rawValue) }
    func improvementTips(for lang: AppLanguage)  -> [String] { pickList(improvementTipsAll,   lang: lang.rawValue) }

    var labelColor: Color {
        switch label {
        case "apply_immediately": return .green
        case "top_candidate":     return .blue
        case "acceptable":        return .orange
        case "not_suitable":      return .gray
        default:                  return .gray
        }
    }

    func labelText(lm: LanguageManager) -> String {
        switch label {
        case "apply_immediately": return lm.t(.matchLabelApplyImmediately)
        case "top_candidate":     return lm.t(.matchLabelTopCandidate)
        case "acceptable":        return lm.t(.matchLabelAcceptable)
        case "not_suitable":      return lm.t(.matchLabelNotSuitable)
        default:                  return "—"
        }
    }

    /// In DEBUG builds rewrites the logo URL host to `Config.imageHost`
    /// so the simulator / device can reach MinIO on the local network.
    /// In production the URL is used as-is. Mirrors `ProjectItem.resolvedLogoURL`.
    var resolvedLogoURL: URL? {
        guard !companyLogoUrl.isEmpty, var components = URLComponents(string: companyLogoUrl) else { return nil }
#if DEBUG
        if let base = URLComponents(string: Config.imageHost) {
            components.scheme = base.scheme
            components.host   = base.host
            components.port   = base.port
        }
#endif
        return components.url
    }

    func relativeDate(for language: AppLanguage) -> String? {
        guard !scoredAt.isEmpty else { return nil }
        let iso = ISO8601DateFormatter()
        iso.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let date = iso.date(from: scoredAt) ?? ISO8601DateFormatter().date(from: scoredAt)
        guard let date else { return nil }
        let fmt = RelativeDateTimeFormatter()
        fmt.locale = Locale(identifier: language.localeIdentifier)
        return fmt.localizedString(for: date, relativeTo: .now)
    }
}

struct MatchScores: Decodable {
    let skillsMatch:           Double
    let seniorityFit:          Double
    let domainExperience:      Double
    let communicationClarity:  Double
    let projectRelevance:      Double
    let techStackOverlap:      Double

    static let empty = MatchScores(
        skillsMatch: 0, seniorityFit: 0, domainExperience: 0,
        communicationClarity: 0, projectRelevance: 0, techStackOverlap: 0
    )

    enum CodingKeys: String, CodingKey {
        case skillsMatch          = "skills_match"
        case seniorityFit         = "seniority_fit"
        case domainExperience     = "domain_experience"
        case communicationClarity = "communication_clarity"
        case projectRelevance     = "project_relevance"
        case techStackOverlap     = "tech_stack_overlap"
    }
}
