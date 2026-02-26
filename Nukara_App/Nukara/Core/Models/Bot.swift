import Foundation

enum BotGender: String, Codable, Equatable, CaseIterable, Identifiable {
    case unknown
    case male
    case female
    case nonBinary

    var id: String { rawValue }

    var displayName: String {
        switch self {
        case .unknown: return "未设置"
        case .male: return "男"
        case .female: return "女"
        case .nonBinary: return "非二元"
        }
    }
}

enum ChatBackgroundStyle: String, Codable, Equatable, CaseIterable, Identifiable {
    case lightPaper
    case softMint
    case peachGlow
    case calmNight

    var id: String { rawValue }

    var displayName: String {
        switch self {
        case .lightPaper: return "浅纸色"
        case .softMint: return "薄荷"
        case .peachGlow: return "暖桃"
        case .calmNight: return "静夜"
        }
    }
}

struct Bot: Codable, Equatable, Identifiable {
    let id: String
    var name: String
    var avatarURL: URL?
    var avatarData: Data?
    var summary: String
    var speakingStyle: String
    var background: String
    var traits: [String]
    var gender: BotGender
    var chatBackgroundStyle: ChatBackgroundStyle
    var createdAt: Date
}
