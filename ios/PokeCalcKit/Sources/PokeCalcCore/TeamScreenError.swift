// TeamScreenError: PokeCalcError を構築画面(一覧・編集)が出し分けられる種類へ写す(P6-2c)。
//
// 失敗の原因の分類と文言は `ServiceFailureKind`(ADR-0534)に従う。番号・コード・サーバーの英語は画面に出さない。
// `CalcScreenError`(P6-2a)と同じ形。別の型にする理由: 将来 `APITeamStore`(ADR-0500 §4)に切り替わったときの
// 通信エラーが計算画面のエラー文言と混ざらないようにするため(ADR-0501「P6-2c」3章)。

/// 構築画面(一覧・編集)が表示するエラーの種類。
public enum TeamScreenError: Equatable, Sendable {
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

    private static let feature = "構築"
}

/// チーム全体のフィールドエラー(名前・メンバー数。ADR-0501「P6-2c」3章)。
public enum TeamFieldError: Equatable, Sendable {
    /// チーム名が空(前後空白のみを含む)。
    case emptyName
    /// メンバーを `TeamLimits.maxMembers` を超えて追加しようとした。
    case tooManyMembers
}

/// メンバー単位のフィールドエラー(技・SP。ADR-0501「P6-2c」3章)。
public enum TeamMemberFieldError: Equatable, Sendable {
    /// 既に持っている技 ID を重複して追加しようとした。
    case duplicateMove
    /// 技を `TeamLimits.maxMovesPerMember` を超えて追加しようとした。
    case tooManyMoves
    /// SP が `SPLimits.maxPerStat`(1ステータス)を超える、または範囲外(負の値を含む)。
    case spPerStatExceeded
    /// SP の合計が `SPLimits.maxTotal` を超える。
    case spTotalExceeded
}
