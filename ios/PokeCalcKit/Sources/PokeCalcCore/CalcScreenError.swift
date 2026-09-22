// CalcScreenError: PokeCalcError を計算画面が出し分けられる種類へ写す(P6-2a)。
//
// `PokeCalcService` は `PokeCalcError` だけを投げる契約だが、画面はそれ以外の Error が来ても
// 止まらない(`unexpectedResponse` に落とす)。

/// 計算画面が表示するエラーの種類。
public enum CalcScreenError: Equatable, Sendable {
    /// 通信できない(`PokeCalcError.Code.transport`)。
    case transport
    /// 応答の形が期待と違う(デコード失敗、または `PokeCalcError` 以外の Error)。
    case unexpectedResponse
    /// サーバーが返した `code`(またはクライアント側の `natureUnavailable` 等)をそのまま運ぶ。
    case service(code: String, message: String)

    public init(_ error: any Error) {
        guard let pokeCalcError = error as? PokeCalcError else {
            self = .unexpectedResponse
            return
        }
        switch pokeCalcError.code {
        case PokeCalcError.Code.transport:
            self = .transport
        case PokeCalcError.Code.decode:
            self = .unexpectedResponse
        default:
            self = .service(code: pokeCalcError.code, message: pokeCalcError.message)
        }
    }

    /// 画面に出す文言。種類ごとに別の文言にする(`service` は code と説明を含める。原因が分かるように)。
    public var message: String {
        switch self {
        case .transport:
            return "通信に失敗しました。接続を確認してもう一度お試しください。"
        case .unexpectedResponse:
            return "応答の形が想定と違うため、結果を表示できません。"
        case .service(let code, let message):
            return "エラー(\(code)): \(message)"
        }
    }
}
