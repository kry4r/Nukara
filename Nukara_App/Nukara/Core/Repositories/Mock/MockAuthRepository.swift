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

    func requestSMSCode(phone: String, purpose: AuthCodePurpose) async throws {
        let normalizedPhone = phone.trimmingCharacters(in: .whitespacesAndNewlines)
        guard normalizedPhone.count >= 11 else {
            throw NetworkError.transport("请输入正确手机号")
        }
        _ = purpose
    }

    func login(phone: String, smsCode: String) async throws -> AuthSession {
        let normalizedPhone = phone.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !normalizedPhone.isEmpty, !smsCode.isEmpty else {
            throw NetworkError.transport("Phone and code are required")
        }

        let token = "mock-token-\(UUID().uuidString)"
        keychain.save(token, for: "access_token")

        let user = User(id: "mock-user", nickname: "用户\(normalizedPhone.suffix(4))", avatarURL: nil)
        return AuthSession(accessToken: token, refreshToken: nil, user: user)
    }

    func register(phone: String, smsCode: String, nickname: String) async throws -> AuthSession {
        let session = try await login(phone: phone, smsCode: smsCode)
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
