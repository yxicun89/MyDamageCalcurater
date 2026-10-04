import XCTest

@testable import PokeCalcCore

/// 結果の行の「素早さに反映した補正」「反映していない入力」の整形(ADR-0512)。
/// 空なら nil(出さない)・自分側(attacker*)と候補側(defender*)を取り違えない・並びは応答のまま・未知の値でも落ちない。
final class JudgeResultDisplaySpeedNotesTests: XCTestCase {
    private static let sp = StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0)
    private static let species = ["9101-000": SpeciesSummary(key: "9101-000", dexNo: 9101, form: 0, nameJa: "テストアルファ", types: [.fire])]

    private func display(_ matchups: [JudgeMatchup]) -> [JudgeMatchupDisplay] {
        let request = JudgeRequest(
            attacker: JudgeIndividual(speciesKey: "9001-000", natureId: "n", sp: Self.sp), moveId: "stub-move-self",
            defenders: matchups.map { _ in
                JudgeDefender(individual: JudgeIndividual(speciesKey: "9101-000", natureId: "n", sp: Self.sp), moveId: "stub-move")
            })
        return JudgeResultDisplayBuilder.make(
            response: JudgeResponse(matchups: matchups), request: request, species: Self.species,
            names: UnsupportedMarkNames(moveNames: [:], itemNames: [:], abilityNames: [:])).rows
    }

    private func row(
        _ index: Int = 0, attackerApplied: [String] = [], defenderApplied: [String] = [], attackerIgnored: [String] = [],
        defenderIgnored: [String] = []
    ) -> JudgeMatchup {
        var matchup = StubJudge.matchup(index)
        matchup.attackerSpeedApplied = attackerApplied
        matchup.defenderSpeedApplied = defenderApplied
        matchup.attackerSpeedIgnored = attackerIgnored
        matchup.defenderSpeedIgnored = defenderIgnored
        return matchup
    }

    func testEmptyArraysProduceNoText() {
        let rows = display([row()])
        XCTAssertNil(rows[0].attackerSpeedAppliedText)
        XCTAssertNil(rows[0].defenderSpeedAppliedText)
        XCTAssertNil(rows[0].attackerSpeedIgnoredText)
        XCTAssertNil(rows[0].defenderSpeedIgnoredText)
    }

    func testSidesAreNotMixedUp() {
        let rows = display([
            row(attackerApplied: ["rank", "tailwind"], defenderApplied: ["choiceScarf", "paralysis"],
                attackerIgnored: ["abilityId"], defenderIgnored: ["itemId", "fieldWeather"])
        ])
        XCTAssertEqual(rows[0].attackerSpeedAppliedText, "自分の素早さに反映: ランク補正・追い風")
        XCTAssertEqual(rows[0].defenderSpeedAppliedText, "相手の素早さに反映: こだわりスカーフ・まひ")
        XCTAssertEqual(rows[0].attackerSpeedIgnoredText, "自分の素早さに特性は反映していません")
        XCTAssertEqual(rows[0].defenderSpeedIgnoredText, "相手の素早さに持ち物・天候は反映していません")
    }

    func testOneSideOnlyLeavesTheOtherSideEmpty() {
        let rows = display([row(defenderApplied: ["paralysis"], attackerIgnored: ["itemId"])])
        XCTAssertNil(rows[0].attackerSpeedAppliedText)
        XCTAssertEqual(rows[0].defenderSpeedAppliedText, "相手の素早さに反映: まひ")
        XCTAssertEqual(rows[0].attackerSpeedIgnoredText, "自分の素早さに持ち物は反映していません")
        XCTAssertNil(rows[0].defenderSpeedIgnoredText)
    }

    func testOrderIsKeptAsReturnedByTheServer() {
        let rows = display([row(attackerApplied: ["paralysis", "rank"], attackerIgnored: ["fieldWeather", "abilityId"])])
        XCTAssertEqual(rows[0].attackerSpeedAppliedText, "自分の素早さに反映: まひ・ランク補正", "並べ替えない")
        XCTAssertEqual(rows[0].attackerSpeedIgnoredText, "自分の素早さに天候・特性は反映していません")
    }

    func testEachRowUsesItsOwnMatchupNotAnotherRow() {
        let rows = display([
            row(0, defenderApplied: ["tailwind"]),
            row(1, defenderIgnored: ["abilityId"]),
            row(2),
        ])
        XCTAssertEqual(rows.map(\.defenderIndex), [0, 1, 2])
        XCTAssertEqual(rows[0].defenderSpeedAppliedText, "相手の素早さに反映: 追い風")
        XCTAssertNil(rows[0].defenderSpeedIgnoredText)
        XCTAssertNil(rows[1].defenderSpeedAppliedText)
        XCTAssertEqual(rows[1].defenderSpeedIgnoredText, "相手の素早さに特性は反映していません")
        XCTAssertNil(rows[2].defenderSpeedAppliedText)
        XCTAssertNil(rows[2].defenderSpeedIgnoredText)
    }

    func testRowsAreStillLookedUpByDefenderIndexWhenTheResponseIsOutOfOrder() {
        let rows = display([row(1, defenderApplied: ["item"]), row(0, defenderApplied: ["ability"])])
        XCTAssertEqual(rows.map(\.defenderIndex), [0, 1])
        XCTAssertEqual(rows[0].defenderSpeedAppliedText, "相手の素早さに反映: 特性")
        XCTAssertEqual(rows[1].defenderSpeedAppliedText, "相手の素早さに反映: 持ち物")
    }

    func testUnknownValuesAreShownAsGenericLabelsAndDoNotCrash() {
        let rows = display([row(attackerApplied: ["rank", "futureFactor"], defenderIgnored: ["futureInput", "itemId"])])
        XCTAssertEqual(rows[0].attackerSpeedAppliedText, "自分の素早さに反映: ランク補正・その他の補正")
        XCTAssertEqual(rows[0].defenderSpeedIgnoredText, "相手の素早さにその他の入力・持ち物は反映していません")
    }

    func testSpeedNotesDoNotChangeTheOtherDisplayedValues() {
        let plain = display([row()])[0]
        let noted = display([row(attackerApplied: ["rank"], defenderIgnored: ["abilityId"])])[0]
        XCTAssertEqual(noted.speedText, plain.speedText)
        XCTAssertEqual(noted.priorityText, plain.priorityText)
        XCTAssertEqual(noted.speedComparisonText, plain.speedComparisonText)
        XCTAssertEqual(noted.turnOrderText, plain.turnOrderText)
        XCTAssertEqual(noted.attackerKoText, plain.attackerKoText)
        XCTAssertEqual(noted.defenderKoText, plain.defenderKoText)
        XCTAssertEqual(noted.attackerUnsupportedNote, plain.attackerUnsupportedNote)
    }
}
