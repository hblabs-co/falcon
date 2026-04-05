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
        #if DEBUG
        userID = UserDefaults.standard.string(forKey: "user_id") ?? Config.userID
        #else
        // In production userID is set after successful falcon-auth login.
        userID = UserDefaults.standard.string(forKey: "user_id") ?? ""
        #endif
    }

    /// Called after a successful falcon-auth login with the decoded JWT subject.
    func setFromJWT(_ id: String) {
        userID = id
        UserDefaults.standard.set(id, forKey: "user_id")
    }

    func clearSession() {
        userID = ""
        UserDefaults.standard.removeObject(forKey: "user_id")
    }

#if DEBUG
    /// Dev-only: allows overriding userID from the Settings config section.
    func devSetUserID(_ id: String) {
        userID = id
        UserDefaults.standard.set(id, forKey: "user_id")
    }
#endif
}
