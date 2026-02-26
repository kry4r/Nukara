import Foundation

protocol PushRegistrationServiceProtocol {
    func registerDeviceToken(_ token: String) async
}

struct NoopPushRegistrationService: PushRegistrationServiceProtocol {
    func registerDeviceToken(_ token: String) async {
        _ = token
    }
}

final class APIPushRegistrationService: PushRegistrationServiceProtocol {
    private let apiClient: APIClientProtocol

    init(apiClient: APIClientProtocol) {
        self.apiClient = apiClient
    }

    func registerDeviceToken(_ token: String) async {
        guard !token.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return }
        do {
            let endpoint = try APIEndpoints.registerDeviceToken(deviceToken: token, platform: "ios")
            try await apiClient.requestVoid(endpoint)
        } catch {
            // Push token report is best effort and should not break UX.
        }
    }
}
