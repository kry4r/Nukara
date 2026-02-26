import Foundation
import SwiftUI

enum AppEnvironmentMode: String {
    case mock
    case real
}

struct AppEnvironment {
    let mode: AppEnvironmentMode
    let authRepository: AuthRepositoryProtocol
    let botRepository: BotRepositoryProtocol
    let conversationRepository: ConversationRepositoryProtocol
    let chatRepository: ChatRepositoryProtocol
    let pushRegistrationService: PushRegistrationServiceProtocol
    let userStatusService: UserStatusServiceProtocol

    static func makeDefault() -> AppEnvironment {
        let processInfo = ProcessInfo.processInfo
        let arguments = processInfo.arguments
        let env = processInfo.environment

        let mode: AppEnvironmentMode
        if arguments.contains("--mock-api") || env["NUKARA_ENV"] == "mock" {
            mode = .mock
        } else if arguments.contains("--real-api") || env["NUKARA_ENV"] == "real" {
            mode = .real
        } else {
            mode = .real
        }
        print("[Nukara] AppEnvironment mode: \(mode.rawValue)")

        let keychain = KeychainStore()
        let conversationCache = ConversationCacheStore()
        let messageCache = MessageCacheStore()

        switch mode {
        case .mock:
            let backend = MockBackendStore()
            let authRepository = MockAuthRepository(keychain: keychain)
            let botRepository = MockBotRepository(backend: backend)
            let conversationRepository = MockConversationRepository(
                backend: backend,
                conversationCache: conversationCache,
                messageCache: messageCache
            )
            let chatRepository = MockChatRepository(backend: backend)
            return AppEnvironment(
                mode: mode,
                authRepository: authRepository,
                botRepository: botRepository,
                conversationRepository: conversationRepository,
                chatRepository: chatRepository,
                pushRegistrationService: NoopPushRegistrationService(),
                userStatusService: NoopUserStatusService()
            )

        case .real:
            let baseURLString = env["NUKARA_BASE_URL"] ?? "http://192.168.1.102:8080"
            let baseURL = URL(string: baseURLString) ?? URL(string: "http://192.168.1.102:8080")!

            let apiClient = APIClient(baseURL: baseURL) {
                keychain.load("access_token")
            }

            var wsComponents = URLComponents(url: baseURL, resolvingAgainstBaseURL: false)
            wsComponents?.scheme = baseURL.scheme == "https" ? "wss" : "ws"
            wsComponents?.path = "/ws/chat"
            let socketURL = wsComponents?.url ?? URL(string: "ws://192.168.1.102:8080/ws/chat")!

            let authRepository = RealAuthRepository(apiClient: apiClient, keychain: keychain)
            let botRepository = RealBotRepository(apiClient: apiClient)
            let conversationRepository = RealConversationRepository(
                apiClient: apiClient,
                conversationCache: conversationCache,
                messageCache: messageCache
            )
            let chatRepository = RealChatRepository(
                webSocket: WebSocketClient(),
                apiClient: apiClient,
                socketURL: socketURL,
                accessTokenProvider: { keychain.load("access_token") }
            )
            let pushRegistrationService = APIPushRegistrationService(apiClient: apiClient)

            return AppEnvironment(
                mode: mode,
                authRepository: authRepository,
                botRepository: botRepository,
                conversationRepository: conversationRepository,
                chatRepository: chatRepository,
                pushRegistrationService: pushRegistrationService,
                userStatusService: APIUserStatusService(apiClient: apiClient)
            )
        }
    }
}

private struct AppEnvironmentKey: EnvironmentKey {
    static let defaultValue: AppEnvironment = AppEnvironment.makeDefault()
}

extension EnvironmentValues {
    var appEnvironment: AppEnvironment {
        get { self[AppEnvironmentKey.self] }
        set { self[AppEnvironmentKey.self] = newValue }
    }
}
