#if canImport(ActivityKit)
import ActivityKit

/// Shared between Falcon (main app) and FalconWidget targets.
/// In Xcode: select this file → File Inspector → Target Membership → tick both Falcon and FalconWidget.
struct FalconJobsAttributes: ActivityAttributes {
    struct ContentState: Codable, Hashable {
        var projectCount: Int
        var latestTitle: String
    }
}
#endif
