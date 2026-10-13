import XCTest

@testable import PokeCalcCore

/// G-05 第 4 弾(ADR-0540): 調整画面の新しい部品の判定(− / + の可否・横棒・持ち物の印・「選んだ相手から」)。
/// 画面のコードにロジックを持たせない(ADR-0500 §1)ので、判定は `PokeCalcCore` で確かめる。
final class G05AdjustNumberStepTests: XCTestCase {
    private let sp = AdjustNumberStep.spRange

    func testSPRangeIsZeroToThirtyTwo() {
        XCTAssertEqual(AdjustNumberStep.spRange, 0...SPLimits.maxPerStat)
        XCTAssertEqual(AdjustNumberStep.spRange.upperBound, 32)
    }

    func testClampedReadsAsciiDigitsOnly() {
        XCTAssertEqual(AdjustNumberStep.clamped(text: "12", range: sp), 12)
        XCTAssertEqual(AdjustNumberStep.clamped(text: "", range: sp), 0, "空は 0")
        XCTAssertEqual(AdjustNumberStep.clamped(text: "abc", range: sp), 0, "読めない文字は 0")
        XCTAssertEqual(AdjustNumberStep.clamped(text: "１２", range: sp), 0, "全角の数字は読まない(ASCII の数字だけ)")
        XCTAssertEqual(AdjustNumberStep.clamped(text: "-3", range: sp), 0, "負の数は 0 に丸める")
        XCTAssertEqual(AdjustNumberStep.clamped(text: "40", range: sp), 32, "上限を超えたら上限")
        XCTAssertEqual(AdjustNumberStep.clamped(text: "99999999999999999999", range: sp), 32, "桁あふれでも落ちずに上限")
        XCTAssertEqual(AdjustNumberStep.clamped(text: "0", range: 1...10), 1, "下端が 1 の欄(発数)は 1 に丸める")
    }

    func testButtonsStopAtTheEnds() {
        XCTAssertFalse(AdjustNumberStep.canDecrease(text: "0", range: sp), "0 で − は押せない")
        XCTAssertFalse(AdjustNumberStep.canDecrease(text: "", range: sp), "空は 0 と同じ")
        XCTAssertTrue(AdjustNumberStep.canIncrease(text: "", range: sp))
        XCTAssertTrue(AdjustNumberStep.canDecrease(text: "1", range: sp))
        XCTAssertTrue(AdjustNumberStep.canIncrease(text: "31", range: sp))
        XCTAssertFalse(AdjustNumberStep.canIncrease(text: "32", range: sp), "32 で + は押せない")
        XCTAssertFalse(AdjustNumberStep.canIncrease(text: "40", range: sp), "範囲外の文字は上端として扱う")
        XCTAssertTrue(AdjustNumberStep.canDecrease(text: "40", range: sp))
        XCTAssertFalse(AdjustNumberStep.canDecrease(text: "1", range: 1...10))
        XCTAssertFalse(AdjustNumberStep.canIncrease(text: "10", range: 1...10))
    }

    func testSteppedReturnsClampedText() {
        XCTAssertEqual(AdjustNumberStep.stepped(text: "", delta: 1, range: sp), "1")
        XCTAssertEqual(AdjustNumberStep.stepped(text: "5", delta: 1, range: sp), "6")
        XCTAssertEqual(AdjustNumberStep.stepped(text: "5", delta: -1, range: sp), "4")
        XCTAssertEqual(AdjustNumberStep.stepped(text: "0", delta: -1, range: sp), "0", "下端を割らない")
        XCTAssertEqual(AdjustNumberStep.stepped(text: "32", delta: 1, range: sp), "32", "上端を超えない")
        XCTAssertEqual(AdjustNumberStep.stepped(text: "40", delta: -1, range: sp), "31", "範囲外は上端に丸めてから動かす")
        XCTAssertEqual(AdjustNumberStep.stepped(text: "abc", delta: 1, range: sp), "1")
    }
}

final class G05AdjustSPBarTests: XCTestCase {
    func testFractionIsSPOverThirtyTwoClamped() {
        XCTAssertEqual(AdjustSPBar.fraction(sp: 0), 0)
        XCTAssertEqual(AdjustSPBar.fraction(sp: 16), 0.5, accuracy: 1e-9)
        XCTAssertEqual(AdjustSPBar.fraction(sp: 32), 1)
        XCTAssertEqual(AdjustSPBar.fraction(sp: 40), 1, "範囲外は丸める")
        XCTAssertEqual(AdjustSPBar.fraction(sp: -4), 0)
    }
}

@MainActor
final class G05AdjustViewModelMarksTests: XCTestCase {
    private typealias Support = AdjustGoalsTestSupport
    private typealias Mega = StubMegaMaster

    // MARK: - 持ち物の印(ポケモンカードに出す)

    func testOwnItemMarkFollowsSelectionAndMegaStone() async {
        let viewModel = await Support.makeFixture().viewModel
        XCTAssertNil(viewModel.ownItemMark, "持ち物を選ぶまで印は無い")
        viewModel.selectOwnItem(id: Mega.both.id)
        XCTAssertEqual(viewModel.ownItemMark, Mega.both.nameJa)
        await viewModel.selectOwnSpecies(key: Mega.megaAlpha.key)
        XCTAssertEqual(viewModel.ownItemMark, Mega.megaAlphaStoneName, "メガ種族は固定のストーンの名前")
        await viewModel.selectOwnSpecies(key: StubMaster.beta.key)
        XCTAssertNil(viewModel.ownItemMark, "メガでなくなるとストーンを残さない")
    }

    func testOpponentItemMarkIsOnlyTheMegaStone() async {
        let viewModel = await Support.makeFixture().viewModel
        XCTAssertNil(viewModel.opponentItemMark)
        await viewModel.selectOpponentSpecies(key: StubMaster.gamma.key)
        XCTAssertNil(viewModel.opponentItemMark, "相手には持ち物を選ばせない。メガ以外は印なし")
        await viewModel.selectOpponentSpecies(key: Mega.megaAlpha.key)
        XCTAssertEqual(viewModel.opponentItemMark, Mega.megaAlphaStoneName)
        await viewModel.selectOpponentSpecies(key: Mega.megaMissingStone.key)
        XCTAssertNil(viewModel.opponentItemMark, "ストーンが見つからないメガは印なし")
    }

    func testGoalOpponentItemMarkIsOnlyTheMegaStone() async {
        let viewModel = await Support.makeFixture().viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        let none = await Support.addGoal(viewModel)
        let normal = await Support.addGoal(viewModel, opponent: StubMaster.gamma.key)
        let mega = await Support.addGoal(viewModel, opponent: Mega.megaAlpha.key)
        func draft(_ id: Int) -> AdjustGoalDraft { viewModel.goalDrafts.first { $0.id == id }! }
        XCTAssertNil(viewModel.goalOpponentItemMark(draft(none)), "相手を選ぶまで印は無い")
        XCTAssertNil(viewModel.goalOpponentItemMark(draft(normal)))
        XCTAssertEqual(viewModel.goalOpponentItemMark(draft(mega)), Mega.megaAlphaStoneName)
    }

    // MARK: - 「選んだ相手から」(相手の領域 → 各目標の順・重ならない)

    func testRecentOpponentsAreUniqueInOrderOfChoice() async {
        let viewModel = await Support.makeFixture().viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        XCTAssertEqual(viewModel.recentOpponents.map(\.key), [], "まだ誰も選んでいない")
        await viewModel.selectOpponentSpecies(key: StubMaster.gamma.key)
        let first = await Support.addGoal(viewModel, opponent: Mega.megaAlpha.key)
        await Support.addGoal(viewModel, opponent: StubMaster.gamma.key)
        await Support.addGoal(viewModel)
        XCTAssertEqual(
            viewModel.recentOpponents.map(\.key), [StubMaster.gamma.key, Mega.megaAlpha.key],
            "相手の領域の相手が先、同じ種族は 1 つ、未選択の目標は除く")
        viewModel.removeGoal(id: first)
        XCTAssertEqual(viewModel.recentOpponents.map(\.key), [StubMaster.gamma.key], "外した目標の相手は消える")
    }

    func testRecentOpponentsWorkWithoutOpponentRegion() async {
        let viewModel = await Support.makeFixture().viewModel
        await Support.selectOwnAndGoalsMode(viewModel)
        await Support.addGoal(viewModel, opponent: StubMaster.beta.key)
        XCTAssertEqual(viewModel.recentOpponents.map(\.key), [StubMaster.beta.key])
    }
}
