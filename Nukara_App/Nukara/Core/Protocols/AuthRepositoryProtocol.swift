import Foundation

enum AuthCodePurpose: String {
    case login
    case register
}

protocol AuthRepositoryProtocol {
    func restoreSession() async -> AuthSession?
    func requestSMSCode(phone: String, purpose: AuthCodePurpose) async throws
    func login(phone: String, smsCode: String) async throws -> AuthSession
    func register(phone: String, smsCode: String, nickname: String) async throws -> AuthSession
    func logout() async
}
