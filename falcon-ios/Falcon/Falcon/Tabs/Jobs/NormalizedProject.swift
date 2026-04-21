import Foundation

// MARK: - API Response

struct ProjectsResponse: Decodable {
    let data: [ProjectItem]
    let pagination: Pagination
    let todayCount: Int

    enum CodingKeys: String, CodingKey {
        case data, pagination
        case todayCount = "today_count"
    }
}

struct Pagination: Decodable {
    let page: Int
    let pageSize: Int
    let total: Int
    let totalPages: Int

    enum CodingKeys: String, CodingKey {
        case page
        case pageSize   = "page_size"
        case total
        case totalPages = "total_pages"
    }
}

// MARK: - Project item (envelope)

struct RecruiterRodeoStats: Decodable {
    let overallRating:      Double
    let recommendationRate: String
    let reviewCount:        Int

    enum CodingKeys: String, CodingKey {
        case overallRating      = "overall_rating"
        case recommendationRate = "recommendation_rate"
        case reviewCount        = "review_count"
    }
}

struct ProjectItem: Decodable, Identifiable {
    let projectId: String
    let platform: String
    let displayUpdatedAt: String
    let companyName: String
    let companyLogoUrl: String
    let recruiterRodeoStats: RecruiterRodeoStats?
    let normalizedAt: String
    let data: NormalizedData

    var id: String { projectId }

    enum CodingKeys: String, CodingKey {
        case projectId           = "project_id"
        case platform
        case displayUpdatedAt    = "display_updated_at"
        case companyName         = "company_name"
        case companyLogoUrl      = "company_logo_url"
        case recruiterRodeoStats = "recruiter_rodeo_stats"
        case normalizedAt        = "normalized_at"
        case data
    }

    /// Display title: prefers normalized LLM value, falls back to envelope company name.
    var displayTitle: String {
        data.title?.display?.nilIfEmpty ?? projectId
    }

    var displayCompany: String {
        data.company?.name?.nilIfEmpty ?? companyName.nilIfEmpty ?? platform
    }

    /// Single-line location string, e.g. "Frankfurt · Hybrid"
    var displayLocation: String? {
        let city = data.location?.cities?.first
        let mode = data.location?.remotePolicy?.type?.capitalized
        return [city, mode].compactMap { $0?.nilIfEmpty }.joined(separator: " · ").nilIfEmpty
    }

    var displayRate: String? {
        guard let comp = data.compensation else { return nil }
        if let raw = comp.raw?.nilIfEmpty { return raw }
        if let min = comp.amountMin, let cur = comp.currency {
            return "\(Int(min)) \(cur)"
        }
        return nil
    }

    /// In DEBUG builds rewrites the logo URL host to `Config.imageHost` so the device
    /// can reach MinIO on the local network. In production the URL is used as-is.
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
        guard !displayUpdatedAt.isEmpty else { return nil }
        let iso = ISO8601DateFormatter()
        iso.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let date = iso.date(from: displayUpdatedAt)
            ?? ISO8601DateFormatter().date(from: displayUpdatedAt)
        guard let date else { return nil }
        let fmt = RelativeDateTimeFormatter()
        fmt.locale = Locale(identifier: language.localeIdentifier)
        return fmt.localizedString(for: date, relativeTo: .now)
    }
}

// MARK: - Normalized content (English)

struct NormalizedData: Decodable {
    let title:            ProjectTitle?
    let company:          ProjectCompany?
    let location:         ProjectLocation?
    let compensation:     ProjectCompensation?
    let status:           ProjectStatus?
    let summary:          ProjectSummary?
    let classification:   ProjectClassification?
    let ui:               ProjectUI?
    let requirements:     ProjectRequirements?
    let responsibilities: [String]?
    let source:           ProjectSource?
    let contact:          ProjectContact?
}

struct ProjectTitle: Decodable {
    let raw:        String?
    let normalized: String?
    let display:    String?
}

struct ProjectCompany: Decodable {
    let name:           String?
    let hiringType:     String?
    let isDirectClient: Bool?

    enum CodingKeys: String, CodingKey {
        case name
        case hiringType     = "hiring_type"
        case isDirectClient = "is_direct_client"
    }
}

struct ProjectLocation: Decodable {
    let raw:          String?
    let countryCode:  String?
    let cities:       [String]?
    let remotePolicy: RemotePolicy?

    enum CodingKeys: String, CodingKey {
        case raw
        case countryCode  = "country_code"
        case cities
        case remotePolicy = "remote_policy"
    }
}

struct RemotePolicy: Decodable {
    let type:           String?
    let remoteAllowed:  Bool?
    let onsiteRequired: Bool?
    let onsiteDays:     Int?
    let notes:          String?

    enum CodingKeys: String, CodingKey {
        case type
        case remoteAllowed  = "remote_allowed"
        case onsiteRequired = "onsite_required"
        case onsiteDays     = "onsite_days"
        case notes
    }

    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        type           = try? c.decodeIfPresent(String.self, forKey: .type)
        remoteAllowed  = try? c.decodeIfPresent(Bool.self,   forKey: .remoteAllowed)
        onsiteRequired = try? c.decodeIfPresent(Bool.self,   forKey: .onsiteRequired)
        notes          = try? c.decodeIfPresent(String.self, forKey: .notes)
        if let v = try? c.decodeIfPresent(Int.self, forKey: .onsiteDays) {
            onsiteDays = v
        } else if let s = try? c.decodeIfPresent(String.self, forKey: .onsiteDays) {
            onsiteDays = Int(s)
        } else {
            onsiteDays = nil
        }
    }
}

struct ProjectCompensation: Decodable {
    let rateType:       String?
    let currency:       String?
    let amountMin:      Double?
    let amountMax:      Double?
    let rateVisibility: String?
    let raw:            String?

    enum CodingKeys: String, CodingKey {
        case rateType       = "rate_type"
        case currency
        case amountMin      = "amount_min"
        case amountMax      = "amount_max"
        case rateVisibility = "rate_visibility"
        case raw
    }

    // LLM sometimes encodes numeric fields as strings — decode both gracefully.
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        rateType       = try? c.decodeIfPresent(String.self, forKey: .rateType)
        currency       = try? c.decodeIfPresent(String.self, forKey: .currency)
        rateVisibility = try? c.decodeIfPresent(String.self, forKey: .rateVisibility)
        raw            = try? c.decodeIfPresent(String.self, forKey: .raw)
        amountMin      = Self.flexDouble(c, key: .amountMin)
        amountMax      = Self.flexDouble(c, key: .amountMax)
    }

    private static func flexDouble(_ c: KeyedDecodingContainer<CodingKeys>, key: CodingKeys) -> Double? {
        if let v = try? c.decodeIfPresent(Double.self, forKey: key) { return v }
        if let s = try? c.decodeIfPresent(String.self, forKey: key) { return Double(s) }
        return nil
    }
}

struct ProjectStatus: Decodable {
    let isActive:            Bool?
    let startsAt:            String?
    let endsAt:              String?
    let applicationDeadline: String?

    enum CodingKeys: String, CodingKey {
        case isActive            = "is_active"
        case startsAt            = "starts_at"
        case endsAt              = "ends_at"
        case applicationDeadline = "application_deadline"
    }
}

struct ProjectSummary: Decodable {
    let short:      String?
    let highlights: [String]?
}

struct ProjectClassification: Decodable {
    let jobFamily:  String?
    let seniority:  String?
    let keywords:   [String]?

    enum CodingKeys: String, CodingKey {
        case jobFamily = "job_family"
        case seniority
        case keywords
    }
}

struct ProjectUI: Decodable {
    let badges:           [UIBadge]?
    let heroFacts:        [HeroFact]?
    let warnings:         [String]?
    let requirementChips: [String]?

    enum CodingKeys: String, CodingKey {
        case badges
        case heroFacts        = "hero_facts"
        case warnings
        case requirementChips = "requirement_chips"
    }
}

struct UIBadge: Decodable {
    let type:  String?
    let label: String?
}

struct HeroFact: Decodable {
    let label: String?
    let value: String?
}

struct ProjectRequirements: Decodable {
    let mustHave:   [RequirementItem]?
    let shouldHave: [RequirementItem]?
    let niceToHave: [RequirementItem]?

    enum CodingKeys: String, CodingKey {
        case mustHave   = "must_have"
        case shouldHave = "should_have"
        case niceToHave = "nice_to_have"
    }
}

struct RequirementItem: Decodable {
    let name:           String?
    let normalizedName: String?
    let category:       String?
    let minYears:       Int?
    let relatedTools:   [String]?
    let required:       Bool?

    enum CodingKeys: String, CodingKey {
        case name
        case normalizedName = "normalized_name"
        case category
        case minYears       = "min_years"
        case relatedTools   = "related_tools"
        case required
    }

    // min_years can come as Int or String from LLM
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        name           = try? c.decodeIfPresent(String.self, forKey: .name)
        normalizedName = try? c.decodeIfPresent(String.self, forKey: .normalizedName)
        category       = try? c.decodeIfPresent(String.self, forKey: .category)
        required       = try? c.decodeIfPresent(Bool.self,   forKey: .required)
        relatedTools   = try? c.decodeIfPresent([String].self, forKey: .relatedTools)
        if let v = try? c.decodeIfPresent(Int.self, forKey: .minYears) {
            minYears = v
        } else if let s = try? c.decodeIfPresent(String.self, forKey: .minYears) {
            minYears = Int(s)
        } else {
            minYears = nil
        }
    }
}

struct ProjectSource: Decodable {
    let url:        String?
    let platformId: String?
    enum CodingKeys: String, CodingKey {
        case url
        case platformId = "platform_id"
    }
}

struct ProjectContact: Decodable {
    let name:    String?
    let company: String?
    let email:   String?
    let phone:   String?
    let role:    String?
    let image:   String?
}

// MARK: - Helpers

extension String {
    var nilIfEmpty: String? { isEmpty ? nil : self }
}
