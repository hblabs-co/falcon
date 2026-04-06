import Foundation

enum AppLanguage: String, CaseIterable, Identifiable {
    case english = "en"
    case german  = "de"
    case spanish = "es"

    var id: String { rawValue }

    var displayName: String {
        switch self {
        case .english: "English"
        case .german:  "Deutsch"
        case .spanish: "Español"
        }
    }

    var flag: String {
        switch self {
        case .english: "🇬🇧"
        case .german:  "🇩🇪"
        case .spanish: "🇪🇸"
        }
    }

    /// Reads the first preferred system language and maps it to a supported language.
    /// Falls back to English if the system language is not supported.
    static var fromSystem: AppLanguage {
        let code = String((Locale.preferredLanguages.first ?? "en").prefix(2))
        return AppLanguage(rawValue: code) ?? .english
    }
}
