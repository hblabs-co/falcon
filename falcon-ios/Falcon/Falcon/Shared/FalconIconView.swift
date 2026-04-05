import SwiftUI

/// Displays the Falcon logo from the "FalconLogo" asset.
/// Falls back to a styled placeholder until the asset is added in Xcode.
///
/// To add the asset:
///   Assets.xcassets → + → New Image Set → name it "FalconLogo" → drag your PNG
struct FalconIconView: View {
    var size: CGFloat = 96
    var cornerRadius: CGFloat? = nil

    private var resolvedCornerRadius: CGFloat {
        cornerRadius ?? size * 0.225
    }

    var body: some View {
        Group {
            if UIImage(named: "FalconLogo") != nil {
                Image("FalconLogo")
                    .resizable()
                    .scaledToFit()
            } else {
                placeholder
            }
        }
        .frame(width: size, height: size)
        .clipShape(RoundedRectangle(cornerRadius: resolvedCornerRadius, style: .continuous))
    }

    private var placeholder: some View {
        ZStack {
            LinearGradient(
                colors: [Color.accentColor, Color.accentColor.opacity(0.7)],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
            Image(systemName: "bird.fill")
                .font(.system(size: size * 0.42, weight: .semibold))
                .foregroundStyle(.white)
        }
    }
}
