#if canImport(ActivityKit)
import ActivityKit

/// Shared between Falcon (main app) and FalconWidget targets.
/// In Xcode: select this file → File Inspector → Target Membership → tick both Falcon and FalconWidget.
///
/// Started by NotificationManager on foreground MATCH_RESULT push arrivals so
/// the user sees a live summary on Lock Screen / Dynamic Island without having
/// to leave whatever they were doing. Auto-dismisses via staleDate.
struct FalconMatchAttributes: ActivityAttributes {
    struct ContentState: Codable, Hashable {
        /// 0–10, one decimal. Drives the badge color (red/orange/green).
        var score: Double
        /// Raw MatchLabel value ("apply_immediately" | "top_candidate" |
        /// "acceptable" | "not_suitable"). Widget maps to color + localized text.
        var label: String
        /// Device language code ("de" | "en" | "es") so the widget can pick
        /// localized strings (widgets can't read LanguageManager at runtime).
        var lang: String
        var projectTitle: String
        /// Authoritative company name (from companies collection). Empty if unknown.
        var companyName: String
        /// Public MinIO URL for the company logo. Empty when the company
        /// has no logo — widget falls back to a circular initials chip.
        var companyLogoUrl: String
        /// Total number of matches the user has right now. Shown in the
        /// activity header so the user sees progress at a glance.
        var totalMatches: Int
        /// One-sentence LLM summary, already localized to the device's language.
        var summary: String
        /// IDs used to build the deep link in .widgetURL so tapping the
        /// activity opens MatchDetailView in the app (falcon://match?...).
        var projectID: String
        var cvID: String
        /// Six score dimensions (0–10 each) rendered as progress bars.
        var skillsMatch:          Double
        var seniorityFit:         Double
        var domainExperience:     Double
        var communicationClarity: Double
        var projectRelevance:     Double
        var techStackOverlap:     Double
    }
}
#endif
