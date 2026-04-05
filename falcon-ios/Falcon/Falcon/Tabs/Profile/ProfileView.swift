import SwiftUI

struct ProfileView: View {
    @Environment(LanguageManager.self) var lm
    @Environment(SessionManager.self) var session

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 20) {
                    avatarSection
                    cvSection
                    skillsSection
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
                .padding(.bottom, 110)
            }
            .navigationTitle(lm.t(.tabProfile))
        }
    }

    // MARK: - Avatar

    private var avatarSection: some View {
        VStack(spacing: 12) {
            ZStack {
                Circle()
                    .fill(.quaternary)
                    .frame(width: 80, height: 80)
                Image(systemName: "person.fill")
                    .font(.system(size: 36))
                    .foregroundStyle(.tertiary)
            }

            VStack(spacing: 4) {
                Text(lm.t(.profileAnonymous))
                    .font(.system(size: 18, weight: .semibold, design: .rounded))

                Text(session.userID)
                    .font(.system(.caption2, design: .monospaced))
                    .foregroundStyle(.tertiary)
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
        }
        .frame(maxWidth: .infinity)
        .padding(20)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
    }

    // MARK: - CV

    private var cvSection: some View {
        VStack(alignment: .leading, spacing: 14) {
            Label(lm.t(.profileCVTitle), systemImage: "doc.text.fill")
                .font(.system(size: 15, weight: .semibold))

            HStack(spacing: 14) {
                ZStack {
                    RoundedRectangle(cornerRadius: 12, style: .continuous)
                        .fill(.quaternary)
                        .frame(width: 52, height: 52)
                    Image(systemName: "doc.badge.plus")
                        .font(.system(size: 22))
                        .foregroundStyle(.tertiary)
                }

                VStack(alignment: .leading, spacing: 4) {
                    Text(lm.t(.profileCVNone))
                        .font(.subheadline)
                        .foregroundStyle(.secondary)
                    Text(lm.t(.profileCVHint))
                        .font(.caption)
                        .foregroundStyle(.tertiary)
                }

                Spacer()
            }

            Button {
                // TODO: upload CV — pending falcon-api
            } label: {
                Label(lm.t(.profileCVUpload), systemImage: "arrow.up.doc")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.borderedProminent)
            .disabled(true)
            .opacity(0.5)

            Text(lm.t(.profileCVUploadPending))
                .font(.caption2)
                .foregroundStyle(.tertiary)
                .frame(maxWidth: .infinity, alignment: .center)
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
    }

    // MARK: - Skills

    private var skillsSection: some View {
        VStack(alignment: .leading, spacing: 14) {
            Label(lm.t(.profileSkillsTitle), systemImage: "star.fill")
                .font(.system(size: 15, weight: .semibold))

            Text(lm.t(.profileSkillsNone))
                .font(.subheadline)
                .foregroundStyle(.tertiary)
                .frame(maxWidth: .infinity, alignment: .center)
                .padding(.vertical, 20)
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
    }
}
