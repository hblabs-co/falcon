import Foundation
import CryptoKit
import UIKit
import OSLog

/// Unified-logging channel for realtime diagnostics. Unlike print(), this
/// works in Xcode's "Wait for the executable to be launched" mode AND in
/// Console.app. All interpolated values must be `privacy: .public` or they
/// render as "<private>".
private let rtLog = FalconLog.make(category: "realtime")

/// Event-name vocabulary shared with falcon-realtime. Kept in lockstep
/// with common/models/realtime.go — if you add/rename here, do the same
/// there. Names mirror the Go constants (strip the "Realtime" prefix;
/// case intentionally matches Go's PascalCase on purpose).
enum RealtimeEvent {
    static let SessionStarted      = "session_started"
    static let AppOpened           = "app_opened"
    static let AppForegrounded     = "app_foregrounded"
    static let AppBackgrounded     = "app_backgrounded"
    static let NotificationOpen    = "notification_opened"
    static let LiveActivityOpen    = "live_activity_opened"
    static let ProjectViewed       = "project_viewed"
    static let MatchViewed         = "match_viewed"
    static let OriginalOpened      = "original_opened"
    static let ContactCalled       = "contact_called"
    static let ContactEmailed      = "contact_emailed"
    static let MagicLinkRequested  = "magic_link_requested"
    // Server-emitted only — kept here for completeness / parity with Go:
    static let DeviceOnline        = "device_online"
    static let DeviceOffline       = "device_offline"
    static let UserBound           = "user_bound"
    static let UserUnbound         = "user_unbound"
}

/// Client for falcon-realtime. Keeps one WebSocket connection alive for the
/// lifetime of the app session, signs the handshake with HMAC-SHA256 (the
/// same scheme the server's auth.go verifies), auto-reconnects with
/// exponential backoff, and dispatches server push envelopes via
/// notifications.
///
/// Architecture mirrors NotificationManager: singleton + @Observable so
/// SwiftUI views can reflect connection state. Events the client emits
/// upstream are best-effort — if the socket is down, the event is dropped
/// (persistence is the server's job; we don't queue locally).
@Observable
@MainActor
final class RealtimeClient: NSObject {

    static let shared = RealtimeClient()

    // MARK: - Observable state

    enum ConnectionState {
        case idle
        case connecting
        case connected
        case disconnected(reason: String)
    }

    private(set) var state: ConnectionState = .idle

    // MARK: - Private

    private var task: URLSessionWebSocketTask?
    private var session: URLSession?
    private var reconnectTask: Task<Void, Never>?
    private var readTask: Task<Void, Never>?
    private var keepaliveTask: Task<Void, Never>?
    private var appOpenedTask: Task<Void, Never>?
    /// Set by noteAppOpenSource before the delayed app_opened emit fires.
    /// Lets deep-link handlers (onOpenURL for falcon://auth) override the
    /// default source="foreground" with e.g. "magic_link". Cleared after
    /// each emit so the next app_opened starts fresh.
    private var pendingAppOpenSource: String?
    /// Flipped to true after the first scheduleAppOpened fires in this
    /// process. Subsequent calls emit app_foregrounded instead of
    /// app_opened so cold launches vs warm resumes stay separately
    /// countable.
    private var appOpenedEmitted = false
    /// How this session was triggered. First-setter wins — set by whoever
    /// fires first among: authHandler (magic_link), didReceive
    /// (push_notification), live activity deep link, SessionManager
    /// restore (faceid). Read by emitSessionStartedOnce.
    private var pendingSessionSource: String = ""
    private var reconnectAttempt = 0
    /// user_id captured at the last successful handshake. Compared against
    /// SessionManager on each state change so a login/logout triggers a
    /// reconnect (handshake headers are immutable mid-connection).
    private var boundUserID: String = ""
    /// Once true, subsequent connect() calls (rebind on login, reconnect
    /// after network hiccup) will NOT re-emit session_started. "Session"
    /// here means "process lifetime" — one per app launch.
    private var sessionStartedEmitted = false

    private override init() { super.init() }

    // MARK: - Public API

    /// Connects to falcon-realtime. Safe to call multiple times; duplicate
    /// calls while already connecting/connected are no-ops. Invoke from
    /// app launch and again whenever SessionManager.userID changes.
    func connect() {
        if case .connected = state { return }
        if case .connecting = state { return }

        let currentUser = SessionManager.shared.userID
        boundUserID = currentUser
        state = .connecting

        guard let url = URL(string: realtimeURL()) else {
            state = .disconnected(reason: "invalid REALTIME_URL")
            return
        }

        let timestamp = String(Int(Date().timeIntervalSince1970))
        let deviceID = KeychainHelper.deviceID
        let platform = "ios"
        let signature = sign(timestamp: timestamp, deviceID: deviceID, platform: platform)

        var request = URLRequest(url: url)
        request.setValue(timestamp,   forHTTPHeaderField: "X-Falcon-Timestamp")
        request.setValue(deviceID,    forHTTPHeaderField: "X-Falcon-Device-ID")
        request.setValue(platform,    forHTTPHeaderField: "X-Falcon-Platform")
        request.setValue(currentUser, forHTTPHeaderField: "X-Falcon-User-ID")
        request.setValue(signature,   forHTTPHeaderField: "X-Falcon-Signature")

        // Own the URLSession so we get didCompleteWithError callbacks — the
        // shared session silently swallows them, making reconnect logic
        // unreliable. timeoutIntervalForRequest bounds how long the WS
        // upgrade handshake (or any idle period) is allowed to hang —
        // 60s is the iOS default and fits our 30s keep-alive cadence.
        // We intentionally do NOT set timeoutIntervalForResource: that
        // caps the TOTAL task lifetime, which for a persistent WebSocket
        // would kill the connection on a timer (default: 7 days, fine).
        let config = URLSessionConfiguration.default
        config.waitsForConnectivity = false
        config.timeoutIntervalForRequest = 60
        let s = URLSession(configuration: config, delegate: self, delegateQueue: nil)
        session = s
        task = s.webSocketTask(with: request)
        task?.resume()

        // State stays .connecting until the delegate's didOpenWithProtocol
        // confirms the WS upgrade. If the server is down, didCompleteWithError
        // fires first and scheduleReconnect flips us back to .disconnected —
        // the green indicator never turns on.
        reconnectAttempt = 0
        startReadLoop()
        startKeepalive()
    }

    /// Sends a WebSocket ping every 30s. URLSessionWebSocketTask doesn't
    /// reliably reply to server-sent pings, so we drive the heartbeat
    /// from this side — the server's PingHandler resets its read deadline
    /// on each ping, keeping the connection alive indefinitely (as long
    /// as the app is foreground). Stopped in disconnect().
    private func startKeepalive() {
        keepaliveTask?.cancel()
        rtLog.info("keepalive: starting")
        keepaliveTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(nanoseconds: 30 * 1_000_000_000)
                if Task.isCancelled {
                    rtLog.info("keepalive: cancelled")
                    return
                }
                guard let self, let task = await self.task else {
                    rtLog.info("keepalive: task gone — exiting")
                    return
                }
                rtLog.info("keepalive: sending ping")
                task.sendPing { error in
                    if let error {
                        rtLog.error("keepalive: ping FAILED: \(error.localizedDescription, privacy: .public)")
                    } else {
                        rtLog.info("keepalive: pong received")
                    }
                }
            }
        }
    }

    /// Disconnects and disables auto-reconnect. Called on explicit logout
    /// or when the app wants to pause the stream (e.g. on entering
    /// background for long periods — the OS will reap the connection
    /// anyway, but disconnecting explicitly avoids a noisy "lost connection"
    /// log on resume).
    func disconnect(reason: String = "client") {
        reconnectTask?.cancel()
        reconnectTask = nil
        readTask?.cancel()
        readTask = nil
        keepaliveTask?.cancel()
        keepaliveTask = nil
        task?.cancel(with: .goingAway, reason: Data(reason.utf8))
        task = nil
        session?.invalidateAndCancel()
        session = nil
        state = .disconnected(reason: reason)
    }

    /// Sends a user_bind / user_unbind frame over the existing socket when
    /// the session user_id changes (login, restore, logout). No reconnect:
    /// the server updates its hub mapping in place, so there's exactly one
    /// device_online/device_offline per real connection. user_bound and
    /// user_unbound are recorded separately for analytics.
    func rebindIfUserChanged() {
        let current = SessionManager.shared.userID
        if current == boundUserID { return }
        boundUserID = current

        guard let task else { return }
        let payload: [String: Any] = current.isEmpty
            ? ["event": "user_unbind"]
            : ["event": "user_bind", "user_id": current]
        guard let data = try? JSONSerialization.data(withJSONObject: payload),
              let text = String(data: data, encoding: .utf8) else { return }
        task.send(.string(text)) { error in
            if let error {
                rtLog.error("rebind send failed: \(error.localizedDescription, privacy: .public)")
            }
        }
    }

    /// Fires an event up to falcon-realtime. Returns immediately — delivery
    /// is best-effort; if the socket is disconnected the event is dropped
    /// (the server never knew about it). Use for analytics-style events
    /// that are fine to lose; use falcon-api for anything transactional.
    func emit(_ event: String, metadata: [String: Any]? = nil) {
        guard let task else { return }
        var payload: [String: Any] = ["event": event]
        if let metadata { payload["metadata"] = metadata }
        guard let data = try? JSONSerialization.data(withJSONObject: payload),
              let text = String(data: data, encoding: .utf8) else { return }
        task.send(.string(text)) { error in
            if let error {
                rtLog.error("emit \(event, privacy: .public) failed: \(error.localizedDescription, privacy: .public)")
            }
        }
    }

    // MARK: - Well-known events (thin wrappers for discoverability)

    /// Emits session_started at most once per process, AND only once the
    /// handshake is bound to a real user_id. Pre-login launches (where
    /// the socket is up but no one is logged in) are not counted as
    /// sessions — we'd rather know about authenticated sessions than
    /// every app open by an anonymous visitor. Call this from every
    /// place that could move us into the "user known" state: scenePhase
    /// .active (in case restore was instant) and RootView's onChange(of:
    /// session.userID) (when login/restore finishes after launch).
    func emitSessionStartedOnce() {
        guard !sessionStartedEmitted else { return }
        guard !boundUserID.isEmpty else { return }
        // Don't consume the once-flag if we can't actually send — otherwise
        // a call that arrives before the socket is ready (e.g. RootView's
        // userID onChange firing before connect() finished) would silently
        // discard the event AND block all future attempts.
        guard task != nil else { return }
        sessionStartedEmitted = true
        let sys = UIDeviceInfo.current
        // source = how the user authenticated this session. "faceid"
        // means Keychain/biometric restore, "magic_link" means a fresh
        // email-link sign-in. SessionManager sets it in restore()/saveJWT().
        emit(RealtimeEvent.SessionStarted, metadata: [
            "os":          sys.os,
            "os_version":  sys.osVersion,
            "app_version": sys.appVersion,
            "model":       sys.model,
            // Whichever entry path fired first: "magic_link" | "faceid" |
            // "push_notification" | "live_activity". Falls back to "unknown"
            // only if no source was noted (shouldn't normally happen since
            // restore() always notes "faceid" and login flows note their
            // own before session_started can emit).
            "source": pendingSessionSource.isEmpty ? "unknown" : pendingSessionSource
        ])
    }

    /// Emits app_backgrounded and awaits the socket flush before returning.
    /// Called from scenePhase → .background so the server sees the event
    /// arrive just before the connection closes. If the app is *killed*
    /// (swipe up, crash), this never runs — the server falls back to its
    /// 90s read deadline to reap the dead connection.
    func emitAppBackgroundedAndClose() async {
        guard let task else { return }
        let payload: [String: Any] = ["event": RealtimeEvent.AppBackgrounded]
        if let data = try? JSONSerialization.data(withJSONObject: payload),
           let text = String(data: data, encoding: .utf8) {
            await withCheckedContinuation { cont in
                task.send(.string(text)) { _ in cont.resume() }
            }
        }
        disconnect(reason: "app_background")
    }

    /// Schedules an app_opened emit for the NEXT main-actor tick. That's
    /// enough to let SwiftUI finish the current view update, during which
    /// `.onOpenURL` also fires — so if a deep-link handler noted a source
    /// (e.g. "magic_link") synchronously, the emit picks it up. No sleep:
    /// Task.yield returns to the scheduler, any pending synchronous work
    /// in the same tick runs, then the continuation resumes and emits.
    func scheduleAppOpened(defaultSource: String) {
        rtLog.error("scheduleAppOpened(defaultSource=\(defaultSource, privacy: .public)), current pending=\(self.pendingAppOpenSource ?? "nil", privacy: .public)")
        appOpenedTask?.cancel()
        appOpenedTask = Task { @MainActor [weak self] in
            // Defer to the NEXT main runloop iteration. A plain Task.yield
            // only hops the Swift concurrency scheduler and isn't enough —
            // SwiftUI delivers onOpenURL for Dynamic Island taps in a
            // later runloop iteration than scenePhase .active. Bridging
            // through DispatchQueue.main guarantees we resume after the
            // whole current iteration is done, giving every URL path
            // (Lock Screen, DI compact, DI expanded) a chance to set the
            // source before we emit.
            await withCheckedContinuation { cont in
                DispatchQueue.main.async { cont.resume() }
            }
            if Task.isCancelled { return }
            guard let self else { return }
            let source = self.pendingAppOpenSource ?? defaultSource
            rtLog.error("app_opened microtask resolved source=\(source, privacy: .public) (pending was \(self.pendingAppOpenSource ?? "nil", privacy: .public))")
            self.pendingAppOpenSource = nil
            let event = self.appOpenedEmitted
                ? RealtimeEvent.AppForegrounded
                : RealtimeEvent.AppOpened
            self.appOpenedEmitted = true
            self.emitAppOpenedNow(event: event, source: source)
        }
    }

    /// Called by deep-link handlers (onOpenURL) to set the source of the
    /// pending app_opened. Must be invoked BEFORE the scheduled emit ticks
    /// — in practice that means "in the same SwiftUI view update as
    /// scenePhase .active", which is how onOpenURL delivers.
    func noteAppOpenSource(_ source: String) {
        pendingAppOpenSource = source
    }

    /// Records how this session was triggered. First-setter wins, so every
    /// entry path can call this unconditionally and the earliest signal
    /// (notification tap, magic-link URL, live-activity URL, faceid
    /// restore) becomes session_started's source. Later callers no-op.
    func noteSessionSource(_ source: String) {
        if pendingSessionSource.isEmpty {
            pendingSessionSource = source
        }
    }

    private func emitAppOpenedNow(event: String, source: String) {
        let sys = UIDeviceInfo.current
        let locale = Locale.current
        emit(event, metadata: [
            "source":      source,           // "foreground" | "magic_link" | ...
            "os":          sys.os,           // "iOS"
            "os_version":  sys.osVersion,    // "17.5.1"
            "app_version": sys.appVersion,   // "1.3.0 (42)"
            "model":       sys.model,        // "iPhone"
            "locale":      locale.identifier,                  // "de_DE"
            "language":    LanguageManager.shared.appLanguage.rawValue, // "de"/"en"/"es"
            "timezone":    TimeZone.current.identifier,        // "Europe/Berlin"
            "device_id":   KeychainHelper.deviceID             // persistent per install
        ])
    }

    func emitProjectViewed(projectID: String, source: String = "unknown") {
        emit(RealtimeEvent.ProjectViewed, metadata: [
            "project_id": projectID,
            "source":     source
        ])
    }

    func emitMatchViewed(projectID: String, cvID: String) {
        emit(RealtimeEvent.MatchViewed, metadata: ["project_id": projectID, "cv_id": cvID])
    }

    func emitOriginalOpened(projectID: String, source: String = "unknown") {
        emit(RealtimeEvent.OriginalOpened, metadata: [
            "project_id": projectID,
            "source":     source
        ])
    }

    func emitContactCalled(projectID: String, source: String = "unknown") {
        emit(RealtimeEvent.ContactCalled, metadata: [
            "project_id": projectID,
            "source":     source
        ])
    }

    func emitContactEmailed(projectID: String, source: String = "unknown") {
        emit(RealtimeEvent.ContactEmailed, metadata: [
            "project_id": projectID,
            "source":     source
        ])
    }

    // MARK: - Read loop

    /// Reads server frames until the socket closes. Each frame is a JSON
    /// envelope `{"type": "...", "payload": {...}}` — we forward it via
    /// NotificationCenter so any view can subscribe without coupling to
    /// this client.
    private func startReadLoop() {
        rtLog.info("readLoop: starting")
        // Capture the task we started with. If disconnect+connect swaps
        // the task mid-flight (rebind on login), receive() on the old
        // task throws — we must NOT call scheduleReconnect in that case,
        // otherwise we'd tear down the fresh connection that just opened.
        guard let ownTask = self.task else { return }
        readTask = Task { [weak self] in
            guard let self else { return }
            while !Task.isCancelled {
                do {
                    let message = try await ownTask.receive()
                    switch message {
                    case .string(let text):
                        rtLog.info("readLoop: received text (\(text.count, privacy: .public) chars)")
                        self.dispatch(text: text)
                    case .data(let data):
                        rtLog.info("readLoop: received data (\(data.count, privacy: .public) bytes)")
                        if let text = String(data: data, encoding: .utf8) {
                            self.dispatch(text: text)
                        }
                    @unknown default:
                        break
                    }
                } catch {
                    // Only reconnect if we are STILL the active read loop.
                    // Otherwise a stale error from a swapped-out task would
                    // tear down the live connection.
                    guard ownTask === self.task else {
                        rtLog.info("readLoop: stale error ignored (\(error.localizedDescription, privacy: .public))")
                        return
                    }
                    rtLog.error("readLoop: ERROR — \(error.localizedDescription, privacy: .public) — reconnecting")
                    self.scheduleReconnect(reason: "read: \(error.localizedDescription)")
                    return
                }
            }
            rtLog.info("readLoop: exited cleanly (cancelled)")
        }
    }

    private func dispatch(text: String) {
        guard let data = text.data(using: .utf8),
              let obj  = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let type = obj["type"] as? String else { return }
        let payload = obj["payload"] as? [String: Any] ?? [:]
        NotificationCenter.default.post(
            name: .realtimeMessage,
            object: nil,
            userInfo: ["type": type, "payload": payload]
        )
    }

    // MARK: - Reconnect

    /// Exponential backoff — 1s, 2s, 4s, 8s, capped at 30s. Resets on each
    /// successful connection. Runs at most one reconnect task at a time.
    private func scheduleReconnect(reason: String) {
        rtLog.error("scheduleReconnect: reason=\(reason, privacy: .public), attempt=\(self.reconnectAttempt + 1, privacy: .public)")
        state = .disconnected(reason: reason)
        task = nil
        keepaliveTask?.cancel()
        keepaliveTask = nil
        session?.invalidateAndCancel()
        session = nil
        reconnectTask?.cancel()

        reconnectAttempt += 1
        let delay = min(pow(2.0, Double(reconnectAttempt - 1)), 30)
        rtLog.info("scheduleReconnect: sleeping \(delay, privacy: .public)s before reconnect")

        reconnectTask = Task { [weak self] in
            try? await Task.sleep(nanoseconds: UInt64(delay * 1_000_000_000))
            if Task.isCancelled { return }
            await MainActor.run {
                self?.connect()
            }
        }
    }

    // MARK: - HMAC

    /// Matches common signing used by the Go server — see
    /// falcon-realtime/realtime/auth.go. "timestamp|device_id|platform"
    /// signed with the shared secret. Keep this composition stable —
    /// rotating the key means touching both sides in lockstep.
    private func sign(timestamp: String, deviceID: String, platform: String) -> String {
        let secret = realtimeSecret()
        let key = SymmetricKey(data: Data(secret.utf8))
        let payload = "\(timestamp)|\(deviceID)|\(platform)"
        let mac = HMAC<SHA256>.authenticationCode(for: Data(payload.utf8), using: key)
        return Data(mac).map { String(format: "%02x", $0) }.joined()
    }

    // MARK: - Config resolution

    private func realtimeURL() -> String {
        #if DEBUG
        return UserDefaults.standard.string(forKey: "realtime_url")
            ?? Config.realtimeURL
        #else
        return "wss://realtime.falcon.hblabs.co/ws"
        #endif
    }

    private func realtimeSecret() -> String {
        #if DEBUG
        return UserDefaults.standard.string(forKey: "realtime_secret")
            ?? Config.realtimeSecret
        #else
        // Production secret — same constant the falcon-realtime binary is
        // deployed with. Release builds ship this baked into the app.
        return Config.realtimeSecret
        #endif
    }
}

// MARK: - URLSessionWebSocketDelegate

extension RealtimeClient: URLSessionWebSocketDelegate {
    nonisolated func urlSession(
        _ session: URLSession,
        webSocketTask: URLSessionWebSocketTask,
        didOpenWithProtocol protocol: String?
    ) {
        rtLog.info("delegate: didOpenWithProtocol")
        Task { @MainActor in
            self.state = .connected
            self.reconnectAttempt = 0
        }
    }

    nonisolated func urlSession(
        _ session: URLSession,
        webSocketTask: URLSessionWebSocketTask,
        didCloseWith closeCode: URLSessionWebSocketTask.CloseCode,
        reason: Data?
    ) {
        let reasonStr = reason.flatMap { String(data: $0, encoding: .utf8) } ?? "code=\(closeCode.rawValue)"
        rtLog.info("delegate: didCloseWith code=\(closeCode.rawValue, privacy: .public) reason=\(reasonStr, privacy: .public)")
        Task { @MainActor in
            // Ignore callbacks for URLSessions we've already replaced
            // (rebind on login, manual reconnect). The old session fires
            // didCompleteWithError on cancel and would otherwise trigger
            // a ghost reconnect cycle.
            guard session === self.session else {
                rtLog.info("delegate: close from stale session ignored")
                return
            }
            self.scheduleReconnect(reason: "closed: \(reasonStr)")
        }
    }

    nonisolated func urlSession(
        _ session: URLSession,
        task: URLSessionTask,
        didCompleteWithError error: Error?
    ) {
        if let error {
            rtLog.error("delegate: didCompleteWithError \(error.localizedDescription, privacy: .public)")
            Task { @MainActor in
                guard session === self.session else {
                    rtLog.info("delegate: error from stale session ignored")
                    return
                }
                self.scheduleReconnect(reason: "net: \(error.localizedDescription)")
            }
        } else {
            rtLog.info("delegate: didCompleteWithError (nil error — clean finish)")
        }
    }
}

// MARK: - NotificationCenter name

extension Notification.Name {
    /// Posted whenever falcon-realtime delivers a push envelope. userInfo:
    /// `["type": String, "payload": [String: Any]]`. Subscribers switch
    /// on `type` — currently "match.result" and "project.normalized".
    static let realtimeMessage = Notification.Name("falcon.realtime.message")
}

// MARK: - Device info

/// Minimal wrapper around UIDevice + Bundle so the session_started
/// payload stays consistent across the codebase. Kept here because
/// it's only used for realtime telemetry.
private struct UIDeviceInfo {
    let os: String
    let osVersion: String
    let appVersion: String
    let model: String

    static var current: UIDeviceInfo {
        let device = UIDevice.current
        let version = Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? ""
        let build = Bundle.main.infoDictionary?["CFBundleVersion"] as? String ?? ""
        return UIDeviceInfo(
            os:         device.systemName,
            osVersion:  device.systemVersion,
            appVersion: "\(version) (\(build))",
            model:      device.model
        )
    }
}

