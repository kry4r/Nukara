import Foundation
import Combine

@MainActor
final class SessionStore: ObservableObject {
    @Published private(set) var session: AuthSession?
    @Published private(set) var isLoading = false
    @Published private(set) var isRequestingCode = false
    @Published var tipsMessage: String?
    @Published var errorMessage: String?

    private let authRepository: AuthRepositoryProtocol

    init(authRepository: AuthRepositoryProtocol) {
        self.authRepository = authRepository
    }

    func restoreSessionIfNeeded() async {
        guard session == nil else { return }
        session = await authRepository.restoreSession()
    }

    func login(phone: String, smsCode: String) async {
        isLoading = true
        defer { isLoading = false }

        do {
            session = try await authRepository.login(phone: phone, smsCode: smsCode)
            errorMessage = nil
            tipsMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func register(phone: String, smsCode: String, nickname: String) async {
        isLoading = true
        defer { isLoading = false }

        do {
            session = try await authRepository.register(phone: phone, smsCode: smsCode, nickname: nickname)
            errorMessage = nil
            tipsMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func requestSMSCode(phone: String, purpose: AuthCodePurpose) async {
        isRequestingCode = true
        defer { isRequestingCode = false }

        do {
            try await authRepository.requestSMSCode(phone: phone, purpose: purpose)
            errorMessage = nil
            tipsMessage = "验证码已发送"
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func clearMessages() {
        errorMessage = nil
        tipsMessage = nil
    }

    func logout() async {
        await authRepository.logout()
        session = nil
    }
}
