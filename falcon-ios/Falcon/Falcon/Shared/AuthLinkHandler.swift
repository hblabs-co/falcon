import Foundation
import OSLog

private let log = Logger(subsystem: "co.hblabs.falcon", category: "auth")

/// Handles the `falcon://auth?token=xxx` deep link produced by the magic-link email.
/// Calls `GET /api/auth/verify?token=xxx`, extracts the JWT, and persists it via SessionManager.
@Observable
final class AuthLinkHandler {
    var isVerifying = false
    var errorMessage: String?
    var errorKey: StringKey?

    func handle(_ url: URL) {
        log.info("onOpenURL: \(url.absoluteString, privacy: .public)")
        log.info("scheme=\(url.scheme ?? "nil", privacy: .public) host=\(url.host ?? "nil", privacy: .public)")

        guard url.scheme == "falcon",
              url.host == "auth",
              let components = URLComponents(url: url, resolvingAgainstBaseURL: false),
              let token = components.queryItems?.first(where: { $0.name == "token" })?.value
        else {
            log.error("URL did not match expected format")
            return
        }

        if SessionManager.shared.isAuthenticated {
            log.info("already authenticated — ignoring link")
            return
        }

        log.info("token extracted, verifying...")
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
        log.info("GET \(urlStr, privacy: .public)")
        guard let url = URL(string: urlStr) else {
            log.error("invalid url")
            errorMessage = "Invalid verification URL"
            return
        }

        do {
            let (data, response) = try await URLSession.shared.data(from: url)
            guard let http = response as? HTTPURLResponse else {
                log.error("no HTTP response")
                errorMessage = "No response from server"
                return
            }
            log.info("verify status: \(http.statusCode, privacy: .public)")
            guard (200...299).contains(http.statusCode) else {
                let body = String(data: data, encoding: .utf8) ?? "n/a"
                log.error("verify error body: \(body, privacy: .public)")
                let serverError = (try? JSONDecoder().decode(ErrorResponse.self, from: data))?.error ?? ""
                errorKey = mapServerError(serverError)
                return
            }

            let result = try JSONDecoder().decode(VerifyResponse.self, from: data)
            log.info("verify OK — saving JWT for user \(result.userID, privacy: .public) email \(result.email ?? "?", privacy: .public)")
            SessionManager.shared.saveJWT(result.token, userID: result.userID, email: result.email ?? "")
            log.info("session.isAuthenticated = \(SessionManager.shared.isAuthenticated, privacy: .public)")
        } catch {
            log.error("verify exception: \(error.localizedDescription, privacy: .public)")
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
