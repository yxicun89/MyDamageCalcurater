// AdjustSpeedPreset: 目標「素早さを上回る」の相手の振り方(F-11。ADR-0331 §5 の表。Web の `SPEED_PRESETS` と同じ)。
//
// 入力の作り方の型でマスタではない。性格 ID は一覧(マスタ)から規則で選ぶだけで、直書きしない。

/// 相手の素早さの振り方。`CaseIterable` の順(速い順)が画面の並び。
public enum AdjustSpeedPreset: String, CaseIterable, Sendable, Hashable {
    /// 最速: S 32 + 素早さ上昇性格。
    case fastest
    /// 準速: S 32 + 無補正性格。
    case neutralMax = "neutral_max"
    /// 無振り: SP 0 + 無補正性格。
    case none

    /// 既定(最速)。
    public static let defaultPreset: AdjustSpeedPreset = .fastest

    public var label: String {
        switch self {
        case .fastest: return "最速"
        case .neutralMax: return "準速"
        case .none: return "無振り"
        }
    }

    /// 性格 ID と SP。該当する性格が `natures`(一覧の順で探す)に無ければ `natureUnavailable`(別の性格で代えない)。
    /// 最速は「素早さ上昇・攻撃下降」を優先し、無ければ素早さ上昇の最初の性格(下降の置き先は素早さの比較に影響しない)。
    public func build(natures: [Nature]) throws -> (natureId: String, sp: StatBlock) {
        let speed = self == .none ? 0 : SPLimits.maxPerStat
        let sp = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: speed)
        let nature: Nature?
        switch self {
        case .fastest:
            nature = natures.first { $0.plus == .spe && $0.minus == .atk } ?? natures.first { $0.plus == .spe }
        case .neutralMax, .none:
            nature = natures.first { $0.plus == nil }
        }
        guard let nature else {
            throw PokeCalcError(code: PokeCalcError.Code.natureUnavailable, message: "素早さの振り方に合う性格が見つからない")
        }
        return (nature.id, sp)
    }
}
