import XCTest

@testable import PokeCalcCore

/// 素早さ画面(相手を選んで上を取る最低ライン)の文言(I-speed-4c。ADR-0610 §1・§4)。Web(I-speed-4b)と同じ語にそろえる。
/// 値を埋め込む文言は例で固定する。見出しは「補正なしの最小 SP → 上昇の最小 SP → 届かない」の順に決まる(ADR-0610 §1 の B)。
final class SpeedMatchupLabelsTests: XCTestCase {
    // MARK: - 定数

    func testHeadingsAndInputLabels() {
        XCTAssertEqual(SpeedMatchupLabels.opponentRegion, "相手のポケモン")
        XCTAssertEqual(SpeedMatchupLabels.assumptionGroup, "相手の振り方の想定")
        XCTAssertEqual(SpeedMatchupLabels.currentGroup, "自分の今の振り")
        XCTAssertEqual(SpeedMatchupLabels.currentToggle, "今の振りも入れる")
        XCTAssertEqual(SpeedMatchupLabels.matchupRegion, "先に動くための最低ライン")
        XCTAssertEqual(SpeedMatchupLabels.matchupRegionTrickRoom, "先に動ける上限(トリックルーム中)")
        XCTAssertEqual(SpeedMatchupLabels.reachRegion, "これ以上")
        XCTAssertEqual(SpeedMatchupLabels.comboLink, "この相手で連続技を計算")
    }

    /// 説明・案内の文は Web の `speedScreenText` と同じ(最低 1 つの注記・各ヒント・空状態の案内)。
    func testHintsMatchTheWebSentences() {
        XCTAssertEqual(SpeedMatchupLabels.assumptionMinimumNotice, "想定は少なくとも1つ選びます")
        XCTAssertEqual(SpeedMatchupLabels.currentHint, "入れると、今の素早さで先に動けるかと、「これ以上」の一覧が出ます")
        XCTAssertEqual(SpeedMatchupLabels.assumptionHint, "相手がどこまで素早さを振っているかは分からないので、いくつか並べて見ます")
        XCTAssertEqual(SpeedMatchupLabels.trickRoomHint, "トリックルーム中は、素早さが低いほうが先に動きます")
        XCTAssertEqual(SpeedMatchupLabels.matchupHint, "自分と相手のポケモンを選ぶと、相手より先に動くために必要な素早さの振り方を表示します")
        XCTAssertEqual(SpeedMatchupLabels.reachHint, "「今の振りも入れる」をオンにすると、ここに「これ以上」の一覧が出ます")
        XCTAssertEqual(SpeedMatchupLabels.matchupLoading, "最低ラインを計算中")
        XCTAssertEqual(SpeedMatchupLabels.reachLoading, "「これ以上」を計算中")
        XCTAssertTrue(SpeedMatchupLabels.reachIntro.contains("「100 族」"), "「族」の言葉の説明を画面に出す")
    }

    func testOrderBadgesUseWordsNotOnlyColor() {
        XCTAssertEqual(SpeedMatchupLabels.orderBadge(.selfFirst), "今の振りなら先に動く")
        XCTAssertEqual(SpeedMatchupLabels.orderBadge(.opponentFirst), "今の振りなら後に動く")
        XCTAssertEqual(SpeedMatchupLabels.orderBadge(.speedTie), "今の振りなら同速")
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
        XCTAssertEqual(SpeedMatchupLabels.firstLine(SpeedSPRange(min: 31, max: 32)), "S31 以上で抜ける")
        XCTAssertEqual(SpeedMatchupLabels.firstLine(SpeedSPRange(min: 0, max: 32)), "振らなくても抜ける")
        XCTAssertEqual(SpeedMatchupLabels.firstLine(nil), "上を取れません")
    }

    func testTrickRoomLine() {
        XCTAssertEqual(SpeedMatchupLabels.trickRoomLine(SpeedSPRange(min: 0, max: 12)), "S12 まで振っても先に動ける")
        XCTAssertEqual(SpeedMatchupLabels.trickRoomLine(SpeedSPRange(min: 0, max: 32)), "S32 まで振っても先に動ける")
        XCTAssertEqual(SpeedMatchupLabels.trickRoomLine(nil), "先に動けません")
    }

    func testTieLine() {
        XCTAssertEqual(SpeedMatchupLabels.tieLine(SpeedSPRange(min: 17, max: 17)), "S17 で同速")
        XCTAssertEqual(SpeedMatchupLabels.tieLine(SpeedSPRange(min: 10, max: 12)), "S10〜12 で同速")
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
            SpeedMatchupLabels.headline(natures: natures(plus: SpeedSPRange(min: 2, max: 32))), "最速なら S2 以上で抜ける")
    }

    func testHeadlineSaysUnreachableWhenNothingReaches() {
        XCTAssertEqual(SpeedMatchupLabels.headline(natures: natures()), "どう振っても上を取れません")
    }

    // MARK: - 見出し(トリックルーム中)

    func testTrickRoomHeadlineSaysMaxWhenEvenTheSpeedUpNatureCanStillMoveAfter() {
        let all = SpeedSPRange(min: 0, max: 32)
        XCTAssertEqual(SpeedMatchupLabels.trickRoomHeadline(natures: natures(minus: all, neutral: all, plus: all)), "最速でも先に動ける")
    }

    func testTrickRoomHeadlineSaysNeutralMaxWhenNeutralReaches32() {
        let all = SpeedSPRange(min: 0, max: 32)
        XCTAssertEqual(
            SpeedMatchupLabels.trickRoomHeadline(natures: natures(minus: all, neutral: all, plus: SpeedSPRange(min: 0, max: 20))),
            "準速でも先に動ける")
    }

    func testTrickRoomHeadlineSaysTheNeutralLimitOtherwise() {
        XCTAssertEqual(
            SpeedMatchupLabels.trickRoomHeadline(
                natures: natures(
                    minus: SpeedSPRange(min: 0, max: 20), neutral: SpeedSPRange(min: 0, max: 12), plus: SpeedSPRange(min: 0, max: 4))),
            "S12 まで振っても先に動ける")
    }

    func testTrickRoomHeadlineFallsBackToTheMinusNatureThenToNothing() {
        XCTAssertEqual(
            SpeedMatchupLabels.trickRoomHeadline(natures: natures(minus: SpeedSPRange(min: 0, max: 5))), "下降なら S5 まで振っても先に動ける")
        XCTAssertEqual(SpeedMatchupLabels.trickRoomHeadline(natures: natures()), "どう振っても先に動けません")
    }

    // MARK: - 「これ以上」

    func testReachSlowerNormal() {
        XCTAssertEqual(
            SpeedMatchupLabels.reachSlower(preset: "最速", range: SpeedBaseSpeedRange(min: 1, max: 102), trickRoom: false),
            "最速 102 族まで抜ける")
        XCTAssertEqual(
            SpeedMatchupLabels.reachSlower(preset: "最速", range: nil, trickRoom: false), "抜ける族はありません")
    }

    func testReachSlowerUnderTrickRoomReadsAsMovingAfter() {
        XCTAssertEqual(
            SpeedMatchupLabels.reachSlower(preset: "最速", range: SpeedBaseSpeedRange(min: 1, max: 102), trickRoom: true),
            "最速 102 族までは後に動く")
        XCTAssertEqual(
            SpeedMatchupLabels.reachSlower(preset: "最速", range: nil, trickRoom: true), "後に動く族はありません")
    }

    func testReachTie() {
        XCTAssertEqual(
            SpeedMatchupLabels.reachTie(preset: "最速", range: SpeedBaseSpeedRange(min: 103, max: 103)), "最速 103 族と同速")
        XCTAssertEqual(
            SpeedMatchupLabels.reachTie(preset: "最速", range: SpeedBaseSpeedRange(min: 10, max: 12)), "最速 10〜12 族と同速")
    }

    func testReachFasterNormal() {
        XCTAssertEqual(
            SpeedMatchupLabels.reachFaster(preset: "準速", range: SpeedBaseSpeedRange(min: 119, max: 255), trickRoom: false),
            "準速 119 族以上に抜かれる")
        XCTAssertEqual(
            SpeedMatchupLabels.reachFaster(preset: "準速", range: nil, trickRoom: false), "抜かれる族はありません")
    }

    func testReachFasterUnderTrickRoomReadsAsMovingFirst() {
        XCTAssertEqual(
            SpeedMatchupLabels.reachFaster(preset: "準速", range: SpeedBaseSpeedRange(min: 119, max: 255), trickRoom: true),
            "準速 119 族以上には先に動ける")
        XCTAssertEqual(
            SpeedMatchupLabels.reachFaster(preset: "準速", range: nil, trickRoom: true), "先に動ける族はありません")
    }

    func testCountAndExample() {
        XCTAssertEqual(SpeedMatchupLabels.count(4), "4体")
        XCTAssertEqual(SpeedMatchupLabels.count(0), "0体")
        XCTAssertEqual(SpeedMatchupLabels.example("テストデンキネズミ"), "例: テストデンキネズミ")
    }
}
