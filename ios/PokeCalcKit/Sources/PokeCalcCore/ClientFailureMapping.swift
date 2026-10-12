import Foundation
import OpenAPIRuntime
import OpenAPIURLSession

/// 生成クライアントが投げる失敗を `PokeCalcError` にする規則(`APIPokeCalcService`・`APIBalanceService`・
/// `APISpeedService` で共通。ADR-0534)。接続できない・時間切れ・読み取れない・契約外のステータスを区別し、
/// 応答があるときは HTTP の番号(`statusCode`)を運ぶ。
enum ClientFailureMapping {
    /// 1 回の通信の時間切れ(秒)。URLSession の既定(60 秒)より短くし、返事が無いときに早めに「時間切れ」を伝える。
    static let requestTimeoutSeconds: TimeInterval = 30

    /// 時間切れを `requestTimeoutSeconds` にした既定の transport(3 つの通信層で共通)。
    static func makeTransport() -> URLSessionTransport {
        let configuration = URLSessionConfiguration.default
        configuration.timeoutIntervalForRequest = requestTimeoutSeconds
        return URLSessionTransport(configuration: .init(session: URLSession(configuration: configuration)))
    }

    /// 本文を読む上限(エラー本文は小さい。ゲートウェイの HTML などを無制限に読まない)。
    private static let undocumentedBodyLimit = 64 * 1024

    /// キャンセルは失敗ではない(呼び出し側が `CancellationError` として扱う)。
    static func isCancellation(_ error: any Error) -> Bool {
        guard let clientError = error as? ClientError else { return false }
        if clientError.underlyingError is CancellationError { return true }
        // URLSession 経由のキャンセルは CancellationError ではなく URLError(.cancelled) として届く。
        if let urlError = clientError.underlyingError as? URLError, urlError.code == .cancelled { return true }
        return false
    }

    /// キャンセル以外の失敗。デコード失敗は `decode`(応答が 2xx 以外ならその番号を運ぶ。ゲートウェイの HTML や
    /// 契約の列挙に無い code など)、時間切れは `timeout`、他は `transport`。
    static func pokeCalcError(from error: any Error) -> PokeCalcError {
        let clientError = error as? ClientError
        if clientError?.underlyingError is DecodingError {
            let status = clientError?.response.map { Int($0.status.code) }
            return PokeCalcError(code: PokeCalcError.Code.decode, message: "\(error)", statusCode: status)
        }
        if let urlError = clientError?.underlyingError as? URLError, urlError.code == .timedOut {
            return PokeCalcError(code: PokeCalcError.Code.timeout, message: "\(error)")
        }
        return PokeCalcError(code: PokeCalcError.Code.transport, message: "\(error)")
    }

    /// 契約外のステータス。本文が `{code,message}` ならその code(gateway の 404/405。ADR-0802)、
    /// 読めなければ専用のコード。どちらも番号を運ぶ。
    static func undocumentedError(status: Int, payload: UndocumentedPayload) async -> PokeCalcError {
        struct Body: Decodable {
            let code: String
            let message: String
        }
        if let httpBody = payload.body,
            let data = try? await Data(collecting: httpBody, upTo: undocumentedBodyLimit),
            let body = try? JSONDecoder().decode(Body.self, from: data)
        {
            return PokeCalcError(code: body.code, message: body.message, statusCode: status)
        }
        return PokeCalcError(code: PokeCalcError.Code.unexpectedStatus, message: "HTTP \(status)", statusCode: status)
    }
}
