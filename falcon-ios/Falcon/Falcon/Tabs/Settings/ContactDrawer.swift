import SwiftUI
import UIKit

struct ContactDrawer: View {
    let info: ContactDrawerInfo
    let lm: LanguageManager
    let onDismiss: () -> Void

    @State private var isVisible = false
    @Environment(\.openURL) private var openURL

    var body: some View {
        ZStack(alignment: .bottom) {
            // Dim background
            Color.black.opacity(0.35)
                .ignoresSafeArea()
                .onTapGesture { dismiss() }

            // Drawer card
            VStack(spacing: 0) {
                // Handle
                Capsule()
                    .fill(.quaternary)
                    .frame(width: 36, height: 4)
                    .padding(.top, 12)
                    .padding(.bottom, 16)

                // Card header: logo + name + bio
                HStack(alignment: .top, spacing: 14) {
                    logoImage(info.logoName, fallback: info.fallbackIcon, size: 56)

                    VStack(alignment: .leading, spacing: 5) {
                        Text(info.name)
                            .font(.system(size: 16, weight: .semibold, design: .rounded))
                        Text(info.bio)
                            .font(.system(size: 12))
                            .foregroundStyle(.secondary)
                            .fixedSize(horizontal: false, vertical: true)
                    }

                    Spacer()
                }
                .padding(.horizontal, 20)
                .padding(.bottom, 16)

                Divider()

                // Visit website
                drawerButton(
                    icon: "globe.americas.fill",
                    label: lm.t(.drawerVisitWebsite),
                    subtitle: info.websiteLabel,
                    color: .blue
                ) {
                    openURL(info.websiteURL)
                    dismiss()
                }

                Divider().padding(.leading, 56)

                // Send email
                drawerButton(
                    icon: "envelope.circle.fill",
                    label: lm.t(.drawerSendEmail),
                    subtitle: info.emailURL.absoluteString
                        .replacingOccurrences(of: "mailto:", with: "")
                        .components(separatedBy: "?").first ?? "",
                    color: .blue
                ) {
                    openURL(info.emailURL)
                    dismiss()
                }

                Divider()

                // Cancel
                Button(role: .cancel) {
                    dismiss()
                } label: {
                    Text(lm.t(.drawerCancel))
                        .font(.system(size: 17, weight: .medium))
                        .foregroundStyle(.primary)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 18)
                }
            }
            .background(
                RoundedRectangle(cornerRadius: 24, style: .continuous)
                    .fill(.regularMaterial)
            )
            .padding(.horizontal, 8)
            .padding(.bottom, 12)
            .offset(y: isVisible ? 0 : 400)
        }
        .opacity(isVisible ? 1 : 0)
        .onAppear {
            withAnimation(.spring(response: 0.4, dampingFraction: 0.82)) {
                isVisible = true
            }
        }
    }

    private func dismiss() {
        withAnimation(.spring(response: 0.3, dampingFraction: 0.9)) {
            isVisible = false
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.25) {
            onDismiss()
        }
    }

    @ViewBuilder
    private func logoImage(_ name: String, fallback: String, size: CGFloat) -> some View {
        if UIImage(named: name) != nil {
            Image(name)
                .resizable()
                .scaledToFill()
                .frame(width: size, height: size)
                .clipShape(RoundedRectangle(cornerRadius: size * 0.25, style: .continuous))
        } else {
            ZStack {
                RoundedRectangle(cornerRadius: size * 0.25, style: .continuous)
                    .fill(.quaternary)
                    .frame(width: size, height: size)
                Image(systemName: fallback)
                    .font(.system(size: size * 0.45))
                    .foregroundStyle(.secondary)
            }
        }
    }

    @ViewBuilder
    private func drawerButton(
        icon: String,
        label: String,
        subtitle: String,
        color: Color = .primary,
        action: @escaping () -> Void
    ) -> some View {
        Button(action: action) {
            HStack(spacing: 14) {
                Image(systemName: icon)
                    .font(.system(size: 24))
                    .foregroundStyle(color)
                    .frame(width: 32)

                VStack(alignment: .leading, spacing: 2) {
                    Text(label)
                        .font(.system(size: 16, weight: .medium))
                        .foregroundStyle(.primary)
                    Text(subtitle)
                        .font(.system(size: 12))
                        .foregroundStyle(.secondary)
                }

                Spacer()
            }
            .padding(.horizontal, 20)
            .padding(.vertical, 14)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }
}
