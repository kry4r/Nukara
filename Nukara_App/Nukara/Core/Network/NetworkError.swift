import Foundation

enum NetworkError: LocalizedError, Equatable {
    case invalidURL
    case invalidResponse
    case unauthorized
    case httpStatus(Int)
    case decoding(String)
    case transport(String)

    var errorDescription: String? {
        switch self {
        case .invalidURL:
            return "Invalid request URL"
        case .invalidResponse:
            return "Invalid server response"
        case .unauthorized:
            return "Unauthorized"
        case .httpStatus(let code):
            return "Server returned status code \(code)"
        case .decoding(let detail):
            return "Response decode failed: \(detail)"
        case .transport(let detail):
            return "Network transport error: \(detail)"
        }
    }
}
