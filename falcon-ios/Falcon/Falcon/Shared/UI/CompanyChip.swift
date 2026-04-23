import SwiftUI

/// Circular company avatar — renders the company logo when a URL is
/// available and falls back to a two-letter initials chip otherwise
/// (including while the image is in flight or on load failure).
///
/// Used across every surface that shows a company: match cards, match
/// detail, project detail, and — with its widget-target twin
/// (CompanyLogoOrInitials in FalconWidgetLiveActivity) — Live Activities.
/// Keeping one source of truth here means an ACME chip in the list
/// looks identical to the ACME chip in the detail sheet.
struct CompanyChip: View {
    let name: String
    let logoURL: URL?
    let size: CGFloat

    var body: some View {
        if let url = logoURL {
            AsyncImage(url: url) { phase in
                switch phase {
                case .success(let img):
                    img.resizable().scaledToFill()
                default:
                    CompanyInitialsChip(name: name, size: size)
                }
            }
            .frame(width: size, height: size)
            .clipShape(Circle())
        } else {
            CompanyInitialsChip(name: name, size: size)
        }
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
