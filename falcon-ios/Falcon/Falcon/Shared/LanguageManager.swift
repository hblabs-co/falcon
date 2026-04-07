import SwiftUI

/// Manages the app language preference.
@Observable
final class LanguageManager {
    static let shared = LanguageManager()

    /// Language for the app UI. Persisted per device.
    var appLanguage: AppLanguage {
        didSet { UserDefaults.standard.set(appLanguage.rawValue, forKey: "app_language") }
    }

    private init() {
        let stored = UserDefaults.standard.string(forKey: "app_language")
        appLanguage = stored.flatMap(AppLanguage.init) ?? .fromSystem
    }

    /// Shorthand to translate a key using the current app language.
    func t(_ key: StringKey) -> String {
        Strings.get(key, language: appLanguage)
    }

    /// Applies configs from a /me API response. Called by CVUploadViewModel.restoreFromServer().
    func applyConfigs(_ configs: [String: AnyCodable]?) {
        guard let configs else { return }
        if let raw = configs["app_language"]?.value as? String, let lang = AppLanguage(rawValue: raw) {
            appLanguage = lang
        }
    }
}
