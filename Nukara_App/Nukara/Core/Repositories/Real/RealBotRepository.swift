import Foundation

final class RealBotRepository: BotRepositoryProtocol {
    private let apiClient: APIClientProtocol

    init(apiClient: APIClientProtocol) {
        self.apiClient = apiClient
    }

    func listBots() async throws -> [Bot] {
        let dtoList = try await apiClient.request(APIEndpoints.bots(), as: [BotDTO].self)
        return dtoList.map { $0.toDomain() }
    }

    func getBot(botID: String) async throws -> Bot {
        let dto = try await apiClient.request(APIEndpoints.bot(botID: botID), as: BotDTO.self)
        return dto.toDomain()
    }

    func createBot(_ input: CreateBotInput) async throws -> Bot {
        let endpoint = try APIEndpoints.createBot(input)
        let dto = try await apiClient.request(endpoint, as: BotDTO.self)
        return dto.toDomain()
    }

    func appendPersona(botID: String, input: AppendBotPersonaInput) async throws -> Bot {
        do {
            let endpoint = try APIEndpoints.appendBotPersona(botID: botID, input: input)
            let dto = try await apiClient.request(endpoint, as: BotDTO.self)
            return dto.toDomain()
        } catch {
            let endpoint = try APIEndpoints.appendBotPersonaLegacy(botID: botID, input: input)
            let dto = try await apiClient.request(endpoint, as: BotDTO.self)
            return dto.toDomain()
        }
    }
}

private struct BotDTO: Decodable {
    let id: String
    let name: String
    let avatar: String?
    let avatarBase64: String?
    let summary: String?
    let description: String?
    let speakingStyle: String?
    let background: String?
    let traits: [String]?
    let gender: String?
    let chatBackgroundStyle: String?
    let createdAt: Date?

    enum CodingKeys: String, CodingKey {
        case id
        case name
        case avatar
        case avatarBase64 = "avatar_base64"
        case summary
        case description
        case speakingStyle = "speaking_style"
        case background
        case traits
        case gender
        case chatBackgroundStyle = "chat_background_style"
        case createdAt = "created_at"
    }

    func toDomain() -> Bot {
        Bot(
            id: id,
            name: name,
            avatarURL: avatar.flatMap(URL.init(string:)),
            avatarData: decodeAvatarBase64(avatarBase64),
            summary: summary ?? description ?? "",
            speakingStyle: speakingStyle ?? "",
            background: background ?? "",
            traits: traits ?? [],
            gender: BotGender(rawValue: gender ?? "") ?? .unknown,
            chatBackgroundStyle: ChatBackgroundStyle(rawValue: chatBackgroundStyle ?? "") ?? .lightPaper,
            createdAt: createdAt ?? Date()
        )
    }

    private func decodeAvatarBase64(_ rawValue: String?) -> Data? {
        guard let rawValue else { return nil }
        if let data = Data(base64Encoded: rawValue) {
            return data
        }
        guard let payload = rawValue.split(separator: ",", maxSplits: 1).last else {
            return nil
        }
        return Data(base64Encoded: String(payload))
    }
}
