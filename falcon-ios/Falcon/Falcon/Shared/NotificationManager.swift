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

    // API URL source of truth:
    // - DEBUG   → UserDefaults (editable from Settings) with Config.apiURL as fallback
    // - Release → hardcoded production falcon-api URL, never user-editable
    private(set) var apiURL: String

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
        apiURL = UserDefaults.standard.string(forKey: "api_url") ?? Config.apiURL
        #else
        apiURL = "https://api.falcon.hblabs.co" // TODO: confirm production falcon-api URL
        #endif
        super.init()
        deviceToken = UserDefaults.standard.string(forKey: "apns_device_token")
        SessionManager.shared.onUserIDAvailable = { [weak self] id in
            self?.registerWithSignal(userID: id)
        }
    }

#if DEBUG
    func devSetAPIURL(_ url: String) {
        apiURL = url
        UserDefaults.standard.set(url, forKey: "api_url")
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
        let userID = SessionManager.shared.userID
        if !userID.isEmpty {
            registerWithSignal(userID: userID)
        }
    }

    func onRegistrationFailed(_ error: Error) {
        registrationError = error.localizedDescription
    }

    // MARK: - Register with falcon-api

    func registerWithSignal(userID: String) {
        guard let token = deviceToken else { return }
        guard let url = URL(string: "\(apiURL)/device-token") else { return }

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
