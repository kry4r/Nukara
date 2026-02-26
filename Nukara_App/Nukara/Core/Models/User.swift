import Foundation

struct User: Codable, Equatable, Identifiable {
    let id: String
    var nickname: String
    var avatarURL: URL?
}

struct TokenBalance: Codable, Equatable {
    var remaining: Int
    var consumedToday: Int
}
