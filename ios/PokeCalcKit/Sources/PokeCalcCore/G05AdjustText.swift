// G05AdjustText: 調整画面の見た目の作り直し(G-05 第 4 弾。ADR-0540)で足す語・純粋な判定の置き場。
//
// 画面は文言と判定をここから引く(ロジックを View に持たせない。ADR-0500 §1)。

extension AdjustText {
    /// 目標の相手を、これまでに選んだ相手の絵から選ぶ列の見出し(Web の `goalRecentOpponentsLabel`)。
    public static let goalRecentOpponents = "選んだ相手から"

    /// 増減ボタンの名前(用語集「増やす/減らす」。Web の `<欄の名前>` + `uiText.stepperIncrease`)。
    public static func stepperIncrease(_ fieldName: String) -> String { "\(fieldName)を増やす" }
    public static func stepperDecrease(_ fieldName: String) -> String { "\(fieldName)を減らす" }
}

/// 数値欄(固定する能力ポイント・上限・発数)の − / + の判定(Web の `NumberField` の stepper と同じ。純粋)。
/// 文字は ASCII の数字だけを数として読む。空・読めない文字は 0、範囲外は範囲の端に丸めてから ±1 する。
public enum AdjustNumberStep {
    /// 能力ポイントの 1 欄(固定 SP・上限)の範囲。
    public static let spRange = 0...32

    /// 欄の文字を範囲に丸めた値(空・読めない文字は 0 とみなしてから範囲に丸める)。
    public static func clamped(text: String, range: ClosedRange<Int>) -> Int {
        let digits = text.utf8.allSatisfy { $0 >= 0x30 && $0 <= 0x39 }
        // 桁あふれ(Int に収まらない数字列)は上端。
        let value = digits && !text.isEmpty ? (Int(text) ?? Int.max) : 0
        return min(max(value, range.lowerBound), range.upperBound)
    }
    /// + が押せる(丸めた値が範囲の上端未満)。
    public static func canIncrease(text: String, range: ClosedRange<Int>) -> Bool { clamped(text: text, range: range) < range.upperBound }
    /// − が押せる(丸めた値が範囲の下端より大きい)。
    public static func canDecrease(text: String, range: ClosedRange<Int>) -> Bool { clamped(text: text, range: range) > range.lowerBound }
    /// 丸めた値に delta を足し、範囲に収めた結果の文字。
    public static func stepped(text: String, delta: Int, range: ClosedRange<Int>) -> String {
        let next = clamped(text: text, range: range) + delta
        return String(min(max(next, range.lowerBound), range.upperBound))
    }
}

/// 結果の振り方の能力ごとの横棒(装飾。幅 = 能力ポイント / 32。Web の `SpBars`)。
public enum AdjustSPBar {
    /// 満ち具合(0...1。範囲外は丸める)。
    public static func fraction(sp: Int) -> Double { Double(min(max(sp, 0), 32)) / 32 }
}

extension AdjustViewModel {
    /// 自分のポケモンカードの持ち物の印(選んだ持ち物の名前。メガ種族は固定のストーンの名前。無ければ nil)。
    public var ownItemMark: String? {
        if case .locked(_, let name) = ownItemLock { return name }
        return ownItemId.map { itemLabel(for: $0) }
    }

    /// 相手のポケモンカードの持ち物の印(メガ種族だけ、固定のストーンの名前。それ以外は nil)。
    public var opponentItemMark: String? { megaStoneMark(speciesKey: opponentSpeciesKey) }

    /// 目標の相手のポケモンカードの持ち物の印(メガ種族だけ、固定のストーンの名前。それ以外は nil)。
    public func goalOpponentItemMark(_ draft: AdjustGoalDraft) -> String? { megaStoneMark(speciesKey: draft.opponentSpeciesKey) }

    /// 目標の相手を選ぶ「選んだ相手から」の候補(相手の領域の相手 → 各目標の相手の順で、同じ種族は 1 つ。未選択は除く)。
    public var recentOpponents: [SpeciesSummary] {
        var seen = Set<String>()
        var result: [SpeciesSummary] = []
        let keys = [opponentSpeciesKey] + goalDrafts.map(\.opponentSpeciesKey)
        for case let key? in keys where seen.insert(key).inserted {
            if let species = speciesDictionary[key] { result.append(species) }
        }
        return result
    }

    private func megaStoneMark(speciesKey: String?) -> String? {
        guard let speciesKey else { return nil }
        if case .locked(_, let name) = MegaItemLock.make(for: megaInfo[speciesKey], allItems: itemOptions) { return name }
        return nil
    }
}
