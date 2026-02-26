import Foundation

final class RealChatRepository: ChatRepositoryProtocol {
    var events: AsyncStream<ChatSocketEvent> { eventStream }

    private let webSocket: WebSocketClientProtocol
    private let apiClient: APIClientProtocol?
    private let socketURL: URL
    private let accessTokenProvider: () -> String?
    private let eventStream: AsyncStream<ChatSocketEvent>
    private let continuation: AsyncStream<ChatSocketEvent>.Continuation
    private var forwardingTask: Task<Void, Never>?

    init(
        webSocket: WebSocketClientProtocol,
        apiClient: APIClientProtocol? = nil,
        socketURL: URL,
        accessTokenProvider: @escaping () -> String?
    ) {
        self.webSocket = webSocket
        self.apiClient = apiClient
        self.socketURL = socketURL
        self.accessTokenProvider = accessTokenProvider

        var capturedContinuation: AsyncStream<ChatSocketEvent>.Continuation?
        self.eventStream = AsyncStream { continuation in
            capturedContinuation = continuation
        }
        guard let continuation = capturedContinuation else {
            fatalError("Failed to initialize real chat stream")
        }
        self.continuation = continuation
    }

    func connect() async throws {
        webSocket.connect(url: socketURL, accessToken: accessTokenProvider())

        forwardingTask?.cancel()
        forwardingTask = Task { [weak self] in
            guard let self else { return }
            for await text in webSocket.textEvents {
                continuation.yield(parseEvent(from: text))
            }
        }
    }

    func disconnect() async {
        forwardingTask?.cancel()
        forwardingTask = nil
        webSocket.disconnect()
    }

    func sendText(conversationID: String, text: String, clientMessageID: String) async throws {
        let payload: [String: Any] = [
            "type": "message",
            "conversation_id": conversationID,
            "client_msg_id": clientMessageID,
            "content": [
                "type": "text",
                "text": text
            ]
        ]
        do {
            let data = try JSONSerialization.data(withJSONObject: payload)
            let json = String(decoding: data, as: UTF8.self)
            try await webSocket.send(json)
        } catch {
            try await sendViaHTTP(
                conversationID: conversationID,
                clientMessageID: clientMessageID,
                content: ConversationSendContentDTO(
                    type: "text",
                    text: text,
                    imageBase64: nil,
                    latitude: nil,
                    longitude: nil,
                    name: nil
                )
            )
        }
    }

    func sendImage(conversationID: String, imageData: Data, clientMessageID: String) async throws {
        let base64 = imageData.base64EncodedString()
        let payload: [String: Any] = [
            "type": "message",
            "conversation_id": conversationID,
            "client_msg_id": clientMessageID,
            "content": [
                "type": "image",
                "image_base64": base64
            ]
        ]

        do {
            let data = try JSONSerialization.data(withJSONObject: payload)
            let json = String(decoding: data, as: UTF8.self)
            try await webSocket.send(json)
        } catch {
            try await sendViaHTTP(
                conversationID: conversationID,
                clientMessageID: clientMessageID,
                content: ConversationSendContentDTO(
                    type: "image",
                    text: nil,
                    imageBase64: base64,
                    latitude: nil,
                    longitude: nil,
                    name: nil
                )
            )
        }
    }

    func sendLocation(conversationID: String, location: ChatLocation, clientMessageID: String) async throws {
        let payload: [String: Any] = [
            "type": "message",
            "conversation_id": conversationID,
            "client_msg_id": clientMessageID,
            "content": [
                "type": "location",
                "latitude": location.latitude,
                "longitude": location.longitude,
                "name": location.name
            ]
        ]

        do {
            let data = try JSONSerialization.data(withJSONObject: payload)
            let json = String(decoding: data, as: UTF8.self)
            try await webSocket.send(json)
        } catch {
            try await sendViaHTTP(
                conversationID: conversationID,
                clientMessageID: clientMessageID,
                content: ConversationSendContentDTO(
                    type: "location",
                    text: nil,
                    imageBase64: nil,
                    latitude: location.latitude,
                    longitude: location.longitude,
                    name: location.name
                )
            )
        }
    }

    private func sendViaHTTP(
        conversationID: String,
        clientMessageID: String,
        content: ConversationSendContentDTO
    ) async throws {
        guard let apiClient else {
            throw NetworkError.transport("WebSocket send failed and HTTP fallback is unavailable")
        }
        let endpoint = try APIEndpoints.conversationSendMessage(
            conversationID: conversationID,
            request: ConversationSendMessageRequestDTO(
                clientMessageID: clientMessageID,
                content: content
            )
        )
        let response = try await apiClient.request(endpoint, as: ConversationSendResponseDTO.self)
        continuation.yield(
            .ack(
                clientMessageID: response.ack.clientMessageID,
                serverMessageID: response.ack.serverMessageID,
                timestamp: Date(timeIntervalSince1970: response.ack.timestamp)
            )
        )

        let botMessage = response.botMessage.toDomain()
        let replyGroupID = UUID().uuidString
        continuation.yield(.multiReplyStart(conversationID: botMessage.conversationID, replyGroupID: replyGroupID, count: 0))
        continuation.yield(.message(botMessage))
        continuation.yield(.multiReplyEnd(conversationID: botMessage.conversationID, replyGroupID: replyGroupID))

        if let status = response.botStatusUpdate {
            continuation.yield(
                .botStatusUpdate(
                    conversationID: status.conversationID,
                    status: BotStatus(emoji: status.emoji, text: status.text)
                )
            )
        }
    }

    private func parseEvent(from text: String) -> ChatSocketEvent {
        guard let data = text.data(using: .utf8),
              let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let type = object["type"] as? String else {
            return .unknown(type: "decode_failed")
        }

        switch type {
        case "ack":
            let clientID = object["client_msg_id"] as? String ?? ""
            let serverID = object["server_msg_id"] as? String ?? UUID().uuidString
            let timestamp = Date(timeIntervalSince1970: object["timestamp"] as? TimeInterval ?? Date().timeIntervalSince1970)
            return .ack(clientMessageID: clientID, serverMessageID: serverID, timestamp: timestamp)

        case "multi_reply_start":
            return .multiReplyStart(
                conversationID: object["conversation_id"] as? String ?? "",
                replyGroupID: object["reply_group_id"] as? String ?? "",
                count: object["count"] as? Int ?? 0
            )

        case "multi_reply_end":
            return .multiReplyEnd(
                conversationID: object["conversation_id"] as? String ?? "",
                replyGroupID: object["reply_group_id"] as? String ?? ""
            )

        case "typing":
            return .typing(
                conversationID: object["conversation_id"] as? String ?? "",
                isTyping: object["is_typing"] as? Bool ?? false
            )

        case "proactive_message":
            let conversationID = object["conversation_id"] as? String ?? ""
            let content = object["content"] as? [String: Any]
            let text = content?["text"] as? String ?? ""
            let message = ChatMessage(
                id: object["msg_id"] as? String ?? UUID().uuidString,
                conversationID: conversationID,
                sender: .bot,
                text: text,
                imageData: nil,
                location: nil,
                timestamp: Date(timeIntervalSince1970: object["timestamp"] as? TimeInterval ?? Date().timeIntervalSince1970),
                status: .delivered,
                isProactive: true,
                emotionTag: object["emotion_context"] as? String
            )
            return .proactiveMessage(message)

        case "message":
            let conversationID = object["conversation_id"] as? String ?? ""
            let senderType = object["sender_type"] as? String ?? "bot"
            let msgID = object["msg_id"] as? String ?? UUID().uuidString
            let timestamp = Date(timeIntervalSince1970: object["timestamp"] as? TimeInterval ?? Date().timeIntervalSince1970)
            let content = object["content"] as? [String: Any]
            let contentType = content?["type"] as? String ?? "text"
            let textValue = content?["text"] as? String ?? ""
            let imageData = (content?["image_base64"] as? String).flatMap { Data(base64Encoded: $0) }

            var location: ChatLocation?
            if contentType == "location" {
                location = ChatLocation(
                    latitude: content?["latitude"] as? Double ?? 0,
                    longitude: content?["longitude"] as? Double ?? 0,
                    name: content?["name"] as? String ?? "位置"
                )
            }

            var message = ChatMessage(
                id: msgID,
                conversationID: conversationID,
                sender: senderType == "user" ? .user : .bot,
                text: textValue,
                imageData: imageData,
                location: location,
                timestamp: timestamp,
                status: .delivered,
                isProactive: object["is_proactive"] as? Bool ?? false,
                emotionTag: object["emotion_tag"] as? String
            )
            message.replyGroupID = object["reply_group_id"] as? String
            message.sequence = object["sequence"] as? Int
            return .message(message)

        case "bot_status_update":
            let conversationID = object["conversation_id"] as? String ?? ""
            let status = object["status"] as? [String: Any]
            let emoji = (object["emoji"] as? String) ?? (status?["emoji"] as? String) ?? BotStatus.default.emoji
            let text = (object["text"] as? String) ?? (status?["text"] as? String) ?? BotStatus.default.text
            return .botStatusUpdate(conversationID: conversationID, status: BotStatus(emoji: emoji, text: text))

        case "error":
            let message = object["message"] as? String ?? "unknown error"
            return .error(message: message)

        default:
            return .unknown(type: type)
        }
    }
}

private struct ConversationSendResponseDTO: Decodable {
    struct AckDTO: Decodable {
        let clientMessageID: String
        let serverMessageID: String
        let timestamp: TimeInterval

        enum CodingKeys: String, CodingKey {
            case clientMessageID = "client_msg_id"
            case serverMessageID = "server_msg_id"
            case timestamp
        }
    }

    struct StatusUpdateDTO: Decodable {
        let conversationID: String
        let emoji: String
        let text: String

        enum CodingKeys: String, CodingKey {
            case conversationID = "conversation_id"
            case emoji
            case text
        }
    }

    struct MessageDTO: Decodable {
        let id: String
        let conversationID: String
        let senderType: String
        let content: ContentDTO
        let isProactive: Bool?
        let emotionTag: String?
        let createdAt: Date?

        struct ContentDTO: Decodable {
            let type: String?
            let text: String?
        }

        enum CodingKeys: String, CodingKey {
            case id
            case conversationID = "conversation_id"
            case senderType = "sender_type"
            case content
            case isProactive = "is_proactive"
            case emotionTag = "emotion_tag"
            case createdAt = "created_at"
        }

        func toDomain() -> ChatMessage {
            ChatMessage(
                id: id,
                conversationID: conversationID,
                sender: senderType == "user" ? .user : .bot,
                text: content.text ?? "",
                imageData: nil,
                location: nil,
                timestamp: createdAt ?? Date(),
                status: .delivered,
                isProactive: isProactive ?? false,
                emotionTag: emotionTag
            )
        }
    }

    let ack: AckDTO
    let botMessage: MessageDTO
    let botStatusUpdate: StatusUpdateDTO?

    enum CodingKeys: String, CodingKey {
        case ack
        case botMessage = "bot_message"
        case botStatusUpdate = "bot_status_update"
    }
}
