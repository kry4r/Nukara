import Foundation

final class MessageCacheStore {
    private let fileURL: URL

    init(fileManager: FileManager = .default) {
        let baseURL = fileManager.urls(for: .cachesDirectory, in: .userDomainMask).first
            ?? URL(fileURLWithPath: NSTemporaryDirectory())
        self.fileURL = baseURL.appendingPathComponent("nukara_messages.json")
    }

    func loadMessages(conversationID: String, limit: Int) -> [ChatMessage] {
        let map = loadMap()
        return Array((map[conversationID] ?? []).suffix(limit))
    }

    func saveMessages(_ messages: [ChatMessage], for conversationID: String) {
        var map = loadMap()
        map[conversationID] = messages
        saveMap(map)
    }

    private func loadMap() -> [String: [ChatMessage]] {
        guard let data = try? Data(contentsOf: fileURL) else { return [:] }
        return (try? JSONDecoder.nukara.decode([String: [ChatMessage]].self, from: data)) ?? [:]
    }

    private func saveMap(_ map: [String: [ChatMessage]]) {
        guard let data = try? JSONEncoder.nukara.encode(map) else { return }
        try? data.write(to: fileURL, options: .atomic)
    }
}
