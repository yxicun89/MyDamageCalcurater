import Foundation
import HTTPTypes
import OpenAPIRuntime
import PokeCalcAPI
import PokeCalcBalanceAPI
import PokeCalcSpeedAPI
import XCTest

@testable import PokeCalcCore

/// 通信層(生成クライアント+偽の transport)が、失敗を `ServiceFailureKind` に正しく分類できるだけの情報
/// (`PokeCalcError.code` と `statusCode`)を持たせること(I-ios-19)。監査 3.3 の 1・4・11・13 件目に当たる。
/// フィクスチャは架空。
final class ServiceFailureTransportTests: XCTestCase {

    private let identity = ClientIdentity(
        deviceID: "0B7A2D2E-5C1F-4E43-9D0A-3F7E1B6C2A11", sessionID: "6F1C8E94-2B3D-4A57-8E60-7D9A0C1B2E33")
    private let baseURL = URL(string: "https://pokecalc.example.invalid")!

    /// 送信のたびに指定の `URLError` を投げる transport。
    private struct ThrowingTransport: ClientTransport {
        let error: URLError
        func send(_ request: HTTPRequest, body: HTTPBody?, baseURL: URL, operationID: String) async throws
            -> (HTTPResponse, HTTPBody?)
        {
            throw error
        }
    }

    /// 操作ごとの呼び出し(同じ分類を 3 つの通信層に当てる)。
    private typealias Call = (any ClientTransport) async throws -> Void

    private var calls: [(name: String, call: Call)] {
        let identity = self.identity
        let baseURL = self.baseURL
        return [
            ("計算・マスタ(APIPokeCalcService)", { transport in
                let service = APIPokeCalcService(client: PokeCalcAPI.Client(serverURL: baseURL, transport: transport), identity: identity)
                _ = try await service.natures()
            }),
            ("タイプバランス(APIBalanceService)", { transport in
                let service = APIBalanceService(client: PokeCalcBalanceAPI.Client(serverURL: baseURL, transport: transport), identity: identity)
                _ = try await service.moveRange(moveIds: ["9001"])
            }),
            ("素早さ(APISpeedService)", { transport in
                let service = APISpeedService(client: PokeCalcSpeedAPI.Client(serverURL: baseURL, transport: transport), identity: identity)
                _ = try await service.pokemon()
            }),
        ]
    }

    private func failure(_ call: Call, _ transport: any ClientTransport) async -> PokeCalcError? {
        do {
            try await call(transport)
            XCTFail("失敗するはず")
            return nil
        } catch let error as PokeCalcError {
            return error
        } catch {
            XCTFail("PokeCalcError 以外: \(error)")
            return nil
        }
    }

    func testOfflineURLErrorsAreOffline() async {
        let codes: [URLError.Code] = [.notConnectedToInternet, .cannotFindHost, .cannotConnectToHost, .networkConnectionLost]
        for (name, call) in calls {
            for code in codes {
                let error = await failure(call, ThrowingTransport(error: URLError(code)))
                XCTAssertEqual(error.map { ServiceFailureKind($0) }, .offline, "\(name) / \(code.rawValue)")
            }
        }
    }

    func testTimedOutIsTimeoutNotOffline() async {
        for (name, call) in calls {
            let error = await failure(call, ThrowingTransport(error: URLError(.timedOut)))
            XCTAssertEqual(error?.code, PokeCalcError.Code.timeout, name)
            XCTAssertEqual(error.map { ServiceFailureKind($0) }, .timeout, name)
        }
    }

    /// ingress などが返す HTML 本文の 5xx。契約に無い応答でも「サーバーの障害」になり、番号を保持する。
    func testHTMLGatewayErrorsAreServerFaultWithStatus() async {
        for status in [500, 502, 503, 504] {
            for (name, call) in calls {
                let error = await failure(call, RecordingTransport(status: status, json: "<html>Bad Gateway</html>"))
                XCTAssertEqual(error.map { ServiceFailureKind($0) }, .serverFault, "\(name) / \(status)")
                XCTAssertEqual(error?.statusCode, status, "\(name) / \(status)")
            }
        }
    }

    /// 経路が無い(ゲートウェイが知らないパス)404 は「まだ使えない機能」。入力の誤りの `not_found` とは別。
    func testHTMLNotFoundIsUnsupported() async {
        for (name, call) in calls {
            let error = await failure(call, RecordingTransport(status: 404, json: "<html>404 page not found</html>"))
            XCTAssertEqual(error.map { ServiceFailureKind($0) }, .unsupported, name)
            XCTAssertEqual(error?.statusCode, 404, name)
        }
    }

    /// JSON のエラー本文があるときはサーバーの code を優先し、番号も運ぶ。
    func testJSONErrorBodyKeepsServerCodeAndStatus() async {
        let body = #"{"code":"internal_error","message":"English server message"}"#
        for (name, call) in calls {
            let error = await failure(call, RecordingTransport(status: 500, json: body))
            XCTAssertEqual(error?.code, "internal_error", name)
            XCTAssertEqual(error?.statusCode, 500, name)
            XCTAssertEqual(error.map { ServiceFailureKind($0) }, .serverFault, name)
        }
    }

    /// 200 なのに本文の形が違う(アプリとサーバーの版のずれ)は「応答を読み取れない」。接続できないではない。
    func testBrokenOKBodyIsUnreadable() async {
        for (name, call) in calls {
            let error = await failure(call, RecordingTransport(status: 200, json: #"{"unexpected":true}"#))
            XCTAssertEqual(error.map { ServiceFailureKind($0) }, .unreadable, name)
        }
    }

    /// タイプバランスの契約外ステータスは、もう transport(接続できない)にしない(監査 3.3 の 1 件目)。
    func testBalanceUndocumentedStatusIsNotTransport() async {
        let call = calls[1].call
        let error = await failure(call, RecordingTransport(status: 502, json: nil))
        XCTAssertNotEqual(error?.code, PokeCalcError.Code.transport)
        XCTAssertEqual(error?.statusCode, 502)
    }
}
