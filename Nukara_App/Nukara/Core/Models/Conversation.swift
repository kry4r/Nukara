import Foundation

struct BotStatus: Codable, Equatable {
    var emoji: String
    var text: String

    static let `default` = BotStatus(emoji: "🙂", text: "在线")
}

struct Conversation: Codable, Equatable, Identifiable {
    let id: String
    var botID: String
    var botName: String
    var botAvatarURL: URL?
    var botAvatarData: Data?
    var botStatus: BotStatus = .default
    var lastMessage: String
    var lastMessageTime: Date
    var unreadCount: Int
    var isTyping: Bool
    var isProactiveMessage: Bool
}

extension Conversation {
    enum CodingKeys: String, CodingKey {
        case id
        case botID
        case botName
        case botAvatarURL
        case botAvatarData
        case botStatus
        case lastMessage
        case lastMessageTime
        case unreadCount
        case isTyping
        case isProactiveMessage
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(String.self, forKey: .id)
        botID = try container.decode(String.self, forKey: .botID)
        botName = try container.decode(String.self, forKey: .botName)
        botAvatarURL = try container.decodeIfPresent(URL.self, forKey: .botAvatarURL)
        botAvatarData = try container.decodeIfPresent(Data.self, forKey: .botAvatarData)
        botStatus = try container.decodeIfPresent(BotStatus.self, forKey: .botStatus) ?? .default
        lastMessage = try container.decode(String.self, forKey: .lastMessage)
        lastMessageTime = try container.decode(Date.self, forKey: .lastMessageTime)
        unreadCount = try container.decode(Int.self, forKey: .unreadCount)
        isTyping = try container.decode(Bool.self, forKey: .isTyping)
        isProactiveMessage = try container.decode(Bool.self, forKey: .isProactiveMessage)
    }
}
