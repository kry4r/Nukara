import Foundation

@MainActor
final class PushRegistrationBridge {
    static let shared = PushRegistrationBridge()

    var service: PushRegistrationServiceProtocol = NoopPushRegistrationService()

    private init() {}

    func submit(tokenData: Data) {
        let token = tokenData.map { String(format: "%02x", $0) }.joined()
        Task {
            await service.registerDeviceToken(token)
        }
    }
}
