import SwiftUI
import UIKit // needed for UIPasteboard

struct ContactDrawerInfo: Identifiable {
    let id = UUID()
    let name: String
    let logoName: String
    let fallbackIcon: String
    let bio: String
    let websiteLabel: String
    let websiteURL: URL
    let emailURL: URL
}

struct SettingsView: View {
    @Environment(NotificationManager.self) var nm
    @Environment(LanguageManager.self) var lm
    @Environment(SessionManager.self) var session

    @Binding var contactDrawer: ContactDrawerInfo?

    var body: some View {
        NavigationStack {
            List {
                startSection
                statusSection
                languageSection
                tokenSection
                #if DEBUG
                configSection
                #endif
                aboutSection
                if session.isAuthenticated {
                    logoutSection
                }
            }
            .navigationTitle(lm.t(.tabSettings))
            .withLoginToolbar()
            .task { await nm.refreshStatus() }
            .safeAreaInset(edge: .bottom) { Color.clear.frame(height: 90) }
        }
    }

    // MARK: - Sections

    private var statusSection: some View {
        Section(lm.t(.sectionNotifications)) {
            HStack {
                Text(lm.t(.notifStatusLabel))
                Spacer()
                statusBadge
            }
            if nm.authStatus != .authorized {
                Button(lm.t(.notifEnableButton)) {
                    nm.requestPermission()
                }
            }
            if session.userID.isEmpty {
                noCVWarningRow
            }
        }
    }

    private var noCVWarningRow: some View {
        Label {
            VStack(alignment: .leading, spacing: 2) {
                Text(lm.t(.noCVWarningTitle))
                    .font(.system(size: 13, weight: .medium))
                    .foregroundStyle(.primary)
                Text(lm.t(.noCVWarningBody))
                    .font(.system(size: 12))
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
        } icon: {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(.orange)
        }
    }

    private var configSection: some View {
        Section(lm.t(.sectionConfiguration)) {
            LabeledContent(lm.t(.configAPIURL)) {
                TextField("http://localhost:8080", text: Binding(
                    get: { nm.apiURL },
                    set: { nm.devSetAPIURL($0) }
                ))
                .multilineTextAlignment(.trailing)
                .keyboardType(.URL)
                .autocorrectionDisabled()
                .textInputAutocapitalization(.never)
            }
            Button(action: register) {
                HStack {
                    Text(registerButtonLabel)
                    Spacer()
                    if case .registering = nm.signalStatus {
                        ProgressView()
                    }
                }
            }
            .disabled(!canRegister)

            if case .failed(let msg) = nm.signalStatus {
                Text(msg).font(.caption).foregroundStyle(.red)
            }
        }
    }

    private var languageSection: some View {
        Section(lm.t(.sectionLanguage)) {
            Picker(lm.t(.langAppLabel), selection: Binding(
                get: { lm.appLanguage },
                set: { lang in
                    lm.appLanguage = lang
                    saveLanguageConfig(lang)
                }
            )) {
                ForEach(AppLanguage.allCases) { lang in
                    Text("\(lang.flag) \(lang.displayName)").tag(lang)
                }
            }
        }
    }

    // MARK: - Config API

    private func saveLanguageConfig(_ lang: AppLanguage) {
        guard session.isAuthenticated,
              let url = URL(string: "\(nm.apiURL)/me/config"),
              let jwt = session.cachedJWT else { return }
        var req = URLRequest(url: url)
        req.httpMethod = "PUT"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.setValue("Bearer \(jwt)", forHTTPHeaderField: "Authorization")
        // Scope the setting to this specific device so a user with multiple
        // iOS devices can pick a different language on each. Signal reads the
        // same device_id when localizing match-result pushes.
        req.httpBody = try? JSONSerialization.data(withJSONObject: [
            "platform":  "ios",
            "device_id": KeychainHelper.deviceID,
            "name":      "app_language",
            "value":     lang.rawValue
        ])
        URLSession.shared.dataTask(with: req).resume()
    }

    private var tokenSection: some View {
        Section(lm.t(.sectionDeviceToken)) {
            if let token = nm.deviceToken {
                Text(token)
                    .font(.system(.caption2, design: .monospaced))
                    .foregroundStyle(.secondary)
                    .contextMenu {
                        Button(lm.t(.tokenCopy)) { UIPasteboard.general.string = token }
                    }
            } else if let err = nm.registrationError {
                Text(err).font(.caption).foregroundStyle(.red)
            } else {
                Text(lm.t(.tokenNone)).foregroundStyle(.secondary)
            }
        }
    }

    // private var lastNotificationSection: some View {
    //     Section(lm.t(.sectionLastNotification)) {
    //         if let n = nm.lastNotification {
    //             VStack(alignment: .leading, spacing: 4) {
    //                 Text(n.title).fontWeight(.semibold)
    //                 Text(n.body).foregroundStyle(.secondary)
    //                 Text(n.receivedAt.formatted(date: .omitted, time: .standard))
    //                     .font(.caption2)
    //                     .foregroundStyle(.tertiary)
    //             }
    //         } else {
    //             Text(lm.t(.lastNotifNone)).foregroundStyle(.secondary)
    //         }
    //     }
    // }

    private var startSection: some View {
        let version = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "—"
        let build   = Bundle.main.infoDictionary?["CFBundleVersion"] as? String ?? "—"

        return Section {
            VStack(spacing: 6) {
                FalconIconView(size: 60)
                Text("Falcon")
                    .font(.system(size: 17, weight: .semibold, design: .rounded))
                Link(destination: URL(string: "https://falcon.hblabs.co")!) {
                    Text("falcon.hblabs.co")
                        .font(.system(size: 12, weight: .medium))
                        .foregroundStyle(.blue)
                }
                Text(lm.t(.aboutCraftedIn))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Text("\(lm.t(.aboutCreatedBy)) **Helmer Barcos** · HB Labs SAS")
                    .font(.caption)
                    .foregroundStyle(.secondary)
                    .padding(.top, 2)
                HStack(spacing: 8) {
                    Text("\(lm.t(.aboutVersion)) \(version)")
                    Text("·").foregroundStyle(.quaternary)
                    Text("\(lm.t(.aboutBuild)) \(build)")
                }
                .font(.system(size: 11))
                .foregroundStyle(.quaternary)
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 8)
            .listRowBackground(Color.clear)
            .listSectionSeparator(.hidden)
        }
    }

    private var aboutSection: some View {

        return Group {

            // Creator card
            Section(lm.t(.aboutCreator)) {
                contactCard(
                    logoName: "CreatorLogo",
                    fallbackIcon: "person.fill",
                    name: "Helmer Barcos",
                    bio: String(format: lm.t(.aboutCreatorBio), yearsOfExperience),
                    websiteLabel: "barcos.co",
                    websiteURL: URL(string: "https://barcos.co")!,
                    emailURL: mailURL(to: "helmer@barcos.co")
                )
                .listRowInsets(EdgeInsets(top: 12, leading: 16, bottom: 12, trailing: 16))
            }

            // Company card
            Section(lm.t(.aboutCompany)) {
                contactCard(
                    logoName: "CompanyLogo",
                    fallbackIcon: "building.2.fill",
                    name: "HB Labs SAS",
                    bio: lm.t(.aboutCompanyBio),
                    websiteLabel: "hblabs.co",
                    websiteURL: URL(string: "https://hblabs.co")!,
                    emailURL: mailURL(to: "general@hblabs.co")
                )
                .listRowInsets(EdgeInsets(top: 12, leading: 16, bottom: 12, trailing: 16))
            }

              // GDPR note
            Section {
                Text(lm.t(.aboutGdprNote))
                    .font(.caption)
                    .foregroundStyle(.secondary)
                Link(destination: URL(string: "https://legal.mistral.ai/terms/privacy-policy")!) {
                    Label(lm.t(.aboutMistralPrivacy), systemImage: "lock.shield.fill")
                        .foregroundStyle(.blue)
                }
            } header: {
                Text("GDPR / DSGVO")
            }

            // Legal links
            Section {
                NavigationLink {
                    LegalDocumentView(filename: "privacy_policy")
                } label: {
                    Label(lm.t(.aboutPrivacyPolicy), systemImage: "lock.shield")
                }
                NavigationLink {
                    LegalDocumentView(filename: "terms_of_use")
                } label: {
                    Label(lm.t(.aboutTermsOfUse), systemImage: "doc.text")
                }
                Link(destination: URL(string: "https://github.com/hblabs-co/falcon")!) {
                    Label(lm.t(.aboutSourceCode), systemImage: "chevron.left.forwardslash.chevron.right")
                }
            }

        }
    }

    // MARK: - Logout

    private var logoutSection: some View {
        Section {
            Button(role: .destructive) {
                session.logout()
            } label: {
                HStack {
                    Spacer()
                    Text(lm.t(.settingsLogout))
                        .font(.system(size: 15, weight: .medium))
                    Spacer()
                }
            }
        } footer: {
            Text(session.email)
                .font(.caption2)
                .foregroundStyle(.tertiary)
                .frame(maxWidth: .infinity)
        }
    }

    // MARK: - Helpers

    private var yearsOfExperience: Int {
        let start = Calendar.current.date(from: DateComponents(year: 2016, month: 1, day: 1))!
        return Calendar.current.dateComponents([.year], from: start, to: Date()).year ?? 0
    }

    private func mailURL(to address: String) -> URL {
        let version = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "—"
        let build   = Bundle.main.infoDictionary?["CFBundleVersion"] as? String ?? "—"
        let body = "\n\n---\nUser ID: \(session.userID)\nVersion: \(version) (\(build))\nAPI URL: \(nm.apiURL)"
        let params = "subject=Falcon Support&body=\(body)"
            .addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? ""
        return URL(string: "mailto:\(address)?\(params)") ?? URL(string: "mailto:\(address)")!
    }

    @ViewBuilder
    private func contactCard(
        logoName: String,
        fallbackIcon: String,
        name: String,
        bio: String,
        websiteLabel: String,
        websiteURL: URL,
        emailURL: URL
    ) -> some View {
        Button {
            contactDrawer = ContactDrawerInfo(
                name: name,
                logoName: logoName,
                fallbackIcon: fallbackIcon,
                bio: bio,
                websiteLabel: websiteLabel,
                websiteURL: websiteURL,
                emailURL: emailURL
            )
        } label: {
            HStack(alignment: .center, spacing: 16) {
                logoImage(logoName, fallback: fallbackIcon, size: 120)

                VStack(alignment: .leading, spacing: 8) {
                    Text(name)
                        .font(.system(size: 20, weight: .semibold, design: .rounded))
                        .foregroundStyle(.primary)

                    Text(bio)
                        .font(.system(size: 11))
                        .foregroundStyle(.secondary)
                        .fixedSize(horizontal: false, vertical: true)

                    HStack(spacing: 6) {
                        HStack(spacing: 4) {
                            Image(systemName: "globe.americas.fill")
                            Text(websiteLabel)
                        }
                        HStack(spacing: 4) {
                            Image(systemName: "envelope.circle.fill")
                            Text(emailURL.host ?? "email")
                        }
                    }
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(.blue)
                }

                Spacer()
            }
        }
        .buttonStyle(.plain)
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
    private var statusBadge: some View {
        switch nm.authStatus {
        case .authorized:
            Label(lm.t(.notifStatusActive), systemImage: "checkmark.circle.fill").foregroundStyle(.green)
        case .denied:
            Label(lm.t(.notifStatusDenied), systemImage: "xmark.circle.fill").foregroundStyle(.red)
        case .provisional:
            Label(lm.t(.notifStatusProvisional), systemImage: "exclamationmark.circle.fill").foregroundStyle(.orange)
        default:
            Label(lm.t(.notifStatusPending), systemImage: "clock.circle.fill").foregroundStyle(.secondary)
        }
    }

    private var canRegister: Bool {
        nm.deviceToken != nil && !session.userID.isEmpty && !nm.apiURL.isEmpty
    }

    private var registerButtonLabel: String {
        switch nm.signalStatus {
        case .registered:  return lm.t(.configRegistered)
        case .registering: return lm.t(.configRegistering)
        default:           return lm.t(.configRegister)
        }
    }

    private func register() {
        nm.registerWithSignal(userID: session.userID)
    }
}
