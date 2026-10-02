// BalanceResultDisplay: タイプバランスの応答を画面に出す形に整える(P6-26。ADR-0505 §6)。
//
// 規則: 応答の値(倍率の文字列・分類・集計の数・有効/抜群の真偽)をそのまま運ぶ。**倍率の計算・分類の判定し直し・弱点の偏りの判定・タイプ相性表の参照をしない**。
// 並びは応答のまま(メンバーは要求の順・タイプは契約の正準順 normal … fairy)。名前は**送信時点の要求**から index で引く(同じ種族が重複していても取り違えない)。

/// メンバー1体の表示名(送信時点の構築から作る)。`name` はニックネーム → 種族名 → speciesKey、`abilityName` は特性があるときだけ(名前が引けなければ ID)。
public struct BalanceMemberLabel: Equatable, Sendable {
    public var name: String
    public var abilityName: String?

    public init(name: String, abilityName: String? = nil) {
        self.name = name
        self.abilityName = abilityName
    }
}

// MARK: - 防御相性

public struct BalanceDefenseCell: Equatable, Sendable {
    public let attackType: PokeType
    /// 「×2 弱点」(`BalanceLabels.defenseText`)。
    public let text: String
    public let category: BalanceDefenseCategory
    /// `source == .ability` の行だけ非 nil(`BalanceLabels.abilityNote(effect)`)。色だけで特性の影響を示さない。
    public let abilityNote: String?
}

public struct BalanceDefenseMemberRow: Equatable, Sendable {
    /// 要求の位置(0 始まり)。
    public let index: Int
    public let name: String
    public let abilityName: String?
    public let types: [PokeType]
    public let cells: [BalanceDefenseCell]
}

public struct BalanceSummaryRow: Equatable, Sendable {
    public let attackType: PokeType
    public let weak: Int
    public let quadWeak: Int
    public let resist: Int
    public let immune: Int
    public let neutral: Int
    /// `BalanceLabels.summaryText`。
    public let text: String
}

public struct BalanceDefenseDisplay: Equatable, Sendable {
    public let members: [BalanceDefenseMemberRow]
    public let summary: [BalanceSummaryRow]
}

// MARK: - 攻撃範囲

public struct BalanceCoverageCell: Equatable, Sendable {
    public let defenseType: PokeType
    /// 「×2 抜群」・攻撃技が無ければ「攻撃技なし」(`BalanceLabels.coverageText`)。
    public let text: String
    public let effective: Bool
    public let superEffective: Bool
}

public struct BalanceCoverageMemberRow: Equatable, Sendable {
    public let index: Int
    public let name: String
    public let attackTypes: [PokeType]
    /// `attackTypes` が空(攻撃技が無い)のとき false。
    public let hasAttackMove: Bool
    public let cells: [BalanceCoverageCell]
}

public struct BalanceCoverageTeamRow: Equatable, Sendable {
    public let defenseType: PokeType
    /// 最大倍率の表示(`BalanceLabels.coverageText`。全員攻撃技が無ければ「攻撃技なし」)。
    public let text: String
    public let effectiveMembers: Int
    public let superEffectiveMembers: Int
    /// `BalanceLabels.teamCoverageText`。
    public let membersText: String
}

public struct BalanceCoverageDisplay: Equatable, Sendable {
    public let members: [BalanceCoverageMemberRow]
    public let team: [BalanceCoverageTeamRow]
}

// MARK: - 組み立て

public enum BalanceResultDisplayBuilder {
    /// `labels` は要求の順(index で引く。足りない index は名前を捏造せず、応答の `pokemonId` を使う)。
    public static func defense(_ response: BalanceAnalyzeResponse, labels: [BalanceMemberLabel]) -> BalanceDefenseDisplay {
        let members = response.members.enumerated().map { index, member in
            let label = labels.indices.contains(index) ? labels[index] : nil
            return BalanceDefenseMemberRow(
                index: index, name: label?.name ?? member.pokemonId, abilityName: label?.abilityName, types: member.types,
                cells: member.defense.map { entry in
                    BalanceDefenseCell(
                        attackType: entry.attackType, text: BalanceLabels.defenseText(multiplier: entry.multiplier, category: entry.category),
                        category: entry.category, abilityNote: entry.source == .ability ? BalanceLabels.abilityNote(entry.effect) : nil)
                })
        }
        let summary = response.teamSummary.map { entry in
            BalanceSummaryRow(
                attackType: entry.attackType, weak: entry.weak, quadWeak: entry.quadWeak, resist: entry.resist, immune: entry.immune,
                neutral: entry.neutral,
                text: BalanceLabels.summaryText(
                    weak: entry.weak, quadWeak: entry.quadWeak, resist: entry.resist, immune: entry.immune, neutral: entry.neutral))
        }
        return BalanceDefenseDisplay(members: members, summary: summary)
    }

    public static func coverage(_ response: BalanceCoverageResponse, labels: [BalanceMemberLabel]) -> BalanceCoverageDisplay {
        let members = response.members.enumerated().map { index, member in
            BalanceCoverageMemberRow(
                index: index, name: labels.indices.contains(index) ? labels[index].name : member.pokemonId,
                attackTypes: member.attackTypes, hasAttackMove: !member.attackTypes.isEmpty,
                cells: member.coverage.map { entry in
                    BalanceCoverageCell(
                        defenseType: entry.defenseType, text: BalanceLabels.coverageText(entry.bestMultiplier),
                        effective: entry.effective, superEffective: entry.superEffective)
                })
        }
        let team = response.teamCoverage.map { entry in
            BalanceCoverageTeamRow(
                defenseType: entry.defenseType, text: BalanceLabels.coverageText(entry.bestMultiplier),
                effectiveMembers: entry.effectiveMembers, superEffectiveMembers: entry.superEffectiveMembers,
                membersText: BalanceLabels.teamCoverageText(
                    effectiveMembers: entry.effectiveMembers, superEffectiveMembers: entry.superEffectiveMembers))
        }
        return BalanceCoverageDisplay(members: members, team: team)
    }
}
