import SwiftUI

struct LoginToolbarModifier: ViewModifier {
    var showEmail = false
    @Environment(LanguageManager.self) var lm
    @Environment(SessionManager.self) var session
    @State private var showLoginSheet = false

    func body(content: Content) -> some View {
        content
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    if session.isAuthenticated {
                        HStack(spacing: 6) {
                            Image(systemName: "checkmark.shield.fill")
                                .foregroundStyle(.green)
                            Text(displayText)
                                .font(.system(size: 13, weight: .medium))
                                .foregroundStyle(.secondary)
                                .lineLimit(1)
                        }
                        .padding(.horizontal, 8)
                    } else {
                        Button {
                            showLoginSheet = true
                        } label: {
                            HStack(spacing: 5) {
                                Image(systemName: "person.badge.key.fill")
                                Text(lm.t(.profileLoginButton))
                            }
                            .font(.system(size: 14, weight: .medium))
                            .padding(.horizontal, 8)
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
    }

    private var displayText: String {
        if showEmail {
            return session.email
        }
        return session.displayName.isEmpty ? session.email : session.displayName
    }
}

extension View {
    func withLoginToolbar(showEmail: Bool = false) -> some View {
        modifier(LoginToolbarModifier(showEmail: showEmail))
    }
}
