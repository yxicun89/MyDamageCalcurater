// BalanceService: タイプバランス画面が依存する境界(ADR-0415 §6)。
// `PokeCalcService` とは別のプロトコル(全スタブ・モックへの影響を避ける)。エラーは `PokeCalcError` に統一する
// (キャンセルだけは `CancellationError` のまま)。

public protocol BalanceService: Sendable {
    /// チームの防御相性とチーム集計(`pokemonId`・`abilityId` だけを送る)。
    func analyze(members: [BalanceMemberInput]) async throws -> BalanceDefenseAnalysis
    /// チームの攻撃範囲(`pokemonId`・`moveIds` だけを送る)。
    func coverage(members: [BalanceMemberInput]) async throws -> BalanceCoverageAnalysis
}

/// 接続先が無い構成(モック。`POKECALC_USE_MOCK` / 接続先なし)の実装。架空の相性表を返さず、常に
/// `balance_unavailable` で失敗する(ADR-0415 §4。Web の ADR-0411 と同じ)。
public struct UnavailableBalanceService: BalanceService {
    /// 画面のエラー文言の表(`BalanceErrorText`)が引くコード。
    public static let unavailableCode = "balance_unavailable"

    public init() {}

    public func analyze(members: [BalanceMemberInput]) async throws -> BalanceDefenseAnalysis {
        throw Self.error
    }

    public func coverage(members: [BalanceMemberInput]) async throws -> BalanceCoverageAnalysis {
        throw Self.error
    }

    private static var error: PokeCalcError {
        PokeCalcError(code: unavailableCode, message: "balance service is not configured")
    }
}
