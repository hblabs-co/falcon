import Foundation

@Observable
final class CVUploadViewModel {

    enum ProcessingPhase {
        case indexing    // server: pending_upload / indexing / indexed
        case normalizing // server: normalizing
    }

    enum State {
        case idle
        case emailEntry(data: Data, filename: String)
        case uploading(progress: Double)
        case processing(ProcessingPhase)
        case done
        case failed(String)
    }

    var state: State = .idle
    var normalizedCV: NormalizedCVData? = nil

    private var pollingTask: Task<Void, Never>?

    /// Call from the fileImporter callback while the security-scoped resource is still open.
    func fileSelected(_ url: URL) {
        let accessed = url.startAccessingSecurityScopedResource()
        defer { if accessed { url.stopAccessingSecurityScopedResource() } }

        do {
            let data = try Data(contentsOf: url)
            print("[CV] file read ok — \(url.lastPathComponent) \(data.count) bytes")
            state = .emailEntry(data: data, filename: url.lastPathComponent)
        } catch {
            print("[CV] file read failed: \(error)")
            state = .failed("Could not read file: \(error.localizedDescription)")
        }
    }

    func reset() {
        pollingTask?.cancel()
        pollingTask = nil
        normalizedCV = nil
        state = .idle
    }

    /// Restores CV state from the server after app relaunch.
    /// Must only be called after session is fully restored and network is available.
    @MainActor
    func restoreFromServer() async {
        guard case .idle = state else { return }
        let session = SessionManager.shared
        guard session.isAuthenticated else { return }
        guard let url = URL(string: "\(apiBase)/me?platform=ios") else { return }
        guard let jwt = session.cachedJWT else { return }

        var req = URLRequest(url: url)
        req.setValue("Bearer \(jwt)", forHTTPHeaderField: "Authorization")
        do {
            let (data, response) = try await URLSession.shared.data(for: req)
            guard let http = response as? HTTPURLResponse, (200...299).contains(http.statusCode) else {
                print("[CV] restore: server returned \((response as? HTTPURLResponse)?.statusCode ?? -1)")
                return
            }
            let me = try JSONDecoder().decode(MeResponse.self, from: data)

            // Apply configs (language, etc.) from the same /me response.
            LanguageManager.shared.applyConfigs(me.configs)

            guard let cv = me.cv else {
                print("[CV] restore: no CV found for user")
                return
            }

            print("[CV] restore from server: cv_id=\(cv.id) status=\(cv.status)")

            if let normalized = cv.normalized {
                normalizedCV = normalized
                updateDisplayName(normalized)
            }
            applyServerStatus(cv.status)

            if case .processing = state {
                startPolling(cvID: cv.id)
            }
        } catch {
            print("[CV] restore error: \(error)")
        }
    }

    // MARK: - Upload pipeline

    func upload(data: Data, filename: String, email: String) async {
        do {
            state = .uploading(progress: 0)

            // Step 1: obtain pre-signed upload URL + cvId from falcon-api
            let prepared = try await prepare(filename: filename)

            // Step 2: PUT bytes to MinIO using the pre-signed URL
            try await uploadToStorage(urlString: prepared.uploadUrl, data: data, filename: filename)

            // Step 3: trigger indexing (server returns 202 immediately)
            state = .processing(.indexing)
            try await index(cvID: prepared.cvId, email: email)

            // Step 4: poll until server reaches normalized or failed
            startPolling(cvID: prepared.cvId)
        } catch {
            print("[CV] upload pipeline failed: \(error)")
            state = .failed(error.localizedDescription)
        }
    }

    // MARK: - Polling

    private func startPolling(cvID: String) {
        pollingTask?.cancel()
        pollingTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(5))
                guard !Task.isCancelled else { return }
                await self?.pollStatus(cvID: cvID)
            }
        }
    }

    @MainActor
    private func pollStatus(cvID: String) async {
        guard let url = URL(string: "\(apiBase)/cv/\(cvID)") else { return }
        do {
            let (data, response) = try await URLSession.shared.data(from: url)
            guard let http = response as? HTTPURLResponse, (200...299).contains(http.statusCode) else { return }
            let doc = try JSONDecoder().decode(CVStatusResponse.self, from: data)
            print("[CV] poll status: \(doc.status)")
            if let normalized = doc.normalized {
                normalizedCV = normalized
                updateDisplayName(normalized)
            }
            applyServerStatus(doc.status)
        } catch {
            print("[CV] poll error: \(error)")
        }
    }

    @MainActor
    private func applyServerStatus(_ status: String) {
        switch status {
        case "pending_upload", "uploaded", "indexing", "indexed":
            if case .processing(.indexing) = state { } else {
                state = .processing(.indexing)
            }
        case "normalizing":
            state = .processing(.normalizing)
        case "normalized":
            pollingTask?.cancel()
            state = .done
        case "failed":
            pollingTask?.cancel()
            state = .failed("")
        default:
            break
        }
    }

    // MARK: - Steps

    private func prepare(filename: String) async throws -> CVPreparedResponse {
        let url = URL(string: "\(apiBase)/cv/prepare")!
        var req = URLRequest(url: url)
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONEncoder().encode(["filename": filename])

        let (data, response) = try await URLSession.shared.data(for: req)
        print("[CV] prepare status: \((response as? HTTPURLResponse)?.statusCode ?? -1)")
        try assertHTTP(response, context: "prepare")
        return try JSONDecoder().decode(CVPreparedResponse.self, from: data)
    }

    private func uploadToStorage(urlString: String, data: Data, filename: String) async throws {
        guard let url = URL(string: urlString) else {
            throw CVUploadError.invalidURL
        }
        var req = URLRequest(url: url)
        req.httpMethod = "PUT"
        req.setValue("application/vnd.openxmlformats-officedocument.wordprocessingml.document",
                     forHTTPHeaderField: "Content-Type")

        let delegate = UploadProgressDelegate { [weak self] progress in
            Task { @MainActor [weak self] in
                self?.state = .uploading(progress: progress)
            }
        }
        let session = URLSession(configuration: .default, delegate: delegate, delegateQueue: nil)
        let (_, response) = try await session.upload(for: req, from: data)
        print("[CV] storage upload status: \((response as? HTTPURLResponse)?.statusCode ?? -1)")
        try assertHTTP(response, context: "storage upload")
    }

    private func index(cvID: String, email: String) async throws {
        let url = URL(string: "\(apiBase)/cv/\(cvID)/index")!
        var req = URLRequest(url: url)
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONEncoder().encode(["email": email])

        let (_, response) = try await URLSession.shared.data(for: req)
        try assertHTTP(response, context: "index")
    }

    // MARK: - Helpers

    private func updateDisplayName(_ cv: NormalizedCVData) {
        let lang = LanguageManager.shared.appLanguage
        if let l = cv.lang(for: lang), let name = l.fullName {
            SessionManager.shared.displayName = name
        }
    }

    private var apiBase: String { NotificationManager.shared.apiURL }

    private func assertHTTP(_ response: URLResponse, context: String) throws {
        guard let http = response as? HTTPURLResponse, (200...299).contains(http.statusCode) else {
            throw CVUploadError.httpError(context: context)
        }
    }
}

// MARK: - Supporting types

struct CVPreparedResponse: Decodable {
    let cvId: String
    let uploadUrl: String
    let expiresAt: String

    enum CodingKeys: String, CodingKey {
        case cvId      = "cv_id"
        case uploadUrl = "upload_url"
        case expiresAt = "expires_at"
    }
}

struct CVStatusResponse: Decodable {
    let status: String
    let normalized: NormalizedCVData?
}

struct MeResponse: Decodable {
    let configs: [String: AnyCodable]?
    let cv: MeCVResponse?
}

struct MeCVResponse: Decodable {
    let id: String
    let status: String
    let normalized: NormalizedCVData?
}

/// Minimal wrapper to decode heterogeneous JSON values in configs.
struct AnyCodable: Decodable {
    let value: Any
    init(from decoder: Decoder) throws {
        let c = try decoder.singleValueContainer()
        if let v = try? c.decode(String.self)  { value = v }
        else if let v = try? c.decode(Bool.self)   { value = v }
        else if let v = try? c.decode(Int.self)    { value = v }
        else if let v = try? c.decode(Double.self) { value = v }
        else { value = "" }
    }
}

// MARK: - Normalized CV model

struct NormalizedCVData: Decodable {
    let de: Lang?
    let en: Lang?
    let es: Lang?

    func lang(for language: AppLanguage) -> Lang? {
        switch language {
        case .english: return en
        case .german:  return de
        case .spanish: return es
        }
    }

    struct Lang: Decodable {
        let firstName: String?
        let lastName: String?
        let summary: String?
        let experience: [ExperienceEntry]
        let technologies: Technologies

        enum CodingKeys: String, CodingKey {
            case firstName = "first_name"
            case lastName  = "last_name"
            case summary, experience, technologies
        }

        init(from decoder: Decoder) throws {
            let c      = try decoder.container(keyedBy: CodingKeys.self)
            firstName  = try c.decodeIfPresent(String.self, forKey: .firstName)
            lastName   = try c.decodeIfPresent(String.self, forKey: .lastName)
            summary    = try c.decodeIfPresent(String.self, forKey: .summary)
            experience = (try? c.decode([ExperienceEntry].self, forKey: .experience)) ?? []
            technologies = (try? c.decode(Technologies.self, forKey: .technologies)) ?? Technologies()
        }

        var fullName: String? {
            [firstName, lastName].compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: " ").nilIfEmpty
        }

        var initials: String {
            let parts = [firstName?.first, lastName?.first].compactMap { $0 }
            return parts.isEmpty ? "?" : String(parts.map { String($0) }.joined()).uppercased()
        }
    }

    struct ExperienceEntry: Decodable, Identifiable {
        let id = UUID()
        let company: String?
        let role: String?
        let start: String?
        let end: String?
        let duration: String?
        let shortDescription: String?
        let longDescription: String?
        let highlights: [String]
        let tasks: [String]
        let technologies: [String]

        enum CodingKeys: String, CodingKey {
            case company, role, start, end, duration, highlights, tasks, technologies
            case shortDescription = "short_description"
            case longDescription  = "long_description"
        }

        init(from decoder: Decoder) throws {
            let c    = try decoder.container(keyedBy: CodingKeys.self)
            company  = try c.decodeIfPresent(String.self, forKey: .company)
            role     = try c.decodeIfPresent(String.self, forKey: .role)
            start    = try c.decodeIfPresent(String.self, forKey: .start)
            end      = try c.decodeIfPresent(String.self, forKey: .end)
            duration = try c.decodeIfPresent(String.self, forKey: .duration)
            shortDescription = try c.decodeIfPresent(String.self, forKey: .shortDescription)
            longDescription  = try c.decodeIfPresent(String.self, forKey: .longDescription)
            highlights   = (try? c.decode([String].self, forKey: .highlights))   ?? []
            tasks        = (try? c.decode([String].self, forKey: .tasks))        ?? []
            technologies = (try? c.decode([String].self, forKey: .technologies)) ?? []
        }
    }

    struct Technologies: Decodable {
        let frontend:  [String]
        let backend:   [String]
        let databases: [String]
        let devops:    [String]
        let tools:     [String]
        let others:    [String]

        init() { frontend = []; backend = []; databases = []; devops = []; tools = []; others = [] }

        init(from decoder: Decoder) throws {
            let c     = try decoder.container(keyedBy: CodingKeys.self)
            frontend  = (try? c.decode([String].self, forKey: .frontend))  ?? []
            backend   = (try? c.decode([String].self, forKey: .backend))   ?? []
            databases = (try? c.decode([String].self, forKey: .databases)) ?? []
            devops    = (try? c.decode([String].self, forKey: .devops))    ?? []
            tools     = (try? c.decode([String].self, forKey: .tools))     ?? []
            others    = (try? c.decode([String].self, forKey: .others))    ?? []
        }

        enum CodingKeys: String, CodingKey {
            case frontend, backend, databases, devops, tools, others
        }
    }
}

enum CVUploadError: LocalizedError {
    case invalidURL
    case httpError(context: String)

    var errorDescription: String? {
        switch self {
        case .invalidURL:           return "Invalid upload URL received from server."
        case .httpError(let ctx):   return "Request failed (\(ctx))."
        }
    }
}

private final class UploadProgressDelegate: NSObject, URLSessionTaskDelegate {
    private let onProgress: (Double) -> Void

    init(onProgress: @escaping (Double) -> Void) {
        self.onProgress = onProgress
    }

    func urlSession(
        _ session: URLSession,
        task: URLSessionTask,
        didSendBodyData bytesSent: Int64,
        totalBytesSent: Int64,
        totalBytesExpectedToSend: Int64
    ) {
        guard totalBytesExpectedToSend > 0 else { return }
        onProgress(Double(totalBytesSent) / Double(totalBytesExpectedToSend))
    }
}
