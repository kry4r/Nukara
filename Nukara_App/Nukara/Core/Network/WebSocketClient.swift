import Foundation

protocol WebSocketClientProtocol {
    var textEvents: AsyncStream<String> { get }

    func connect(url: URL, accessToken: String?)
    func disconnect()
    func send(_ text: String) async throws
}

final class WebSocketClient: WebSocketClientProtocol {
    private let session: URLSession
    private var task: URLSessionWebSocketTask?
    private var continuation: AsyncStream<String>.Continuation?

    private var socketURL: URL?
    private var accessToken: String?
    private var isManuallyDisconnected = false
    private var reconnectAttempts = 0
    private var reconnectTask: Task<Void, Never>?
    private var heartbeatTask: Task<Void, Never>?

    lazy var textEvents: AsyncStream<String> = {
        AsyncStream { continuation in
            self.continuation = continuation
        }
    }()

    init(session: URLSession = .shared) {
        self.session = session
    }

    func connect(url: URL, accessToken: String?) {
        socketURL = url
        self.accessToken = accessToken
        isManuallyDisconnected = false
        reconnectAttempts = 0
        openConnection()
    }

    func disconnect() {
        isManuallyDisconnected = true
        reconnectAttempts = 0
        reconnectTask?.cancel()
        reconnectTask = nil
        stopHeartbeat()
        task?.cancel(with: .goingAway, reason: nil)
        task = nil
    }

    func send(_ text: String) async throws {
        // If task is nil, attempt immediate reconnect before failing
        if task == nil, !isManuallyDisconnected {
            openConnection()
            try? await Task.sleep(nanoseconds: 500_000_000)
        }

        guard let task else {
            throw NetworkError.transport("WebSocket is not connected")
        }
        do {
            try await task.send(.string(text))
        } catch {
            invalidateTask()
            scheduleReconnect()
            throw NetworkError.transport(error.localizedDescription)
        }
    }

    private func openConnection() {
        reconnectTask?.cancel()
        reconnectTask = nil
        stopHeartbeat()

        // Cancel stale task before creating a new one
        task?.cancel(with: .abnormalClosure, reason: nil)
        task = nil

        guard let socketURL else { return }

        var request = URLRequest(url: socketURL)
        if let token = accessToken, !token.isEmpty {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        let task = session.webSocketTask(with: request)
        self.task = task
        task.resume()

        startHeartbeat()
        receiveLoop(for: task)
    }

    private func receiveLoop(for task: URLSessionWebSocketTask) {
        task.receive { [weak self] result in
            guard let self else { return }
            switch result {
            case .success(let message):
                self.reconnectAttempts = 0
                switch message {
                case .string(let text):
                    self.continuation?.yield(text)
                case .data(let data):
                    let text = String(decoding: data, as: UTF8.self)
                    self.continuation?.yield(text)
                @unknown default:
                    break
                }
                self.receiveLoop(for: task)
            case .failure(let error):
                self.invalidateTask()
                self.continuation?.yield("{\"type\":\"error\",\"message\":\"\(error.localizedDescription)\"}")
                self.scheduleReconnect()
            }
        }
    }

    // Sends application-level ping that resets the backend's read deadline,
    // plus a protocol-level ping to verify the transport is alive.
    private func startHeartbeat() {
        heartbeatTask?.cancel()
        heartbeatTask = Task { [weak self] in
            while let self, !Task.isCancelled {
                try? await Task.sleep(nanoseconds: 25 * 1_000_000_000)
                guard !self.isManuallyDisconnected else { return }
                guard let task = self.task else {
                    self.scheduleReconnect()
                    return
                }

                // Application-level ping — resets backend's 5-min read deadline
                do {
                    try await task.send(.string("{\"type\":\"ping\"}"))
                } catch {
                    self.invalidateTask()
                    self.scheduleReconnect()
                    return
                }

                // Protocol-level ping — verifies transport is alive
                let ok = await self.sendPing(on: task)
                if !ok {
                    self.invalidateTask()
                    self.scheduleReconnect()
                    return
                }
            }
        }
    }

    private func stopHeartbeat() {
        heartbeatTask?.cancel()
        heartbeatTask = nil
    }

    private func sendPing(on task: URLSessionWebSocketTask) async -> Bool {
        await withCheckedContinuation { continuation in
            task.sendPing { error in
                continuation.resume(returning: error == nil)
            }
        }
    }

    private func invalidateTask() {
        task?.cancel(with: .abnormalClosure, reason: nil)
        task = nil
    }

    private func scheduleReconnect() {
        guard !isManuallyDisconnected else { return }
        guard socketURL != nil else { return }

        reconnectTask?.cancel()
        reconnectAttempts = min(reconnectAttempts + 1, 8)
        let delaySeconds = min(Int(pow(2.0, Double(reconnectAttempts))), 30)

        reconnectTask = Task { [weak self] in
            try? await Task.sleep(nanoseconds: UInt64(delaySeconds) * 1_000_000_000)
            guard let self else { return }
            guard !self.isManuallyDisconnected else { return }
            self.openConnection()
        }
    }
}
