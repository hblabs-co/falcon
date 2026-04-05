import SwiftUI

struct JobsView: View {
    @Environment(LanguageManager.self) var lm

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 16) {
                    heroBanner
                    jobsList
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
                .padding(.bottom, 110)
            }
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .principal) {
                    Text(lm.t(.tabJobs))
                        .font(.system(size: 17, weight: .semibold, design: .rounded))
                }
            }
        }
    }

    // MARK: - Hero banner

    private var heroBanner: some View {
        HStack(spacing: 14) {
            FalconIconView(size: 48, cornerRadius: 11)

            VStack(alignment: .leading, spacing: 3) {
                Text("Falcon")
                    .font(.system(size: 18, weight: .bold, design: .rounded))
                Text(lm.t(.jobsBannerTagline))
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(.secondary)
            }

            Spacer()

            VStack(alignment: .trailing, spacing: 2) {
                Text("—")
                    .font(.system(size: 22, weight: .bold, design: .rounded))
                Text(lm.t(.jobsBannerMatchCount))
                    .font(.system(size: 10, weight: .medium))
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.horizontal, 18)
        .padding(.vertical, 14)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.07), radius: 14, x: 0, y: 4)
        )
    }

    // MARK: - Cards

    private var jobsList: some View {
        LazyVStack(spacing: 14) {
            ForEach(0..<6, id: \.self) { _ in
                JobCard()
            }
        }
    }
}

struct JobCard: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(alignment: .top, spacing: 12) {
                RoundedRectangle(cornerRadius: 10, style: .continuous)
                    .fill(.quaternary)
                    .frame(width: 46, height: 46)

                VStack(alignment: .leading, spacing: 5) {
                    RoundedRectangle(cornerRadius: 4)
                        .fill(.quaternary)
                        .frame(width: 170, height: 13)
                    RoundedRectangle(cornerRadius: 4)
                        .fill(.quaternary)
                        .frame(width: 110, height: 11)
                }

                Spacer()

                RoundedRectangle(cornerRadius: 6)
                    .fill(.quaternary)
                    .frame(width: 48, height: 20)
            }

            VStack(alignment: .leading, spacing: 6) {
                RoundedRectangle(cornerRadius: 4)
                    .fill(.quaternary)
                    .frame(maxWidth: .infinity)
                    .frame(height: 11)
                RoundedRectangle(cornerRadius: 4)
                    .fill(.quaternary)
                    .frame(width: 220, height: 11)
            }

            HStack(spacing: 8) {
                ForEach(0..<3, id: \.self) { _ in
                    RoundedRectangle(cornerRadius: 20)
                        .fill(.quaternary)
                        .frame(width: 64, height: 24)
                }
            }

            HStack {
                Spacer()
                RoundedRectangle(cornerRadius: 8)
                    .fill(.quaternary)
                    .frame(width: 100, height: 28)
            }
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
    }
}
