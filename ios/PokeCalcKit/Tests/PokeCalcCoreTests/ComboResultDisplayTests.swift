import XCTest

@testable import PokeCalcCore

/// 連続技の結果の整形(AC-i7・AC-i9。ADR-0535 §8・§10)。API の値を書式にするだけで、計算し直さない。
final class ComboResultDisplayTests: XCTestCase {

    private let names = UnsupportedMarkNames(
        moves: ComboStub.allMoves,
        items: [Item(id: "stub-item-choice", nameJa: "テストこだわりどうぐ")],
        abilities: [Ability(id: "stub-ability-full", nameJa: "テストまんたんとくせい")])

    private func display(_ result: ComboResult, moves: [Move?]? = nil, trickRoom: Bool = false) -> ComboResultDisplay {
        ComboResultDisplay(
            result: result, stepMoves: moves ?? [ComboStub.attackMove, ComboStub.boostMove], names: names, trickRoom: trickRoom)
    }

    func testStepsShowOrdinalNameChipsDamageAndOrder() {
        let result = ComboStub.result(steps: [
            ComboStub.step(ComboStub.attackMove, damage: ComboStub.calcResult(), order: .opponentFirst),
            ComboStub.step(ComboStub.boostMove, damage: nil, order: .selfFirst),
            ComboStub.step(ComboStub.priorityMove, damage: ComboStub.calcResult(minPercent: 12.0, maxPercent: 14.5)),
        ])
        let shown = display(result, moves: [ComboStub.attackMove, ComboStub.boostMove, ComboStub.priorityMove])
        XCTAssertEqual(shown.steps.map(\.ordinalText), ["1手目", "2手目", "3手目"])
        XCTAssertEqual(shown.steps.map(\.moveName), ["テストこうげきわざ", "テストまいわざ", "テストせんせいわざ"])
        XCTAssertEqual(shown.steps.map(\.damageText), ["40.2\u{301C}47.8%", "ダメージなし", "12.0\u{301C}14.5%"])
        XCTAssertEqual(shown.steps.map(\.orderText), ["相手が先", "先に動く", nil], "決まらない手は出さない")
        XCTAssertEqual(shown.steps[1].chips, ComboRankChips.chips(for: ComboStub.boostMove))
        XCTAssertEqual(shown.steps[0].chips, [])
    }

    func testUnknownStepMoveFallsBackToTheMoveId() {
        let result = ComboStub.result()
        let shown = display(result, moves: [nil, ComboStub.boostMove])
        XCTAssertEqual(shown.steps[0].moveName, ComboStub.attackMove.id, "名前を引けなければ ID のまま(黙って消さない)")
    }

    func testTotalBarKOAndSpeedWhenOpponentSpeedIsUnknown() {
        let shown = display(ComboStub.result())
        XCTAssertEqual(shown.totalText, "合計 91.4\u{301C}108.6%")
        XCTAssertEqual(shown.totalBarFraction, 1.0, accuracy: 1e-9, "100% 超はバーいっぱい")
        XCTAssertEqual(shown.koText, "2 手で倒す 51.6%")
        XCTAssertEqual(shown.speedSummaryText, "素早さ 132 → 198")
        XCTAssertEqual(shown.speedLineText, "197 以下を抜く / 198 は同速 / 199 以上は相手が先", "最後の手の線")
        XCTAssertNil(shown.opponentSpeedText)
        XCTAssertTrue(shown.showsSpeedExample)
        XCTAssertEqual(shown.lastSelfSpeed, 198)
        XCTAssertNil(shown.surviveText)
        XCTAssertNil(shown.actionsText)
        XCTAssertNil(shown.perHitText)
        XCTAssertNil(shown.unsupportedNotice)
    }

    func testBarFractionBelowHundred() {
        let shown = display(ComboStub.result(minPercent: 30.0, maxPercent: 45.5))
        XCTAssertEqual(shown.totalBarFraction, 0.455, accuracy: 1e-9)
    }

    func testKnownOpponentSpeedShowsTheValueAndNoLineOrExample() {
        let result = ComboStub.result(
            speed: ComboSpeedSummary(firstSelfSpeed: 132, lastSelfSpeed: 198, opponentSpeed: 169, opponentSpeedKnown: true))
        let shown = display(result)
        XCTAssertEqual(shown.opponentSpeedText, "相手の素早さ 169")
        XCTAssertNil(shown.speedLineText)
        XCTAssertFalse(shown.showsSpeedExample)
    }

    func testTrickRoomHidesTheSpeedExampleButKeepsTheLine() {
        let shown = display(ComboStub.result(), trickRoom: true)
        XCTAssertNotNil(shown.speedLineText)
        XCTAssertFalse(shown.showsSpeedExample, "reach は向きを入れ替えないので、トリックルーム中は例を出さない(ADR-0359 段階 C)")
    }

    func testOpponentAttackUsesTheFastScenario() {
        let shown = display(ComboStub.result(opponentAttack: ComboStub.opponentAttack(fastSurvive: 87.5)))
        XCTAssertEqual(shown.surviveText, "耐える 87.5%")
        XCTAssertEqual(shown.actionsText, "相手の行動 1\u{301C}2回")
        XCTAssertEqual(shown.perHitText, "1回ぶん 25.0\u{301C}31.7%")
        let noActions = ComboOpponentAttack(
            selfHp: 132, minActions: 0, maxActions: 0,
            fast: ComboOpponentScenario(actions: [], surviveChancePercent: 100),
            slow: ComboOpponentScenario(actions: [], surviveChancePercent: 100))
        let none = display(ComboStub.result(opponentAttack: noActions))
        XCTAssertEqual(none.surviveText, "耐える 100%")
        XCTAssertEqual(none.actionsText, "相手の行動 0回")
        XCTAssertNil(none.perHitText, "相手が行動しないなら 1回ぶんは出さない")
    }

    func testGuaranteedAndCannotKO() {
        XCTAssertEqual(display(ComboStub.result(ko: ComboStub.ko(display: 100, guaranteed: true))).koText, "2 手で倒す(確定)")
        XCTAssertEqual(display(ComboStub.result(ko: ComboStub.ko(display: 0))).koText, "倒せない")
    }

    func testUnsupportedNoticeUsesComboWordsAndCalcWords() {
        let marks: [ComboMark] = [
            .calc(UnsupportedMark(target: .move, reason: .multiHit, id: ComboStub.attackMove.id)),
            .combo(target: .attackerItem, reason: .choiceLocked, id: "stub-item-choice"),
            .combo(target: .defenderAbility, reason: .defenderFullHPAssumed, id: "stub-ability-full"),
            .combo(target: .move, reason: .speedUndetermined, id: ComboStub.boostMove.id),
        ]
        let shown = display(ComboStub.result(unsupported: marks))
        XCTAssertEqual(
            shown.unsupportedNotice,
            "この結果は正確でない可能性があります(未対応: 技「テストこうげきわざ」(多段技)、"
                + "攻撃側の持ち物「テストこだわりどうぐ」(技を変えられない持ち物)、"
                + "防御側の特性「テストまんたんとくせい」(2手目以降も相手が満タンとして計算)、"
                + "技「テストまいわざ」(素早さへの影響を判定できない))")
    }

    func testComboMarkReasonWords() {
        XCTAssertEqual(ComboText.markReason(.choiceLocked), "技を変えられない持ち物")
        XCTAssertEqual(ComboText.markReason(.defenderFullHPAssumed), "2手目以降も相手が満タンとして計算")
        XCTAssertEqual(ComboText.markReason(.speedUndetermined), "素早さへの影響を判定できない")
        XCTAssertEqual(ComboMarkReason.allCases.map(\.rawValue), ["choice_locked", "defender_full_hp_assumed", "speed_undetermined"])
    }
}
