// TeamScreenError: PokeCalcError を構築画面が出し分けられる種類へ写す(P6-2c)。
//
// `CalcScreenError`(P6-2a)と同じ3ケース・同じ init。別の型にする理由: 将来 `APITeamStore`
// (ADR-0500 §4「team-svc の契約ができたら API 実装を足す」)に切り替わったときの通信エラーが、
// 計算画面のエラー文言と混ざらないようにするため(ADR-0501「P6-2c」3章)。

/// 構築画面(一覧・編集)が表示するエラーの種類。
public enum TeamScreenError: Equatable, Sendable {
    /// 通信できない(`PokeCalcError.Code.transport`)。
    case transport
    /// 応答の形が期待と違う(デコード失敗、または `PokeCalcError` 以外の Error)。
    case unexpectedResponse
    /// `TeamStore` / マスタ参照が返した `code`(クライアント側の `teamNameEmpty` 等を含む)をそのまま運ぶ。
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

    /// 画面に出す文言(`CalcScreenError.message` と同じ組み立て方)。
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
