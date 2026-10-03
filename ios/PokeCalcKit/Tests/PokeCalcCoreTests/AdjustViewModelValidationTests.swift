import XCTest

@testable import PokeCalcCore

/// AJ7: 送信前の検査(ADR-0502 §5・AC4)。違反なら API を呼ばず、日本語の理由を `alertMessage` に出す。
/// 検査の順は ADR-0502 §5 の表のとおり(先に当たった1つだけを出す)。
@MainActor
final class AdjustViewModelValidationTests: XCTestCase {
    private typealias Support = AdjustTestSupport

    func testOwnSpeciesAndNatureAreRequired() async {
        let fixture = await Support.makeFixture()
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.ownRequired)

        await fixture.viewModel.selectOwnSpecies(key: StubMaster.beta.key)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.ownRequired)
    }

    func testFixedSPMustBeIntegerInRange() async {
        for text in ["33", "-1", "abc", "1.5", " 4"] {
            let fixture = await Support.makeFixture()
            await Support.selectOwnBeta(fixture.viewModel)
            fixture.viewModel.setFixedSPText(text, for: .def)
            await fixture.viewModel.submit()
            await Support.assertRejected(fixture, message: AdjustText.spRangeInvalid)
        }
    }

    func testFixedSPTotalMustNotExceedSixtySix() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.setFixedSPText("32", for: .hp)
        fixture.viewModel.setFixedSPText("32", for: .def)
        fixture.viewModel.setFixedSPText("3", for: .spd)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.spTotalExceeded)
    }

    func testFixedSPTotalOfExactlySixtySixIsAccepted() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.setFixedSPText("32", for: .hp)
        fixture.viewModel.setFixedSPText("32", for: .def)
        fixture.viewModel.setFixedSPText("2", for: .spd)
        await fixture.viewModel.submit()
        XCTAssertNil(fixture.viewModel.alertMessage)
        let calls = await fixture.adjust.calls
        XCTAssertEqual(calls.count, 1)
    }

    func testMinKoRequiresOwnMoveThenOpponent() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.selectMode(.minKo)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.ownMoveRequired)

        fixture.viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.opponentRequired)
    }

    func testMinSurviveRequiresOpponentAndOpponentMove() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.selectMode(.minSurvive)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.opponentRequired)

        await fixture.viewModel.selectOpponentSpecies(key: StubMaster.gamma.key)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.opponentMoveRequired)
    }

    func testBulkGoalRequiresOpponentMoveButNoGoalDoesNot() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnAndOpponent(fixture.viewModel)
        fixture.viewModel.selectMode(.bulk)
        fixture.viewModel.setUseGoal(true)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.opponentMoveRequired)

        fixture.viewModel.setUseGoal(false)
        await fixture.viewModel.submit()
        XCTAssertNil(fixture.viewModel.alertMessage, "目標なしの耐久側は相手が要らない")
    }

    func testCeilingMustNotBeBelowFixedSPForRotatedStats() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.selectMode(.bulk)
        fixture.viewModel.setFixedSPText("20", for: .def)
        fixture.viewModel.setCeiling(10, for: .def)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.ceilingBelowFixed)
    }

    func testCeilingOfStatNotRotatedIsNotChecked() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.selectMode(.bulk)
        fixture.viewModel.setFixedSPText("20", for: .spe)
        fixture.viewModel.setCeiling(10, for: .spe)
        await fixture.viewModel.submit()
        XCTAssertNil(fixture.viewModel.alertMessage, "耐久側は S を回さないので S の上限は見ない")
        let allocation = await fixture.adjust.calls(of: .allocation)
        XCTAssertEqual(allocation.count, 1, "検査を通って送信する")
    }

    func testOffenseGoalRequiresCategoryToMatchOwnMove() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnAndOpponent(fixture.viewModel)
        fixture.viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        fixture.viewModel.selectMode(.offense)
        fixture.viewModel.setUseGoal(true)
        fixture.viewModel.selectOffenseCategory(.special)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.categoryMismatch)
    }

    func testOffenseMinSpeedMustBeNonNegativeInteger() async {
        for text in ["-1", "abc", "1.5"] {
            let fixture = await Support.makeFixture()
            await Support.selectOwnBeta(fixture.viewModel)
            fixture.viewModel.selectMode(.offense)
            fixture.viewModel.setMinSpeedText(text)
            await fixture.viewModel.submit()
            await Support.assertRejected(fixture, message: AdjustText.minSpeedInvalid)
        }
    }

    func testPresetNatureMissingFromMasterIsRejected() async {
        let fixture = await Support.makeFixture(natures: [StubMaster.neutralNature, StubMaster.atkUpNature])
        await Support.selectOwnAndOpponent(fixture.viewModel)
        fixture.viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        fixture.viewModel.selectMode(.minKo)
        fixture.viewModel.selectOpponentDefenderPreset(.full)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.natureNotFound)
    }

    func testFixingTheInputClearsTheMessageOnNextSubmit() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.setFixedSPText("40", for: .hp)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.spRangeInvalid)

        fixture.viewModel.setFixedSPText("32", for: .hp)
        await fixture.viewModel.submit()
        XCTAssertNil(fixture.viewModel.alertMessage)
        XCTAssertNotNil(fixture.viewModel.outcome)
    }

    // MARK: - 検査の順(Web の AdjustScreen.tsx と同じ: 上限 → 素早さ → 目標の必須 → 分類 → 性格)

    func testCeilingBelowFixedWinsOverLaterViolations() async {
        // 上限 < 固定 SP・素早さの目標が不正・目標の自分の技が無い・性格がマスタに無い、が全部重なる。
        let fixture = await Support.makeFixture(natures: [StubMaster.neutralNature])
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.selectMode(.offense)
        fixture.viewModel.setUseGoal(true)
        fixture.viewModel.setFixedSPText("20", for: .atk)
        fixture.viewModel.setCeiling(10, for: .atk)
        fixture.viewModel.setMinSpeedText("abc")
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.ceilingBelowFixed)
    }

    func testMinSpeedInvalidWinsOverGoalRequirements() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.selectMode(.offense)
        fixture.viewModel.setUseGoal(true)
        fixture.viewModel.setMinSpeedText("-1")
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.minSpeedInvalid)
    }

    func testOffenseGoalCategoryMismatchWinsOverMissingOpponentAndNature() async {
        // 相手が未選択・性格がマスタに無い、より前に、分類の不一致を言う。
        let fixture = await Support.makeFixture(natures: [StubMaster.neutralNature])
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.selectOwnMove(id: StubMaster.physicalMove.id)
        fixture.viewModel.selectMode(.offense)
        fixture.viewModel.setUseGoal(true)
        fixture.viewModel.selectOffenseCategory(.special)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.categoryMismatch)
    }

    func testBulkCeilingBelowFixedWinsOverMissingOpponent() async {
        let fixture = await Support.makeFixture()
        await Support.selectOwnBeta(fixture.viewModel)
        fixture.viewModel.selectMode(.bulk)
        fixture.viewModel.setUseGoal(true)
        fixture.viewModel.setFixedSPText("20", for: .def)
        fixture.viewModel.setCeiling(10, for: .def)
        await fixture.viewModel.submit()
        await Support.assertRejected(fixture, message: AdjustText.ceilingBelowFixed)
    }

    // MARK: - 境界

    func testCeilingEqualToFixedSPPasses() async {
        for mode in [AdjustMode.bulk, .offense] {
            let fixture = await Support.makeFixture()
            await Support.selectOwnBeta(fixture.viewModel)
            fixture.viewModel.selectMode(mode)
            let stat: StatKey = mode == .bulk ? .def : .spe
            fixture.viewModel.setFixedSPText("20", for: stat)
            fixture.viewModel.setCeiling(20, for: stat)
            await fixture.viewModel.submit()
            XCTAssertNil(fixture.viewModel.alertMessage, "上限 = 固定 SP は通る(\(mode))")
            let allocation = await fixture.adjust.calls(of: .allocation)
            XCTAssertEqual(allocation.count, 1)
        }
    }
}
