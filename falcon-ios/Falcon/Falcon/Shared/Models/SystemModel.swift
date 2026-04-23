import Foundation

// DTOs for the GET /system endpoint. Mirrors the wire shape emitted
// by `falcon-api/system/routes.go`. Kept under Shared/Models/ — one
// file per endpoint / resource — so transport (FalconApiClient) stays
// separated from shape, and adding a new endpoint later is a drop-in
// `NewResourceModel.swift` without touching the client file.

/// Wire shape of `GET /system`. Extended over time as the server adds
/// well-known fields (version, git commit, etc.); the decoder uses
/// `try?` / defaults so old clients survive server-side additions
/// without breaking.
struct SystemResponse: Decodable {
    let services:  [SystemServiceEntry]
    let updatedAt: Date
    let count:     Int

    enum CodingKeys: String, CodingKey {
        case services
        case updatedAt = "updated_at"
        case count
    }
}

/// Per-service entry in the /system response. One row per backend
/// service (falcon-api, falcon-signal, etc.). `updatedAt` ticks on
/// every restart even if `publishDate` doesn't, so UI that wants a
/// "last alive" heartbeat can read the former while leaving the
/// semantic "when was this version published" to the latter.
struct SystemServiceEntry: Decodable, Hashable, Identifiable {
    let serviceName: String
    let publishDate: Date
    let updatedAt:   Date

    var id: String { serviceName }

    enum CodingKeys: String, CodingKey {
        case serviceName = "service_name"
        case publishDate = "publish_date"
        case updatedAt   = "updated_at"
    }
}
