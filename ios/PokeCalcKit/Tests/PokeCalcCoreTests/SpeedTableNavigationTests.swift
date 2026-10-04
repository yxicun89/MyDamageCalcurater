import XCTest

@testable import PokeCalcCore

/// 素早さ表の「自分の位置」の純粋な計算(I-speed-2 / F-06。ADR-0517)。
/// 遅延描画では自分の行が画面外にあるので、位置・ジャンプの向き・読み上げを行の並びだけから決める。
final class SpeedTableNavigationTests: XCTestCase {
    private func tier(_ speed: Int, isSelf: Bool = false) -> SpeedTableRow {
        .tier(SpeedTierDisplay(speed: speed, speedLabel: "素早さ \(speed)", isTie: false, isSelf: isSelf, entries: []))
    }

    // MARK: - 自分の行

    func testSelfRowIsTheHighlightedTier() {
        let rows = [tier(300), tier(250, isSelf: true), tier(200)]
        XCTAssertEqual(SpeedTableNavigation.selfRowIndex(in: rows), 1)
    }

    func testSelfRowIsTheBoundaryWhenNoTierMatches() {
        let rows = [tier(300), .selfBoundary, tier(200)]
        XCTAssertEqual(SpeedTableNavigation.selfRowIndex(in: rows), 1)
    }

    func testSelfRowIsNilWithoutAPosition() {
        XCTAssertNil(SpeedTableNavigation.selfRowIndex(in: [tier(300), tier(200)]))
        XCTAssertNil(SpeedTableNavigation.selfRowIndex(in: []))
    }

    /// トリックルームの昇順でも、表の並びのまま先頭側にいれば 0。
    func testSelfRowWorksForAscendingOrderToo() {
        let rows: [SpeedTableRow] = [.selfBoundary, tier(100), tier(200)]
        XCTAssertEqual(SpeedTableNavigation.selfRowIndex(in: rows), 0)
    }

    // MARK: - ジャンプの向き

    func testDirectionIsVisibleWhenTheSelfRowIsOnScreen() {
        XCTAssertEqual(SpeedTableNavigation.direction(selfIndex: 5, visible: [4, 5, 6]), .visible)
    }

    func testDirectionIsAboveWhenEveryVisibleRowIsBelowSelf() {
        XCTAssertEqual(SpeedTableNavigation.direction(selfIndex: 2, visible: [7, 8]), .above)
    }

    func testDirectionIsBelowWhenEveryVisibleRowIsAboveSelf() {
        XCTAssertEqual(SpeedTableNavigation.direction(selfIndex: 20, visible: [3, 4]), .below)
    }

    func testDirectionIsNilWithoutSelfRow() {
        XCTAssertNil(SpeedTableNavigation.direction(selfIndex: nil, visible: [1]))
    }

    /// まだ何も描かれていない間は向きを決めない(ボタンを出さない)。
    func testDirectionIsVisibleWhileNothingIsRenderedYet() {
        XCTAssertEqual(SpeedTableNavigation.direction(selfIndex: 3, visible: []), .visible)
    }

    // MARK: - 位置の読み上げ

    func testSummaryForASelfTier() {
        let rows = [tier(300), tier(250, isSelf: true), tier(200)]
        XCTAssertEqual(SpeedTableNavigation.summary(rows: rows), "全3段・自分は2段目")
    }

    func testSummaryForABoundaryBetweenTiers() {
        let rows = [tier(300), .selfBoundary, tier(200)]
        XCTAssertEqual(SpeedTableNavigation.summary(rows: rows), "全2段・自分は1段目と2段目の間")
    }

    func testSummaryForABoundaryAtTheEnds() {
        XCTAssertEqual(SpeedTableNavigation.summary(rows: [.selfBoundary, tier(300)]), "全1段・自分は1段目の前")
        XCTAssertEqual(SpeedTableNavigation.summary(rows: [tier(300), tier(200), .selfBoundary]), "全2段・自分は2段目の後")
    }

    func testSummaryWithoutSelfIsOnlyTheTotal() {
        XCTAssertEqual(SpeedTableNavigation.summary(rows: [tier(300), tier(200)]), "全2段")
    }

    func testSummaryIsNilForAnEmptyTable() {
        XCTAssertNil(SpeedTableNavigation.summary(rows: []))
    }

    func testJumpLabelsFollowTheDirection() {
        XCTAssertEqual(SpeedTableNavigation.jumpLabel(.above), "自分の位置へ(上)")
        XCTAssertEqual(SpeedTableNavigation.jumpLabel(.below), "自分の位置へ(下)")
        XCTAssertEqual(SpeedTableNavigation.jumpLabel(.visible), "自分の位置へ")
    }
}
