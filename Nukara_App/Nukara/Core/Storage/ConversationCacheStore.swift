import Foundation

final class ConversationCacheStore {
    private let fileURL: URL

    init(fileManager: FileManager = .default) {
        let baseURL = fileManager.urls(for: .cachesDirectory, in: .userDomainMask).first
            ?? URL(fileURLWithPath: NSTemporaryDirectory())
        self.fileURL = baseURL.appendingPathComponent("nukara_conversations.json")
    }

    func loadConversations() -> [Conversation] {
        guard let data = try? Data(contentsOf: fileURL) else { return [] }
        return (try? JSONDecoder.nukara.decode([Conversation].self, from: data)) ?? []
    }

    func saveConversations(_ conversations: [Conversation]) {
        guard let data = try? JSONEncoder.nukara.encode(conversations) else { return }
        try? data.write(to: fileURL, options: .atomic)
    }
}
