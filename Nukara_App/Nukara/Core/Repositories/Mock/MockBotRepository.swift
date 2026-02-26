import Foundation

final class MockBotRepository: BotRepositoryProtocol {
    private let backend: MockBackendStore

    init(backend: MockBackendStore) {
        self.backend = backend
    }

    func listBots() async throws -> [Bot] {
        await backend.listBots()
    }

    func getBot(botID: String) async throws -> Bot {
        if let bot = await backend.getBot(botID: botID) {
            return bot
        }
        throw NetworkError.transport("角色不存在")
    }

    func createBot(_ input: CreateBotInput) async throws -> Bot {
        await backend.createBot(input)
    }

    func appendPersona(botID: String, input: AppendBotPersonaInput) async throws -> Bot {
        if let bot = await backend.appendPersona(botID: botID, input: input) {
            return bot
        }
        throw NetworkError.transport("角色不存在")
    }
}
