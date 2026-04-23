import Foundation

/// Per-request config for `FalconHttpClient` — headers + query items
/// built fluently so call sites read like sentences:
///
///   FalconHttpConfig()
///       .withAuthHeader(jwt)
///       .withQuery("true", forKey: "only_unread")
///
/// The type is a plain value, so each call site builds an independent
/// config. No mutation across requests.
struct FalconHttpConfig {
    var headers: [String: String]
    var queryItems: [URLQueryItem]
    /// Explicit decoder so callers that expect a non-default date
    /// strategy (or a snake-case keyDecodingStrategy) can swap it per
    /// request without the client needing to know. Defaults to ISO-8601
    /// dates since that's what falcon-api emits everywhere today.
    var decoder: JSONDecoder

    init() {
        self.headers = [:]
        self.queryItems = []
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        self.decoder = decoder
    }

    // MARK: - Fluent mutators
    //
    // All return a modified copy so chaining doesn't mutate the
    // caller's instance. Cheap for value types — the underlying
    // dictionaries copy-on-write.

    func withHeader(_ value: String, forKey key: String) -> Self {
        var copy = self
        copy.headers[key] = value
        return copy
    }

    /// Stamps a Bearer token on the Authorization header. Pass the
    /// raw JWT (without the "Bearer " prefix) — the client handles
    /// the format so call sites can't get it wrong.
    func withAuthHeader(_ jwt: String) -> Self {
        withHeader("Bearer \(jwt)", forKey: "Authorization")
    }

    func withQuery(_ value: String, forKey key: String) -> Self {
        var copy = self
        copy.queryItems.append(URLQueryItem(name: key, value: value))
        return copy
    }

    func withDecoder(_ decoder: JSONDecoder) -> Self {
        var copy = self
        copy.decoder = decoder
        return copy
    }
}
