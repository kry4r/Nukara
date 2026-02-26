import Foundation

enum ChatSocketEvent: Equatable {
    case ack(clientMessageID: String, serverMessageID: String, timestamp: Date)
    case multiReplyStart(conversationID: String, replyGroupID: String, count: Int)
    case multiReplyEnd(conversationID: String, replyGroupID: String)
    case message(ChatMessage)
    case botStatusUpdate(conversationID: String, status: BotStatus)
    case proactiveMessage(ChatMessage)
    case typing(conversationID: String, isTyping: Bool)
    case error(message: String)
    case unknown(type: String)
}
