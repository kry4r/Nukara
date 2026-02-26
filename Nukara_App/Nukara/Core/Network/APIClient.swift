import Foundation

protocol APIClientProtocol {
    func request<T: Decodable>(_ endpoint: APIEndpoint, as type: T.Type) async throws -> T
    func requestVoid(_ endpoint: APIEndpoint) async throws
}

final class APIClient: APIClientProtocol {
    private let requestBuilder: RequestBuilder
    private let session: URLSession
    private let accessTokenProvider: () -> String?

    init(baseURL: URL, session: URLSession = .shared, accessTokenProvider: @escaping () -> String?) {
        self.requestBuilder = RequestBuilder(baseURL: baseURL)
        self.session = session
        self.accessTokenProvider = accessTokenProvider
    }

    func request<T: Decodable>(_ endpoint: APIEndpoint, as type: T.Type) async throws -> T {
        let request = try requestBuilder.buildRequest(for: endpoint, accessToken: accessTokenProvider())
        let (data, response) = try await performRequest(request)

        do {
            return try JSONDecoder.nukara.decode(type, from: data)
        } catch {
            throw NetworkError.decoding(error.localizedDescription)
        }
    }

    func requestVoid(_ endpoint: APIEndpoint) async throws {
        let request = try requestBuilder.buildRequest(for: endpoint, accessToken: accessTokenProvider())
        _ = try await performRequest(request)
    }

    private func performRequest(_ request: URLRequest) async throws -> (Data, HTTPURLResponse) {
        do {
            let (data, response) = try await session.data(for: request)
            guard let httpResponse = response as? HTTPURLResponse else {
                throw NetworkError.invalidResponse
            }
            switch httpResponse.statusCode {
            case 200 ..< 300:
                return (data, httpResponse)
            case 401:
                throw NetworkError.unauthorized
            default:
                throw NetworkError.httpStatus(httpResponse.statusCode)
            }
        } catch let networkError as NetworkError {
            throw networkError
        } catch {
            throw NetworkError.transport(error.localizedDescription)
        }
    }
}
