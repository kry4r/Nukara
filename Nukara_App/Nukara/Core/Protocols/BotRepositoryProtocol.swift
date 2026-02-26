import Foundation

struct CreateBotInput: Equatable {
    var name: String
    var summary: String
    var speakingStyle: String
    var background: String
    var traits: [String]
    var gender: BotGender
    var avatarData: Data?
}

struct AppendBotPersonaInput: Equatable {
    var speakingStyleAdds: [String]
    var backgroundAdds: [String]
    var traitAdds: [String]
    var gender: BotGender?
}

protocol BotRepositoryProtocol {
    func listBots() async throws -> [Bot]
    func getBot(botID: String) async throws -> Bot
    func createBot(_ input: CreateBotInput) async throws -> Bot
    func appendPersona(botID: String, input: AppendBotPersonaInput) async throws -> Bot
}
