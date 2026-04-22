import SwiftUI

/// Floating status pill that surfaces realtime-socket health to the user.
///
/// Lifecycle:
///  1. Connection drops  → 2s debounce (ignore brief reconnect hiccups)
///     → show "offline" banner (red, antenna-slash icon). Persists until
///     either the socket returns or the user swipes it up.
///  2. Connection returns while "offline" is visible → morphs into the
///     "reconnected" banner (green, checkmark) and auto-dismisses after
///     5s.
///  3. User swipes up at any time → dismissed immediately; a later
///     disconnect re-triggers the flow fresh.
///
/// Mounted at MainTabView level so it's always on screen, with a 54pt
/// top padding to clear the navigation bar like LiveNewItemsBanner.
struct ServerStatusBanner: View {
    @Environment(RealtimeClient.self) var realtime
    @Environment(LanguageManager.self) var lm
    @Environment(\.scenePhase) private var scenePhase

    enum Status {
        case hidden
        case offline
        case reconnected
    }

    @State private var status: Status = .hidden
    /// Holds the 2s "is the disconnect real?" timer — cancelled if the
    /// socket reconnects before the timer fires. Prevents the banner
    /// from flashing on every momentary drop.
    @State private var offlineDebounce: Task<Void, Never>?
    /// Holds the 5s success-dismiss timer — cancelled if the socket
    /// flaps back offline before the timer fires.
    @State private var successDismiss: Task<Void, Never>?

    var body: some View {
        Group {
            if status != .hidden {
                banner
            }
        }
        .onChange(of: isConnected) { _, connected in
            // Connection events that happen while the app is not in the
            // foreground are ignored — otherwise a background socket
            // teardown + silent re-login on resume would always greet
            // the user with a "back online" banner they never asked for.
            guard scenePhase == .active else { return }
            handleConnectionChange(connected)
        }
        .onChange(of: scenePhase) { _, phase in
            if phase != .active {
                // Leaving foreground: wipe any pending work so we don't
                // quietly flip to `.offline` behind the user's back. On
                // return, the next genuine disconnect will start fresh.
                offlineDebounce?.cancel()
                successDismiss?.cancel()
                withAnimation(.easeOut(duration: 0.2)) {
                    status = .hidden
                }
            }
        }
    }

    private var isConnected: Bool {
        if case .connected = realtime.state { return true }
        return false
    }

    // MARK: - State machine

    private func handleConnectionChange(_ connected: Bool) {
        if !connected {
            // Drop happened. Start (or restart) the 2s debounce — long
            // enough to swallow instant reconnects (WiFi → cellular
            // handoff, app resume), short enough that a real outage
            // surfaces fast.
            successDismiss?.cancel()
            offlineDebounce?.cancel()
            offlineDebounce = Task { @MainActor in
                try? await Task.sleep(for: .seconds(2))
                if Task.isCancelled { return }
                withAnimation(.spring(response: 0.45, dampingFraction: 0.75)) {
                    status = .offline
                }
            }
        } else {
            // We're back. Cancel any pending "you're offline" pop-in;
            // only morph into "reconnected" if the user actually saw
            // the offline state. Otherwise there's nothing to celebrate.
            offlineDebounce?.cancel()
            guard status == .offline else { return }
            withAnimation(.spring(response: 0.45, dampingFraction: 0.75)) {
                status = .reconnected
            }
            successDismiss = Task { @MainActor in
                try? await Task.sleep(for: .seconds(5))
                if Task.isCancelled { return }
                withAnimation(.easeIn(duration: 0.18)) {
                    status = .hidden
                }
            }
        }
    }

    // MARK: - Visual

    private var banner: some View {
        HStack(alignment: .center, spacing: 10) {
            // Offline uses the iCloud-slashed glyph (wider iOS
            // availability than `cloud.slash.fill`), same metaphor as
            // the floating RealtimeStatusIcon. Reconnect stays on ✓
            // — universal success cue.
            Image(systemName: status == .offline ? "icloud.slash.fill" : "checkmark.circle.fill")
                .font(.system(size: 14, weight: .bold))
            VStack(alignment: .leading, spacing: 1) {
                Text(lm.t(status == .offline ? .serverOfflineTitle : .serverReconnectedTitle))
                    .font(.system(size: 13, weight: .bold, design: .rounded))
                Text(lm.t(status == .offline ? .serverOfflineBody : .serverReconnectedBody))
                    .font(.system(size: 10, weight: .medium))
                    .foregroundStyle(.white.opacity(0.85))
            }
            Spacer(minLength: 0)
        }
        .foregroundStyle(.white)
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
        .background(
            Capsule()
                .fill(status == .offline ? Color.red : Color.green)
                .shadow(color: .black.opacity(0.2), radius: 10, x: 0, y: 4)
        )
        .padding(.horizontal, 20)
        // Same offset used by LiveNewItemsBanner so both play nicely:
        // they clear the inline nav bar (~44pt) plus a bit of air.
        .padding(.top, 54)
        // Asymmetric transition: slide in from the top so the user
        // notices the arrival, but exit by dissolving in place —
        // a tiny scale(0.96) + opacity fade. No travel distance means
        // no "awkward half-slide" feel; the badge just disappears.
        .transition(.asymmetric(
            insertion: .move(edge: .top).combined(with: .opacity),
            removal:   .scale(scale: 0.96).combined(with: .opacity)
        ))
        // Swipe up to dismiss — matches the iOS "flick up to hide a
        // notification" muscle memory. Works for both offline and
        // reconnected states; a fresh disconnect re-triggers normally.
        .gesture(
            DragGesture(minimumDistance: 10)
                .onEnded { value in
                    if value.translation.height < -20 {
                        successDismiss?.cancel()
                        withAnimation(.easeIn(duration: 0.18)) {
                            status = .hidden
                        }
                    }
                }
        )
    }
}
