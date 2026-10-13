// ComboSelfSpeed: 連続技の「自分の素早さ」の入力(I-web-32 相当。ADR-0535・ADR-0501「連続技」AC-i6)。
//
// 連続技の API は素早さを攻撃側の個体(SP・性格・ランク)から導くので、この入力は個体の素早さに関わる項目だけを
// 上書きする。性格は素早さの補正だけを差し替え、Web の `applySpeedNature` と同じ規則で性格 ID を引き直す。

/// 自分の素早さの性格補正の指定。`keep` = 計算画面の性格のまま。
public enum ComboSelfSpeedNature: String, CaseIterable, Equatable, Sendable {
    case keep, plus, neutral, minus
}

/// 自分の素早さの入力。`sp`・`rank` が nil のものは攻撃側の個体の値のまま。
public struct ComboSelfSpeedInput: Equatable, Sendable {
    public var sp: Int?
    public var nature: ComboSelfSpeedNature
    public var rank: Int?

    public init(sp: Int? = nil, nature: ComboSelfSpeedNature = .keep, rank: Int? = nil) {
        self.sp = sp
        self.nature = nature
        self.rank = rank
    }

    /// 既定。`ComboSelfSpeed.apply` が個体をそのまま返す値。
    public static let `default` = ComboSelfSpeedInput(sp: nil, nature: .keep, rank: nil)
}

public enum ComboSelfSpeed {
    /// 素早さの性格を差し替えるときの代わりの下降・上昇(攻撃側の得意な能力を削らない)。
    private static let fallbackPlusForMinus: StatKey = .def

    /// 素早さに振れる SP の上限(1 能力 32・合計 66 から、素早さ以外の SP を引いた残り)。
    public static func maxSP(for attacker: Individual) -> Int {
        let sp = attacker.sp
        let used = sp.hp + sp.atk + sp.def + sp.spa + sp.spd
        return max(0, min(SPLimits.maxPerStat, SPLimits.maxTotal - used))
    }

    /// 素早さの SP を 0〜上限に丸める(入力欄の表示と要求の値を揃える)。
    public static func clampSP(_ sp: Int, for attacker: Individual) -> Int {
        min(max(0, sp), maxSP(for: attacker))
    }

    /// 攻撃側の個体に、自分の素早さの入力を重ねる。既定のままなら同じ個体を返す。
    /// 性格を差し替える組がデータに無いときは `natureUnavailable` を投げる(送れない要求を作らない)。
    public static func apply(_ input: ComboSelfSpeedInput, to attacker: Individual, natures: [Nature]) throws -> Individual {
        guard input != .default else { return attacker }
        var result = attacker
        if let sp = input.sp { result.sp.spe = clampSP(sp, for: attacker) }
        if let rank = input.rank { result.ranks.spe = min(RankLimits.max, max(RankLimits.min, rank)) }
        if input.nature != .keep { result.natureId = try replacedNatureId(for: attacker, choice: input.nature, natures: natures) }
        return result
    }

    private static func replacedNatureId(for attacker: Individual, choice: ComboSelfSpeedNature, natures: [Nature]) throws -> String {
        guard let current = natures.first(where: { $0.id == attacker.natureId }) else { throw unavailable() }
        let target = replacedPair(plus: current.plus, minus: current.minus, choice: choice)
        if target.plus == current.plus && target.minus == current.minus { return current.id }
        // (上昇, 下降) の組が一致する性格のうち ID 昇順の最初(ADR-0200 §2)。
        guard let found = natures.filter({ $0.plus == target.plus && $0.minus == target.minus }).map(\.id).min() else {
            throw unavailable()
        }
        return found
    }

    /// Web の `applySpeedNature` と同じ規則。
    private static func replacedPair(plus: StatKey?, minus: StatKey?, choice: ComboSelfSpeedNature) -> (plus: StatKey?, minus: StatKey?) {
        switch choice {
        case .keep:
            return (plus, minus)
        case .plus:
            let newMinus = (minus == nil || minus == .spe) ? (plus == .spa ? StatKey.atk : StatKey.spa) : minus
            return (.spe, newMinus)
        case .minus:
            let newPlus = (plus == nil || plus == .spe) ? fallbackPlusForMinus : plus
            return (newPlus, .spe)
        case .neutral:
            return (plus == .spe || minus == .spe) ? (nil, nil) : (plus, minus)
        }
    }

    private static func unavailable() -> PokeCalcError {
        PokeCalcError(code: PokeCalcError.Code.natureUnavailable, message: ComboText.selfSpeedNatureUnavailable)
    }
}
