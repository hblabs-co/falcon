import Foundation
import OSLog

private let log = FalconLog.make(category: "api-client")

/// Thin wrapper around the Falcon HTTP API. Currently exposes just
/// the `/system` endpoint — this is the first step of a gradual
/// migration: other callers (NotificationManager, MatchesViewModel,
/// MatchesAPI, etc.) will move in here over time so the whole app
/// funnels through one URLSession, one JWT injection point, and one
/// place to swap staging↔production.
///
/// Why a client instead of free functions? Centralising the base URL
/// + decoder config means "add a field to every DTO" stops being a
/// drive-by change that skips a model. It also gives us one spot to
/// stamp headers (trace id, app version) when we get there.
@MainActor
final class FalconApiClient {
    static let shared = FalconApiClient()
    private init() {}

    // MARK: - Paths
    //
    // One const per route. Keeping them here — not scattered as
    // string literals at call sites — means a rename on the server
    // (e.g. /system → /status) is a one-line patch in the iOS app.
    enum Path {
        static let system = "/system"
    }

    // MARK: - System

    /// GET /system — returns the per-service metadata document
    /// collection. Public endpoint, no auth needed. Used by the
    /// launch + periodic poller to keep the in-memory view fresh;
    /// callers that need a one-shot read can hit this directly.
    func fetchSystem() async throws -> SystemResponse {
        try await FalconHttpClient.get(url(Path.system), as: SystemResponse.self)
    }

    // MARK: - Helpers

    /// Absolute URL for a path against the same base host
    /// `NotificationManager` already uses. This dependency will flip
    /// later — once all callers live in the client, NotificationManager
    /// will call into here instead of the other way around.
    private func url(_ path: String) -> String {
        "\(NotificationManager.shared.apiURL)\(path)"
    }
}

