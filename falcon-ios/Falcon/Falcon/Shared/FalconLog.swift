import Foundation
import OSLog

/// Centralised logger factory. Every `Logger` in the app goes through
/// `FalconLog.make(category:)` so we can gate *all* logging in one place.
///
/// Matrix of behaviour:
/// ```
///                DEBUG build          RELEASE build
/// Default        real Logger,         Logger(.disabled) — no-op,
///                full output           zero runtime cost
///
/// With override  real Logger,         real Logger iff UserDefaults
///                full output           "falcon_logs_enabled" == true
/// ```
///
/// The Release-mode override exists for TestFlight debugging: ship a
/// production binary but flip a single key (via a hidden Settings toggle,
/// `defaults write`, or a debug URL scheme) and logs light up again.
/// When the key is absent or false, `Logger(OSLog.disabled)` discards
/// every call before it does any string formatting, so even if call
/// sites use expensive interpolation there's no runtime penalty.
enum FalconLog {

    /// Subsystem shared by every category. Matches the bundle ID so
    /// Console.app filtering + os_log CLI work predictably.
    static let subsystem = "co.hblabs.falcon"

    /// UserDefaults key that flips Release-mode logging on. Safe to set
    /// from the app itself (a hidden Settings toggle) or from outside
    /// (USB → Xcode → Product → Edit Scheme → Arguments → launch args
    /// `-falcon_logs_enabled YES`).
    static let releaseOverrideKey = "falcon_logs_enabled"

    /// Returns a real `Logger` or a disabled one depending on build mode
    /// and the TestFlight override. Intended for use at file scope:
    ///
    /// ```swift
    /// private let log = FalconLog.make(category: "matches")
    /// ```
    static func make(category: String) -> Logger {
        #if DEBUG
        return Logger(subsystem: subsystem, category: category)
        #else
        if UserDefaults.standard.bool(forKey: releaseOverrideKey) {
            return Logger(subsystem: subsystem, category: category)
        }
        return Logger(OSLog.disabled)
        #endif
    }
}
