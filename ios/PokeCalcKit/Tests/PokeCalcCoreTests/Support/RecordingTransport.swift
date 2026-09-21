import Foundation
import HTTPTypes
import OpenAPIRuntime

/// テスト用の偽 `ClientTransport`。送られた HTTP リクエストと body を記録し、決めたレスポンスを返す。
/// ネットワークには出ない。生成された `Client` に渡して `APIPokeCalcService` の写像を検証する。
final class RecordingTransport: ClientTransport, @unchecked Sendable {
    /// 記録した1回分の送信。
    struct Recorded: Sendable {
        let request: HTTPRequest
        let body: Data?
        let operationID: String

        /// パス(クエリを除く)。例 `/api/pokedex/species`
        var pathWithoutQuery: String {
            URLComponents(string: request.path ?? "")?.path ?? ""
        }

        /// クエリを名前 → 値の辞書にしたもの(同名が複数あれば最後の値)。
        var queryItems: [String: String] {
            let items = URLComponents(string: request.path ?? "")?.queryItems ?? []
            var result: [String: String] = [:]
            for item in items {
                result[item.name] = item.value ?? ""
            }
            return result
        }

        func header(_ name: String) -> String? {
            guard let fieldName = HTTPField.Name(name) else { return nil }
            return request.headerFields[fieldName]
        }

        /// リクエスト body を JSON オブジェクトとして読む。
        func jsonBody() throws -> [String: Any] {
            guard let body else { throw RecordingTransportError.missingBody }
            guard let object = try JSONSerialization.jsonObject(with: body) as? [String: Any] else {
                throw RecordingTransportError.bodyIsNotJSONObject
            }
            return object
        }
    }

    /// 返すレスポンス。`json` が nil なら body 無し。
    struct Stub: Sendable {
        var status: Int
        var json: String?
    }

    private let lock = NSLock()
    private var recorded: [Recorded] = []
    private let responder: @Sendable (HTTPRequest) throws -> Stub

    init(status: Int = 200, json: String?) {
        let stub = Stub(status: status, json: json)
        self.responder = { _ in stub }
    }

    init(responder: @escaping @Sendable (HTTPRequest) throws -> Stub) {
        self.responder = responder
    }

    var requests: [Recorded] {
        lock.withLock { recorded }
    }

    func send(
        _ request: HTTPRequest,
        body: HTTPBody?,
        baseURL: URL,
        operationID: String
    ) async throws -> (HTTPResponse, HTTPBody?) {
        var bodyData: Data?
        if let body {
            bodyData = try await Data(collecting: body, upTo: 1 << 20)
        }
        let entry = Recorded(request: request, body: bodyData, operationID: operationID)
        lock.withLock { recorded.append(entry) }

        let stub = try responder(request)
        var fields = HTTPFields()
        var responseBody: HTTPBody?
        if let json = stub.json {
            fields[.contentType] = "application/json"
            responseBody = HTTPBody(json)
        }
        return (HTTPResponse(status: .init(code: stub.status), headerFields: fields), responseBody)
    }
}

/// 送信のたびに `URLError` を投げる transport(接続できない状況の再現)。
struct FailingTransport: ClientTransport {
    func send(
        _ request: HTTPRequest,
        body: HTTPBody?,
        baseURL: URL,
        operationID: String
    ) async throws -> (HTTPResponse, HTTPBody?) {
        throw URLError(.notConnectedToInternet)
    }
}

enum RecordingTransportError: Error {
    case missingBody
    case bodyIsNotJSONObject
}
