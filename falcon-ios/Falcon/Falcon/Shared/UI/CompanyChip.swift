import SwiftUI
import UIKit

/// Circular company avatar — renders the company logo when a local
/// cached file exists in the App Group container, falls back to a
/// two-letter initials chip otherwise.
///
/// Why disk-only and not `AsyncImage`:
///   - Live Activities / widgets render via snapshot; any network
///     request is racy with the snapshot deadline and usually loses.
///   - The companies cache (`CompaniesSync`) pre-downloads every
///     logo once a week to the App Group, so the main app never
///     needs to hit the network here either — one code path, same
///     behaviour across surfaces.
///   - A cache miss (user installed today, sync hasn't run yet) just
///     shows initials, which is the correct first-launch experience.
struct CompanyChip: View {
    let name: String
    /// Canonical MinIO URL used as the cache key. Same URL the server
    /// persists on `match_results.company_logo_url` and ships in
    /// Live Activity `companyLogoUrl`. Empty string means "no logo
    /// known" — initials fallback. Non-empty but file missing locally
    /// also falls back gracefully.
    let logoURL: String
    let size: CGFloat

    var body: some View {
        if let uiImage = cachedImage() {
            Image(uiImage: uiImage)
                .resizable()
                .scaledToFill()
                .frame(width: size, height: size)
                .clipShape(Circle())
        } else {
            CompanyInitialsChip(name: name, size: size)
        }
    }

    private func cachedImage() -> UIImage? {
        guard !logoURL.isEmpty,
              let local = CompanyLogoFilename.localURL(for: logoURL),
              FileManager.default.fileExists(atPath: local.path)
        else { return nil }
        return UIImage(contentsOfFile: local.path)
    }
}

/// Pure initials fallback exposed separately so callers that
/// deliberately don't have a logo URL (e.g. platform-only contexts)
/// can skip the AsyncImage attempt and avoid the empty-phase tick.
struct CompanyInitialsChip: View {
    let name: String
    let size: CGFloat

    private var initials: String {
        let words = name.split(separator: " ").prefix(2)
        return words.compactMap { $0.first }.map(String.init).joined().uppercased()
    }

    var body: some View {
        ZStack {
            Circle().fill(Color.accentColor.opacity(0.18))
            Text(initials)
                .font(.system(size: size * 0.48, weight: .bold, design: .rounded))
                .foregroundStyle(Color.accentColor)
        }
        .frame(width: size, height: size)
    }
}
