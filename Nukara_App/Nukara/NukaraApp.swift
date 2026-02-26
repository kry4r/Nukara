import SwiftUI

@main
struct NukaraApp: App {
    @UIApplicationDelegateAdaptor(NukaraPushDelegate.self) private var pushDelegate
    private let appEnvironment: AppEnvironment
    @StateObject private var sessionStore: SessionStore

    init() {
        let environment = AppEnvironment.makeDefault()
        self.appEnvironment = environment
        _sessionStore = StateObject(wrappedValue: SessionStore(authRepository: environment.authRepository))
        PushRegistrationBridge.shared.service = environment.pushRegistrationService
    }

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(\.appEnvironment, appEnvironment)
                .environmentObject(sessionStore)
        }
    }
}
