import SwiftUI

/// A layout that arranges subviews in horizontal rows, wrapping to the next
/// row when the available width is exceeded — like CSS flex-wrap.
struct FlowLayout: Layout {
    var spacing: CGFloat = 6

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let rows = computeRows(proposal: proposal, subviews: subviews)
        let height = rows.enumerated().reduce(CGFloat.zero) { acc, pair in
            let rowHeight = pair.element.map { $0.size.height }.max() ?? 0
            return acc + rowHeight + (pair.offset > 0 ? spacing : 0)
        }
        return CGSize(width: proposal.width ?? 0, height: height)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        let rows = computeRows(proposal: proposal, subviews: subviews)
        var y = bounds.minY
        for row in rows {
            let rowHeight = row.map { $0.size.height }.max() ?? 0
            var x = bounds.minX
            for item in row {
                item.subview.place(at: CGPoint(x: x, y: y), proposal: ProposedViewSize(item.size))
                x += item.size.width + spacing
            }
            y += rowHeight + spacing
        }
    }

    private struct Item {
        let subview: LayoutSubview
        let size: CGSize
    }

    private func computeRows(proposal: ProposedViewSize, subviews: Subviews) -> [[Item]] {
        let maxWidth = proposal.width ?? .infinity
        var rows: [[Item]] = [[]]
        var rowWidth: CGFloat = 0

        for subview in subviews {
            let size = subview.sizeThatFits(.unspecified)
            if !rows[rows.count - 1].isEmpty && rowWidth + spacing + size.width > maxWidth {
                rows.append([])
                rowWidth = 0
            }
            if !rows[rows.count - 1].isEmpty { rowWidth += spacing }
            rows[rows.count - 1].append(Item(subview: subview, size: size))
            rowWidth += size.width
        }

        return rows
    }
}
