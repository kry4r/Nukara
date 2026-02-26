import Foundation

final class MockChatRepository: ChatRepositoryProtocol {
    var events: AsyncStream<ChatSocketEvent> { eventStream }

    private let backend: MockBackendStore
    private let eventStream: AsyncStream<ChatSocketEvent>
    private let continuation: AsyncStream<ChatSocketEvent>.Continuation

    init(backend: MockBackendStore) {
        self.backend = backend

        var capturedContinuation: AsyncStream<ChatSocketEvent>.Continuation?
        self.eventStream = AsyncStream { continuation in
            capturedContinuation = continuation
        }
        guard let continuation = capturedContinuation else {
            fatalError("Failed to initialize mock chat stream")
        }
        self.continuation = continuation
    }

    func connect() async throws {
        // No-op for mock.
    }

    func disconnect() async {
        // Keep stream alive during app lifecycle.
    }

    func sendText(conversationID: String, text: String, clientMessageID: String) async throws {
        let userMessage = ChatMessage(
            id: UUID().uuidString,
            conversationID: conversationID,
            sender: .user,
            text: text,
            imageData: nil,
            location: nil,
            timestamp: Date(),
            status: .sent,
            isProactive: false,
            emotionTag: nil
        )
        await backend.appendMessage(userMessage)

        continuation.yield(
            .ack(
                clientMessageID: clientMessageID,
                serverMessageID: userMessage.id,
                timestamp: Date()
            )
        )

        let replyText = await generateReply(for: conversationID, userText: text)
        let replyID = UUID().uuidString

        let replyGroupID = UUID().uuidString
        continuation.yield(.typing(conversationID: conversationID, isTyping: true))
        continuation.yield(.multiReplyStart(conversationID: conversationID, replyGroupID: replyGroupID, count: 0))

        try await Task.sleep(for: .milliseconds(200))

        let botMessage = ChatMessage(
            id: replyID,
            conversationID: conversationID,
            sender: .bot,
            text: replyText,
            imageData: nil,
            location: nil,
            timestamp: Date(),
            status: .delivered,
            isProactive: false,
            emotionTag: "gentle",
            replyGroupID: replyGroupID,
            sequence: 0
        )
        continuation.yield(.message(botMessage))
        await backend.appendMessage(botMessage)

        continuation.yield(.multiReplyEnd(conversationID: conversationID, replyGroupID: replyGroupID))
        continuation.yield(.typing(conversationID: conversationID, isTyping: false))
        await emitStatusUpdate(conversationID: conversationID)
    }

    func sendImage(conversationID: String, imageData: Data, clientMessageID: String) async throws {
        let userMessage = ChatMessage(
            id: UUID().uuidString,
            conversationID: conversationID,
            sender: .user,
            text: "",
            imageData: imageData,
            location: nil,
            timestamp: Date(),
            status: .sent,
            isProactive: false,
            emotionTag: nil
        )
        await backend.appendMessage(userMessage)

        continuation.yield(
            .ack(
                clientMessageID: clientMessageID,
                serverMessageID: userMessage.id,
                timestamp: Date()
            )
        )

        let replyText = "我看到你发来的图片了，看起来很有意思。"
        let replyID = UUID().uuidString
        let replyGroupID = UUID().uuidString
        continuation.yield(.multiReplyStart(conversationID: conversationID, replyGroupID: replyGroupID, count: 0))
        try await Task.sleep(for: .milliseconds(200))

        let botMessage = ChatMessage(
            id: replyID,
            conversationID: conversationID,
            sender: .bot,
            text: replyText,
            imageData: nil,
            location: nil,
            timestamp: Date(),
            status: .delivered,
            isProactive: false,
            emotionTag: nil,
            replyGroupID: replyGroupID,
            sequence: 0
        )
        continuation.yield(.message(botMessage))
        await backend.appendMessage(botMessage)
        continuation.yield(.multiReplyEnd(conversationID: conversationID, replyGroupID: replyGroupID))
        await emitStatusUpdate(conversationID: conversationID)
    }

    func sendLocation(conversationID: String, location: ChatLocation, clientMessageID: String) async throws {
        let userMessage = ChatMessage(
            id: UUID().uuidString,
            conversationID: conversationID,
            sender: .user,
            text: "",
            imageData: nil,
            location: location,
            timestamp: Date(),
            status: .sent,
            isProactive: false,
            emotionTag: nil
        )
        await backend.appendMessage(userMessage)

        continuation.yield(
            .ack(
                clientMessageID: clientMessageID,
                serverMessageID: userMessage.id,
                timestamp: Date()
            )
        )

        let replyText = "已收到你分享的位置：\(location.name)。看起来很不错。"
        let replyID = UUID().uuidString
        let replyGroupID = UUID().uuidString
        continuation.yield(.multiReplyStart(conversationID: conversationID, replyGroupID: replyGroupID, count: 0))
        try await Task.sleep(for: .milliseconds(200))

        let botMessage = ChatMessage(
            id: replyID,
            conversationID: conversationID,
            sender: .bot,
            text: replyText,
            imageData: nil,
            location: nil,
            timestamp: Date(),
            status: .delivered,
            isProactive: false,
            emotionTag: nil,
            replyGroupID: replyGroupID,
            sequence: 0
        )
        continuation.yield(.message(botMessage))
        await backend.appendMessage(botMessage)
        continuation.yield(.multiReplyEnd(conversationID: conversationID, replyGroupID: replyGroupID))
        await emitStatusUpdate(conversationID: conversationID)
    }

    private func generateReply(for conversationID: String, userText: String) async -> String {
        _ = await backend.bot(for: conversationID)
        return "我记住啦，你刚刚说的是“\(userText)”。想继续聊聊这件事吗？"
    }

    private func emitStatusUpdate(conversationID: String) async {
        let statuses: [BotStatus] = [
            BotStatus(emoji: "💭", text: "在想你"),
            BotStatus(emoji: "☕️", text: "慢慢聊"),
            BotStatus(emoji: "🎵", text: "听歌中"),
            BotStatus(emoji: "🌙", text: "夜聊"),
            BotStatus(emoji: "✨", text: "灵感中")
        ]

        guard let bot = await backend.bot(for: conversationID) else { return }
        let index = abs(conversationID.hashValue + Int(Date().timeIntervalSince1970)) % statuses.count
        let status = statuses[index]
        await backend.updateBotStatus(botID: bot.id, status: status)

        continuation.yield(.botStatusUpdate(conversationID: conversationID, status: status))
    }
}
