// JudgeResultDisplay: 判定の応答(`JudgeResponse`)を画面に出す形へ整形する(P6-25。ADR-0504 §6)。
//
// 純粋な整形だけを持つ(`BulkResultDisplay` と同じ方針)。値は judge の応答のまま出し、「勝ち」「負け」に丸めない
// (ADR-0700 §6-1)。行は要求の候補と `defenderIndex` で対応づける(位置ではなく index で引く。取り違え防止。ADR-0705 §8)。
// 未対応の印(ADR-0708)は **方向ごとに分けて** 整形する。順方向(`attackerKoUnsupported`)と逆方向(`defenderKoUnsupported`)を
// 1つの配列にまとめない(どちらの確定数が疑わしいか分からなくなる。`target` の attacker/defender はその計算から見た役割)。

import Foundation

/// 結果の1行(相手候補1件)。文言はすべて `JudgeLabels` から作る。
public struct JudgeMatchupDisplay: Equatable, Identifiable, Sendable {
    public var id: Int { defenderIndex }
    /// 要求の `defenders` の位置(0 始まり)。
    public let defenderIndex: Int
    /// 「相手候補1」(番号は 1 始まり)。
    public let candidateLabel: String
    /// 種族名(要求の入力から引く。judge は種族名を返さない)。引けなければ speciesKey、候補が引けなければ空。
    public let nameJa: String
    /// エンブレム色用の先頭タイプ ID(種族が引けたときだけ)。
    public let primaryType: String?
    /// 「素早さ 150 対 120」
    public let speedText: String
    /// 「優先度 0 対 0」
    public let priorityText: String
    /// 「素早さで上回る」/「素早さで下回る」/「同速」(同速は `speedTie` が true のとき。真偽値1つに丸めない)。
    public let speedComparisonText: String
    /// 「自分が先に動く」/「相手が先に動く」/「どちらが先に動くか決まらない」(`turnOrderTie` が true のとき)。
    public let turnOrderText: String
    /// 「自分の技で相手を確定2発」「…を乱数3発(37.5%)」「…を倒せない」。
    public let attackerKoText: String
    /// 「相手の技で自分が確定1発」など。
    public let defenderKoText: String
    /// 順方向の印が1つでもあるときだけ非 nil(その確定数を「確定した数」として見せない旨。置き場所の共通・個別によらず行ごとに付く)。
    public let attackerKoUnreliableText: String?
    /// 逆方向の印が1つでもあるときだけ非 nil。順方向とは独立(片方だけ非 nil になりうる)。
    public let defenderKoUnreliableText: String?
    /// 順方向の印のうち、**全候補に共通ではない**ものの一覧(行ごとの注記。無ければ nil)。共通の印は `JudgeResultDisplay.attackerUnsupportedSummary`。
    public let attackerUnsupportedNote: String?
    /// 逆方向の同上。
    public let defenderUnsupportedNote: String?

    public init(
        defenderIndex: Int, candidateLabel: String, nameJa: String, primaryType: String?, speedText: String,
        priorityText: String, speedComparisonText: String, turnOrderText: String, attackerKoText: String,
        defenderKoText: String, attackerKoUnreliableText: String?, defenderKoUnreliableText: String?,
        attackerUnsupportedNote: String?, defenderUnsupportedNote: String?
    ) {
        self.defenderIndex = defenderIndex
        self.candidateLabel = candidateLabel
        self.nameJa = nameJa
        self.primaryType = primaryType
        self.speedText = speedText
        self.priorityText = priorityText
        self.speedComparisonText = speedComparisonText
        self.turnOrderText = turnOrderText
        self.attackerKoText = attackerKoText
        self.defenderKoText = defenderKoText
        self.attackerKoUnreliableText = attackerKoUnreliableText
        self.defenderKoUnreliableText = defenderKoUnreliableText
        self.attackerUnsupportedNote = attackerUnsupportedNote
        self.defenderUnsupportedNote = defenderUnsupportedNote
    }
}

/// 結果全体(行 + 方向ごとの、全候補に共通する印の注記)。
public struct JudgeResultDisplay: Equatable, Sendable {
    /// `defenderIndex` の昇順。
    public let rows: [JudgeMatchupDisplay]
    /// 全候補の順方向に共通する印の注記(無ければ nil)。`UnsupportedPlacement` で順方向だけを見て決める。
    public let attackerUnsupportedSummary: String?
    /// 全候補の逆方向に共通する印の注記(無ければ nil)。順方向とは混ぜない。
    public let defenderUnsupportedSummary: String?

    public init(rows: [JudgeMatchupDisplay], attackerUnsupportedSummary: String?, defenderUnsupportedSummary: String?) {
        self.rows = rows
        self.attackerUnsupportedSummary = attackerUnsupportedSummary
        self.defenderUnsupportedSummary = defenderUnsupportedSummary
    }
}

/// 応答 → 表示の整形(純粋関数)。
public enum JudgeResultDisplayBuilder {
    /// - Parameters:
    ///   - response: judge の応答。行は `defenderIndex` の昇順に並べる(要求と同じ順序で返る契約だが、index で引く)。
    ///   - request: **送信時点の**要求(候補の speciesKey を引く)。
    ///   - species: speciesKey → 種族(名前・タイプ)。引けない候補は speciesKey を名前に出す。
    ///   - names: 印の `id` を日本語名へ引く辞書(技・持ち物・特性)。
    public static func make(
        response: JudgeResponse, request: JudgeRequest, species: [String: SpeciesSummary], names: UnsupportedMarkNames
    ) -> JudgeResultDisplay {
        let ordered = response.matchups.sorted { $0.defenderIndex < $1.defenderIndex }
        // 順方向・逆方向は別々に置き場所を決める(混ぜない。ADR-0708 §4・§5)。
        let forward = UnsupportedPlacement(ordered.map(\.attackerKoUnsupported))
        let reverse = UnsupportedPlacement(ordered.map(\.defenderKoUnsupported))
        let rows = ordered.enumerated().map { position, matchup in
            row(matchup, request: request, species: species, names: names, forward: forward.perEntry[position],
                reverse: reverse.perEntry[position])
        }
        return JudgeResultDisplay(
            rows: rows,
            attackerUnsupportedSummary: UnsupportedNoticeText.rowNote(forward.common, names: names)
                .map(JudgeLabels.attackerKoUnsupportedNotice(detail:)),
            defenderUnsupportedSummary: UnsupportedNoticeText.rowNote(reverse.common, names: names)
                .map(JudgeLabels.defenderKoUnsupportedNotice(detail:)))
    }

    private static func row(
        _ matchup: JudgeMatchup, request: JudgeRequest, species: [String: SpeciesSummary], names: UnsupportedMarkNames,
        forward: [UnsupportedMark], reverse: [UnsupportedMark]
    ) -> JudgeMatchupDisplay {
        let index = matchup.defenderIndex
        let speciesKey = request.defenders.indices.contains(index) ? request.defenders[index].individual.speciesKey : nil
        let summary = speciesKey.flatMap { species[$0] }
        let comparison: String
        if matchup.speedTie {
            comparison = JudgeLabels.speedTie
        } else {
            comparison = matchup.outspeeds ? JudgeLabels.outspeedsTrue : JudgeLabels.outspeedsFalse
        }
        let turnOrder: String
        if matchup.turnOrderTie {
            turnOrder = JudgeLabels.turnOrderTie
        } else {
            turnOrder = matchup.attackerMovesFirst ? JudgeLabels.attackerMovesFirst : JudgeLabels.defenderMovesFirst
        }
        return JudgeMatchupDisplay(
            defenderIndex: index, candidateLabel: JudgeLabels.candidate(index + 1),
            nameJa: summary?.nameJa ?? speciesKey ?? "", primaryType: summary?.types.first?.rawValue,
            speedText: JudgeLabels.speed(attacker: matchup.attackerSpeed, defender: matchup.defenderSpeed),
            priorityText: JudgeLabels.priority(attacker: matchup.attackerMovePriority, defender: matchup.defenderMovePriority),
            speedComparisonText: comparison, turnOrderText: turnOrder,
            attackerKoText: JudgeLabels.attackerKoPrefix + koText(matchup.attackerKo),
            defenderKoText: JudgeLabels.defenderKoPrefix + koText(matchup.defenderKo),
            attackerKoUnreliableText: matchup.attackerKoUnsupported.isEmpty ? nil : JudgeLabels.attackerKoUnreliable,
            defenderKoUnreliableText: matchup.defenderKoUnsupported.isEmpty ? nil : JudgeLabels.defenderKoUnreliable,
            attackerUnsupportedNote: UnsupportedNoticeText.rowNote(forward, names: names)
                .map(JudgeLabels.attackerKoUnsupportedNotice(detail:)),
            defenderUnsupportedNote: UnsupportedNoticeText.rowNote(reverse, names: names)
                .map(JudgeLabels.defenderKoUnsupportedNotice(detail:)))
    }

    /// `BulkRowDisplay.koText` と同じ規則(確率は小数第1位固定)。
    private static func koText(_ ko: JudgeKOChance) -> String {
        BulkRowDisplay.koText(
            KOChance(hits: ko.hits, guaranteed: ko.guaranteed, chancePercent: 0, displayChancePercent: ko.displayChancePercent))
    }
}
