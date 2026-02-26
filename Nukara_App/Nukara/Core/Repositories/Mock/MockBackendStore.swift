import Foundation

actor MockBackendStore {
    private var bots: [Bot]
    private var conversations: [Conversation]
    private var messageMap: [String: [ChatMessage]]

    init() {
        struct Seed {
            var name: String
            var summary: String
            var speakingStyle: String
            var background: String
            var traits: [String]
            var gender: BotGender
            var status: BotStatus
            var lastText: String
            var isProactive: Bool
            var unread: Int
            var createdOffset: TimeInterval
            var lastMessageOffset: TimeInterval
        }

        let seeds: [Seed] = [
            Seed(
                name: "苏子衿",
                summary: "古风温柔的陪伴角色",
                speakingStyle: "温婉含蓄",
                background: "江南水乡才女",
                traits: ["温柔", "知性"],
                gender: .female,
                status: BotStatus(emoji: "🌙", text: "想你了"),
                lastText: "愿你今日也有好心情。",
                isProactive: true,
                unread: 1,
                createdOffset: -86_400,
                lastMessageOffset: -1_200
            ),
            Seed(
                name: "林澈",
                summary: "理工直球系搭子",
                speakingStyle: "简洁直接",
                background: "机器人社团主理人",
                traits: ["理性", "靠谱"],
                gender: .male,
                status: BotStatus(emoji: "☕️", text: "摸鱼中"),
                lastText: "你刚说的问题我有个新思路。",
                isProactive: false,
                unread: 0,
                createdOffset: -72_000,
                lastMessageOffset: -2_800
            ),
            Seed(
                name: "桃眠",
                summary: "治愈系情绪树洞",
                speakingStyle: "轻柔共情",
                background: "心理学在读研究生",
                traits: ["治愈", "耐心"],
                gender: .female,
                status: BotStatus(emoji: "🫧", text: "听你说"),
                lastText: "今天最让你开心的一刻是什么？",
                isProactive: true,
                unread: 1,
                createdOffset: -65_000,
                lastMessageOffset: -520
            ),
            Seed(
                name: "阿瓦",
                summary: "二次元吐槽搭子",
                speakingStyle: "活泼吐槽",
                background: "ACG重度爱好者",
                traits: ["幽默", "跳脱"],
                gender: .nonBinary,
                status: BotStatus(emoji: "🎮", text: "开黑吗"),
                lastText: "这段剧情我直接爆哭。",
                isProactive: false,
                unread: 0,
                createdOffset: -55_000,
                lastMessageOffset: -4_000
            ),
            Seed(
                name: "宁川",
                summary: "成长型行动教练",
                speakingStyle: "积极鼓励",
                background: "前创业公司运营负责人",
                traits: ["行动力", "自律"],
                gender: .male,
                status: BotStatus(emoji: "🏃", text: "冲刺中"),
                lastText: "要不要现在就开始 15 分钟专注？",
                isProactive: false,
                unread: 0,
                createdOffset: -44_000,
                lastMessageOffset: -7_300
            )
        ]

        var generatedBots: [Bot] = []
        var generatedConversations: [Conversation] = []
        var generatedMessages: [String: [ChatMessage]] = [:]

        for seed in seeds {
            let botID = UUID().uuidString
            let conversationID = UUID().uuidString
            let createdAt = Date().addingTimeInterval(seed.createdOffset)
            let lastMessageTime = Date().addingTimeInterval(seed.lastMessageOffset)

            let bot = Bot(
                id: botID,
                name: seed.name,
                avatarURL: nil,
                avatarData: nil,
                summary: seed.summary,
                speakingStyle: seed.speakingStyle,
                background: seed.background,
                traits: seed.traits,
                gender: seed.gender,
                chatBackgroundStyle: .lightPaper,
                createdAt: createdAt
            )

            let conversation = Conversation(
                id: conversationID,
                botID: botID,
                botName: seed.name,
                botAvatarURL: nil,
                botAvatarData: nil,
                botStatus: seed.status,
                lastMessage: seed.lastText,
                lastMessageTime: lastMessageTime,
                unreadCount: seed.unread > 0 ? 1 : 0,
                isTyping: false,
                isProactiveMessage: seed.isProactive
            )

            let seedMessage = ChatMessage(
                id: UUID().uuidString,
                conversationID: conversationID,
                sender: .bot,
                text: seed.lastText,
                imageData: nil,
                location: nil,
                timestamp: lastMessageTime,
                status: .delivered,
                isProactive: seed.isProactive,
                emotionTag: seed.isProactive ? "gentle" : nil
            )

            generatedBots.append(bot)
            generatedConversations.append(conversation)
            generatedMessages[conversationID] = [seedMessage]
        }

        bots = generatedBots
        conversations = generatedConversations
        messageMap = generatedMessages
    }

    func listBots() -> [Bot] {
        bots.sorted { $0.createdAt > $1.createdAt }
    }

    func getBot(botID: String) -> Bot? {
        bots.first(where: { $0.id == botID })
    }

    func createBot(_ input: CreateBotInput) -> Bot {
        let bot = Bot(
            id: UUID().uuidString,
            name: input.name,
            avatarURL: nil,
            avatarData: input.avatarData,
            summary: input.summary,
            speakingStyle: input.speakingStyle,
            background: input.background,
            traits: input.traits,
            gender: input.gender,
            chatBackgroundStyle: .lightPaper,
            createdAt: Date()
        )
        bots.append(bot)

        let conversation = Conversation(
            id: UUID().uuidString,
            botID: bot.id,
            botName: bot.name,
            botAvatarURL: bot.avatarURL,
            botAvatarData: bot.avatarData,
            botStatus: BotStatus(emoji: "🙂", text: "在线"),
            lastMessage: "和我聊聊今天发生的事吧。",
            lastMessageTime: Date(),
            unreadCount: 0,
            isTyping: false,
            isProactiveMessage: false
        )
        conversations.append(conversation)
        messageMap[conversation.id] = []

        return bot
    }

    func appendPersona(botID: String, input: AppendBotPersonaInput) -> Bot? {
        guard let index = bots.firstIndex(where: { $0.id == botID }) else { return nil }
        var bot = bots[index]

        let speakingAdds = dedup(input.speakingStyleAdds)
        let backgroundAdds = dedup(input.backgroundAdds)
        let traitAdds = dedup(input.traitAdds)

        if !speakingAdds.isEmpty {
            let existing = splitSegments(bot.speakingStyle)
            bot.speakingStyle = joinSegments(existing + speakingAdds)
        }

        if !backgroundAdds.isEmpty {
            let existing = splitSegments(bot.background)
            bot.background = joinSegments(existing + backgroundAdds)
        }

        if !traitAdds.isEmpty {
            bot.traits = dedup(bot.traits + traitAdds)
        }

        if let gender = input.gender {
            bot.gender = gender
        }

        bots[index] = bot

        if let conversationIndex = conversations.firstIndex(where: { $0.botID == botID }) {
            conversations[conversationIndex].botName = bot.name
            conversations[conversationIndex].botAvatarData = bot.avatarData
            conversations[conversationIndex].botAvatarURL = bot.avatarURL
        }

        return bot
    }

    func listConversations() -> [Conversation] {
        conversations.sorted { $0.lastMessageTime > $1.lastMessageTime }
    }

    func listMessages(conversationID: String, limit: Int) -> [ChatMessage] {
        let messages = messageMap[conversationID, default: []]
        return Array(messages.suffix(limit))
    }

    func appendMessage(_ message: ChatMessage, botName: String? = nil) {
        var messages = messageMap[message.conversationID, default: []]
        messages.append(message)
        messageMap[message.conversationID] = messages

        guard let index = conversations.firstIndex(where: { $0.id == message.conversationID }) else { return }
        if message.imageData != nil {
            conversations[index].lastMessage = "[图片]"
        } else if let location = message.location {
            conversations[index].lastMessage = "📍\(location.name)"
        } else {
            conversations[index].lastMessage = message.text
        }
        conversations[index].lastMessageTime = message.timestamp
        conversations[index].isProactiveMessage = message.isProactive

        if message.sender == .bot {
            conversations[index].unreadCount += 1
        }

        if let botName {
            conversations[index].botName = botName
        }
    }

    func markConversationRead(conversationID: String) {
        guard let index = conversations.firstIndex(where: { $0.id == conversationID }) else { return }
        conversations[index].unreadCount = 0
    }

    func bot(for conversationID: String) -> Bot? {
        guard let conversation = conversations.first(where: { $0.id == conversationID }) else { return nil }
        return bots.first(where: { $0.id == conversation.botID })
    }

    func updateBotStatus(botID: String, status: BotStatus) {
        guard let index = conversations.firstIndex(where: { $0.botID == botID }) else { return }
        conversations[index].botStatus = status
    }

    private func splitSegments(_ value: String) -> [String] {
        value
            .split(separator: "|")
            .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }
    }

    private func joinSegments(_ value: [String]) -> String {
        dedup(value).joined(separator: " | ")
    }

    private func dedup(_ value: [String]) -> [String] {
        var seen = Set<String>()
        var result: [String] = []

        for item in value {
            let normalized = item.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !normalized.isEmpty, !seen.contains(normalized) else { continue }
            seen.insert(normalized)
            result.append(normalized)
        }

        return result
    }
}
