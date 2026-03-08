import Foundation

enum HTTPMethod: String {
    case get = "GET"
    case post = "POST"
    case put = "PUT"
    case patch = "PATCH"
    case delete = "DELETE"
}

struct APIEndpoint {
    let method: HTTPMethod
    let path: String
    var queryItems: [URLQueryItem] = []
    var headers: [String: String] = [:]
    var body: Data? = nil
    var requiresAuth: Bool = true
}

enum APIEndpoints {
    static func requestEmailCode(email: String, purpose: AuthCodePurpose) throws -> APIEndpoint {
        let payload = RequestEmailCodeDTO(email: email, purpose: purpose.rawValue)
        let body = try JSONEncoder.nukara.encode(payload)
        return APIEndpoint(method: .post, path: "/api/v1/auth/email/send", body: body, requiresAuth: false)
    }

    static func login(email: String, emailCode: String) throws -> APIEndpoint {
        let payload = LoginRequestDTO(email: email, emailCode: emailCode)
        let body = try JSONEncoder.nukara.encode(payload)
        return APIEndpoint(method: .post, path: "/api/v1/auth/login", body: body, requiresAuth: false)
    }

    static func register(email: String, emailCode: String, nickname: String) throws -> APIEndpoint {
        let payload = RegisterRequestDTO(email: email, emailCode: emailCode, nickname: nickname)
        let body = try JSONEncoder.nukara.encode(payload)
        return APIEndpoint(method: .post, path: "/api/v1/auth/register", body: body, requiresAuth: false)
    }

    static func bots() -> APIEndpoint {
        APIEndpoint(method: .get, path: "/api/v1/bots")
    }

    static func createBot(_ input: CreateBotInput) throws -> APIEndpoint {
        let payload = CreateBotRequestDTO(
            name: input.name,
            description: input.summary,
            speakingStyle: input.speakingStyle,
            background: input.background,
            traits: input.traits,
            gender: input.gender.rawValue,
            avatarBase64: input.avatarData?.base64EncodedString()
        )
        let body = try JSONEncoder.nukara.encode(payload)
        return APIEndpoint(method: .post, path: "/api/v1/bots", body: body)
    }

    static func bot(botID: String) -> APIEndpoint {
        APIEndpoint(method: .get, path: "/api/v1/bots/\(botID)")
    }

    static func appendBotPersona(botID: String, input: AppendBotPersonaInput) throws -> APIEndpoint {
        let payload = AppendBotPersonaRequestDTO(
            speakingStyleAdds: input.speakingStyleAdds,
            backgroundAdds: input.backgroundAdds,
            traitAdds: input.traitAdds,
            gender: input.gender?.rawValue
        )
        let body = try JSONEncoder.nukara.encode(payload)
        return APIEndpoint(method: .patch, path: "/api/v1/bots/\(botID)", body: body)
    }

    static func appendBotPersonaLegacy(botID: String, input: AppendBotPersonaInput) throws -> APIEndpoint {
        let payload = AppendBotPersonaRequestDTO(
            speakingStyleAdds: input.speakingStyleAdds,
            backgroundAdds: input.backgroundAdds,
            traitAdds: input.traitAdds,
            gender: input.gender?.rawValue
        )
        let body = try JSONEncoder.nukara.encode(payload)
        return APIEndpoint(method: .patch, path: "/api/v1/bots/\(botID)/persona", body: body)
    }

    static func conversations() -> APIEndpoint {
        APIEndpoint(method: .get, path: "/api/v1/conversations")
    }

    static func conversationMessages(conversationID: String, limit: Int) -> APIEndpoint {
        APIEndpoint(
            method: .get,
            path: "/api/v1/conversations/\(conversationID)/messages",
            queryItems: [URLQueryItem(name: "limit", value: "\(limit)")]
        )
    }

    static func markConversationRead(conversationID: String) -> APIEndpoint {
        APIEndpoint(method: .post, path: "/api/v1/conversations/\(conversationID)/mark-read")
    }

    static func markConversationReadLegacy(conversationID: String) -> APIEndpoint {
        APIEndpoint(method: .post, path: "/api/v1/conversations/\(conversationID)/read")
    }

    static func conversationSendMessage(conversationID: String, request: ConversationSendMessageRequestDTO) throws -> APIEndpoint {
        let body = try JSONEncoder.nukara.encode(request)
        return APIEndpoint(method: .post, path: "/api/v1/conversations/\(conversationID)/send", body: body)
    }

    static func registerDeviceToken(deviceToken: String, platform: String) throws -> APIEndpoint {
        let payload = RegisterDeviceTokenRequestDTO(deviceToken: deviceToken, platform: platform)
        let body = try JSONEncoder.nukara.encode(payload)
        return APIEndpoint(method: .post, path: "/api/v1/users/device-token", body: body)
    }

    static func updateUserStatus(emoji: String, text: String) throws -> APIEndpoint {
        let payload = UserStatusRequestDTO(emoji: emoji, text: text)
        let body = try JSONEncoder.nukara.encode(payload)
        return APIEndpoint(method: .put, path: "/api/v1/users/status", body: body)
    }

    static func notificationSettings() -> APIEndpoint {
        APIEndpoint(method: .get, path: "/api/v1/users/notification-settings")
    }

    static func updateNotificationSettings(_ input: NotificationSettingsRequestDTO) throws -> APIEndpoint {
        let body = try JSONEncoder.nukara.encode(input)
        return APIEndpoint(method: .put, path: "/api/v1/users/notification-settings", body: body)
    }
}

struct LoginRequestDTO: Codable {
    let email: String
    let emailCode: String

    enum CodingKeys: String, CodingKey {
        case email
        case emailCode = "email_code"
    }
}

struct RequestEmailCodeDTO: Codable {
    let email: String
    let purpose: String
}

struct RegisterRequestDTO: Codable {
    let email: String
    let emailCode: String
    let nickname: String

    enum CodingKeys: String, CodingKey {
        case email
        case emailCode = "email_code"
        case nickname
    }
}

struct CreateBotRequestDTO: Codable {
    let name: String
    let description: String
    let speakingStyle: String
    let background: String
    let traits: [String]
    let gender: String
    let avatarBase64: String?

    enum CodingKeys: String, CodingKey {
        case name
        case description
        case speakingStyle = "speaking_style"
        case background
        case traits
        case gender
        case avatarBase64 = "avatar_base64"
    }
}

struct AppendBotPersonaRequestDTO: Codable {
    let speakingStyleAdds: [String]
    let backgroundAdds: [String]
    let traitAdds: [String]
    let gender: String?

    enum CodingKeys: String, CodingKey {
        case speakingStyleAdds = "speaking_style_adds"
        case backgroundAdds = "background_adds"
        case traitAdds = "trait_adds"
        case gender
    }
}

struct RegisterDeviceTokenRequestDTO: Codable {
    let deviceToken: String
    let platform: String

    enum CodingKeys: String, CodingKey {
        case deviceToken = "device_token"
        case platform
    }
}

struct NotificationSettingsRequestDTO: Codable {
    let proactiveEnabled: Bool
    let dndStart: String?
    let dndEnd: String?
    let frequency: String

    enum CodingKeys: String, CodingKey {
        case proactiveEnabled = "proactive_enabled"
        case dndStart = "dnd_start"
        case dndEnd = "dnd_end"
        case frequency
    }
}

struct UserStatusRequestDTO: Codable {
    let emoji: String
    let text: String
}

struct ConversationSendMessageRequestDTO: Codable {
    let clientMessageID: String
    let content: ConversationSendContentDTO

    enum CodingKeys: String, CodingKey {
        case clientMessageID = "client_msg_id"
        case content
    }
}

struct ConversationSendContentDTO: Codable {
    let type: String
    let text: String?
    let imageBase64: String?
    let latitude: Double?
    let longitude: Double?
    let name: String?

    enum CodingKeys: String, CodingKey {
        case type
        case text
        case imageBase64 = "image_base64"
        case latitude
        case longitude
        case name
    }
}
