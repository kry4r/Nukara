import Foundation

protocol UserStatusServiceProtocol {
    func syncStatus(emoji: String, text: String) async
}

struct NoopUserStatusService: UserStatusServiceProtocol {
    func syncStatus(emoji: String, text: String) async {}
}

final class APIUserStatusService: UserStatusServiceProtocol {
    private let apiClient: APIClientProtocol

    init(apiClient: APIClientProtocol) {
        self.apiClient = apiClient
    }

    func syncStatus(emoji: String, text: String) async {
        do {
            let endpoint = try APIEndpoints.updateUserStatus(emoji: emoji, text: text)
            try await apiClient.requestVoid(endpoint)
        } catch {
            // Best effort — don't break UX if sync fails.
        }
    }
}
