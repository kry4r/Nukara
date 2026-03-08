import Foundation

final class MockAuthRepository: AuthRepositoryProtocol {
    private let keychain: KeychainStore

    init(keychain: KeychainStore) {
        self.keychain = keychain
    }

    func restoreSession() async -> AuthSession? {
        guard let token = keychain.load("access_token") else { return nil }
        let user = User(id: "mock-user", nickname: "Nukara User", avatarURL: nil)
        return AuthSession(accessToken: token, refreshToken: nil, user: user)
    }

    func requestEmailCode(email: String, purpose: AuthCodePurpose) async throws {
        let normalizedEmail = email.trimmingCharacters(in: .whitespacesAndNewlines)
        guard normalizedEmail.contains("@") else {
            throw NetworkError.transport("请输入正确邮箱")
        }
        _ = purpose
    }

    func login(email: String, emailCode: String) async throws -> AuthSession {
        let normalizedEmail = email.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !normalizedEmail.isEmpty, !emailCode.isEmpty else {
            throw NetworkError.transport("Email and code are required")
        }

        let token = "mock-token-\(UUID().uuidString)"
        keychain.save(token, for: "access_token")

        let suffix = normalizedEmail.split(separator: "@").first.map(String.init) ?? normalizedEmail
        let user = User(id: "mock-user", nickname: "用户\(suffix.suffix(4))", avatarURL: nil)
        return AuthSession(accessToken: token, refreshToken: nil, user: user)
    }

    func register(email: String, emailCode: String, nickname: String) async throws -> AuthSession {
        let session = try await login(email: email, emailCode: emailCode)
        let finalNickname = nickname.trimmingCharacters(in: .whitespacesAndNewlines)

        return AuthSession(
            accessToken: session.accessToken,
            refreshToken: session.refreshToken,
            user: User(
                id: session.user.id,
                nickname: finalNickname.isEmpty ? session.user.nickname : finalNickname,
                avatarURL: session.user.avatarURL
            )
        )
    }

    func logout() async {
        keychain.delete("access_token")
    }
}
