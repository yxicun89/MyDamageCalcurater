import XCTest

@testable import PokeCalcCore

/// `JudgeResultDisplayBuilder`(P6-25。ADR-0504 §6): 応答 → 画面に出す形。値は応答のまま(丸めない)、行は `defenderIndex` で要求の候補と対応づける、
/// 未対応の印は方向ごとに分けて出す(ADR-0708 §4・§5。混ぜない)。
final class JudgeResultDisplayTests: XCTestCase {
    private static let sp = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)

    private static let speciesA = SpeciesSummary(key: "9101-000", dexNo: 9101, form: 0, nameJa: "テストアルファ", types: [.fire, .flying])
    private static let speciesB = SpeciesSummary(key: "9102-000", dexNo: 9102, form: 0, nameJa: "テストベータ", types: [.water])
    private static let speciesC = SpeciesSummary(key: "9103-000", dexNo: 9103, form: 0, nameJa: "テストガンマ", types: [.grass])
    private static let species = [speciesA.key: speciesA, speciesB.key: speciesB, speciesC.key: speciesC]

    private static func defender(_ key: String, move: String = "stub-move") -> JudgeDefender {
        JudgeDefender(individual: JudgeIndividual(speciesKey: key, natureId: "n", sp: sp), moveId: move)
    }

    private static func request(_ keys: [String]) -> JudgeRequest {
        JudgeRequest(
            attacker: JudgeIndividual(speciesKey: "9001-000", natureId: "n", sp: sp), moveId: "stub-move-self",
            defenders: keys.map { defender($0) })
    }

    private static let names = UnsupportedMarkNames(
        moveNames: ["stub-move-self": "テストわざれんぞく", "stub-move-x": "テストわざへんどう"],
        itemNames: ["stub-item-a": "テストどうぐA"], abilityNames: [:])

    private func display(
        _ matchups: [JudgeMatchup], keys: [String] = ["9101-000", "9102-000", "9103-000"]
    ) -> JudgeResultDisplay {
        JudgeResultDisplayBuilder.make(
            response: JudgeResponse(matchups: matchups), request: Self.request(keys), species: Self.species, names: Self.names)
    }

    private func matchup(
        _ index: Int, attackerMarks: [UnsupportedMark] = [], defenderMarks: [UnsupportedMark] = []
    ) -> JudgeMatchup {
        var row = StubJudge.matchup(index)
        row.attackerKoUnsupported = attackerMarks
        row.defenderKoUnsupported = defenderMarks
        return row
    }

    private static let forwardMark = UnsupportedMark(target: .move, reason: .multiHit, id: "stub-move-self")
    private static let reverseMark = UnsupportedMark(target: .move, reason: .variablePower, id: "stub-move-x")

    // MARK: - 行の対応づけ

    func testRowsAreOrderedByDefenderIndexAndNamedFromTheRequestCandidates() {
        let result = display([matchup(2), matchup(0), matchup(1)])
        XCTAssertEqual(result.rows.map(\.defenderIndex), [0, 1, 2])
        XCTAssertEqual(result.rows.map(\.nameJa), ["テストアルファ", "テストベータ", "テストガンマ"], "種族名は要求の入力から index で引く")
        XCTAssertEqual(result.rows.map(\.candidateLabel), ["相手候補1", "相手候補2", "相手候補3"])
        XCTAssertEqual(result.rows.map(\.primaryType), ["fire", "water", "grass"], "エンブレム用の先頭タイプ")
        XCTAssertEqual(result.rows.map(\.id), [0, 1, 2])
    }

    func testRowValuesBelongToTheirOwnCandidate() {
        // StubJudge.matchup は index ごとに違う値(defenderSpeed = 100 + 10 × index)。取り違えると文言がずれる。
        let result = display([matchup(2), matchup(0), matchup(1)])
        XCTAssertEqual(result.rows.map(\.speedText), ["素早さ 200 対 100", "素早さ 200 対 110", "素早さ 200 対 120"])
    }

    func testSameSpeciesTwiceKeepsBothRows() {
        let result = display([matchup(0), matchup(1)], keys: ["9101-000", "9101-000"])
        XCTAssertEqual(result.rows.map(\.nameJa), ["テストアルファ", "テストアルファ"], "同じ候補が重複していても取りまとめない(ADR-0703 §6)")
        XCTAssertEqual(result.rows.map(\.candidateLabel), ["相手候補1", "相手候補2"])
    }

    func testUnknownSpeciesFallsBackToTheKeyAndAnOutOfRangeIndexToAnEmptyName() {
        let unknown = JudgeResultDisplayBuilder.make(
            response: JudgeResponse(matchups: [matchup(0), matchup(5)]), request: Self.request(["9999-000"]), species: Self.species,
            names: Self.names)
        XCTAssertEqual(unknown.rows[safe: 0]?.nameJa, "9999-000", "名前が引けなければ speciesKey(黙って消さない)")
        XCTAssertNil(unknown.rows[safe: 0]?.primaryType)
        XCTAssertEqual(unknown.rows[safe: 1]?.nameJa, "", "要求に無い index は名前を捏造しない")
        XCTAssertEqual(unknown.rows[safe: 1]?.candidateLabel, "相手候補6")
    }

    // MARK: - 素早さ・優先度・行動順

    func testSpeedComparisonTexts() {
        var higher = matchup(0)
        higher.outspeeds = true
        higher.speedTie = false
        var lower = matchup(1)
        lower.outspeeds = false
        lower.speedTie = false
        var tie = matchup(2)
        tie.outspeeds = false
        tie.speedTie = true
        let result = display([higher, lower, tie])
        XCTAssertEqual(result.rows.map(\.speedComparisonText), ["素早さで上回る", "素早さで下回る", "同速"], "同速は真偽値1つに丸めない")
    }

    func testTurnOrderTexts() {
        var attackerFirst = matchup(0)
        attackerFirst.attackerMovesFirst = true
        attackerFirst.turnOrderTie = false
        var defenderFirst = matchup(1)
        defenderFirst.attackerMovesFirst = false
        defenderFirst.turnOrderTie = false
        var tie = matchup(2)
        tie.attackerMovesFirst = false
        tie.turnOrderTie = true
        let result = display([attackerFirst, defenderFirst, tie])
        XCTAssertEqual(result.rows.map(\.turnOrderText), ["自分が先に動く", "相手が先に動く", "どちらが先に動くか決まらない"])
    }

    /// 素早さで上回っていても、優先度で相手が先に動くことがある(outspeeds と行動順は別の欄)。
    func testPriorityCanContradictOutspeeds() {
        var row = matchup(0)
        row.outspeeds = true
        row.attackerMovePriority = 0
        row.defenderMovePriority = 1
        row.attackerMovesFirst = false
        row.turnOrderTie = false
        let shown = display([row]).rows[safe: 0]
        XCTAssertEqual(shown?.speedComparisonText, "素早さで上回る")
        XCTAssertEqual(shown?.priorityText, "優先度 0 対 1")
        XCTAssertEqual(shown?.turnOrderText, "相手が先に動く")
    }

    // MARK: - 確定数

    func testKoTexts() {
        var row = matchup(0)
        row.attackerKo = JudgeKOChance(hits: 2, guaranteed: true, displayChancePercent: 100)
        row.defenderKo = JudgeKOChance(hits: 3, guaranteed: false, displayChancePercent: 37.5)
        var none = matchup(1)
        none.attackerKo = JudgeKOChance(hits: 0, guaranteed: false, displayChancePercent: 0)
        none.defenderKo = JudgeKOChance(hits: 1, guaranteed: false, displayChancePercent: 12)
        let result = display([row, none])
        XCTAssertEqual(result.rows[safe: 0]?.attackerKoText, "自分の技で相手を確定2発")
        XCTAssertEqual(result.rows[safe: 0]?.defenderKoText, "相手の技で自分が乱数3発(37.5%)")
        XCTAssertEqual(result.rows[safe: 1]?.attackerKoText, "自分の技で相手を倒せない")
        XCTAssertEqual(result.rows[safe: 1]?.defenderKoText, "相手の技で自分が乱数1発(12.0%)", "確率は小数第1位固定")
    }

    // MARK: - 未対応の印(方向ごとに分ける)

    func testNoMarksMeansNoNoticesAndNoCaveats() {
        let result = display([matchup(0), matchup(1)])
        XCTAssertNil(result.attackerUnsupportedSummary)
        XCTAssertNil(result.defenderUnsupportedSummary)
        for row in result.rows {
            XCTAssertNil(row.attackerKoUnreliableText)
            XCTAssertNil(row.defenderKoUnreliableText)
            XCTAssertNil(row.attackerUnsupportedNote)
            XCTAssertNil(row.defenderUnsupportedNote)
        }
    }

    /// 順方向は全行に共通 → 結果の上に1回(行には出さない)。逆方向は 2 行目だけ → その行に(結果の上には出さない)。
    func testCommonMarksGoToTheSummaryAndPerRowMarksToTheRowSeparatelyPerDirection() {
        let result = display([
            matchup(0, attackerMarks: [Self.forwardMark]),
            matchup(1, attackerMarks: [Self.forwardMark], defenderMarks: [Self.reverseMark]),
            matchup(2, attackerMarks: [Self.forwardMark]),
        ])
        XCTAssertEqual(
            result.attackerUnsupportedSummary,
            JudgeLabels.attackerKoUnsupportedNotice(detail: "未対応: 技「テストわざれんぞく」(多段技)"), "順方向の共通の印は結果の上に1回")
        XCTAssertNil(result.defenderUnsupportedSummary, "逆方向は全行に共通ではないので結果の上には出さない")
        XCTAssertEqual(result.rows.map(\.attackerUnsupportedNote), [nil, nil, nil], "共通の印は行に重ねて出さない")
        XCTAssertEqual(
            result.rows.map(\.defenderUnsupportedNote),
            [nil, JudgeLabels.defenderKoUnsupportedNotice(detail: "未対応: 技「テストわざへんどう」(威力が変化)"), nil])
    }

    /// 添え書き(その方向の確定数を確定として見せない旨)は、置き場所(共通・個別)によらず、印のある行の、その方向だけに付く。
    func testUnreliableCaveatIsPerRowAndPerDirection() {
        let result = display([
            matchup(0, attackerMarks: [Self.forwardMark]),
            matchup(1, attackerMarks: [Self.forwardMark], defenderMarks: [Self.reverseMark]),
            matchup(2, attackerMarks: [Self.forwardMark]),
        ])
        XCTAssertEqual(result.rows.map(\.attackerKoUnreliableText), Array(repeating: JudgeLabels.attackerKoUnreliable, count: 3))
        XCTAssertEqual(result.rows.map(\.defenderKoUnreliableText), [nil, JudgeLabels.defenderKoUnreliable, nil])
    }

    /// 同じ印の中身が両方向にあっても混ぜない(方向ごとの注記は別々)。片方向だけなら、もう片方の添え書きは出ない。
    func testTheSameMarkInBothDirectionsStaysSeparate() {
        let both = display([matchup(0, attackerMarks: [Self.forwardMark], defenderMarks: [Self.forwardMark])]).rows[safe: 0]
        XCTAssertNotNil(both?.attackerKoUnreliableText)
        XCTAssertNotNil(both?.defenderKoUnreliableText)
        let result = display([matchup(0, attackerMarks: [Self.forwardMark], defenderMarks: [Self.forwardMark])])
        XCTAssertNotEqual(result.attackerUnsupportedSummary, result.defenderUnsupportedSummary, "方向の語が違う")
        XCTAssertTrue(result.attackerUnsupportedSummary?.hasPrefix("自分の技の確定数") == true)
        XCTAssertTrue(result.defenderUnsupportedSummary?.hasPrefix("相手の技の確定数") == true)

        let forwardOnly = display([matchup(0, attackerMarks: [Self.forwardMark])]).rows[safe: 0]
        XCTAssertNotNil(forwardOnly?.attackerKoUnreliableText)
        XCTAssertNil(forwardOnly?.defenderKoUnreliableText, "順方向の印が逆方向の確定数を疑わしく見せない")
    }

    /// 1 件だけなら、その候補の印はすべて「全行に共通」なので結果の上に出る(行の注記は無い)。
    func testSingleCandidateMarksAreAllCommon() {
        let result = display([matchup(0, attackerMarks: [Self.forwardMark], defenderMarks: [Self.reverseMark])], keys: ["9101-000"])
        XCTAssertNotNil(result.attackerUnsupportedSummary)
        XCTAssertNotNil(result.defenderUnsupportedSummary)
        XCTAssertNil(result.rows[safe: 0]?.attackerUnsupportedNote)
        XCTAssertNil(result.rows[safe: 0]?.defenderUnsupportedNote)
        XCTAssertNotNil(result.rows[safe: 0]?.attackerKoUnreliableText, "共通の印でも、行ごとの添え書きは付く")
    }

    /// 逆方向の `target` はその calc から見た役割(attacker_* = その候補)のまま、印の文言(`UnsupportedMarkLabel`)で出す。書き換えない(ADR-0708 §4・§5)。
    func testReverseMarkTargetsAreRenderedInTheCalcsOwnFrameWithoutSwapping() {
        let reverseItem = UnsupportedMark(target: .attackerItem, reason: .unsupportedEffect, id: "stub-item-a")
        let result = display([matchup(0, defenderMarks: [reverseItem])], keys: ["9101-000"])
        XCTAssertEqual(
            result.defenderUnsupportedSummary,
            JudgeLabels.defenderKoUnsupportedNotice(detail: "未対応: 攻撃側の持ち物「テストどうぐA」"),
            "相手の技の確定数の見出しの下で、attacker_item は『攻撃側の持ち物』のまま(自分の持ち物と読み替えない)")
    }

    func testMarkIdsWithoutANameFallBackToTheId() {
        let unknownMove = UnsupportedMark(target: .move, reason: .multiHit, id: "stub-move-unknown")
        let result = display([matchup(0, attackerMarks: [unknownMove])], keys: ["9101-000"])
        XCTAssertEqual(
            result.attackerUnsupportedSummary,
            JudgeLabels.attackerKoUnsupportedNotice(detail: "未対応: 技「stub-move-unknown」(多段技)"), "引けない ID は ID のまま出す(黙って消さない)")
    }
}
