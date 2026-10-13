// ComboContext / ComboDefenderOptions: 連続技へ渡す計算画面の値(AC-i4・AC-i5。UC-5: 別の入力欄を作らない)。

/// 相手の振り1つ(計算画面の一括計算の行)。`key` は結果の行の ID と同じ。
public struct ComboDefenderOption: Equatable, Sendable {
    public var key: String
    public var label: String
    /// 要求の防御側にそのまま使う個体(行の振り・持ち物・特性・計算画面の防御側のランク)。
    public var individual: Individual

    public init(key: String, label: String, individual: Individual) {
        self.key = key
        self.label = label
        self.individual = individual
    }
}

public enum ComboDefenderOptions {
    /// 結果の行(振り × 持ち物 × 特性)を、連続技の防御側の選択肢にする。
    /// 性格 ID は行の `natureId`、無ければ補正が一致する性格の ID 昇順の最初。引けない行は選択肢にしない(送れない要求を作らない)。
    /// `rowIDs[i]` は `rows[i]` の画面の識別子(`CalcViewModel.rows` の `id`)。
    public static func make(
        rows: [BulkCalcRow], rowIDs: [String], defenderSpeciesKey: String, defenderRanks: RankBlock, items: [Item],
        abilityNames: [String: String], natures: [Nature]
    ) -> [ComboDefenderOption] {
        let splitBases = ResultEntryIdentity.splitBaseIDs(rows.map { BulkRowDisplay.baseID(for: $0) })
        return rows.enumerated().compactMap { index, row in
            guard let key = rowIDs[safe: index], let natureId = natureId(for: row.defender, natures: natures) else { return nil }
            var parts = [row.presetLabel]
            if let itemId = row.itemId { parts.append(ItemDisplayName.text(itemId: itemId, items: items)) }
            if splitBases.contains(BulkRowDisplay.baseID(for: row)), !row.abilityIds.isEmpty {
                parts.append(row.abilityIds.map { abilityNames[$0] ?? $0 }.joined(separator: AbilityGroupLabel.separator))
            }
            return ComboDefenderOption(
                key: key, label: parts.joined(separator: " / "),
                individual: Individual(
                    speciesKey: defenderSpeciesKey, natureId: natureId, sp: row.defender.sp, abilityId: row.abilityId,
                    itemId: row.itemId, ranks: defenderRanks))
        }
    }

    private static func natureId(for defender: BulkDefender, natures: [Nature]) -> String? {
        if let id = defender.natureId { return id }
        return natures.filter { $0.plus == defender.nature.plus && $0.minus == defender.nature.minus }.map(\.id).min()
    }
}

/// 連続技へ渡す計算画面の材料。同じ値なら引き直さない(`Equatable`)。
public struct ComboContext: Equatable, Sendable {
    /// 計算に使った攻撃側(ランク・やけど・特性・持ち物込み)。
    public var attacker: Individual
    /// 計算画面の技(1 手目)。
    public var mainMove: Move
    /// メインの急所は計算画面の「急所」。
    public var mainCritical: Bool
    public var field: FieldState
    public var defenders: [ComboDefenderOption]
    /// 攻撃側の覚える技 ID(追加の技の候補の元)。
    public var attackerLearnset: [String]
    public var defenderSpeciesKey: String
    /// 自分の素早さの性格を引き直す材料。
    public var natures: [Nature]
    /// 印の名前の引き先(技・持ち物・特性)。表示のためだけの値で、同じ入力かどうかの比較(`==`)には使わない
    /// (名前の辞書が増えただけで連続技を引き直さないため)。
    public var markNames: UnsupportedMarkNames

    public init(
        attacker: Individual, mainMove: Move, mainCritical: Bool, field: FieldState, defenders: [ComboDefenderOption],
        attackerLearnset: [String], defenderSpeciesKey: String, natures: [Nature],
        markNames: UnsupportedMarkNames = UnsupportedMarkNames()
    ) {
        self.attacker = attacker
        self.mainMove = mainMove
        self.mainCritical = mainCritical
        self.field = field
        self.defenders = defenders
        self.attackerLearnset = attackerLearnset
        self.defenderSpeciesKey = defenderSpeciesKey
        self.natures = natures
        self.markNames = markNames
    }

    public static func == (lhs: ComboContext, rhs: ComboContext) -> Bool {
        lhs.attacker == rhs.attacker && lhs.mainMove == rhs.mainMove && lhs.mainCritical == rhs.mainCritical
            && lhs.field == rhs.field && lhs.defenders == rhs.defenders && lhs.attackerLearnset == rhs.attackerLearnset
            && lhs.defenderSpeciesKey == rhs.defenderSpeciesKey && lhs.natures == rhs.natures
    }
}
