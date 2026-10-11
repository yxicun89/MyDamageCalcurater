// 構築編集の中の「タイプバランス」の表示の規則(ADR-0422 §2・§3・ADR-0532)。View に依存しない純粋な部分だけ。

/// おすすめタイプの候補ごとの、該当ポケモンのタイル(画像 + 名前)。応答の `pokemon` の順のまま出す。
public enum BalancePickTiles {
    /// 1 候補で最初に見せるタイルの数(残りは「もっと見る」)。
    public static let visibleCount = 8

    /// 閉じていれば先頭 `visibleCount` 件、開いていれば全件。
    public static func visible(_ pokemon: [BalanceCandidatePokemon], expanded: Bool) -> [BalanceCandidatePokemon] {
        expanded ? pokemon : Array(pokemon.prefix(visibleCount))
    }

    /// 閉じているときに隠れている件数(開いていれば 0)。
    public static func hiddenCount(_ pokemon: [BalanceCandidatePokemon], expanded: Bool) -> Int {
        expanded ? 0 : max(0, pokemon.count - visibleCount)
    }
}

/// 相性の色の意味(Web と同じ。弱点・抜群=赤、耐性・いまひとつ=水色系、無効=灰、等倍=色なし)。
/// 語は応答の category・倍率から選び、値の範囲から判定し直さない(ADR-0303 §7)。緑は相性に使わない。
public enum BalanceMatchupTone: Equatable, Sendable {
    case weak
    case resist
    case immune
    case neutral

    /// 防御相性(analyze の category)。
    public init(defense category: BalanceDefenseCategory) {
        switch category {
        case .quadWeak, .weak: self = .weak
        case .resist, .quadResist: self = .resist
        case .immune: self = .immune
        case .neutral: self = .neutral
        }
    }

    /// 攻撃範囲(coverage の最大倍率。攻撃技なしは nil)。
    public init(coverage multiplier: BalanceCoverageMultiplier?) {
        switch multiplier {
        case .double: self = .weak
        case .half: self = .resist
        case .zero: self = .immune
        case .neutral, nil: self = .neutral
        }
    }
}
