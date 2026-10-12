// CalcScreenError: PokeCalcError を計算画面が出し分けられる種類へ写す(P6-2a)。
//
// 失敗の原因の分類と文言は `ServiceFailureKind`(ADR-0534)に従う。番号・コード・サーバーの英語は画面に出さない。

/// 計算画面が表示するエラーの種類。
public enum CalcScreenError: Equatable, Sendable {
    /// 接続できない(`ServiceFailureKind.offline`)。
    case transport
    /// 応答を読み取れない(デコード失敗・番号の無い契約外の応答、または `PokeCalcError` 以外の Error)。
    case unexpectedResponse
    /// 画面別の文言が要る失敗(入力の誤り・クライアント側のコード)。`message` は日本語のときだけ画面に出す。
    case service(code: String, message: String)
    /// 時間切れ・サーバーの障害・まだ使えない機能・混み合い(`ServiceFailureKind` の文言をそのまま出す)。
    case failure(ServiceFailureKind)

    public init(_ error: any Error) {
        guard let pokeCalcError = error as? PokeCalcError else {
            self = .unexpectedResponse
            return
        }
        switch ServiceFailureKind(pokeCalcError) {
        case .offline:
            self = .transport
        case .unreadable:
            self = .unexpectedResponse
        case .other:
            self = .service(code: pokeCalcError.code, message: pokeCalcError.message)
        case let kind:
            self = .failure(kind)
        }
    }

    /// 画面に出す文言。
    public var message: String {
        switch self {
        case .transport:
            return ServiceFailureKind.offline.message(feature: Self.feature)
        case .unexpectedResponse:
            return ServiceFailureKind.unreadable.message(feature: Self.feature)
        case .service(let code, let message):
            return ServiceFailureKind.other(code: code).message(feature: Self.feature, detail: message)
        case .failure(let kind):
            return kind.message(feature: Self.feature)
        }
    }

    private static let feature = "計算"
}
