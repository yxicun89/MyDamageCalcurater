// AttackerStatInput: 計算画面の攻撃側「攻撃」「特攻」の2ブロック入力(SP の数値・性格補正)と、
// それを要求の SP・性格に直す純粋関数(ADR-0518。Web の ADR-0329 と同じ規則。F-01 / I-ios-1)。
//
// 規則の正は ADR-0518 と `docs/adr/0329-web-attacker-sp-nature-input.md`(Web 側の同じ関数は
// `web/src/domain/attackerStatInputs.ts`。挙動を一致させる)。

/// 入力ブロックのステータス。`CaseIterable` の順(攻撃 → 特攻)がそのまま画面の並び順。
public enum AttackStat: String, CaseIterable, Sendable, Hashable {
    /// 攻撃(物理・変化の技が使う側)。
    case atk
    /// 特攻(特殊の技が使う側)。
    case spa

    /// 対応する `StatKey`。
    public var statKey: StatKey {
        switch self {
        case .atk: return .atk
        case .spa: return .spa
        }
    }

    /// もう一方のブロック。
    public var other: AttackStat {
        switch self {
        case .atk: return .spa
        case .spa: return .atk
        }
    }
}

/// 性格補正の選択(上昇・補正なし・下降)。`CaseIterable` の順がそのまま画面の並び順。
/// 補正は「そのブロックのステータス(攻撃なら A・特攻なら C)」に掛かる。
public enum NatureChoice: String, CaseIterable, Sendable, Hashable {
    case up
    case neutral
    case down
}

/// 1ブロックの入力。SP は入力途中を保てるよう文字列で持つ(「空欄」「03」なども状態として残せる)。
public struct AttackStatInput: Equatable, Sendable {
    public var spText: String
    public var modifier: NatureChoice

    public init(spText: String, modifier: NatureChoice) {
        self.spText = spText
        self.modifier = modifier
    }
}

/// 画面が持つ攻撃側の入力(攻撃と特攻の両方)。
public struct AttackerStatInputs: Equatable, Sendable {
    public var atk: AttackStatInput
    public var spa: AttackStatInput

    public init(atk: AttackStatInput, spa: AttackStatInput) {
        self.atk = atk
        self.spa = spa
    }

    /// 既定は両ブロックとも SP 0・補正なし(従来の既定 = 無振りと同じ)。
    public static let `default` = AttackerStatInputs(
        atk: AttackStatInput(spText: "0", modifier: .neutral),
        spa: AttackStatInput(spText: "0", modifier: .neutral)
    )

    public subscript(stat: AttackStat) -> AttackStatInput {
        get {
            switch stat {
            case .atk: return atk
            case .spa: return spa
            }
        }
        set {
            switch stat {
            case .atk: atk = newValue
            case .spa: spa = newValue
            }
        }
    }
}

/// 入力が要求にできない理由。
public enum AttackerStatIssue: Equatable, Sendable, Hashable {
    /// そのブロックの SP が 0〜32 の整数でない。
    case sp(AttackStat)
    /// 補正の組み合わせに当たる性格がマスタに無い(同じ向きの組み合わせを含む)。
    case nature
}

/// `AttackerStatRules.resolve` の結果。
public enum AttackerStatResolution: Equatable, Sendable {
    /// 要求にできる(攻撃と特攻の SP を両方載せた SP と、解決した性格の ID)。
    case resolved(AttackerBuild)
    /// できない(`issues` は SP の不正を攻撃 → 特攻の順、性格の解決失敗を最後に並べる。空にはならない)。
    case invalid([AttackerStatIssue])
}

/// 攻撃側の2ブロック入力の純粋関数(状態を持たない)。
public enum AttackerStatRules {
    /// 技の分類が使うブロック(物理・変化 = 攻撃、特殊 = 特攻)。技が無ければ nil(強調しない)。
    public static func attackStat(for category: MoveCategory?) -> AttackStat? {
        guard let category else { return nil }
        return category == .special ? .spa : .atk
    }

    /// プリセットをブロックの値に直す: 無振り = "0"・補正なし、特化 = "32"・上昇、振り(無補正) = "32"・補正なし。
    /// 数値は `SPLimits.maxPerStat`、対応は `AttackerPreset.build` と同じ(`AttackerPresetCatalogContractTests` が正)。
    public static func presetInput(_ preset: AttackerPreset) -> AttackStatInput {
        let full = String(SPLimits.maxPerStat)
        switch preset {
        case .none: return AttackStatInput(spText: "0", modifier: .neutral)
        case .aFull: return AttackStatInput(spText: full, modifier: .up)
        case .aMax: return AttackStatInput(spText: full, modifier: .neutral)
        }
    }

    /// ブロックの値と一致するプリセット。SP が不正、またはどれとも一致しなければ nil(カスタム)。
    /// 比較は数値(`parseSP` の値)で行う("032" も 32 と一致する)。
    public static func matchingPreset(_ input: AttackStatInput) -> AttackerPreset? {
        guard let value = parseSP(input.spText) else { return nil }
        return AttackerPreset.allCases.first { preset in
            let candidate = presetInput(preset)
            return parseSP(candidate.spText) == value && candidate.modifier == input.modifier
        }
    }

    /// SP の1欄の文字列を読む。10進の整数 0〜`SPLimits.maxPerStat` のときだけ値を返し、それ以外は nil(不正)。
    /// 前後の空白は無視、先頭の 0 は可、空欄(空白だけを含む)は 0。
    /// 符号・小数点・指数・全角数字・33 以上・桁区切りは不正(丸めない)。
    public static func parseSP(_ text: String) -> Int? {
        var scalars = Substring(text).unicodeScalars[...]
        while let first = scalars.first, first.properties.isWhitespace { scalars.removeFirst() }
        while let last = scalars.last, last.properties.isWhitespace { scalars.removeLast() }
        if scalars.isEmpty { return 0 }
        // ASCII の 0〜9 だけ(全角数字・符号・小数点・指数は不正)。
        guard scalars.allSatisfy({ $0.value >= 0x30 && $0.value <= 0x39 }) else { return nil }
        // 桁あふれは Int(_:) が nil を返す(不正)。
        guard let value = Int(String(String.UnicodeScalarView(scalars))), value <= SPLimits.maxPerStat else { return nil }
        return value
    }

    /// `stat` のブロックを `modifier` にできるか。上昇・下降は1つのステータスにしか付かないので、
    /// もう一方のブロックが同じ向きなら選べない(補正なしはいつでも選べる)。
    public static func isModifierSelectable(
        inputs: AttackerStatInputs, stat: AttackStat, modifier: NatureChoice
    ) -> Bool {
        modifier == .neutral || inputs[stat.other].modifier != modifier
    }

    /// `stat` のブロックにプリセットを入れられるか。プリセットの補正が `isModifierSelectable` で選べないとき
    /// (もう一方が上昇のときの「特化」)は false。
    public static func isPresetSelectable(
        inputs: AttackerStatInputs, stat: AttackStat, preset: AttackerPreset
    ) -> Bool {
        isModifierSelectable(inputs: inputs, stat: stat, modifier: presetInput(preset).modifier)
    }

    /// マスタの性格が `stat` に `choice` の補正を掛けているか(補正なし = 上昇でも下降でもない)。
    private static func applies(_ nature: Nature, stat: AttackStat, choice: NatureChoice) -> Bool {
        switch choice {
        case .up: return nature.plus == stat.statKey
        case .down: return nature.minus == stat.statKey
        case .neutral: return nature.plus != stat.statKey && nature.minus != stat.statKey
        }
    }

    /// 攻撃と特攻の補正から、マスタの性格を解決する(ADR-0518 §3 = Web ADR-0329 §4)。解決できなければ nil。
    /// 1. 両方補正なし → 無補正(`plus == nil` の性格のうち**一覧の最初**。従来の `AttackerPreset.build` と同じ)
    /// 2. 同じ向き(上昇と上昇・下降と下降)は nil
    /// 3. 技が使う側が上昇でもう一方の攻撃系が補正なしなら、マイナスをもう一方の攻撃系に置く性格
    ///    (物理 = +A/−C、特殊 = +C/−A)があればそれ。無ければ次へ
    /// 4. 攻撃・特攻の両方の補正に合う性格(上昇の側は plus、下降の側は minus、補正なしの側は plus でも minus でもない)の
    ///    うち **ID の昇順で最初**
    /// 5. 無ければ、技の分類が使う側だけを合わせる(使う側が補正なしなら無補正)。それも無ければ nil
    public static func resolveNature(
        natures: [Nature], atk: NatureChoice, spa: NatureChoice, category: MoveCategory
    ) -> Nature? {
        let neutral = natures.first { $0.plus == nil }
        if atk == .neutral && spa == .neutral { return neutral }
        if atk == spa { return nil }
        // ID の昇順は Unicode スカラーの昇順(Web の文字列比較と同じ)。
        let sorted = natures.sorted { $0.id.unicodeScalars.lexicographicallyPrecedes($1.id.unicodeScalars) }
        let choices: [AttackStat: NatureChoice] = [.atk: atk, .spa: spa]
        let used = attackStat(for: category) ?? .atk
        if choices[used] == .up && choices[used.other] == .neutral,
            let representative = sorted.first(where: { $0.plus == used.statKey && $0.minus == used.other.statKey }) {
            return representative
        }
        if let both = sorted.first(where: {
            applies($0, stat: .atk, choice: atk) && applies($0, stat: .spa, choice: spa)
        }) {
            return both
        }
        guard let usedChoice = choices[used] else { return nil }
        if usedChoice == .neutral { return neutral }
        return sorted.first { applies($0, stat: used, choice: usedChoice) }
    }

    /// 画面の入力を要求の SP・性格にする。H・B・D・S の SP は 0。攻撃と特攻の SP は**両方**載せる
    /// (技が使わない側もそのまま)。合計は最大 64 で `SPLimits.maxTotal` を超えない。
    /// SP の不正と性格の解決失敗は `issues` に**すべて**入れる(1つ直しても別のが残る)。
    public static func resolve(
        inputs: AttackerStatInputs, natures: [Nature], category: MoveCategory
    ) -> AttackerStatResolution {
        var issues: [AttackerStatIssue] = []
        var sp = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
        if let value = parseSP(inputs.atk.spText) { sp.atk = value } else { issues.append(.sp(.atk)) }
        if let value = parseSP(inputs.spa.spText) { sp.spa = value } else { issues.append(.sp(.spa)) }
        let nature = resolveNature(natures: natures, atk: inputs.atk.modifier, spa: inputs.spa.modifier, category: category)
        if nature == nil { issues.append(.nature) }
        guard issues.isEmpty, let nature else { return .invalid(issues) }
        return .resolved(AttackerBuild(natureId: nature.id, sp: sp))
    }
}

/// 計算画面の技の絞り込み(ADR-0518 §2 = Web ADR-0328 §1)。判定は `Move.category` だけ(技名・ID を直書きしない)。
public enum CalcMoveRules {
    /// 変化技か。
    public static func isStatusMove(_ move: Move) -> Bool {
        move.category == .status
    }

    /// ダメージを与える技(物理・特殊)だけ、順を保って返す。
    public static func damagingMoves(_ moves: [Move]) -> [Move] {
        moves.filter { !isStatusMove($0) }
    }
}
