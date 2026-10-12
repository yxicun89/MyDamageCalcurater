import XCTest

@testable import PokeCalcCore

/// 連続技(CB-8。ADR-0535・ADR-0501「連続技」)の純粋な規則と文言。
/// Web の `domain/combo.ts`・`domain/comboView.ts`・`i18n/combo.ts`(ADR-0359)と同じ語・同じ規則であることを固定する。
final class ComboDomainTests: XCTestCase {

    // MARK: - 手に置ける技(AC-i2)

    func testChangesRanksOnlyWhenSomeStageIsNonZero() {
        XCTAssertTrue(ComboMoveRules.changesRanks(ComboStub.boostMove))
        XCTAssertTrue(ComboMoveRules.changesRanks(ComboStub.slowDownMove), "相手のランクを変える効果も「変わる」")
        XCTAssertFalse(ComboMoveRules.changesRanks(ComboStub.attackMove), "効果なし")
        XCTAssertFalse(ComboMoveRules.changesRanks(ComboStub.zeroEffectMove), "stages がすべて 0 は「変わらない」")
    }

    func testCanBeStepIsAttackMoveOrRankChangingStatusMove() {
        XCTAssertTrue(ComboMoveRules.canBeStep(ComboStub.attackMove))
        XCTAssertTrue(ComboMoveRules.canBeStep(ComboStub.specialMove))
        XCTAssertTrue(ComboMoveRules.canBeStep(ComboStub.boostMove))
        XCTAssertFalse(ComboMoveRules.canBeStep(ComboStub.plainStatusMove), "ランク変化の無い変化技は置けない(API は 400)")
        XCTAssertFalse(ComboMoveRules.canBeStep(ComboStub.zeroEffectMove))
    }

    func testCanBeOpponentMoveIsAttackMoveOnly() {
        XCTAssertTrue(ComboMoveRules.canBeOpponentMove(ComboStub.attackMove))
        XCTAssertTrue(ComboMoveRules.canBeOpponentMove(ComboStub.slowDownMove))
        XCTAssertFalse(ComboMoveRules.canBeOpponentMove(ComboStub.boostMove), "変化技は相手の技にできない(API は 400)")
    }

    func testMaxComboStepsMirrorsTheContract() {
        XCTAssertEqual(RequestLimits.maxComboSteps, 3, "api/openapi.yaml ComboRequest.steps.maxItems(check-request-limits.sh でも照合)")
        XCTAssertEqual(ComboText.maxStepsNotice, "手は最大3つまでです")
    }

    func testMoveEffectDefaultsToNil() {
        let move = Move(id: "x", nameJa: "テストわざ", type: .normal, category: .physical, power: 40)
        XCTAssertNil(move.effect, "既存の Move(...) の呼び出しは効果なしのまま")
    }

    // MARK: - ランクのチップ(AC-i7)

    func testRankChipsUseWebWordsAndStatOrder() {
        XCTAssertEqual(
            ComboRankChips.chips(for: ComboStub.boostMove),
            [ComboRankChip(text: "攻撃 +1", direction: .up), ComboRankChip(text: "素早さ +1", direction: .up)])
        XCTAssertEqual(
            ComboRankChips.chips(for: ComboStub.closeMove),
            [ComboRankChip(text: "防御 -1", direction: .down), ComboRankChip(text: "特防 -1", direction: .down)],
            "能力の並びは攻撃・防御・特攻・特防・素早さ。下降は down")
        XCTAssertEqual(
            ComboRankChips.chips(for: ComboStub.slowDownMove), [ComboRankChip(text: "相手の素早さ -1", direction: .down)],
            "相手のランクを変える技は「相手の…」")
        XCTAssertEqual(ComboRankChips.chips(for: ComboStub.attackMove), [])
        XCTAssertEqual(ComboRankChips.chips(for: ComboStub.zeroEffectMove), [])
    }

    func testRankChipsForSpecialStatsAndBigStages() {
        let move = Move(
            id: "x", nameJa: "テストわざ", type: .normal, category: .status, power: 0,
            effect: MoveRankEffect(target: .user, stages: RankBlock(spa: 2, spe: -2)))
        XCTAssertEqual(
            ComboRankChips.chips(for: move),
            [ComboRankChip(text: "特攻 +2", direction: .up), ComboRankChip(text: "素早さ -2", direction: .down)])
    }

    // MARK: - 文言(AC-i7)

    func testKOText() {
        XCTAssertEqual(ComboText.koText(steps: 2, ko: ComboStub.ko(display: 51.6)), "2 手で倒す 51.6%")
        XCTAssertEqual(ComboText.koText(steps: 3, ko: ComboStub.ko(display: 100, guaranteed: true)), "3 手で倒す(確定)")
        XCTAssertEqual(ComboText.koText(steps: 2, ko: ComboStub.ko(display: 0)), "倒せない")
        XCTAssertEqual(ComboText.koText(steps: 2, ko: ComboStub.ko(display: 0.1)), "2 手で倒す 0.1%")
        XCTAssertEqual(ComboText.koText(steps: 2, ko: ComboStub.ko(display: 99.9)), "2 手で倒す 99.9%", "表示の値をそのまま。丸め直さない")
    }

    func testSurviveTextReservesZeroAndHundred() {
        XCTAssertEqual(ComboText.survive(100), "耐える 100%")
        XCTAssertEqual(ComboText.survive(0), "耐える 0%")
        XCTAssertEqual(ComboText.survive(87.5), "耐える 87.5%")
        XCTAssertEqual(ComboText.survive(99.99), "耐える 99.9%", "100% に見えないよう 99.9 に収める")
        XCTAssertEqual(ComboText.survive(0.01), "耐える 0.1%", "0% に見えないよう 0.1 に収める")
        XCTAssertEqual(ComboText.survive(51.5625), "耐える 51.6%")
    }

    func testSpeedLineText() {
        XCTAssertEqual(ComboText.speedLine(ComboStub.line(198)), "197 以下を抜く / 198 は同速 / 199 以上は相手が先")
        // トリックルーム中は engine が向きを逆にして返す: 範囲の向きのまま言葉にする。
        let trickRoom = ComboSpeedLine(
            selfSpeed: 198, selfFirst: ComboSpeedRange(min: 199, max: 999), opponentFirst: ComboSpeedRange(min: 0, max: 197))
        XCTAssertEqual(ComboText.speedLine(trickRoom), "199 以上を抜く / 198 は同速 / 197 以下は相手が先")
        let emptySelf = ComboSpeedLine(
            selfSpeed: 1, selfFirst: ComboSpeedRange(min: 0, max: -1), opponentFirst: ComboSpeedRange(min: 2, max: 999))
        XCTAssertEqual(ComboText.speedLine(emptySelf), "1 は同速 / 2 以上は相手が先", "空の範囲は省く")
    }

    func testOtherResultTexts() {
        XCTAssertEqual(ComboText.total(minPercent: 91.4, maxPercent: 108.6), "合計 91.4\u{301C}108.6%")
        XCTAssertEqual(ComboText.speedSummary(first: 132, last: 198), "素早さ 132 → 198")
        XCTAssertEqual(ComboText.opponentSpeed(169), "相手の素早さ 169")
        XCTAssertEqual(ComboText.actions(min: 1, max: 1), "相手の行動 1回")
        XCTAssertEqual(ComboText.actions(min: 1, max: 2), "相手の行動 1\u{301C}2回")
        XCTAssertEqual(ComboText.perHit(minPercent: 40.2, maxPercent: 47.8), "1回ぶん 40.2\u{301C}47.8%")
        XCTAssertEqual(ComboText.stepOrdinal(0), "1手目")
        XCTAssertEqual(ComboText.critical(1), "急所(2手目)")
        XCTAssertEqual(ComboText.moveUp("テストわざ"), "テストわざを上へ")
        XCTAssertEqual(ComboText.moveDown("テストわざ"), "テストわざを下へ")
        XCTAssertEqual(ComboText.removeStep("テストわざ"), "テストわざを外す")
        XCTAssertEqual(ComboText.orderLabel(.selfFirst), "先に動く")
        XCTAssertEqual(ComboText.orderLabel(.opponentFirst), "相手が先")
        XCTAssertEqual(ComboText.orderLabel(.speedTie), "同速")
        XCTAssertNil(ComboText.orderLabel(.dependsOnOpponentSpeed), "決まらないときは出さない")
    }

    func testFixedTextsMatchWeb() {
        XCTAssertEqual(ComboText.sectionLabel, "連続技")
        XCTAssertEqual(ComboText.addStepButton, "次の技")
        XCTAssertEqual(ComboText.addPickerLabel, "追加の技")
        XCTAssertEqual(ComboText.closeButton, "閉じる")
        XCTAssertEqual(ComboText.mainStepBadge, "上の技欄の技")
        XCTAssertEqual(ComboText.noDamage, "ダメージなし")
        XCTAssertEqual(ComboText.defenderLabel, "相手の振り")
        XCTAssertEqual(ComboText.opponentMoveLabel, "相手の技")
        XCTAssertEqual(ComboText.opponentMoveNone, "選ばない")
        XCTAssertEqual(ComboText.opponentSpeedFromDefender, "相手の素早さを防御側の振りから計算する")
        XCTAssertEqual(ComboText.speedFieldToggle, "素早さの場")
        XCTAssertEqual(ComboText.trickRoom, "トリックルーム")
        XCTAssertEqual(ComboText.selfTailwind, "追い風(自分)")
        XCTAssertEqual(ComboText.opponentTailwind, "追い風(相手)")
        XCTAssertEqual(ComboText.loading, "連続技を計算中…")
        XCTAssertEqual(ComboText.retryButton, "もう一度")
        XCTAssertEqual(ComboText.defenderReset, "選んでいた相手の振りがなくなったので、先頭の振りに戻しました")
        XCTAssertEqual(ComboText.note, "場は全手で同じ状態が続きます。命中は考えません。")
        XCTAssertEqual(ComboText.selfSpeedLegend, "自分の素早さ")
        XCTAssertEqual(ComboText.selfSpeedSPLabel, "素早さのSP")
        XCTAssertEqual(ComboText.selfSpeedNatureLabel, "素早さの性格補正")
        XCTAssertEqual(ComboText.selfSpeedRankLabel, "素早さのランク")
        XCTAssertEqual(ComboText.selfSpeedRankStatName, "素早さ")
        XCTAssertEqual(ComboText.selfSpeedNatureOption(.keep), "計算画面の性格のまま")
        XCTAssertEqual(ComboText.selfSpeedNatureOption(.plus), "素早さ上がる(×1.1)")
        XCTAssertEqual(ComboText.selfSpeedNatureOption(.neutral), "素早さ補正なし")
        XCTAssertEqual(ComboText.selfSpeedNatureOption(.minus), "素早さ下がる(×0.9)")
        XCTAssertEqual(ComboText.selfSpeedNatureNote, "性格を変えると、攻撃・特攻の補正も(両立できる範囲で)この性格に合わせます。")
        // iOS だけの案内(性格を ID で送るため、置き換えた組の性格がデータに無いと送れない。ADR-0535 §7)。
        XCTAssertEqual(ComboText.selfSpeedNatureUnavailable, "選んだ素早さの性格補正に合う性格がデータにありません")
    }

    func testFailureMessageWrapsTheCalcScreenFailureText() {
        let errors: [any Error] = [
            PokeCalcError(code: PokeCalcError.Code.transport, message: "x"),
            PokeCalcError(code: PokeCalcError.Code.decode, message: "x"),
            PokeCalcError(code: "internal_error", message: "boom"),
            CocoaError(.fileNoSuchFile),
        ]
        for error in errors {
            XCTAssertEqual(
                ComboText.failureMessage(for: error), "連続技を計算できませんでした(\(CalcScreenError(error).message))",
                "原因の文言は計算画面と同じ分類(ADR-0534 が入ると ServiceFailureKind の文言)")
        }
    }

    // MARK: - 素早さの基本値の例(AC-i8)

    func testSpeedExampleLinesUseOnlyTheNeutralMaxLineInFasterSlowerTieOrder() {
        let reach = StubSpeed.reach(speed: 180, lines: [
            StubSpeed.reachLine(.uninvested, slower: SpeedBaseSpeedRange(min: 1, max: 200)),
            StubSpeed.reachLine(
                .neutralMax, slower: SpeedBaseSpeedRange(min: 1, max: 117), tie: SpeedBaseSpeedRange(min: 118, max: 119),
                faster: SpeedBaseSpeedRange(min: 120, max: 255),
                fastestSlower: StubSpeed.pokemonC, slowestFaster: StubSpeed.pokemonD),
        ])
        XCTAssertEqual(ComboText.speedExampleLines(reach), [
            "準速 120 族以上には抜かれる(例: テストデンキネズミ)",
            "準速 117 族までは抜ける(例: テストクサネコ)",
            "準速 118\u{301C}119 族とは同速",
        ])
    }

    func testSpeedExampleLinesOmitEmptyRangesAndMissingNames() {
        let reach = StubSpeed.reach(speed: 999, lines: [
            StubSpeed.reachLine(.neutralMax, slower: SpeedBaseSpeedRange(min: 1, max: 255), tie: SpeedBaseSpeedRange(min: 90, max: 90)),
        ])
        XCTAssertEqual(ComboText.speedExampleLines(reach), ["準速 255 族までは抜ける", "準速 90 族とは同速"])
        XCTAssertEqual(ComboText.speedExampleLines(StubSpeed.reach(speed: 1, lines: [StubSpeed.reachLine(.max)])), [], "準速の行が無ければ空")
        XCTAssertEqual(ComboText.speedExampleListLabel, "素早さの基本値の例")
    }

    // MARK: - 自分の素早さ(AC-i6)

    func testMaxSelfSpeedSPIsLimitedByPerStatAndTotal() {
        XCTAssertEqual(ComboSelfSpeed.maxSP(for: ComboStub.individual(sp: StatBlock(hp: 0, atk: 0, def: 0, spa: 0, spd: 0, spe: 0))), 32)
        XCTAssertEqual(ComboSelfSpeed.maxSP(for: ComboStub.individual(sp: StatBlock(hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 32))), 32)
        XCTAssertEqual(ComboSelfSpeed.maxSP(for: ComboStub.individual(sp: StatBlock(hp: 32, atk: 32, def: 0, spa: 0, spd: 0, spe: 0))), 2)
        XCTAssertEqual(ComboSelfSpeed.maxSP(for: ComboStub.individual(sp: StatBlock(hp: 32, atk: 32, def: 2, spa: 0, spd: 0, spe: 0))), 0)
        let attacker = ComboStub.individual(sp: StatBlock(hp: 32, atk: 32, def: 0, spa: 0, spd: 0, spe: 0))
        XCTAssertEqual(ComboSelfSpeed.clampSP(10, for: attacker), 2)
        XCTAssertEqual(ComboSelfSpeed.clampSP(-3, for: attacker), 0)
    }

    func testDefaultInputReturnsTheSameIndividual() throws {
        let attacker = ComboStub.individual(ranks: RankBlock(atk: 2, spe: 1))
        XCTAssertEqual(ComboSelfSpeedInput.default, ComboSelfSpeedInput(sp: nil, nature: .keep, rank: nil))
        XCTAssertEqual(try ComboSelfSpeed.apply(.default, to: attacker, natures: ComboStub.natures), attacker, "既定は今の挙動と同一")
    }

    func testSPAndRankOverrideOnlyTheSpeedFields() throws {
        let attacker = ComboStub.individual(ranks: RankBlock(atk: 2))
        let applied = try ComboSelfSpeed.apply(
            ComboSelfSpeedInput(sp: 10, nature: .keep, rank: -1), to: attacker, natures: ComboStub.natures)
        XCTAssertEqual(applied.sp, StatBlock(hp: 2, atk: 32, def: 0, spa: 0, spd: 0, spe: 10))
        XCTAssertEqual(applied.ranks, RankBlock(atk: 2, spe: -1), "素早さ以外のランクは残す")
        XCTAssertEqual(applied.natureId, attacker.natureId)
        let clamped = try ComboSelfSpeed.apply(
            ComboSelfSpeedInput(sp: 40, nature: .keep, rank: 9), to: attacker, natures: ComboStub.natures)
        XCTAssertEqual(clamped.sp.spe, 32)
        XCTAssertEqual(clamped.ranks.spe, 6)
    }

    /// 性格は素早さの補正だけを差し替える(Web の applySpeedNature と同じ規則)。同じ組が複数あれば ID の昇順の最初。
    func testNatureReplacementRules() throws {
        let cases: [(from: Nature, choice: ComboSelfSpeedNature, expected: Nature, note: String)] = [
            (ComboStub.atkUp, .plus, ComboStub.speUpSpaDown, "攻撃↑特攻↓ に素早さ↑ → 素早さ↑特攻↓"),
            (ComboStub.spaUp, .plus, ComboStub.speUpAtkDown, "特攻↑攻撃↓ に素早さ↑ → 素早さ↑攻撃↓"),
            (ComboStub.neutral, .plus, ComboStub.speUpSpaDown, "無補正に素早さ↑ → 下降は特攻"),
            (ComboStub.atkUpSpeDown, .plus, ComboStub.speUpSpaDown, "元の下降が素早さなら代わりの下降(攻撃↑の元なら特攻)"),
            (ComboStub.atkUp, .minus, ComboStub.atkUpSpeDown, "攻撃↑特攻↓ に素早さ↓ → 攻撃↑素早さ↓"),
            (ComboStub.neutral, .minus, ComboStub.defUpSpeDown, "無補正に素早さ↓ → 上昇は防御"),
            (ComboStub.speUpSpaDown, .minus, ComboStub.defUpSpeDown, "元の上昇が素早さなら防御↑"),
            (ComboStub.speUpSpaDown, .neutral, ComboStub.neutral, "素早さを含む性格の補正なし → 無補正(ID の昇順の最初)"),
            (ComboStub.atkUp, .neutral, ComboStub.atkUp, "素早さを含まない性格の補正なし → そのまま"),
        ]
        for testCase in cases {
            let attacker = ComboStub.individual(natureId: testCase.from.id)
            let applied = try ComboSelfSpeed.apply(
                ComboSelfSpeedInput(sp: nil, nature: testCase.choice, rank: nil), to: attacker, natures: ComboStub.natures)
            XCTAssertEqual(applied.natureId, testCase.expected.id, testCase.note)
            XCTAssertEqual(applied.sp, attacker.sp, "SP は触らない: \(testCase.note)")
        }
    }

    func testNatureReplacementThrowsWhenNoMatchingNature() {
        let attacker = ComboStub.individual(natureId: ComboStub.atkUp.id)
        XCTAssertThrowsError(
            try ComboSelfSpeed.apply(ComboSelfSpeedInput(sp: nil, nature: .plus, rank: nil), to: attacker, natures: [ComboStub.atkUp])
        ) { error in
            XCTAssertEqual((error as? PokeCalcError)?.code, PokeCalcError.Code.natureUnavailable)
        }
        let unknown = ComboStub.individual(natureId: "test-nature-not-in-master")
        XCTAssertThrowsError(
            try ComboSelfSpeed.apply(ComboSelfSpeedInput(sp: nil, nature: .minus, rank: nil), to: unknown, natures: ComboStub.natures))
    }
}
