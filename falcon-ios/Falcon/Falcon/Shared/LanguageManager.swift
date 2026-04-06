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

    /// Loads persisted configurations from the API and applies them.
    /// No-op if there is no userID (anonymous session).
    func loadFromAPI() async {
        let userID = SessionManager.shared.userID
        let apiURL  = NotificationManager.shared.apiURL
        guard !userID.isEmpty,
              let url = URL(string: "\(apiURL)/me?platform=ios&user_id=\(userID)") else { return }
        guard let (data, _) = try? await URLSession.shared.data(from: url),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let configs = json["configs"] as? [String: Any] else { return }
        if let raw = configs["app_language"] as? String, let lang = AppLanguage(rawValue: raw) {
            appLanguage = lang
        }
    }
}
