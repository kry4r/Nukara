import Foundation
import Testing
@testable import Nukara

struct NukaraTests {

    @Test @MainActor
    func mockLoginPersistsSession() async throws {
        let keychain = KeychainStore()
        let repository = MockAuthRepository(keychain: keychain)

        let session = try await repository.login(phone: "13800138000", smsCode: "123456")
        #expect(session.accessToken.starts(with: "mock-token-"))

        let restored = await repository.restoreSession()
        #expect(restored != nil)
        #expect(restored?.accessToken == session.accessToken)

        await repository.logout()
        let afterLogout = await repository.restoreSession()
        #expect(afterLogout == nil)
    }

    @Test @MainActor
    func chatStoreReceivesStreamReply() async throws {
        let backend = MockBackendStore()
        let conversationRepo = MockConversationRepository(
            backend: backend,
            conversationCache: ConversationCacheStore(),
            messageCache: MessageCacheStore()
        )
        let chatRepo = MockChatRepository(backend: backend)

        let conversations = await backend.listConversations()
        let conversation = try #require(conversations.first)
        let store = ChatStore(
            conversation: conversation,
            conversationRepository: conversationRepo,
            chatRepository: chatRepo
        )

        store.send(.onAppear)
        try await Task.sleep(for: .milliseconds(150))

        store.send(.inputChanged("你好"))
        store.send(.sendTapped)

        try await Task.sleep(for: .milliseconds(1200))

        let userMessages = store.state.messages.filter { $0.sender == .user }
        let botMessages = store.state.messages.filter { $0.sender == .bot }

        #expect(!userMessages.isEmpty)
        #expect(!botMessages.isEmpty)
        #expect(botMessages.last?.text.contains("我记住啦") == true)

        store.send(.onDisappear)
    }

    @Test @MainActor
    func conversationSearchSupportsNameStatusAndMessage() async throws {
        let backend = MockBackendStore()
        let conversations = await backend.listConversations()

        func filter(_ query: String) -> [Conversation] {
            conversations.filter { conversation in
                conversation.botName.localizedCaseInsensitiveContains(query)
                || conversation.lastMessage.localizedCaseInsensitiveContains(query)
                || conversation.botStatus.text.localizedCaseInsensitiveContains(query)
                || conversation.botStatus.emoji.localizedCaseInsensitiveContains(query)
            }
        }

        #expect(!filter("桃眠").isEmpty)
        #expect(!filter("想你了").isEmpty)
        #expect(!filter("🏃").isEmpty)
    }

    @Test @MainActor
    func markConversationReadClearsUnreadCount() async throws {
        let backend = MockBackendStore()
        let conversationRepo = MockConversationRepository(
            backend: backend,
            conversationCache: ConversationCacheStore(),
            messageCache: MessageCacheStore()
        )

        let initial = try await conversationRepo.listConversations()
        let target = try #require(initial.first(where: { $0.unreadCount > 0 }))
        #expect(target.unreadCount > 0)

        try await conversationRepo.markConversationRead(conversationID: target.id)

        let refreshed = try await conversationRepo.listConversations()
        let refreshedTarget = try #require(refreshed.first(where: { $0.id == target.id }))
        #expect(refreshedTarget.unreadCount == 0)
    }

    @Test @MainActor
    func chatStoreRealtimeReplyAfterLocationShare() async throws {
        let backend = MockBackendStore()
        let conversationRepo = MockConversationRepository(
            backend: backend,
            conversationCache: ConversationCacheStore(),
            messageCache: MessageCacheStore()
        )
        let chatRepo = MockChatRepository(backend: backend)

        let conversations = await backend.listConversations()
        let conversation = try #require(conversations.first)
        let store = ChatStore(
            conversation: conversation,
            conversationRepository: conversationRepo,
            chatRepository: chatRepo
        )

        var updatePayloads: [[AnyHashable: Any]] = []
        let token = NotificationCenter.default.addObserver(
            forName: .nukaraConversationUpdated,
            object: nil,
            queue: nil
        ) { notification in
            updatePayloads.append(notification.userInfo ?? [:])
        }
        defer { NotificationCenter.default.removeObserver(token) }

        store.send(.onAppear)
        try await Task.sleep(for: .milliseconds(150))

        store.send(.locationPicked(ChatLocation(latitude: 31.23, longitude: 121.47, name: "人民广场")))
        try await Task.sleep(for: .milliseconds(1200))

        let botReply = store.state.messages.last { $0.sender == .bot }
        #expect(botReply?.text.contains("已收到你分享的位置") == true)
        #expect(updatePayloads.contains { payload in
            (payload["conversation_id"] as? String) == conversation.id
        })

        store.send(.onDisappear)
    }

    @Test @MainActor
    func realBotRepositoryDecodesSummaryAndDataURIAvatar() async throws {
        let apiClient = BotRepositoryTestAPIClient()
        let repository = RealBotRepository(apiClient: apiClient)

        let bots = try await repository.listBots()
        let bot = try #require(bots.first)
        #expect(bot.summary == "测试简介")
        #expect(bot.avatarData != nil)
    }

    @Test @MainActor
    func realChatRepositoryFallsBackToHTTPSendWhenWebSocketSendFails() async throws {
        let webSocket = FailingWebSocketClient()
        let apiClient = ChatFallbackTestAPIClient()
        let repository = RealChatRepository(
            webSocket: webSocket,
            apiClient: apiClient,
            socketURL: URL(string: "ws://localhost:8080/ws/chat")!,
            accessTokenProvider: { "token" }
        )

        var captured: [ChatSocketEvent] = []
        let collectTask = Task {
            for await event in repository.events {
                captured.append(event)
                if captured.count >= 5 { break }
            }
        }

        try await repository.sendText(conversationID: "conv-test", text: "你好", clientMessageID: "client-1")
        try await Task.sleep(for: .milliseconds(100))
        collectTask.cancel()

        #expect(captured.contains { event in
            if case .ack(let clientMessageID, _, _) = event { return clientMessageID == "client-1" }
            return false
        })
        #expect(captured.contains { event in
            if case .streamEnd(_, _) = event { return true }
            return false
        })
    }

    // MARK: - MVP Flow Tests

    @Test @MainActor
    func mvpBotCreationAndConversationSetup() async throws {
        let backend = MockBackendStore()
        let botRepo = MockBotRepository(backend: backend)

        let initialBots = try await botRepo.listBots()
        let initialCount = initialBots.count

        let input = CreateBotInput(
            name: "测试角色",
            summary: "温柔治愈的陪伴者",
            speakingStyle: "温柔细腻",
            background: "来自江南的才女",
            traits: ["温柔", "知性", "体贴"],
            gender: .female,
            avatarData: nil
        )
        let created = try await botRepo.createBot(input)
        #expect(created.name == "测试角色")
        #expect(created.traits.count == 3)
        #expect(created.gender == .female)

        let updatedBots = try await botRepo.listBots()
        #expect(updatedBots.count == initialCount + 1)

        // Verify conversation was auto-created for the new bot.
        let conversations = await backend.listConversations()
        let botConv = conversations.first(where: { $0.botID == created.id })
        #expect(botConv != nil)
        #expect(botConv?.botName == "测试角色")
    }

    @Test @MainActor
    func mvpBotPersonaAppendPreservesExisting() async throws {
        let backend = MockBackendStore()
        let botRepo = MockBotRepository(backend: backend)

        let bots = try await botRepo.listBots()
        let bot = try #require(bots.first)
        let originalTraitCount = bot.traits.count

        let appendInput = AppendBotPersonaInput(
            speakingStyleAdds: ["偶尔撒娇"],
            backgroundAdds: ["喜欢读诗"],
            traitAdds: ["浪漫"],
            gender: nil
        )
        let updated = try await botRepo.appendPersona(botID: bot.id, input: appendInput)
        #expect(updated.traits.count == originalTraitCount + 1)
        #expect(updated.traits.contains("浪漫"))
        #expect(updated.speakingStyle.contains("偶尔撒娇"))
        #expect(updated.background.contains("喜欢读诗"))
    }

    @Test @MainActor
    func mvpMultiRoundChatWithBotStatusUpdate() async throws {
        let backend = MockBackendStore()
        let conversationRepo = MockConversationRepository(
            backend: backend,
            conversationCache: ConversationCacheStore(),
            messageCache: MessageCacheStore()
        )
        let chatRepo = MockChatRepository(backend: backend)

        let conversations = await backend.listConversations()
        let conversation = try #require(conversations.first)
        let store = ChatStore(
            conversation: conversation,
            conversationRepository: conversationRepo,
            chatRepository: chatRepo
        )

        store.send(.onAppear)
        try await Task.sleep(for: .milliseconds(150))

        // Round 1
        store.send(.inputChanged("今天心情不太好"))
        store.send(.sendTapped)
        try await Task.sleep(for: .milliseconds(1200))

        // Round 2
        store.send(.inputChanged("工作压力很大"))
        store.send(.sendTapped)
        try await Task.sleep(for: .milliseconds(1200))

        let userMessages = store.state.messages.filter { $0.sender == .user }
        let botMessages = store.state.messages.filter { $0.sender == .bot }

        // At least 2 user messages + 2 bot replies (plus seed message).
        #expect(userMessages.count >= 2)
        #expect(botMessages.count >= 2)

        // Bot should reference user's input in reply.
        let lastBotReply = botMessages.last
        #expect(lastBotReply?.text.contains("工作压力很大") == true)

        store.send(.onDisappear)
    }

    @Test @MainActor
    func mvpProactiveMessageAppearsInConversationList() async throws {
        let backend = MockBackendStore()
        let conversations = await backend.listConversations()

        // At least one conversation should have a proactive message.
        let proactiveConv = conversations.first(where: { $0.isProactiveMessage })
        #expect(proactiveConv != nil)
        #expect(proactiveConv!.unreadCount > 0)

        // Verify the proactive message exists in the message history.
        let messages = await backend.listMessages(
            conversationID: proactiveConv!.id,
            limit: 10
        )
        let proactiveMsg = messages.first(where: { $0.isProactive })
        #expect(proactiveMsg != nil)
        #expect(proactiveMsg?.emotionTag == "gentle")
    }

    @Test @MainActor
    func mvpConversationListSortedByLastMessageTime() async throws {
        let backend = MockBackendStore()
        let conversations = await backend.listConversations()

        // Conversations should be sorted by lastMessageTime descending.
        for i in 0 ..< conversations.count - 1 {
            #expect(conversations[i].lastMessageTime >= conversations[i + 1].lastMessageTime)
        }
    }

    @Test @MainActor
    func mvpBotStatusHasEmojiAndText() async throws {
        let backend = MockBackendStore()
        let conversations = await backend.listConversations()

        for conversation in conversations {
            #expect(!conversation.botStatus.emoji.isEmpty)
            #expect(!conversation.botStatus.text.isEmpty)
        }
    }

    @Test @MainActor
    func mvpChatStoreEmotionTagInBotReply() async throws {
        let backend = MockBackendStore()
        let conversationRepo = MockConversationRepository(
            backend: backend,
            conversationCache: ConversationCacheStore(),
            messageCache: MessageCacheStore()
        )
        let chatRepo = MockChatRepository(backend: backend)

        let conversations = await backend.listConversations()
        let conversation = try #require(conversations.first)
        let store = ChatStore(
            conversation: conversation,
            conversationRepository: conversationRepo,
            chatRepository: chatRepo
        )

        store.send(.onAppear)
        try await Task.sleep(for: .milliseconds(150))

        store.send(.inputChanged("我好开心"))
        store.send(.sendTapped)
        try await Task.sleep(for: .milliseconds(1200))

        // Bot reply should have an emotion tag.
        let botMessages = store.state.messages.filter { $0.sender == .bot }
        let latestBot = botMessages.last
        #expect(latestBot != nil)
        // Mock always sets "gentle" emotion tag.
        #expect(latestBot?.emotionTag != nil)

        store.send(.onDisappear)
    }
}

// MARK: - Test Helpers

private final class BotRepositoryTestAPIClient: APIClientProtocol {
    func request<T>(_ endpoint: APIEndpoint, as type: T.Type) async throws -> T where T: Decodable {
        let payload = """
        [
          {
            "id": "bot-1",
            "name": "测试Bot",
            "avatar_base64": "data:image/png;base64,aGVsbG8=",
            "summary": "测试简介",
            "speaking_style": "温柔",
            "background": "背景",
            "traits": ["治愈"],
            "gender": "female",
            "chat_background_style": "lightPaper",
            "created_at": "2026-02-14T00:00:00Z"
          }
        ]
        """
        let data = Data(payload.utf8)
        return try JSONDecoder.nukara.decode(type, from: data)
    }

    func requestVoid(_ endpoint: APIEndpoint) async throws {
        _ = endpoint
    }
}

private final class ChatFallbackTestAPIClient: APIClientProtocol {
    func request<T>(_ endpoint: APIEndpoint, as type: T.Type) async throws -> T where T: Decodable {
        let payload = """
        {
          "ack": {
            "client_msg_id": "client-1",
            "server_msg_id": "server-1",
            "timestamp": 1700000000
          },
          "bot_message": {
            "id": "bot-msg-1",
            "conversation_id": "conv-test",
            "sender_type": "bot",
            "content": {
              "type": "text",
              "text": "你好，我在。"
            },
            "is_proactive": false,
            "emotion_tag": "gentle",
            "created_at": "2026-02-14T00:00:00Z"
          },
          "bot_status_update": {
            "conversation_id": "conv-test",
            "emoji": "💭",
            "text": "在想你"
          }
        }
        """
        let data = Data(payload.utf8)
        return try JSONDecoder.nukara.decode(type, from: data)
    }

    func requestVoid(_ endpoint: APIEndpoint) async throws {
        _ = endpoint
    }
}

private final class FailingWebSocketClient: WebSocketClientProtocol {
    var textEvents: AsyncStream<String> {
        AsyncStream { _ in }
    }

    func connect(url: URL, accessToken: String?) {
        _ = url
        _ = accessToken
    }

    func disconnect() {}

    func send(_ text: String) async throws {
        _ = text
        throw NetworkError.transport("mock send failure")
    }
}
