import XCTest

@testable import PokeCalcCore

/// G-05 第 5 弾(ADR-0541)の判定: 攻撃側の SP 欄の − / + と、カードの持ち物の印。
/// SP の規則は Web の `AttackerSPField`(`web/src/screens/CalcScreen.tsx`)と同じ:
/// 空欄は 0・整数でない文字は「0 から数える」・範囲外の整数は端に丸める。欄の不正な値を誤りとして出す仕様(ADR-0518)は変えない。
final class G05CalcLogicTests: XCTestCase {

    // MARK: - SP の − / +(Web と同じ規則)

    func testStopsAtZeroAndThirtyTwo() {
        XCTAssertEqual(AttackerSPStep.minimum, 0)
        XCTAssertEqual(AttackerSPStep.maximum, 32)
        XCTAssertEqual(AttackerSPStep.maximum, SPLimits.maxPerStat)
        XCTAssertFalse(AttackerSPStep.canDecrease(text: "0"), "0 で − は止まる")
        XCTAssertTrue(AttackerSPStep.canIncrease(text: "0"))
        XCTAssertTrue(AttackerSPStep.canDecrease(text: "32"))
        XCTAssertFalse(AttackerSPStep.canIncrease(text: "32"), "32 で + は止まる")
        XCTAssertTrue(AttackerSPStep.canDecrease(text: "1"))
        XCTAssertTrue(AttackerSPStep.canIncrease(text: "31"))
    }

    func testSteppingMovesByOne() {
        XCTAssertEqual(AttackerSPStep.stepped(text: "0", by: 1), "1")
        XCTAssertEqual(AttackerSPStep.stepped(text: "1", by: 1), "2")
        XCTAssertEqual(AttackerSPStep.stepped(text: "32", by: -1), "31")
        XCTAssertEqual(AttackerSPStep.stepped(text: "10", by: -1), "9")
    }

    func testSteppingClampsToRange() {
        XCTAssertEqual(AttackerSPStep.stepped(text: "0", by: -1), "0")
        XCTAssertEqual(AttackerSPStep.stepped(text: "32", by: 1), "32")
    }

    /// 空欄は 0 として扱う(− は止まり、+ で 1)。前後の空白は無視する。
    func testEmptyAndPaddedText() {
        XCTAssertFalse(AttackerSPStep.canDecrease(text: ""))
        XCTAssertTrue(AttackerSPStep.canIncrease(text: ""))
        XCTAssertEqual(AttackerSPStep.stepped(text: "", by: 1), "1")
        XCTAssertEqual(AttackerSPStep.stepped(text: "  ", by: 1), "1")
        XCTAssertEqual(AttackerSPStep.stepped(text: " 5 ", by: 1), "6")
    }

    /// 範囲外の整数: + は押せず(上限以上)、− は 32 に戻す。マイナスは − が押せず、+ で 0 に戻す。
    func testOutOfRangeIntegers() {
        XCTAssertFalse(AttackerSPStep.canIncrease(text: "33"))
        XCTAssertTrue(AttackerSPStep.canDecrease(text: "33"))
        XCTAssertEqual(AttackerSPStep.stepped(text: "33", by: -1), "32")
        XCTAssertEqual(AttackerSPStep.stepped(text: "40", by: -1), "32")
        XCTAssertFalse(AttackerSPStep.canDecrease(text: "-1"))
        XCTAssertTrue(AttackerSPStep.canIncrease(text: "-1"))
        XCTAssertEqual(AttackerSPStep.stepped(text: "-1", by: 1), "0")
    }

    /// 整数でない文字(abc・3.5・全角数字)は両方押せる。押すと 0 から数える(Web の `current = SP_MIN`)。
    func testNonIntegerTextCountsFromZero() {
        for text in ["abc", "3.5", "３"] {
            XCTAssertTrue(AttackerSPStep.canDecrease(text: text), text)
            XCTAssertTrue(AttackerSPStep.canIncrease(text: text), text)
            XCTAssertEqual(AttackerSPStep.stepped(text: text, by: 1), "1", text)
            XCTAssertEqual(AttackerSPStep.stepped(text: text, by: -1), "0", text)
        }
    }

    /// 0 から 32 まで押し続けても、必ず 0〜32 の整数の文字になる。
    func testRepeatedSteppingStaysInRange() {
        var text = "0"
        for _ in 0..<40 { text = AttackerSPStep.stepped(text: text, by: 1) }
        XCTAssertEqual(text, "32")
        for _ in 0..<40 { text = AttackerSPStep.stepped(text: text, by: -1) }
        XCTAssertEqual(text, "0")
    }
}

/// カードの持ち物の印(Web の `PokemonCard` の `itemName`)。選んだ持ち物の名前。メガ種族は固定のストーンの表示名。持ち物なしは nil。
@MainActor
final class G05CalcItemMarkTests: XCTestCase {
    private typealias Mega = StubMegaMaster

    private func loadedViewModel() async -> CalcViewModel {
        let viewModel = CalcViewModel(service: Mega.makeService())
        await viewModel.load()
        return viewModel
    }

    func testAttackerMarkIsNilWithoutItem() async {
        let viewModel = await loadedViewModel()
        XCTAssertNil(viewModel.attackerItemMark, "「なし」なら印を出さない")
    }

    func testAttackerMarkIsTheSelectedItemName() async {
        let viewModel = await loadedViewModel()
        await viewModel.selectAttackerItem(id: Mega.both.id)
        XCTAssertEqual(viewModel.attackerItemMark, Mega.both.nameJa)
        await viewModel.selectAttackerItem(id: nil)
        XCTAssertNil(viewModel.attackerItemMark)
    }

    func testMegaAttackerMarkIsTheFixedStoneName() async {
        let viewModel = await loadedViewModel()
        await viewModel.selectAttacker(speciesKey: Mega.megaAlpha.key)
        XCTAssertEqual(viewModel.attackerItemMark, Mega.megaAlphaStoneName, "固定のストーンの表示名(英字の nameJa を出さない)")
    }

    func testMegaAttackerWithMissingStoneHasNoMark() async {
        let viewModel = await loadedViewModel()
        await viewModel.selectAttacker(speciesKey: Mega.megaMissingStone.key)
        XCTAssertNil(viewModel.attackerItemMark, "ストーンを引けないときは黙って別の持ち物にしない")
    }

    func testDefenderMarkIsOnlyTheMegaStone() async {
        let viewModel = await loadedViewModel()
        XCTAssertNil(viewModel.defenderItemMark, "防御側の持ち物の比較は結果の下のチップ。カードには印を出さない")
        await viewModel.toggleDefenderItemComparison(itemId: Mega.both.id)
        XCTAssertNil(viewModel.defenderItemMark, "比較のチップは印にしない")
        await viewModel.selectDefender(speciesKey: Mega.megaAlpha.key)
        await viewModel.loadDefenderAbilityOptions()
        XCTAssertEqual(viewModel.defenderItemMark, Mega.megaAlphaStoneName)
        await viewModel.selectDefender(speciesKey: StubMaster.gamma.key)
        await viewModel.loadDefenderAbilityOptions()
        XCTAssertNil(viewModel.defenderItemMark, "メガでなくなれば印も消える")
    }
}
