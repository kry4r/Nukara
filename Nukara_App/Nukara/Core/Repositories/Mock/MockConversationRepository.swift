import Foundation

@MainActor
final class MockConversationRepository: ConversationRepositoryProtocol {
    private let backend: MockBackendStore
    private let conversationCache: ConversationCacheStore
    private let messageCache: MessageCacheStore

    init(
        backend: MockBackendStore,
        conversationCache: ConversationCacheStore,
        messageCache: MessageCacheStore
    ) {
        self.backend = backend
        self.conversationCache = conversationCache
        self.messageCache = messageCache
    }

    func listConversations() async throws -> [Conversation] {
        let remote = await backend.listConversations()
        conversationCache.saveConversations(remote)
        return remote
    }

    func listMessages(conversationID: String, limit: Int) async throws -> [ChatMessage] {
        let remote = await backend.listMessages(conversationID: conversationID, limit: limit)
        messageCache.saveMessages(remote, for: conversationID)
        return remote
    }

    func markConversationRead(conversationID: String) async throws {
        await backend.markConversationRead(conversationID: conversationID)
        var cached = conversationCache.loadConversations()
        if let index = cached.firstIndex(where: { $0.id == conversationID }) {
            cached[index].unreadCount = 0
            conversationCache.saveConversations(cached)
        }
    }
}
