import Foundation

/// Owns the authenticated user identity for the current session.
///
/// Source of truth for `userID`:
/// - **DEBUG**: reads from UserDefaults with `Config.userID` as fallback,
///   and can be overridden via the dev config section in Settings.
/// - **Production**: populated by `setFromJWT(_:)` after falcon-auth login.
///   Never editable by the user.
@Observable
final class SessionManager {
    static let shared = SessionManager()

    private(set) var userID: String

    var isAuthenticated: Bool { !userID.isEmpty }

    private init() {
        userID = UserDefaults.standard.string(forKey: "user_id") ?? ""
    }

    /// Registered by NotificationManager at startup. Called whenever a new userID is set.
    var onUserIDAvailable: ((String) -> Void)?

    /// Called after a successful falcon-auth login with the decoded JWT subject.
    /// Fires `onUserIDAvailable` so NotificationManager can auto-register the device token.
    func setFromJWT(_ id: String) {
        userID = id
        UserDefaults.standard.set(id, forKey: "user_id")
        onUserIDAvailable?(id)
    }

    func clearSession() {
        userID = ""
        UserDefaults.standard.removeObject(forKey: "user_id")
    }

}
