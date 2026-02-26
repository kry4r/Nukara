import Foundation

struct ChatLocation: Codable, Equatable {
    var latitude: Double
    var longitude: Double
    var name: String
}

enum MessageSender: String, Codable, Equatable {
    case user
    case bot
}

enum MessageStatus: String, Codable, Equatable {
    case sending
    case sent
    case delivered
    case failed
    case streaming
}

struct ChatMessage: Codable, Equatable, Identifiable {
    let id: String
    let conversationID: String
    var sender: MessageSender
    var text: String
    var imageData: Data?
    var location: ChatLocation?
    var timestamp: Date
    var status: MessageStatus
    var isProactive: Bool
    var emotionTag: String?
    var replyGroupID: String? = nil
    var sequence: Int? = nil
}

extension ChatMessage {
    static func userTextDraft(conversationID: String, text: String, clientMessageID: String) -> ChatMessage {
        ChatMessage(
            id: clientMessageID,
            conversationID: conversationID,
            sender: .user,
            text: text,
            imageData: nil,
            location: nil,
            timestamp: Date(),
            status: .sending,
            isProactive: false,
            emotionTag: nil
        )
    }

    static func userImageDraft(conversationID: String, imageData: Data, clientMessageID: String) -> ChatMessage {
        ChatMessage(
            id: clientMessageID,
            conversationID: conversationID,
            sender: .user,
            text: "",
            imageData: imageData,
            location: nil,
            timestamp: Date(),
            status: .sending,
            isProactive: false,
            emotionTag: nil
        )
    }

    static func userLocationDraft(conversationID: String, location: ChatLocation, clientMessageID: String) -> ChatMessage {
        ChatMessage(
            id: clientMessageID,
            conversationID: conversationID,
            sender: .user,
            text: "",
            imageData: nil,
            location: location,
            timestamp: Date(),
            status: .sending,
            isProactive: false,
            emotionTag: nil
        )
    }
}
