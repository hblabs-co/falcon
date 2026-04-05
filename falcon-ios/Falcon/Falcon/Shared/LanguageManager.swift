import SwiftUI

/// Manages two independent language preferences:
/// - `appLanguage`: language used for the app UI
/// - `notificationLanguage`: language requested for push notification content and LLM match summaries
@Observable
final class LanguageManager {
    static let shared = LanguageManager()

    /// Language for the app UI. Persisted per device.
    var appLanguage: AppLanguage {
        didSet { UserDefaults.standard.set(appLanguage.rawValue, forKey: "app_language") }
    }

    /// Language for notification content and LLM output. Persisted per device.
    var notificationLanguage: AppLanguage {
        didSet { UserDefaults.standard.set(notificationLanguage.rawValue, forKey: "notification_language") }
    }

    private init() {
        let storedApp = UserDefaults.standard.string(forKey: "app_language")
        appLanguage = storedApp.flatMap(AppLanguage.init) ?? .fromSystem

        let storedNotif = UserDefaults.standard.string(forKey: "notification_language")
        notificationLanguage = storedNotif.flatMap(AppLanguage.init) ?? .fromSystem
    }

    /// Shorthand to translate a key using the current app language.
    func t(_ key: StringKey) -> String {
        Strings.get(key, language: appLanguage)
    }
}
