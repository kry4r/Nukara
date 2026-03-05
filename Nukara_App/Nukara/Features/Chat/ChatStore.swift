import Combine
import Foundation

struct ChatState {
    var botID: String
    var botName: String
    var botAvatarData: Data?
    var botStatus: BotStatus
    var messages: [ChatMessage] = []
    var inputText: String = ""
    var isLoading: Bool = false
    var isRemoteTyping: Bool = false
    var chatBackgroundStyle: ChatBackgroundStyle
    var chatBackgroundColorHex: String?
    var chatBackgroundImageData: Data?
    var quotedMessage: ChatMessage?
    var errorMessage: String?
}

enum ChatAction {
    case onAppear
    case onDisappear
    case inputChanged(String)
    case sendTapped
    case imagePicked(Data)
    case locationPicked(ChatLocation)
    case socketEvent(ChatSocketEvent)
    case quoteTapped(ChatMessage)
    case clearQuote
}

extension Notification.Name {
    static let nukaraConversationUpdated = Notification.Name("nukara.conversation.updated")
}

@MainActor
final class ChatStore: ObservableObject {
    @Published private(set) var state: ChatState

    private let conversation: Conversation
    private let conversationRepository: ConversationRepositoryProtocol
    private let chatRepository: ChatRepositoryProtocol
    private let preferenceStore: BotPreferencesStore

    private var eventsTask: Task<Void, Never>?
    private var isConnecting = false
    private var streamDraftByReplyID: [String: String] = [:]

    init(
        conversation: Conversation,
        conversationRepository: ConversationRepositoryProtocol,
        chatRepository: ChatRepositoryProtocol,
        preferenceStore: BotPreferencesStore? = nil
    ) {
        self.conversation = conversation
        self.conversationRepository = conversationRepository
        self.chatRepository = chatRepository
        self.preferenceStore = preferenceStore ?? .shared

        let preference = self.preferenceStore.preference(for: conversation.botID)
        self.state = ChatState(
            botID: conversation.botID,
            botName: conversation.botName,
            botAvatarData: conversation.botAvatarData,
            botStatus: conversation.botStatus,
            chatBackgroundStyle: preference.chatBackgroundStyle,
            chatBackgroundColorHex: preference.chatBackgroundColorHex,
            chatBackgroundImageData: preference.chatBackgroundImageData
        )
    }

    func send(_ action: ChatAction) {
        switch action {
        case .onAppear:
            Task {
                await loadInitialMessages()
                await connectAndObserve()
            }

        case .onDisappear:
            Task {
                eventsTask?.cancel()
                eventsTask = nil
                await chatRepository.disconnect()
            }

        case .inputChanged(let text):
            state.inputText = text

        case .sendTapped:
            sendCurrentTextMessage()

        case .imagePicked(let data):
            sendImage(data)

        case .locationPicked(let location):
            sendLocation(location)

        case .socketEvent(let event):
            apply(event)

        case .quoteTapped(let message):
            state.quotedMessage = message

        case .clearQuote:
            state.quotedMessage = nil
        }
    }

    deinit {
        eventsTask?.cancel()
        let repository = chatRepository
        Task {
            await repository.disconnect()
        }
    }

    func clearError() {
        state.errorMessage = nil
    }

    func refreshPreferences() {
        let preference = preferenceStore.preference(for: state.botID)
        state.chatBackgroundStyle = preference.chatBackgroundStyle
        state.chatBackgroundColorHex = preference.chatBackgroundColorHex
        state.chatBackgroundImageData = preference.chatBackgroundImageData
    }

    private func loadInitialMessages() async {
        state.isLoading = true
        defer { state.isLoading = false }

        do {
            state.messages = try await conversationRepository.listMessages(conversationID: conversation.id, limit: 100)
            try? await conversationRepository.markConversationRead(conversationID: conversation.id)
            postConversationUpdate(lastMessage: state.messages.last, unreadCount: 0)
            state.errorMessage = nil
        } catch {
            state.errorMessage = error.localizedDescription
        }
    }

    private func connectAndObserve() async {
        guard !isConnecting else { return }
        isConnecting = true
        defer { isConnecting = false }

        do {
            try await chatRepository.connect()
        } catch {
            state.errorMessage = error.localizedDescription
            return
        }

        eventsTask?.cancel()
        let stream = chatRepository.events
        eventsTask = Task { [weak self] in
            for await event in stream {
                guard !Task.isCancelled else { break }
                self?.handleIncomingEvent(event)
            }
        }
    }

    private func handleIncomingEvent(_ event: ChatSocketEvent) {
        send(.socketEvent(event))
    }

    private func sendCurrentTextMessage() {
        let text = state.inputText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }

        var displayText = text
        if let quoted = state.quotedMessage {
            let quotedSender = quoted.sender == .bot ? state.botName : "我"
            let quotedPreview = String(quoted.text.prefix(50))
            displayText = "「\(quotedSender): \(quotedPreview)」\n\(text)"
        }

        let clientID = UUID().uuidString
        let draft = ChatMessage.userTextDraft(conversationID: conversation.id, text: displayText, clientMessageID: clientID)
        state.messages.append(draft)
        postConversationUpdate(lastMessage: draft, unreadCount: 0)
        state.inputText = ""
        state.quotedMessage = nil

        Task {
            do {
                await ensureConnectedForSending()
                try await chatRepository.sendText(conversationID: conversation.id, text: displayText, clientMessageID: clientID)
            } catch {
                markMessageFailed(clientMessageID: clientID)
                state.errorMessage = error.localizedDescription
            }
        }
    }

    private func sendImage(_ data: Data) {
        let clientID = UUID().uuidString
        let draft = ChatMessage.userImageDraft(conversationID: conversation.id, imageData: data, clientMessageID: clientID)
        state.messages.append(draft)
        postConversationUpdate(lastMessage: draft, unreadCount: 0)

        Task {
            do {
                await ensureConnectedForSending()
                try await chatRepository.sendImage(conversationID: conversation.id, imageData: data, clientMessageID: clientID)
            } catch {
                markMessageFailed(clientMessageID: clientID)
                state.errorMessage = error.localizedDescription
            }
        }
    }

    private func sendLocation(_ location: ChatLocation) {
        let clientID = UUID().uuidString
        let draft = ChatMessage.userLocationDraft(conversationID: conversation.id, location: location, clientMessageID: clientID)
        state.messages.append(draft)
        postConversationUpdate(lastMessage: draft, unreadCount: 0)

        Task {
            do {
                await ensureConnectedForSending()
                try await chatRepository.sendLocation(
                    conversationID: conversation.id,
                    location: location,
                    clientMessageID: clientID
                )
            } catch {
                markMessageFailed(clientMessageID: clientID)
                state.errorMessage = error.localizedDescription
            }
        }
    }

    private func markMessageFailed(clientMessageID: String) {
        guard let index = state.messages.firstIndex(where: { $0.id == clientMessageID }) else { return }
        var message = state.messages[index]
        message.status = .failed
        state.messages[index] = message
    }

    private func apply(_ event: ChatSocketEvent) {
        switch event {
        case .ack(let clientMessageID, let serverMessageID, let timestamp):
            guard let index = state.messages.firstIndex(where: { $0.id == clientMessageID }) else { return }
            let old = state.messages[index]
            state.messages[index] = ChatMessage(
                id: serverMessageID,
                conversationID: old.conversationID,
                sender: old.sender,
                text: old.text,
                imageData: old.imageData,
                location: old.location,
                timestamp: timestamp,
                status: .sent,
                isProactive: old.isProactive,
                emotionTag: old.emotionTag
            )
            postConversationUpdate(lastMessage: state.messages[index], unreadCount: 0)

        case .streamStart(let conversationID, let replyID):
            guard conversationID == conversation.id else { return }
            state.isRemoteTyping = true
            let draftID = "stream-\(replyID)"
            streamDraftByReplyID[replyID] = draftID
            if !state.messages.contains(where: { $0.id == draftID }) {
                let draft = ChatMessage(
                    id: draftID,
                    conversationID: conversationID,
                    sender: .bot,
                    text: "",
                    imageData: nil,
                    location: nil,
                    timestamp: Date(),
                    status: .sending,
                    isProactive: false,
                    emotionTag: nil
                )
                state.messages.append(draft)
            }

        case .streamChunk(let conversationID, let replyID, let delta):
            guard conversationID == conversation.id else { return }
            guard let draftID = streamDraftByReplyID[replyID] else { return }
            guard let index = state.messages.firstIndex(where: { $0.id == draftID }) else { return }
            let old = state.messages[index]
            state.messages[index] = ChatMessage(
                id: old.id,
                conversationID: old.conversationID,
                sender: old.sender,
                text: old.text + delta,
                imageData: old.imageData,
                location: old.location,
                timestamp: old.timestamp,
                status: .sending,
                isProactive: old.isProactive,
                emotionTag: old.emotionTag
            )

        case .streamEnd(let conversationID, _):
            guard conversationID == conversation.id else { return }
            state.isRemoteTyping = false

        case .multiReplyStart(let conversationID, _, _):
            guard conversationID == conversation.id else { return }
            state.isRemoteTyping = true

        case .multiReplyEnd(let conversationID, _):
            guard conversationID == conversation.id else { return }
            state.isRemoteTyping = false
            markConversationReadAsync()
            if let lastBot = state.messages.last(where: { $0.sender == .bot }) {
                postConversationUpdate(lastMessage: lastBot, unreadCount: 0)
            }

        case .message(let message):
            guard message.conversationID == conversation.id else { return }
            state.isRemoteTyping = false
            if let replyID = message.replyGroupID, let draftID = streamDraftByReplyID[replyID] {
                if let index = state.messages.firstIndex(where: { $0.id == draftID }) {
                    state.messages.remove(at: index)
                }
                streamDraftByReplyID.removeValue(forKey: replyID)
            }
            if let index = state.messages.firstIndex(where: { $0.id == message.id }) {
                state.messages[index] = message
            } else {
                state.messages.append(message)
            }
            if message.sender == .bot {
                markConversationReadAsync()
            }
            postConversationUpdate(lastMessage: message, unreadCount: 0)

        case .botStatusUpdate(let conversationID, let status):
            guard conversationID == conversation.id else { return }
            state.botStatus = status
            postConversationStatusUpdate(status)

        case .botPersonaUpdated(let botID, let summary, _):
            guard botID == conversation.botID else { return }
            if !summary.isEmpty {
                state.errorMessage = summary
            }

        case .proactiveMessage(let message):
            guard message.conversationID == conversation.id else { return }
            state.messages.append(message)
            markConversationReadAsync()
            postConversationUpdate(lastMessage: message, unreadCount: 0)

        case .typing(let conversationID, let isTyping):
            guard conversationID == conversation.id else { return }
            state.isRemoteTyping = isTyping

        case .error(let message):
            state.errorMessage = message

        case .unknown:
            break
        }
    }

    private func markConversationReadAsync() {
        Task {
            try? await conversationRepository.markConversationRead(conversationID: conversation.id)
        }
    }

    private func postConversationUpdate(lastMessage: ChatMessage?, unreadCount: Int) {
        guard let lastMessage else { return }

        let preview: String
        if lastMessage.imageData != nil {
            preview = "[图片]"
        } else if let location = lastMessage.location {
            preview = "📍\(location.name)"
        } else {
            preview = lastMessage.text
        }

        NotificationCenter.default.post(
            name: .nukaraConversationUpdated,
            object: nil,
            userInfo: [
                "conversation_id": conversation.id,
                "last_message": preview,
                "last_message_time": lastMessage.timestamp.timeIntervalSince1970,
                "unread_count": unreadCount,
                "bot_status_emoji": state.botStatus.emoji,
                "bot_status_text": state.botStatus.text
            ]
        )
    }

    private func postConversationStatusUpdate(_ status: BotStatus) {
        NotificationCenter.default.post(
            name: .nukaraConversationUpdated,
            object: nil,
            userInfo: [
                "conversation_id": conversation.id,
                "bot_status_emoji": status.emoji,
                "bot_status_text": status.text
            ]
        )
    }

    private func ensureConnectedForSending() async {
        if eventsTask == nil {
            await connectAndObserve()
        }
    }
}
