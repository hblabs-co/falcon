import Foundation
import UserNotifications
import UIKit

@Observable
final class NotificationManager: NSObject, UNUserNotificationCenterDelegate {

    static let shared = NotificationManager()

    var authStatus: UNAuthorizationStatus = .notDetermined
    var deviceToken: String? {
        didSet { UserDefaults.standard.set(deviceToken, forKey: "apns_device_token") }
    }
    var registrationError: String?
    var signalStatus: SignalStatus = .idle
    var lastNotification: ReceivedNotification?

    // Signal URL source of truth:
    // - DEBUG   → UserDefaults (editable from Settings) with Config.signalURL as fallback
    // - Release → hardcoded production falcon-api URL, never user-editable
    private(set) var signalURL: String

    enum SignalStatus {
        case idle, registering, registered
        case failed(String)
    }

    struct ReceivedNotification {
        let title: String
        let body: String
        let receivedAt: Date
    }

    private override init() {
        #if DEBUG
        signalURL = UserDefaults.standard.string(forKey: "signal_url") ?? Config.signalURL
        #else
        signalURL = "https://api.falcon.hblabs.co" // TODO: confirm production falcon-api URL
        #endif
        super.init()
        deviceToken = UserDefaults.standard.string(forKey: "apns_device_token")
    }

#if DEBUG
    func devSetSignalURL(_ url: String) {
        signalURL = url
        UserDefaults.standard.set(url, forKey: "signal_url")
    }
#endif

    // MARK: - Permission

    func requestPermission() {
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge]) { granted, _ in
            Task { @MainActor in
                await self.refreshStatus()
                if granted {
                    UIApplication.shared.registerForRemoteNotifications()
                }
            }
        }
    }

    func refreshStatus() async {
        let settings = await UNUserNotificationCenter.current().notificationSettings()
        authStatus = settings.authorizationStatus
    }

    // MARK: - Token

    func onTokenReceived(_ token: String) {
        deviceToken = token
        registrationError = nil
    }

    func onRegistrationFailed(_ error: Error) {
        registrationError = error.localizedDescription
    }

    // MARK: - Register with falcon-signal

    func registerWithSignal(userID: String) {
        guard let token = deviceToken else { return }
        guard let url = URL(string: "\(signalURL)/device-token") else { return }

        signalStatus = .registering

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try? JSONSerialization.data(withJSONObject: [
            "user_id": userID,
            "token": token
        ])

        URLSession.shared.dataTask(with: request) { _, response, error in
            Task { @MainActor in
                if let error {
                    self.signalStatus = .failed(error.localizedDescription)
                    return
                }
                let code = (response as? HTTPURLResponse)?.statusCode ?? 0
                self.signalStatus = code == 200 ? .registered : .failed("HTTP \(code)")
            }
        }.resume()
    }

    // MARK: - Foreground notifications

    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        let content = notification.request.content
        Task { @MainActor in
            self.lastNotification = ReceivedNotification(
                title: content.title,
                body: content.body,
                receivedAt: Date()
            )
        }
        completionHandler([.banner, .sound, .badge])
    }
}
