import Foundation

/// Handles the `falcon://auth?token=xxx` deep link produced by the magic-link email.
/// Calls `GET /api/auth/verify?token=xxx`, extracts the JWT, and persists it via SessionManager.
@Observable
final class AuthLinkHandler {
    var isVerifying = false
    var errorMessage: String?
    var errorKey: StringKey?

    func handle(_ url: URL) {
        print("[auth] onOpenURL: \(url)")
        print("[auth] scheme=\(url.scheme ?? "nil") host=\(url.host ?? "nil")")

        guard url.scheme == "falcon",
              url.host == "auth",
              let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
              let token = components.queryItems?.first(where: { $0.name == "token" })?.value
        else {
            print("[auth] URL did not match expected format")
            return
        }

        if SessionManager.shared.isAuthenticated {
            print("[auth] already authenticated — ignoring link")
            return
        }

        print("[auth] token extracted, verifying...")
        Task { @MainActor in
            await verify(token: token)
        }
    }

    @MainActor
    private func verify(token: String) async {
        isVerifying = true
        errorMessage = nil
        defer { isVerifying = false }

        let base = NotificationManager.shared.apiURL
        let urlStr = "\(base)/auth/verify?token=\(token)"
        print("[auth] GET \(urlStr)")
        guard let url = URL(string: urlStr) else {
            print("[auth] invalid url")
            errorMessage = "Invalid verification URL"
            return
        }

        do {
            let (data, response) = try await URLSession.shared.data(from: url)
            guard let http = response as? HTTPURLResponse else {
                print("[auth] no HTTP response")
                errorMessage = "No response from server"
                return
            }
            print("[auth] verify status: \(http.statusCode)")
            guard (200...299).contains(http.statusCode) else {
                let body = String(data: data, encoding: .utf8) ?? "n/a"
                print("[auth] verify error body: \(body)")
                let serverError = (try? JSONDecoder().decode(ErrorResponse.self, from: data))?.error ?? ""
                errorKey = mapServerError(serverError)
                return
            }

            let result = try JSONDecoder().decode(VerifyResponse.self, from: data)
            print("[auth] verify OK — saving JWT for user \(result.userID) email \(result.email ?? "?")")
            SessionManager.shared.saveJWT(result.token, userID: result.userID, email: result.email ?? "")
            print("[auth] session.isAuthenticated = \(SessionManager.shared.isAuthenticated)")
        } catch {
            print("[auth] verify exception: \(error)")
            errorKey = .authErrorGeneric
        }
    }

    private func mapServerError(_ error: String) -> StringKey {
        if error.contains("already used") { return .authErrorTokenUsed }
        if error.contains("expired")      { return .authErrorTokenExpired }
        return .authErrorGeneric
    }
}

// MARK: - Response types

private struct VerifyResponse: Decodable {
    let token: String
    let userID: String
    let email: String?

    enum CodingKeys: String, CodingKey {
        case token
        case userID  = "user_id"
        case email
    }
}

private struct ErrorResponse: Decodable {
    let error: String
}
