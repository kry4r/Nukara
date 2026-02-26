import Foundation

@MainActor
final class RealConversationRepository: ConversationRepositoryProtocol {
    private let apiClient: APIClientProtocol
    private let conversationCache: ConversationCacheStore
    private let messageCache: MessageCacheStore

    init(
        apiClient: APIClientProtocol,
        conversationCache: ConversationCacheStore,
        messageCache: MessageCacheStore
    ) {
        self.apiClient = apiClient
        self.conversationCache = conversationCache
        self.messageCache = messageCache
    }

    func listConversations() async throws -> [Conversation] {
        do {
            let dtoList = try await apiClient.request(APIEndpoints.conversations(), as: [ConversationDTO].self)
            let conversations = dtoList.map { $0.toDomain() }
            conversationCache.saveConversations(conversations)
            return conversations
        } catch {
            let cached = conversationCache.loadConversations()
            if cached.isEmpty { throw error }
            return cached
        }
    }

    func listMessages(conversationID: String, limit: Int) async throws -> [ChatMessage] {
        do {
            let endpoint = APIEndpoints.conversationMessages(conversationID: conversationID, limit: limit)
            let dtoList = try await apiClient.request(endpoint, as: [MessageDTO].self)
            let messages = dtoList.map { $0.toDomain(conversationID: conversationID) }
            messageCache.saveMessages(messages, for: conversationID)
            return messages
        } catch {
            let cached = messageCache.loadMessages(conversationID: conversationID, limit: limit)
            if cached.isEmpty { throw error }
            return cached
        }
    }

    func markConversationRead(conversationID: String) async throws {
        do {
            try await apiClient.requestVoid(APIEndpoints.markConversationRead(conversationID: conversationID))
        } catch {
            do {
                try await apiClient.requestVoid(APIEndpoints.markConversationReadLegacy(conversationID: conversationID))
            } catch {
                // Ignore remote mark-read failures but still keep cache consistent.
            }
        }

        var cached = conversationCache.loadConversations()
        if let index = cached.firstIndex(where: { $0.id == conversationID }) {
            cached[index].unreadCount = 0
            conversationCache.saveConversations(cached)
        }
    }
}

private struct ConversationDTO: Decodable {
    let id: String
    let botID: String
    let botName: String
    let botAvatar: String?
    let botAvatarBase64: String?
    let lastMessage: String?
    let lastMessageAt: Date?
    let unreadCount: Int?
    let isProactiveMessage: Bool?
    let botStatusEmoji: String?
    let botStatusText: String?

    enum CodingKeys: String, CodingKey {
        case id
        case botID = "bot_id"
        case botName = "bot_name"
        case botAvatar = "bot_avatar"
        case botAvatarBase64 = "bot_avatar_base64"
        case lastMessage = "last_message"
        case lastMessageAt = "last_message_at"
        case unreadCount = "unread_count"
        case isProactiveMessage = "is_proactive_message"
        case botStatusEmoji = "bot_status_emoji"
        case botStatusText = "bot_status_text"
    }

    func toDomain() -> Conversation {
        Conversation(
            id: id,
            botID: botID,
            botName: botName,
            botAvatarURL: botAvatar.flatMap(URL.init(string:)),
            botAvatarData: botAvatarBase64.flatMap { Data(base64Encoded: $0) },
            botStatus: BotStatus(
                emoji: botStatusEmoji ?? BotStatus.default.emoji,
                text: botStatusText ?? BotStatus.default.text
            ),
            lastMessage: lastMessage ?? "",
            lastMessageTime: lastMessageAt ?? Date(),
            unreadCount: unreadCount ?? 0,
            isTyping: false,
            isProactiveMessage: isProactiveMessage ?? false
        )
    }
}

private struct MessageDTO: Decodable {
    let id: String
    let senderType: String
    let contentType: String?
    let content: MessageContentDTO?
    let text: String?
    let imageBase64: String?
    let isProactive: Bool?
    let emotionTag: String?
    let createdAt: Date?

    enum CodingKeys: String, CodingKey {
        case id
        case senderType = "sender_type"
        case contentType = "content_type"
        case content
        case text
        case imageBase64 = "image_base64"
        case isProactive = "is_proactive"
        case emotionTag = "emotion_tag"
        case createdAt = "created_at"
    }

    func toDomain(conversationID: String) -> ChatMessage {
        let finalText = content?.text ?? text ?? ""
        let resolvedImageBase64 = content?.imageBase64 ?? imageBase64
        let imageData = resolvedImageBase64.flatMap { Data(base64Encoded: $0) }
        let location: ChatLocation?
        if (content?.type ?? contentType) == "location",
           let latitude = content?.latitude,
           let longitude = content?.longitude {
            location = ChatLocation(latitude: latitude, longitude: longitude, name: content?.name ?? "位置")
        } else {
            location = nil
        }

        return ChatMessage(
            id: id,
            conversationID: conversationID,
            sender: senderType == "user" ? .user : .bot,
            text: finalText,
            imageData: imageData,
            location: location,
            timestamp: createdAt ?? Date(),
            status: .delivered,
            isProactive: isProactive ?? false,
            emotionTag: emotionTag
        )
    }
}

private struct MessageContentDTO: Decodable {
    let type: String?
    let text: String?
    let imageBase64: String?
    let latitude: Double?
    let longitude: Double?
    let name: String?

    enum CodingKeys: String, CodingKey {
        case type
        case text
        case imageBase64 = "image_base64"
        case latitude
        case longitude
        case name
    }
}
