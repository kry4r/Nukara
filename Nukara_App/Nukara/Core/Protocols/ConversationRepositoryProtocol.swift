import Foundation

protocol ConversationRepositoryProtocol {
    func listConversations() async throws -> [Conversation]
    func listMessages(conversationID: String, limit: Int) async throws -> [ChatMessage]
    func markConversationRead(conversationID: String) async throws
}
