import Foundation

struct BotPreference: Codable, Equatable {
    var chatBackgroundStyle: ChatBackgroundStyle
    var chatBackgroundColorHex: String?
    var chatBackgroundImageData: Data?

    init(
        chatBackgroundStyle: ChatBackgroundStyle = .lightPaper,
        chatBackgroundColorHex: String? = nil,
        chatBackgroundImageData: Data? = nil
    ) {
        self.chatBackgroundStyle = chatBackgroundStyle
        self.chatBackgroundColorHex = chatBackgroundColorHex
        self.chatBackgroundImageData = chatBackgroundImageData
    }
}

final class BotPreferencesStore {
    static let shared = BotPreferencesStore()

    private let keyPrefix = "nukara.bot.preference."
    private let defaults: UserDefaults

    init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    func preference(for botID: String) -> BotPreference {
        let key = keyPrefix + botID
        guard let data = defaults.data(forKey: key),
              let preference = try? JSONDecoder.nukara.decode(BotPreference.self, from: data) else {
            return BotPreference()
        }
        return preference
    }

    func updatePreference(_ preference: BotPreference, for botID: String) {
        let key = keyPrefix + botID
        guard let data = try? JSONEncoder.nukara.encode(preference) else { return }
        defaults.set(data, forKey: key)
    }
}
