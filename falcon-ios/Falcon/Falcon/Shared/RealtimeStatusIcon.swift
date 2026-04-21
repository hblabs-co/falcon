import SwiftUI

/// Tiny connection indicator for falcon-realtime. Only two visual states —
/// online or offline — so the UI doesn't flicker through "connecting"
/// during every reconnect attempt. The underlying RealtimeClient state
/// machine is richer; this view collapses it to a boolean.
struct RealtimeStatusIcon: View {
    @Environment(RealtimeClient.self) var realtime

    private var isConnected: Bool {
        if case .connected = realtime.state { return true }
        return false
    }

    var body: some View {
        ZStack {
            Circle()
                .fill(tint.opacity(0.22))
                .frame(width: 22, height: 22)
            Image(systemName: isConnected ? "dot.radiowaves.left.and.right" : "wifi.slash")
                .font(.system(size: 10, weight: .bold))
                .foregroundStyle(tint)
        }
        .frame(width: 22, height: 22)
        .accessibilityLabel(isConnected ? "Realtime connected" : "Realtime offline")
    }

    private var tint: Color {
        isConnected ? .green : .secondary
    }
}
