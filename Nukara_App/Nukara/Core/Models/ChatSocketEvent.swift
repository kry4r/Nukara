import Foundation

enum ChatSocketEvent: Equatable {
    case ack(clientMessageID: String, serverMessageID: String, timestamp: Date)
    case streamStart(conversationID: String, replyID: String)
    case streamChunk(conversationID: String, replyID: String, delta: String)
    case streamEnd(conversationID: String, replyID: String)
    case multiReplyStart(conversationID: String, replyGroupID: String, count: Int)
    case multiReplyEnd(conversationID: String, replyGroupID: String)
    case message(ChatMessage)
    case botStatusUpdate(conversationID: String, status: BotStatus)
    case botPersonaUpdated(botID: String, summary: String, timestamp: Date)
    case proactiveMessage(ChatMessage)
    case typing(conversationID: String, isTyping: Bool)
    case error(message: String)
    case unknown(type: String)
}
