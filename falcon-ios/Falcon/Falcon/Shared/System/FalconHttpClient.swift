import Foundation

/// Thin static HTTP helper that wraps the URLSession + decoder + status
/// check boilerplate every caller otherwise repeats. Two surfaces:
///
///   let res = try await FalconHttpClient.get(url, as: Model.self)
///   let res = try await FalconHttpClient.post(url, body: payload, as: Model.self)
///
/// Optional `config` adds headers / query params:
///
///   try await FalconHttpClient.get(
///       "\(base)/matches",
///       as: MatchesResponse.self,
///       config: FalconHttpConfig()
///           .withAuthHeader(jwt)
///           .withQuery("true", forKey: "only_unread")
///   )
///
/// Errors surface as throws (Swift idiom) rather than a tuple — same
/// ergonomics via `try?` + `guard let`:
///
///   guard let res = try? await FalconHttpClient.get(url, as: Model.self)
///   else { return }
enum FalconHttpClient {

    // MARK: - GET

    /// Fetches `url`, decodes the JSON body as `T`. Query params live
    /// on the `config`, not inline in the URL, so call sites don't
    /// hand-escape.
    static func get<T: Decodable>(
        _ url: String,
        as type: T.Type,
        config: FalconHttpConfig = FalconHttpConfig()
    ) async throws -> T {
        let request = try buildRequest(url: url, method: "GET", body: nil, config: config)
        return try await perform(request, as: type, config: config)
    }

    // MARK: - POST

    /// POSTs `body` (any Encodable) as JSON, decodes the response
    /// into `T`. `Content-Type: application/json` is set automatically.
    static func post<T: Decodable, Body: Encodable>(
        _ url: String,
        body: Body,
        as type: T.Type,
        config: FalconHttpConfig = FalconHttpConfig()
    ) async throws -> T {
        let bodyData = try JSONEncoder().encode(body)
        var requestConfig = config
        // Only stamp Content-Type if the caller hasn't. Respecting the
        // caller's override matters for odd APIs that need a specific
        // charset or media type.
        if requestConfig.headers["Content-Type"] == nil {
            requestConfig = requestConfig.withHeader("application/json", forKey: "Content-Type")
        }
        let request = try buildRequest(url: url, method: "POST", body: bodyData, config: requestConfig)
        return try await perform(request, as: type, config: requestConfig)
    }

    /// POST variant for endpoints that return no body (204 No Content
    /// / PATCH-style fire-and-forget). Skips the decode step.
    static func postVoid<Body: Encodable>(
        _ url: String,
        body: Body,
        config: FalconHttpConfig = FalconHttpConfig()
    ) async throws {
        let bodyData = try JSONEncoder().encode(body)
        var requestConfig = config
        if requestConfig.headers["Content-Type"] == nil {
            requestConfig = requestConfig.withHeader("application/json", forKey: "Content-Type")
        }
        let request = try buildRequest(url: url, method: "POST", body: bodyData, config: requestConfig)
        _ = try await performRaw(request)
    }

    // MARK: - Plumbing

    private static func buildRequest(
        url: String,
        method: String,
        body: Data?,
        config: FalconHttpConfig
    ) throws -> URLRequest {
        guard var components = URLComponents(string: url) else {
            throw URLError(.badURL)
        }
        if !config.queryItems.isEmpty {
            // Append — don't replace — so URLs that already carry
            // params in the input string survive (rare but real:
            // e.g. third-party redirect URLs).
            var existing = components.queryItems ?? []
            existing.append(contentsOf: config.queryItems)
            components.queryItems = existing
        }
        guard let finalURL = components.url else {
            throw URLError(.badURL)
        }

        var request = URLRequest(url: finalURL)
        request.httpMethod = method
        request.httpBody = body
        for (key, value) in config.headers {
            request.setValue(value, forHTTPHeaderField: key)
        }
        return request
    }

    private static func perform<T: Decodable>(
        _ request: URLRequest,
        as type: T.Type,
        config: FalconHttpConfig
    ) async throws -> T {
        let data = try await performRaw(request)
        return try config.decoder.decode(T.self, from: data)
    }

    /// Returns the raw bytes for callers that skip decoding (postVoid,
    /// binary downloads). Status-check lives here so every path
    /// enforces it uniformly.
    private static func performRaw(_ request: URLRequest) async throws -> Data {
        let (data, response) = try await URLSession.shared.data(for: request)
        guard let http = response as? HTTPURLResponse, (200..<300).contains(http.statusCode) else {
            throw URLError(.badServerResponse)
        }
        return data
    }
}
