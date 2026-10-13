import Foundation

// ComboText: 連続技(CB-8。ADR-0535・ADR-0501「連続技」)の文言と、技がランクを変えるかの規則。
// 語は Web の `i18n/combo.ts`(ADR-0359)と同じ。iOS だけの文言は `retryButton`・`selfSpeedNatureUnavailable`。
// 数値は API の値をそのまま書式にするだけで、計算し直さない。

/// 手に置ける技・相手の技にできる技の規則(Web の `domain/combo.ts` と同じ)。
public enum ComboMoveRules {
    /// 追加効果が1つ以上のランクを実際に変えるか(stages に 0 でない値がある)。
    public static func changesRanks(_ move: Move) -> Bool {
        guard let stages = move.effect?.stages else { return false }
        return [stages.atk, stages.def, stages.spa, stages.spd, stages.spe].contains { $0 != 0 }
    }

    /// 連続技の手に置けるか。攻撃技はいつでも、変化技はランクを変えるものだけ(それ以外は API が 400)。
    public static func canBeStep(_ move: Move) -> Bool {
        move.category != .status || changesRanks(move)
    }

    /// 相手の技に置けるか。攻撃技だけ(変化技は API が 400)。
    public static func canBeOpponentMove(_ move: Move) -> Bool {
        move.category != .status
    }
}

/// ランク変化のチップ1つ。`direction` は色(上昇 = brandPrimary、下降 = warning)。文字でも符号を出す。
public struct ComboRankChip: Equatable, Sendable {
    public enum Direction: Equatable, Sendable {
        case up, down
    }

    public var text: String
    public var direction: Direction

    public init(text: String, direction: Direction) {
        self.text = text
        self.direction = direction
    }
}

public enum ComboRankChips {
    /// チップの能力の並び(攻撃・防御・特攻・特防・素早さ)。
    private static let statOrder: [StatKey] = [.atk, .def, .spa, .spd, .spe]

    /// 技のランク変化をチップにする(変えない技は空)。
    public static func chips(for move: Move) -> [ComboRankChip] {
        guard ComboMoveRules.changesRanks(move), let effect = move.effect else { return [] }
        return statOrder.compactMap { stat in
            let stage = stage(of: stat, in: effect.stages)
            guard stage != 0 else { return nil }
            let name = RankMeterLogic.statName(for: stat)
            let label = effect.target == .user ? name : ComboText.opponentRank(name)
            return ComboRankChip(text: "\(label) \(signed(stage))", direction: stage > 0 ? .up : .down)
        }
    }

    private static func stage(of stat: StatKey, in ranks: RankBlock) -> Int {
        switch stat {
        case .atk: return ranks.atk
        case .def: return ranks.def
        case .spa: return ranks.spa
        case .spd: return ranks.spd
        case .spe: return ranks.spe
        case .hp: return 0
        }
    }

    private static func signed(_ stage: Int) -> String {
        stage > 0 ? "+\(stage)" : "\(stage)"
    }
}

public enum ComboText {
    // MARK: 固定の語

    public static let sectionLabel = "連続技"
    /// 手が1つのときの開くボタン。
    public static let addStepButton = "次の技"
    /// 追加の手を選ぶ欄の名前(計算画面の「技」とは別の名前にして取り違えない)。
    public static let addPickerLabel = "追加の技"
    public static let closeButton = "閉じる"
    public static let maxStepsNotice = "手は最大\(RequestLimits.maxComboSteps)つまでです"
    public static let stepsListLabel = "連続技の手"
    public static let resultStepsListLabel = "連続技の各手の結果"
    public static let mainStepBadge = "上の技欄の技"
    public static let noDamage = "ダメージなし"
    public static let defenderLabel = "相手の振り"
    public static let opponentMoveLabel = "相手の技"
    public static let opponentMoveNone = "選ばない"
    public static let opponentSpeedFromDefender = "相手の素早さを防御側の振りから計算する"
    public static let speedFieldToggle = "素早さの場"
    public static let trickRoom = "トリックルーム"
    public static let selfTailwind = "追い風(自分)"
    public static let opponentTailwind = "追い風(相手)"
    public static let loading = "連続技を計算中…"
    /// iOS の失敗表示の「もう一度」(Web は別の導線)。
    public static let retryButton = "もう一度"
    public static let defenderReset = "選んでいた相手の振りがなくなったので、先頭の振りに戻しました"
    public static let note = "場は全手で同じ状態が続きます。命中は考えません。"
    public static let speedExampleListLabel = "素早さの基本値の例"
    /// 合計ダメージのバーの読み上げ名(値は `ComboResultDisplay.totalText`)。
    public static let totalBarLabel = "合計ダメージのバー"

    public static let selfSpeedLegend = "自分の素早さ"
    public static let selfSpeedSPLabel = "素早さのSP"
    public static let selfSpeedNatureLabel = "素早さの性格補正"
    public static let selfSpeedRankLabel = "素早さのランク"
    public static let selfSpeedRankStatName = "素早さ"
    public static let selfSpeedNatureNote = "性格を変えると、攻撃・特攻の補正も(両立できる範囲で)この性格に合わせます。"
    /// iOS だけ: 性格を ID で送るため、置き換えた組の性格がデータに無いと送れない(ADR-0535 §7)。
    public static let selfSpeedNatureUnavailable = "選んだ素早さの性格補正に合う性格がデータにありません"

    public static func selfSpeedNatureOption(_ nature: ComboSelfSpeedNature) -> String {
        switch nature {
        case .keep: return "計算画面の性格のまま"
        case .plus: return "素早さ上がる(×1.1)"
        case .neutral: return "素早さ補正なし"
        case .minus: return "素早さ下がる(×0.9)"
        }
    }

    // MARK: 手

    public static func stepOrdinal(_ index: Int) -> String { "\(index + 1)手目" }
    public static func moveUp(_ name: String) -> String { "\(name)を上へ" }
    public static func moveDown(_ name: String) -> String { "\(name)を下へ" }
    public static func removeStep(_ name: String) -> String { "\(name)を外す" }
    public static func critical(_ index: Int) -> String { "急所(\(index + 1)手目)" }
    /// ランクチップの「相手の…」(相手のランクを変える技)。
    public static func opponentRank(_ statName: String) -> String { "相手の\(statName)" }

    /// 行動順の語。決まらないとき(`dependsOnOpponentSpeed`)は nil(出さない)。
    public static func orderLabel(_ order: ComboTurnOrder) -> String? {
        switch order {
        case .selfFirst: return "先に動く"
        case .opponentFirst: return "相手が先"
        case .speedTie: return "同速"
        case .dependsOnOpponentSpeed: return nil
        }
    }

    // MARK: 結果

    private static let numberLocale = Locale(identifier: "en_US_POSIX")
    /// design.md の%幅の区切り(U+301C 波ダッシュ)。
    private static let rangeSeparator = "\u{301C}"
    private static let lineSeparator = " / "

    static func oneDecimal(_ value: Double) -> String {
        String(format: "%.1f", locale: numberLocale, value)
    }

    public static func total(minPercent: Double, maxPercent: Double) -> String {
        "合計 \(oneDecimal(minPercent))\(rangeSeparator)\(oneDecimal(maxPercent))%"
    }

    /// 合計で倒す確率の文言。確定は `guaranteed`、表示の値が 0 は「倒せない」(0・100.0 は不可能・確定に予約。API の説明)。
    public static func koText(steps: Int, ko: ComboKOChance) -> String {
        if ko.guaranteed { return "\(steps) 手で倒す(確定)" }
        if ko.displayChancePercent <= 0 { return "倒せない" }
        return "\(steps) 手で倒す \(oneDecimal(ko.displayChancePercent))%"
    }

    public static func speedSummary(first: Int, last: Int) -> String { "素早さ \(first) → \(last)" }
    public static func opponentSpeed(_ speed: Int) -> String { "相手の素早さ \(speed)" }

    /// 耐える確率(engine の生値 %)。0 と 100 は不可能・確定に予約し、それ以外は小数第1位に丸めて 0.1〜99.9 に収める。
    public static func survive(_ rawPercent: Double) -> String {
        if rawPercent >= 100 { return "耐える 100%" }
        if rawPercent <= 0 { return "耐える 0%" }
        let rounded = (rawPercent * 10).rounded() / 10
        return "耐える \(oneDecimal(min(99.9, max(0.1, rounded))))%"
    }

    public static func actions(min: Int, max: Int) -> String {
        min == max ? "相手の行動 \(min)回" : "相手の行動 \(min)\(rangeSeparator)\(max)回"
    }

    public static func perHit(minPercent: Double, maxPercent: Double) -> String {
        "1回ぶん \(oneDecimal(minPercent))\(rangeSeparator)\(oneDecimal(maxPercent))%"
    }

    /// 相手の素早さが分からないときの線(「197 以下を抜く / 198 は同速 / 199 以上は相手が先」)。
    /// トリックルーム中は API が向きを逆にして返すので、範囲の向きのまま言葉にする。空の範囲は省く。
    public static func speedLine(_ line: ComboSpeedLine) -> String {
        var parts: [String] = []
        if !line.selfFirst.isEmpty { parts.append("\(rangeWord(line.selfFirst))を抜く") }
        parts.append("\(line.selfSpeed) は同速")
        if !line.opponentFirst.isEmpty { parts.append("\(rangeWord(line.opponentFirst))は相手が先") }
        return parts.joined(separator: lineSeparator)
    }

    /// 下端が 0 なら「N 以下」、そうでなければ「N 以上」(API の線は片側が 0 始まり、反対側が上限まで)。
    private static func rangeWord(_ range: ComboSpeedRange) -> String {
        range.min <= 0 ? "\(range.max) 以下" : "\(range.min) 以上"
    }

    // MARK: 印

    public static func markReason(_ reason: ComboMarkReason) -> String {
        switch reason {
        case .choiceLocked: return "技を変えられない持ち物"
        case .defenderFullHPAssumed: return "2手目以降も相手が満タンとして計算"
        case .speedUndetermined: return "素早さへの影響を判定できない"
        }
    }

    // MARK: 失敗

    /// 節の中だけに出す失敗の文言。原因は計算画面と同じ分類(`CalcScreenError`)。
    public static func failureMessage(for error: any Error) -> String {
        "連続技を計算できませんでした(\(CalcScreenError(error).message))"
    }

    // MARK: 素早さの基本値の例

    /// 自分の素早さに対する「準速○族以上に抜かれる / ○族までは抜ける / ○族とは同速」(ADR-0610 の「族」)。
    /// 区間と名前は speed API の値のまま。準速の行が無ければ空。
    public static func speedExampleLines(_ reach: SpeedReach) -> [String] {
        guard let line = reach.lines.first(where: { $0.preset == .neutralMax }) else { return [] }
        var result: [String] = []
        if let faster = line.faster {
            result.append("準速 \(faster.min) 族以上には抜かれる\(exampleSuffix(line.slowestFaster))")
        }
        if let slower = line.slower {
            result.append("準速 \(slower.max) 族までは抜ける\(exampleSuffix(line.fastestSlower))")
        }
        if let tie = line.tie {
            let range = tie.min == tie.max ? "\(tie.min)" : "\(tie.min)\(rangeSeparator)\(tie.max)"
            result.append("準速 \(range) 族とは同速")
        }
        return result
    }

    private static func exampleSuffix(_ pokemon: SpeedPokemon?) -> String {
        pokemon.map { "(例: \($0.nameJa))" } ?? ""
    }
}
