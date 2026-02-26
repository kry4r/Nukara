import SwiftUI

@MainActor
struct ConversationsView: View {
    @Environment(\.appEnvironment) private var appEnvironment

    @State private var conversations: [Conversation] = []
    @State private var isLoading = false
    @State private var errorMessage: String?
    @State private var searchQuery: String = ""

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                topBar

                if isLoading && conversations.isEmpty {
                    Spacer()
                    ProgressView()
                    Spacer()
                } else {
                    ScrollView {
                        LazyVStack(spacing: 12) {
                            ForEach(filteredConversations) { conversation in
                                NavigationLink {
                                    ChatScene(
                                        conversation: conversation,
                                        conversationRepository: appEnvironment.conversationRepository,
                                        chatRepository: appEnvironment.chatRepository
                                    )
                                } label: {
                                    ConversationCard(conversation: conversation)
                                }
                                .buttonStyle(.plain)
                            }
                        }
                        .padding(16)
                    }
                    .refreshable {
                        await loadConversations()
                    }
                }
            }
            .background(Color.nkBackground.ignoresSafeArea())
            .task {
                await loadConversations()
            }
            .onAppear {
                Task { await loadConversations() }
            }
            .onReceive(NotificationCenter.default.publisher(for: .nukaraConversationUpdated)) { notification in
                applyConversationUpdate(notification.userInfo)
            }
            .alert("加载失败", isPresented: Binding(get: {
                errorMessage != nil
            }, set: { value in
                if !value { errorMessage = nil }
            })) {
                Button("确定", role: .cancel) {}
            } message: {
                Text(errorMessage ?? "未知错误")
            }
        }
    }

    private var topBar: some View {
        VStack(spacing: 10) {
            HStack {
                Spacer()
                Text("纽扣")
                    .font(.title3.weight(.semibold))
                    .foregroundStyle(Color.nkPrimaryText)
                Spacer()
            }

            HStack(spacing: 8) {
                Image(systemName: "magnifyingglass")
                    .foregroundStyle(Color.nkTextMuted)
                TextField("搜索机器人 / 消息 / 状态", text: $searchQuery)
                    .font(.subheadline)
                    .foregroundStyle(Color.nkPrimaryText)
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 9)
            .background(
                RoundedRectangle(cornerRadius: 12, style: .continuous)
                    .fill(Color.nkCardBackground)
            )
        }
        .padding(.horizontal, 16)
        .padding(.top, 8)
        .padding(.bottom, 10)
    }

    private func loadConversations() async {
        isLoading = true
        defer { isLoading = false }

        do {
            conversations = try await appEnvironment.conversationRepository.listConversations()
            errorMessage = nil
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    private func applyConversationUpdate(_ userInfo: [AnyHashable: Any]?) {
        guard
            let userInfo,
            let conversationID = userInfo["conversation_id"] as? String,
            let index = conversations.firstIndex(where: { $0.id == conversationID })
        else {
            return
        }

        if let lastMessage = userInfo["last_message"] as? String {
            conversations[index].lastMessage = lastMessage
        }
        if let timestampRaw = userInfo["last_message_time"] as? TimeInterval {
            conversations[index].lastMessageTime = Date(timeIntervalSince1970: timestampRaw)
        }
        if let unreadCount = userInfo["unread_count"] as? Int {
            conversations[index].unreadCount = unreadCount
        }
        if let statusEmoji = userInfo["bot_status_emoji"] as? String,
           let statusText = userInfo["bot_status_text"] as? String {
            conversations[index].botStatus = BotStatus(emoji: statusEmoji, text: statusText)
        }

        conversations.sort { $0.lastMessageTime > $1.lastMessageTime }
    }

    private var filteredConversations: [Conversation] {
        let query = searchQuery.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !query.isEmpty else { return conversations }

        return conversations.filter { conversation in
            conversation.botName.localizedCaseInsensitiveContains(query)
            || conversation.lastMessage.localizedCaseInsensitiveContains(query)
            || conversation.botStatus.text.localizedCaseInsensitiveContains(query)
            || conversation.botStatus.emoji.localizedCaseInsensitiveContains(query)
        }
    }
}

private struct ConversationCard: View {
    let conversation: Conversation

    var body: some View {
        HStack(spacing: 12) {
            AvatarView(name: conversation.botName, imageData: conversation.botAvatarData, size: 52)

            VStack(alignment: .leading, spacing: 6) {
                HStack(spacing: 8) {
                    Text(conversation.botName)
                        .font(.headline)
                        .foregroundStyle(Color.nkPrimaryText)

                    HStack(spacing: 4) {
                        Text(conversation.botStatus.emoji)
                            .font(.caption2)
                        Text(conversation.botStatus.text)
                            .font(.caption2)
                            .foregroundStyle(Color.nkTextSecondary)
                    }
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(Color.nkAccent.opacity(0.12))
                    .clipShape(Capsule())

                    Spacer(minLength: 0)
                }

                Text(conversation.lastMessage)
                    .font(.subheadline)
                    .foregroundStyle(Color.nkTextSecondary)
                    .lineLimit(1)
            }

            Spacer()

            VStack(alignment: .trailing, spacing: 6) {
                Text(conversation.lastMessageTime, style: .time)
                    .font(.caption2)
                    .foregroundStyle(Color.nkTextMuted)

                if conversation.unreadCount > 0 {
                    Text("\(conversation.unreadCount)")
                        .font(.caption2)
                        .foregroundStyle(.white)
                        .padding(.horizontal, 6)
                        .padding(.vertical, 2)
                        .background(Color.nkAccent)
                        .clipShape(Capsule())
                }
            }
        }
        .padding(14)
        .background(
            RoundedRectangle(cornerRadius: 16, style: .continuous)
                .fill(Color.nkCardBackground)
        )
    }
}
