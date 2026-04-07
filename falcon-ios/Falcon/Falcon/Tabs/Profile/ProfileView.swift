import SwiftUI
import UniformTypeIdentifiers

struct ProfileView: View {
    @Environment(LanguageManager.self) var lm
    @Environment(CVUploadViewModel.self) var vm
    @Environment(SessionManager.self) var session
    @State private var showFilePicker = false
    @State private var showLoginSheet = false

    private var isActivelyProcessing: Bool {
        switch vm.state {
        case .emailEntry, .uploading, .processing: return true
        default: return false
        }
    }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 20) {
                    if case .done = vm.state,
                       let cv = vm.normalizedCV?.lang(for: lm.appLanguage) {
                        CVProfileView(cv: cv, onReplace: { vm.reset() })
                    } else {
                        whyCVCard
                        CVUploadSection(vm: vm, showFilePicker: $showFilePicker)
                        howItWorksSection
                    }
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
                .padding(.bottom, 110)
            }
            .navigationTitle(lm.t(.tabProfile))
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    if session.isAuthenticated {
                        HStack(spacing: 6) {
                            Image(systemName: "checkmark.shield.fill")
                                .foregroundStyle(.green)
                            Text(session.email)
                                .font(.system(size: 13, weight: .medium))
                                .foregroundStyle(.secondary)
                                .lineLimit(1)
                        }
                    } else if !isActivelyProcessing {
                        Button {
                            showLoginSheet = true
                        } label: {
                            HStack(spacing: 5) {
                                Image(systemName: "person.badge.key.fill")
                                Text(lm.t(.profileLoginButton))
                            }
                            .font(.system(size: 14, weight: .medium))
                        }
                    }
                }
            }
            .sheet(isPresented: $showLoginSheet) {
                LoginSheet()
                    .presentationDetents([.medium])
                    .presentationCornerRadius(22)
                    .presentationDragIndicator(.visible)
            }
            .onChange(of: session.isAuthenticated) { _, authenticated in
                if authenticated { showLoginSheet = false }
            }
            .fileImporter(
                isPresented: $showFilePicker,
                allowedContentTypes: [UTType("org.openxmlformats.wordprocessingml.document")!],
                allowsMultipleSelection: false
            ) { result in
                guard case .success(let urls) = result, let url = urls.first else { return }
                _ = url.startAccessingSecurityScopedResource()
                vm.fileSelected(url)
            }
        }
    }

    // MARK: - How it works

    private var howItWorksSection: some View {
        VStack(alignment: .leading, spacing: 0) {
            Text(lm.t(.profileHowTitle))
                .font(.system(size: 13, weight: .semibold))
                .foregroundStyle(.tertiary)
                .textCase(.uppercase)
                .tracking(0.6)
                .padding(.bottom, 14)

            VStack(spacing: 0) {
                howStep(number: 1, icon: "arrow.up.doc.fill",  title: lm.t(.profileStep1Title), body: lm.t(.profileStep1Body), last: false)
                howStep(number: 2, icon: "sparkles",            title: lm.t(.profileStep2Title), body: lm.t(.profileStep2Body), last: false)
                howStep(number: 3, icon: "briefcase.fill",      title: lm.t(.profileStep3Title), body: lm.t(.profileStep3Body), last: true)
            }
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
    }

    // Fixed layout constants — connector math depends on these being stable.
    //   titleH (20) + vSpacing (4) + descH (36) + bottomGap (12) = 72 per non-last row
    //   circle (36) + connectorTopPad (4) + connectorH (32) = 72  ✓
    private let titleH:          CGFloat = 20
    private let descH:           CGFloat = 36
    private let vSpacing:        CGFloat = 4
    private let bottomGap:       CGFloat = 12
    private let circleSize:      CGFloat = 36
    private let connectorTopPad: CGFloat = 4
    // connectorH = titleH + vSpacing + descH + bottomGap − circleSize − connectorTopPad
    private var connectorH: CGFloat { titleH + vSpacing + descH + bottomGap - circleSize - connectorTopPad }

    private func howStep(number: Int, icon: String, title: String, body: String, last: Bool) -> some View {
        HStack(alignment: .top, spacing: 14) {
            // Icon column — zero-height connector on last so the column collapses correctly
            VStack(spacing: 0) {
                ZStack {
                    Circle()
                        .fill(Color.accentColor.opacity(0.1))
                        .frame(width: circleSize, height: circleSize)
                    Image(systemName: icon)
                        .font(.system(size: 15, weight: .medium))
                        .foregroundStyle(Color.accentColor)
                }
                Rectangle()
                    .fill(Color.accentColor.opacity(0.15))
                    .frame(width: 1.5, height: last ? 0 : connectorH)
                    .padding(.top, last ? 0 : connectorTopPad)
            }
            .frame(width: circleSize)

            // Text column — fixed heights so every row is identical regardless of language
            VStack(alignment: .leading, spacing: vSpacing) {
                HStack(spacing: 6) {
                    Text("\(number).")
                        .font(.system(size: 13, weight: .bold, design: .rounded))
                        .foregroundStyle(Color.accentColor)
                    Text(title)
                        .font(.system(size: 14, weight: .semibold))
                        .lineLimit(1)
                }
                .frame(height: titleH, alignment: .center)

                Text(body)
                    .font(.system(size: 13))
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
                    .frame(maxWidth: .infinity, minHeight: descH, maxHeight: descH, alignment: .topLeading)
            }
            .padding(.bottom, last ? 0 : bottomGap)
        }
    }

    // MARK: - Why CV card

    private var whyCVCard: some View {
        HStack(alignment: .top, spacing: 14) {
            Image(systemName: "sparkles")
                .font(.system(size: 22))
                .foregroundStyle(Color.accentColor)
                .frame(width: 32)
                .padding(.top, 2)

            VStack(alignment: .leading, spacing: 6) {
                Text(lm.t(.profileWhyTitle))
                    .font(.system(size: 15, weight: .semibold))

                Text(lm.t(.profileWhyBody))
                    .font(.system(size: 13))
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(Color.accentColor.opacity(0.07))
        )
    }
}

// MARK: - Processing Facts Animation

struct ProcessingFactsView: View {
    @Environment(LanguageManager.self) var lm
    @State private var currentIndex = 0

    private var facts: [String] {
        [
            lm.t(.processingFact1),
            lm.t(.processingFact2),
            lm.t(.processingFact3),
            lm.t(.processingFact4),
            lm.t(.processingFact5),
        ]
    }

    var body: some View {
        Text(facts[currentIndex])
            .font(.system(size: 12))
            .foregroundStyle(.tertiary)
            .multilineTextAlignment(.center)
            .fixedSize(horizontal: false, vertical: true)
            .id(currentIndex)
            .transition(.asymmetric(
                insertion: .move(edge: .bottom).combined(with: .opacity),
                removal: .move(edge: .top).combined(with: .opacity)
            ))
            .animation(.easeInOut(duration: 0.4), value: currentIndex)
            .onAppear { startTimer() }
    }

    private func startTimer() {
        Timer.scheduledTimer(withTimeInterval: 4, repeats: true) { _ in
            withAnimation {
                currentIndex = (currentIndex + 1) % facts.count
            }
        }
    }
}

// MARK: - CV Upload Section

struct CVUploadSection: View {
    let vm: CVUploadViewModel
    @Binding var showFilePicker: Bool
    @Environment(LanguageManager.self) var lm
    @Environment(SessionManager.self) var session

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Label(lm.t(.profileCVTitle), systemImage: "doc.text.fill")
                .font(.system(size: 15, weight: .semibold))

            switch vm.state {
            case .idle:
                dropzone
            case .emailEntry(let data, let filename):
                if session.isAuthenticated && !session.email.isEmpty {
                    // Auto-upload when logged in — no email prompt needed.
                    Color.clear.onAppear {
                        Task { await vm.upload(data: data, filename: filename, email: session.email) }
                    }
                } else {
                    EmailEntryView(vm: vm, data: data, filename: filename)
                }
            case .uploading(let progress):
                uploadingView(progress: progress)
            case .processing(let phase):
                processingView(phase: phase)
            case .done:
                doneView
            case .failed(let msg):
                failedView(msg.isEmpty ? lm.t(.profileCVProcessingFailed) : msg)
            }
        }
        .padding(16)
        .background(
            RoundedRectangle(cornerRadius: 18, style: .continuous)
                .fill(.background)
                .shadow(color: .black.opacity(0.06), radius: 14, x: 0, y: 4)
        )
    }

    // MARK: - Dropzone

    private var dropzone: some View {
        Button { showFilePicker = true } label: {
            VStack(spacing: 16) {
                Image(systemName: "arrow.up.doc.fill")
                    .font(.system(size: 44))
                    .foregroundStyle(Color.accentColor)

                VStack(spacing: 6) {
                    Text(lm.t(.profileCVDropzone))
                        .font(.system(size: 15, weight: .semibold))
                        .foregroundStyle(.primary)
                    Text(lm.t(.profileCVFormats))
                        .font(.system(size: 12))
                        .foregroundStyle(.tertiary)
                }
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 44)
            .background(
                RoundedRectangle(cornerRadius: 14, style: .continuous)
                    .strokeBorder(
                        Color.accentColor.opacity(0.35),
                        style: StrokeStyle(lineWidth: 1.5, dash: [7, 5])
                    )
            )
        }
        .contentShape(RoundedRectangle(cornerRadius: 14, style: .continuous))
        .buttonStyle(.plain)
    }

    // MARK: - Uploading

    private func uploadingView(progress: Double) -> some View {
        VStack(spacing: 16) {
            ProgressView(value: progress)
                .tint(Color.accentColor)

            HStack {
                Text(lm.t(.profileCVUploading))
                    .font(.system(size: 13, weight: .medium))
                    .foregroundStyle(.secondary)
                Spacer()
                Text("\(Int(progress * 100))%")
                    .font(.system(size: 13, weight: .semibold, design: .rounded))
                    .foregroundStyle(Color.accentColor)
            }
        }
        .padding(.vertical, 20)
    }

    // MARK: - Processing (indexing + normalizing)

    private func processingView(phase: CVUploadViewModel.ProcessingPhase) -> some View {
        VStack(spacing: 20) {
            // Step indicator
            HStack(spacing: 0) {
                stepCircle(done: true, active: false)
                stepLine(filled: true)
                stepCircle(done: phase == .normalizing, active: phase == .indexing)
                stepLine(filled: phase == .normalizing)
                stepCircle(done: false, active: phase == .normalizing)
            }

            // Active phase label
            HStack(spacing: 8) {
                ProgressView().tint(Color.accentColor)
                Text(phase == .indexing ? lm.t(.profileCVIndexing) : lm.t(.profileCVNormalizing))
                    .font(.system(size: 13, weight: .medium))
                    .foregroundStyle(.secondary)
            }

            // Rotating facts
            ProcessingFactsView()
        }
        .padding(.vertical, 20)
    }

    private func stepCircle(done: Bool, active: Bool) -> some View {
        ZStack {
            Circle()
                .fill(done ? Color.accentColor : active ? Color.accentColor.opacity(0.12) : Color.secondary.opacity(0.12))
                .frame(width: 30, height: 30)
            if done {
                Image(systemName: "checkmark")
                    .font(.system(size: 11, weight: .bold))
                    .foregroundStyle(.white)
            } else if active {
                ProgressView()
                    .scaleEffect(0.55)
                    .tint(Color.accentColor)
            }
        }
    }

    private func stepLine(filled: Bool) -> some View {
        Rectangle()
            .fill(filled ? Color.accentColor : Color.secondary.opacity(0.2))
            .frame(maxWidth: .infinity)
            .frame(height: 2)
    }

    // MARK: - Done

    private var doneView: some View {
        VStack(spacing: 12) {
            HStack(spacing: 12) {
                Image(systemName: "checkmark.circle.fill")
                    .font(.system(size: 28))
                    .foregroundStyle(.green)
                Text(lm.t(.profileCVUploadDone))
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(.primary)
                Spacer()
            }
            Button(lm.t(.profileCVUpload)) { vm.reset() }
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(.secondary)
                .frame(maxWidth: .infinity)
        }
        .padding(.vertical, 12)
    }

    // MARK: - Failed

    private func failedView(_ message: String) -> some View {
        VStack(spacing: 12) {
            HStack(spacing: 12) {
                Image(systemName: "xmark.circle.fill")
                    .font(.system(size: 24))
                    .foregroundStyle(.red)
                Text(message)
                    .font(.system(size: 13))
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
                Spacer()
            }
            Button(lm.t(.profileCVUploadFailed)) { vm.reset() }
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(Color.accentColor)
                .frame(maxWidth: .infinity)
        }
        .padding(.vertical, 12)
    }
}

// MARK: - Email Entry

struct EmailEntryView: View {
    let vm: CVUploadViewModel
    let data: Data
    let filename: String
    @Environment(LanguageManager.self) var lm
    @Environment(SessionManager.self) var session
    @State private var email = ""
    @FocusState private var focused: Bool

    private var isLoggedIn: Bool { session.isAuthenticated && !session.email.isEmpty }

    var body: some View {
        VStack(spacing: 16) {
            // Selected file indicator
            HStack(spacing: 10) {
                Image(systemName: "doc.fill")
                    .font(.system(size: 16))
                    .foregroundStyle(Color.accentColor)
                Text(filename)
                    .font(.system(size: 13, weight: .medium))
                    .foregroundStyle(.primary)
                    .lineLimit(1)
                    .truncationMode(.middle)
                Spacer()
                Button { vm.reset() } label: {
                    Image(systemName: "xmark.circle.fill")
                        .foregroundStyle(.tertiary)
                }
            }
            .padding(12)
            .background(
                RoundedRectangle(cornerRadius: 10, style: .continuous)
                    .fill(Color.accentColor.opacity(0.07))
            )

            // Email field
            VStack(alignment: .leading, spacing: 6) {
                Text(lm.t(.profileCVEmailLabel))
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundStyle(.secondary)
                    .textCase(.uppercase)
                    .tracking(0.4)

                TextField(lm.t(.profileCVEmailPlaceholder), text: $email)
                    .keyboardType(.emailAddress)
                    .textContentType(.emailAddress)
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)
                    .focused($focused)
                    .disabled(isLoggedIn)
                    .font(.system(size: 15))
                    .foregroundStyle(isLoggedIn ? .secondary : .primary)
                    .padding(.horizontal, 14)
                    .padding(.vertical, 12)
                    .background(
                        RoundedRectangle(cornerRadius: 10, style: .continuous)
                            .fill(isLoggedIn ? Color(UIColor.tertiarySystemBackground) : Color(UIColor.secondarySystemBackground))
                    )
                    .overlay(
                        RoundedRectangle(cornerRadius: 10, style: .continuous)
                            .strokeBorder(focused ? Color.accentColor : Color.clear, lineWidth: 1.5)
                    )

                if !isLoggedIn {
                    Text(lm.t(.profileCVEmailHint))
                        .font(.system(size: 11))
                        .foregroundStyle(.tertiary)
                }
            }

            // CTA
            Button {
                focused = false
                Task { await vm.upload(data: data, filename: filename, email: email) }
            } label: {
                Text(lm.t(.profileCVUploadStart))
                    .font(.system(size: 15, weight: .semibold))
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 13)
                    .background(
                        RoundedRectangle(cornerRadius: 12, style: .continuous)
                            .fill(isValidEmail ? Color.accentColor : Color.accentColor.opacity(0.35))
                    )
                    .foregroundStyle(.white)
            }
            .disabled(!isValidEmail)
        }
        .onAppear {
            if isLoggedIn {
                email = session.email
            } else {
                focused = true
            }
        }
    }

    private var isValidEmail: Bool {
        let pattern = #"^[^@\s]+@[^@\s]+\.[^@\s]+$"#
        return email.range(of: pattern, options: .regularExpression) != nil
    }
}

// MARK: - Login Sheet

struct LoginSheet: View {
    @Environment(LanguageManager.self) var lm
    @Environment(\.dismiss) private var dismiss
    @State private var email = ""
    @State private var isSending = false
    @State private var sent = false
    @State private var errorMessage: String?
    @FocusState private var focused: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            // Handle + title
            VStack(alignment: .leading, spacing: 6) {
                Text(lm.t(.profileLoginTitle))
                    .font(.system(size: 20, weight: .semibold, design: .rounded))

                Text(lm.t(.profileLoginEmailHint))
                    .font(.system(size: 14))
                    .foregroundStyle(.secondary)
                    .fixedSize(horizontal: false, vertical: true)
            }

            if sent {
                // Confirmation after magic link sent
                VStack(spacing: 14) {
                    Image(systemName: "envelope.badge.fill")
                        .font(.system(size: 40))
                        .foregroundStyle(Color.accentColor)
                    Text(lm.t(.loginSentTitle))
                        .font(.system(size: 16, weight: .semibold))
                    Text(try! AttributedString(markdown: lm.t(.loginSentBody).replacingOccurrences(of: "%@", with: email)))
                        .font(.system(size: 14))
                        .foregroundStyle(.secondary)
                        .multilineTextAlignment(.center)
                    Text(lm.t(.loginSentSpamHint))
                        .font(.system(size: 12))
                        .foregroundStyle(.tertiary)
                }
                .frame(maxWidth: .infinity)
                .padding(.vertical, 20)
            } else {
                // Email field
                TextField(lm.t(.profileCVEmailPlaceholder), text: $email)
                    .keyboardType(.emailAddress)
                    .textContentType(.emailAddress)
                    .autocorrectionDisabled()
                    .textInputAutocapitalization(.never)
                    .focused($focused)
                    .font(.system(size: 15))
                    .padding(.horizontal, 14)
                    .padding(.vertical, 13)
                    .background(
                        RoundedRectangle(cornerRadius: 12, style: .continuous)
                            .fill(Color(UIColor.secondarySystemBackground))
                    )
                    .overlay(
                        RoundedRectangle(cornerRadius: 12, style: .continuous)
                            .strokeBorder(focused ? Color.accentColor : Color.clear, lineWidth: 1.5)
                    )

                if let errorMessage {
                    Text(errorMessage)
                        .font(.system(size: 13))
                        .foregroundStyle(.red)
                }

                // CTA
                Button {
                    focused = false
                    Task { await sendMagicLink() }
                } label: {
                    HStack(spacing: 8) {
                        if isSending {
                            ProgressView().tint(.white)
                        }
                        Text(lm.t(.profileLoginCTA))
                            .font(.system(size: 15, weight: .semibold))
                    }
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 14)
                    .background(
                        RoundedRectangle(cornerRadius: 12, style: .continuous)
                            .fill(isValidEmail && !isSending ? Color.accentColor : Color.accentColor.opacity(0.35))
                    )
                    .foregroundStyle(.white)
                }
                .disabled(!isValidEmail || isSending)
            }

            Spacer()
        }
        .padding(24)
        .onAppear { focused = true }
    }

    private var isValidEmail: Bool {
        let pattern = #"^[^@\s]+@[^@\s]+\.[^@\s]+$"#
        return email.range(of: pattern, options: .regularExpression) != nil
    }

    @MainActor
    private func sendMagicLink() async {
        print("[login] sendMagicLink called for \(email)")
        isSending = true
        errorMessage = nil

        let base = NotificationManager.shared.apiURL
        guard let url = URL(string: "\(base)/auth/magic") else {
            print("[login] invalid url: \(base)/auth/magic")
            errorMessage = "Invalid server URL"
            isSending = false
            return
        }

        var req = URLRequest(url: url)
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try? JSONEncoder().encode(["email": email])

        print("[login] POST \(url)")

        do {
            let (data, response) = try await URLSession.shared.data(for: req)
            guard let http = response as? HTTPURLResponse else {
                print("[login] no HTTP response")
                errorMessage = "No response from server"
                isSending = false
                return
            }
            print("[login] status: \(http.statusCode)")
            if (200...299).contains(http.statusCode) {
                withAnimation { sent = true }
            } else {
                let body = String(data: data, encoding: .utf8) ?? "n/a"
                print("[login] error body: \(body)")
                let msg = (try? JSONDecoder().decode([String: String].self, from: data))?["error"]
                errorMessage = msg ?? "Request failed (\(http.statusCode))"
            }
        } catch {
            print("[login] exception: \(error)")
            errorMessage = error.localizedDescription
        }
        isSending = false
    }
}
