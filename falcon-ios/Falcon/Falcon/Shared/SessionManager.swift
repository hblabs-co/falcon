import Foundation

/// Owns the authenticated user identity for the current session.
///
/// Session restoration is deferred — call `restore()` from a `.task` modifier
/// so it runs async after the UI has rendered, avoiding a blocked launch screen.
@Observable
final class SessionManager {
    static let shared = SessionManager()

    private(set) var userID: String = ""
    private(set) var email: String = ""
    private(set) var isRestoring = true
    private(set) var cachedJWT: String?

    var isAuthenticated: Bool { !userID.isEmpty }

    /// Registered by NotificationManager at startup.
    var onUserIDAvailable: ((String) -> Void)?

    private init() {}

    // MARK: - Session lifecycle

    /// Attempts to restore a session from Keychain. Runs on a background thread
    /// so the biometric prompt doesn't block the main thread during launch.
    func restore() async {
        let result: (userID: String, email: String, jwt: String)? = await Task.detached {
            do {
                let jwt = try KeychainHelper.readJWT(reason: "Authenticate to restore your session")
                guard let claims = self.decodeJWTClaims(jwt) else { return nil }
                guard let exp = claims["exp"] as? TimeInterval,
                      Date(timeIntervalSince1970: exp) > Date()
                else {
                    print("[session] jwt expired — clearing session")
                    return nil
                }
                let sub  = claims["sub"]   as? String ?? ""
                let mail = claims["email"] as? String ?? ""
                return (sub, mail, jwt)
            } catch KeychainError.notFound {
                return nil
            } catch KeychainError.userCancelled {
                print("[session] biometry cancelled by user")
                return nil
            } catch {
                print("[session] restore failed: \(error)")
                return nil
            }
        }.value

        await MainActor.run {
            if let result {
                cachedJWT = result.jwt
                apply(userID: result.userID, email: result.email)
            } else {
                KeychainHelper.deleteJWT()
            }
            isRestoring = false
        }
    }

    /// Saves the JWT to Keychain and updates in-memory state.
    func saveJWT(_ jwt: String, userID: String, email: String) {
        do {
            try KeychainHelper.saveJWT(jwt)
        } catch {
            print("[session] keychain save failed: \(error) — falling back to memory only")
        }
        cachedJWT = jwt
        apply(userID: userID, email: email)
    }

    /// Clears the session and removes the JWT from Keychain.
    func logout() {
        KeychainHelper.deleteJWT()
        cachedJWT = nil
        userID = ""
        email = ""
    }

    // MARK: - Helpers

    private func apply(userID: String, email: String) {
        self.userID = userID
        self.email  = email
        if !userID.isEmpty {
            onUserIDAvailable?(userID)
        }
    }

    private func decodeJWTClaims(_ jwt: String) -> [String: Any]? {
        let parts = jwt.split(separator: ".")
        guard parts.count == 3 else { return nil }
        var base64 = String(parts[1])
            .replacingOccurrences(of: "-", with: "+")
            .replacingOccurrences(of: "_", with: "/")
        while base64.count % 4 != 0 { base64 += "=" }
        guard let data = Data(base64Encoded: base64),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
        else { return nil }
        return json
    }
}
