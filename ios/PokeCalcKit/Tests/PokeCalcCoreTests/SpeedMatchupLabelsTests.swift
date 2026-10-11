import XCTest

@testable import PokeCalcCore

/// 素早さ画面(相手を選んで上を取る最低ライン)の文言(I-speed-4c。ADR-0610 §1・§4)。Web(I-speed-4b)と同じ語にそろえる。
/// 値を埋め込む文言は例で固定する。見出しは「補正なしの最小 SP → 上昇の最小 SP → 届かない」の順に決まる(ADR-0610 §1 の B)。
final class SpeedMatchupLabelsTests: XCTestCase {
    // MARK: - 定数

    func testHeadingsAndInputLabels() {
        XCTAssertEqual(SpeedMatchupLabels.opponentRegion, "相手のポケモン")
        XCTAssertEqual(SpeedMatchupLabels.assumptionGroup, "相手の振りの想定")
        XCTAssertEqual(SpeedMatchupLabels.currentGroup, "自分の今の振り")
        XCTAssertEqual(SpeedMatchupLabels.currentToggle, "今の振りを入れる")
        XCTAssertEqual(SpeedMatchupLabels.matchupRegion, "上を取る最低ライン")
        XCTAssertEqual(SpeedMatchupLabels.reachRegion, "これ以上")
        XCTAssertEqual(SpeedMatchupLabels.comboLink, "この相手で連続技を計算")
    }

    func testOrderBadgesUseWordsNotOnlyColor() {
        XCTAssertEqual(SpeedMatchupLabels.orderBadge(.selfFirst), "先に動く")
        XCTAssertEqual(SpeedMatchupLabels.orderBadge(.opponentFirst), "後に動く")
        XCTAssertEqual(SpeedMatchupLabels.orderBadge(.speedTie), "同速")
        XCTAssertEqual(Set(SpeedTurnOrder.allCases.map { SpeedMatchupLabels.orderBadge($0) }).count, 3, "3 つとも別の語")
    }

    // MARK: - 値を埋め込む文言

    func testOpponentSpeed() {
        XCTAssertEqual(SpeedMatchupLabels.opponentSpeed(200), "相手の素早さ 200")
    }

    func testCustomAssumptionNameOmitsRankZeroAndNoScarf() {
        XCTAssertEqual(SpeedMatchupLabels.customAssumption(.init(sp: 20, nature: .plus)), "カスタム(S20・上昇)")
        XCTAssertEqual(
            SpeedMatchupLabels.customAssumption(.init(sp: 32, nature: .neutral, rank: 1, scarf: true)),
            "カスタム(S32・補正なし・+1・スカーフ)")
        XCTAssertEqual(SpeedMatchupLabels.customAssumption(.init(sp: 0, nature: .minus, rank: -2)), "カスタム(S0・下降・-2)")
    }

    func testFirstLine() {
        XCTAssertEqual(SpeedMatchupLabels.firstLine(SpeedSPRange(min: 31, max: 32)), "S31以上で抜ける")
        XCTAssertEqual(SpeedMatchupLabels.firstLine(SpeedSPRange(min: 0, max: 32)), "無振りでも抜ける")
        XCTAssertEqual(SpeedMatchupLabels.firstLine(nil), "届かない")
    }

    func testTrickRoomLine() {
        XCTAssertEqual(SpeedMatchupLabels.trickRoomLine(SpeedSPRange(min: 0, max: 12)), "S12まで下を取れる")
        XCTAssertEqual(SpeedMatchupLabels.trickRoomLine(SpeedSPRange(min: 0, max: 32)), "S32まで下を取れる")
        XCTAssertEqual(SpeedMatchupLabels.trickRoomLine(nil), "下を取れない")
    }

    func testTieLine() {
        XCTAssertEqual(SpeedMatchupLabels.tieLine(SpeedSPRange(min: 17, max: 17)), "S17で同速")
        XCTAssertEqual(SpeedMatchupLabels.tieLine(SpeedSPRange(min: 10, max: 12)), "S10〜S12で同速")
    }

    // MARK: - 見出し(トリックルームなし)

    private func natures(
        minus: SpeedSPRange? = nil, neutral: SpeedSPRange? = nil, plus: SpeedSPRange? = nil
    ) -> [SpeedNatureLine] {
        [
            SpeedNatureLine(nature: .minus, movesFirst: minus, tie: nil),
            SpeedNatureLine(nature: .neutral, movesFirst: neutral, tie: nil),
            SpeedNatureLine(nature: .plus, movesFirst: plus, tie: nil),
        ]
    }

    func testHeadlineSaysUninvestedWhenNeutralNeedsNoSP() {
        let all = SpeedSPRange(min: 0, max: 32)
        XCTAssertEqual(SpeedMatchupLabels.headline(natures: natures(minus: all, neutral: all, plus: all)), "無振りで抜ける")
        XCTAssertEqual(
            SpeedMatchupLabels.headline(natures: natures(neutral: all, plus: all)), "無振りで抜ける", "下降は届かなくても補正なしが 0 なら無振り")
    }

    func testHeadlineSaysNeutralMaxWhenNeutralNeedsSomeSP() {
        XCTAssertEqual(
            SpeedMatchupLabels.headline(natures: natures(neutral: SpeedSPRange(min: 31, max: 32), plus: SpeedSPRange(min: 18, max: 32))),
            "準速で抜ける")
        XCTAssertEqual(
            SpeedMatchupLabels.headline(natures: natures(neutral: SpeedSPRange(min: 1, max: 32), plus: SpeedSPRange(min: 0, max: 32))),
            "準速で抜ける", "補正なしの最小が 1 でも「無振り」ではない")
    }

    func testHeadlineSaysMaxWithItsSPWhenOnlyTheSpeedUpNatureReaches() {
        XCTAssertEqual(
            SpeedMatchupLabels.headline(natures: natures(plus: SpeedSPRange(min: 2, max: 32))), "最速なら S2で抜ける")
    }

    func testHeadlineSaysUnreachableWhenNothingReaches() {
        XCTAssertEqual(SpeedMatchupLabels.headline(natures: natures()), "最速でも届かない")
    }

    // MARK: - 見出し(トリックルーム中)

    func testTrickRoomHeadlineSaysMaxWhenEvenTheSpeedUpNatureCanStillMoveAfter() {
        let all = SpeedSPRange(min: 0, max: 32)
        XCTAssertEqual(SpeedMatchupLabels.trickRoomHeadline(natures: natures(minus: all, neutral: all, plus: all)), "最速でも下を取れる")
    }

    func testTrickRoomHeadlineSaysNeutralMaxWhenNeutralReaches32() {
        let all = SpeedSPRange(min: 0, max: 32)
        XCTAssertEqual(
            SpeedMatchupLabels.trickRoomHeadline(natures: natures(minus: all, neutral: all, plus: SpeedSPRange(min: 0, max: 20))),
            "準速でも下を取れる")
    }

    func testTrickRoomHeadlineSaysTheNeutralLimitOtherwise() {
        XCTAssertEqual(
            SpeedMatchupLabels.trickRoomHeadline(
                natures: natures(
                    minus: SpeedSPRange(min: 0, max: 20), neutral: SpeedSPRange(min: 0, max: 12), plus: SpeedSPRange(min: 0, max: 4))),
            "補正なしなら S12まで下を取れる")
    }

    func testTrickRoomHeadlineFallsBackToTheMinusNatureThenToNothing() {
        XCTAssertEqual(
            SpeedMatchupLabels.trickRoomHeadline(natures: natures(minus: SpeedSPRange(min: 0, max: 5))), "下降なら S5まで下を取れる")
        XCTAssertEqual(SpeedMatchupLabels.trickRoomHeadline(natures: natures()), "どう振っても下を取れない")
    }

    // MARK: - 「これ以上」

    func testReachSlowerNormal() {
        XCTAssertEqual(
            SpeedMatchupLabels.reachSlower(preset: "最速", range: SpeedBaseSpeedRange(min: 1, max: 102), trickRoom: false),
            "最速 102族まで抜ける")
        XCTAssertEqual(
            SpeedMatchupLabels.reachSlower(preset: "最速", range: nil, trickRoom: false), "最速 はどの族も抜けない")
    }

    func testReachSlowerUnderTrickRoomReadsAsMovingAfter() {
        XCTAssertEqual(
            SpeedMatchupLabels.reachSlower(preset: "最速", range: SpeedBaseSpeedRange(min: 1, max: 102), trickRoom: true),
            "最速 102族までは後に動く")
        XCTAssertEqual(
            SpeedMatchupLabels.reachSlower(preset: "最速", range: nil, trickRoom: true), "最速 より後に動く族なし")
    }

    func testReachTie() {
        XCTAssertEqual(
            SpeedMatchupLabels.reachTie(preset: "最速", range: SpeedBaseSpeedRange(min: 103, max: 103)), "最速 103族と同速")
        XCTAssertEqual(
            SpeedMatchupLabels.reachTie(preset: "最速", range: SpeedBaseSpeedRange(min: 10, max: 12)), "最速 10〜12族と同速")
    }

    func testReachFasterNormal() {
        XCTAssertEqual(
            SpeedMatchupLabels.reachFaster(preset: "準速", range: SpeedBaseSpeedRange(min: 119, max: 255), trickRoom: false),
            "準速 119族以上に抜かれる")
        XCTAssertEqual(
            SpeedMatchupLabels.reachFaster(preset: "準速", range: nil, trickRoom: false), "準速 に抜かれる族なし")
    }

    func testReachFasterUnderTrickRoomReadsAsMovingFirst() {
        XCTAssertEqual(
            SpeedMatchupLabels.reachFaster(preset: "準速", range: SpeedBaseSpeedRange(min: 119, max: 255), trickRoom: true),
            "準速 119族以上には先に動ける")
        XCTAssertEqual(
            SpeedMatchupLabels.reachFaster(preset: "準速", range: nil, trickRoom: true), "準速 には先に動けない")
    }

    func testCountAndExample() {
        XCTAssertEqual(SpeedMatchupLabels.count(4), "4体")
        XCTAssertEqual(SpeedMatchupLabels.count(0), "0体")
        XCTAssertEqual(SpeedMatchupLabels.example("テストデンキネズミ"), "例: テストデンキネズミ")
    }
}
