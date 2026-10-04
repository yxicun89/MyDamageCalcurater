import Foundation
import HTTPTypes
import OpenAPIRuntime
import Synchronization

/// 送られたリクエストを記録し、決めた応答を返す `ClientTransport`(実ネットワークは使わない)。
final class RecordingTransport: ClientTransport {
    struct Recorded: Sendable {
        let request: HTTPRequest
        let body: Data
        let baseURL: URL
        let operationID: String

        var json: NSDictionary { jsonObject(body) }
        var bodyText: String { String(decoding: body, as: UTF8.self) }
    }

    struct Reply: Sendable {
        var status: Int
        var body: String?
        var contentType: String? = "application/json"

        static func json(_ body: String) -> Reply { Reply(status: 200, body: body) }
        static func json(_ status: Int, _ body: String) -> Reply { Reply(status: status, body: body) }
        static let noContent = Reply(status: 204, body: nil, contentType: nil)
        static func text(_ status: Int, _ body: String) -> Reply { Reply(status: status, body: body, contentType: "text/html") }
    }

    private let recorded = Mutex<[Recorded]>([])
    private let handler: @Sendable (Recorded) throws -> Reply

    init(_ handler: @escaping @Sendable (Recorded) throws -> Reply) {
        self.handler = handler
    }

    convenience init(reply: Reply) {
        self.init { _ in reply }
    }

    var requests: [Recorded] { recorded.withLock { $0 } }
    var last: Recorded? { requests.last }

    func send(_ request: HTTPRequest, body: HTTPBody?, baseURL: URL, operationID: String) async throws -> (HTTPResponse, HTTPBody?) {
        var data = Data()
        if let body { data = try await Data(collecting: body, upTo: 20_000_000) }
        let entry = Recorded(request: request, body: data, baseURL: baseURL, operationID: operationID)
        recorded.withLock { $0.append(entry) }
        let reply = try handler(entry)
        var response = HTTPResponse(status: .init(code: reply.status))
        if let contentType = reply.contentType { response.headerFields[.contentType] = contentType }
        return (response, reply.body.map { HTTPBody($0) })
    }
}

/// `URLSession` 経由(本物の URLSessionTransport)で URL の連結を確かめるための URLProtocol。
final class StubURLProtocol: URLProtocol {
    private static let state = Mutex<(handler: (@Sendable (URLRequest) -> (Int, Data))?, urls: [URL])>((nil, []))

    static func install(handler: @escaping @Sendable (URLRequest) -> (Int, Data)) {
        state.withLock { $0 = (handler, []) }
    }

    static var requestedURLs: [URL] { state.withLock { $0.urls } }

    static func makeSession() -> URLSession {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.protocolClasses = [StubURLProtocol.self]
        return URLSession(configuration: configuration)
    }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        let request = self.request
        let handler = Self.state.withLock { s -> (@Sendable (URLRequest) -> (Int, Data))? in
            if let url = request.url { s.urls.append(url) }
            return s.handler
        }
        let (status, data) = handler?(request) ?? (500, Data())
        let response = HTTPURLResponse(
            url: request.url ?? URL(fileURLWithPath: "/"), statusCode: status, httpVersion: "HTTP/1.1",
            headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: data)
        client?.urlProtocolDidFinishLoading(self)
    }

    override func stopLoading() {}
}
