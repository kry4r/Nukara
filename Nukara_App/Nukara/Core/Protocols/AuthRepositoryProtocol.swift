import Foundation

enum AuthCodePurpose: String {
    case login
    case register
}

protocol AuthRepositoryProtocol {
    func restoreSession() async -> AuthSession?
    func requestEmailCode(email: String, purpose: AuthCodePurpose) async throws
    func login(email: String, emailCode: String) async throws -> AuthSession
    func register(email: String, emailCode: String, nickname: String) async throws -> AuthSession
    func logout() async
}
