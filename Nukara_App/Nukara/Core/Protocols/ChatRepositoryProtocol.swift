import Foundation

protocol ChatRepositoryProtocol {
    var events: AsyncStream<ChatSocketEvent> { get }

    func connect() async throws
    func disconnect() async
    func sendText(conversationID: String, text: String, clientMessageID: String) async throws
    func sendImage(conversationID: String, imageData: Data, clientMessageID: String) async throws
    func sendLocation(conversationID: String, location: ChatLocation, clientMessageID: String) async throws
}
