import Foundation
import LocalAuthentication
import Security

enum KeychainHelper {

    private static let service = "co.hblabs.falcon"
    private static let jwtAccount = "jwt"

    // MARK: - JWT (biometry-protected)

    /// Saves the JWT to Keychain protected by the current biometry set (Face ID / Touch ID).
    /// If biometry changes (e.g. new finger enrolled), the item is automatically invalidated.
    static func saveJWT(_ token: String) throws {
        let data = Data(token.utf8)

        // Delete any existing item first so we can re-add with fresh access control.
        deleteJWT()

        var error: Unmanaged<CFError>?
        guard let access = SecAccessControlCreateWithFlags(
            kCFAllocatorDefault,
            kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly,
            .biometryCurrentSet,
            &error
        ) else {
            throw KeychainError.accessControlCreation(error?.takeRetainedValue())
        }

        let query: [CFString: Any] = [
            kSecClass:              kSecClassGenericPassword,
            kSecAttrService:        service,
            kSecAttrAccount:        jwtAccount,
            kSecValueData:          data,
            kSecAttrAccessControl:  access,
        ]

        let status = SecItemAdd(query as CFDictionary, nil)
        guard status == errSecSuccess else {
            throw KeychainError.unexpectedStatus(status)
        }
    }

    /// Reads the JWT from Keychain. Triggers the biometric prompt if not yet authenticated.
    /// Pass a `localizedReason` string shown in the Face ID / Touch ID prompt.
    static func readJWT(reason: String = "Authenticate to continue") throws -> String {
        let context = LAContext()
        context.localizedReason = reason

        let query: [CFString: Any] = [
            kSecClass:                  kSecClassGenericPassword,
            kSecAttrService:            service,
            kSecAttrAccount:            jwtAccount,
            kSecReturnData:             true,
            kSecMatchLimit:             kSecMatchLimitOne,
            kSecUseAuthenticationContext: context,
        ]

        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)

        switch status {
        case errSecSuccess:
            guard let data = item as? Data, let token = String(data: data, encoding: .utf8) else {
                throw KeychainError.invalidData
            }
            return token
        case errSecItemNotFound:
            throw KeychainError.notFound
        case errSecUserCanceled, -128: // -128 = errSecAuthFailed / user cancel
            throw KeychainError.userCancelled
        default:
            throw KeychainError.unexpectedStatus(status)
        }
    }

    @discardableResult
    static func deleteJWT() -> Bool {
        let query: [CFString: Any] = [
            kSecClass:       kSecClassGenericPassword,
            kSecAttrService: service,
            kSecAttrAccount: jwtAccount,
        ]
        return SecItemDelete(query as CFDictionary) == errSecSuccess
    }
}

// MARK: - Errors

enum KeychainError: LocalizedError {
    case accessControlCreation(CFError?)
    case unexpectedStatus(OSStatus)
    case invalidData
    case notFound
    case userCancelled

    var errorDescription: String? {
        switch self {
        case .accessControlCreation(let e): return "Access control error: \(e?.localizedDescription ?? "unknown")"
        case .unexpectedStatus(let s):      return "Keychain status: \(s)"
        case .invalidData:                  return "Keychain data could not be decoded."
        case .notFound:                     return "No session found."
        case .userCancelled:                return "Authentication cancelled."
        }
    }
}
