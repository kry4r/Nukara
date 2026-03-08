import Foundation

final class RealAuthRepository: AuthRepositoryProtocol {
    private let apiClient: APIClientProtocol
    private let keychain: KeychainStore

    init(apiClient: APIClientProtocol, keychain: KeychainStore) {
        self.apiClient = apiClient
        self.keychain = keychain
    }

    func restoreSession() async -> AuthSession? {
        guard let token = keychain.load("access_token") else { return nil }
        let user = User(id: "real-user", nickname: "Nukara", avatarURL: nil)
        return AuthSession(accessToken: token, refreshToken: nil, user: user)
    }

    func requestEmailCode(email: String, purpose: AuthCodePurpose) async throws {
        let endpoint = try APIEndpoints.requestEmailCode(email: email, purpose: purpose)
        try await apiClient.requestVoid(endpoint)
    }

    func login(email: String, emailCode: String) async throws -> AuthSession {
        let endpoint = try APIEndpoints.login(email: email, emailCode: emailCode)
        let response = try await apiClient.request(endpoint, as: LoginResponseDTO.self)

        let user = response.user?.toDomain() ?? User(id: "real-user", nickname: "Nukara", avatarURL: nil)
        let session = AuthSession(
            accessToken: response.accessToken,
            refreshToken: response.refreshToken,
            user: user
        )
        keychain.save(session.accessToken, for: "access_token")
        return session
    }

    func register(email: String, emailCode: String, nickname: String) async throws -> AuthSession {
        let endpoint = try APIEndpoints.register(email: email, emailCode: emailCode, nickname: nickname)
        let response = try await apiClient.request(endpoint, as: LoginResponseDTO.self)

        let user = response.user?.toDomain() ?? User(id: "real-user", nickname: nickname, avatarURL: nil)
        let session = AuthSession(
            accessToken: response.accessToken,
            refreshToken: response.refreshToken,
            user: user
        )
        keychain.save(session.accessToken, for: "access_token")
        return session
    }

    func logout() async {
        keychain.delete("access_token")
    }
}

private struct LoginResponseDTO: Decodable {
    let accessToken: String
    let refreshToken: String?
    let user: UserDTO?

    enum CodingKeys: String, CodingKey {
        case accessToken = "access_token"
        case refreshToken = "refresh_token"
        case user
    }
}

private struct UserDTO: Decodable {
    let id: String
    let nickname: String?
    let avatar: String?

    func toDomain() -> User {
        User(
            id: id,
            nickname: nickname ?? "Nukara",
            avatarURL: avatar.flatMap(URL.init(string:))
        )
    }
}
