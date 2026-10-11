import XCTest

@testable import PokeCalcCore

/// `SpeedMatchupDisplayBuilder`(I-speed-4c。ADR-0610 §1・§4): 応答を画面に出す形に整える規則。計算はしない。
/// 文言そのものは `SpeedMatchupLabelsTests` が固定するので、ここでは「どの文言を選ぶか・何を出すか・順序」を見る。
final class SpeedMatchupDisplayTests: XCTestCase {
    private func range(_ min: Int, _ max: Int) -> SpeedSPRange { SpeedSPRange(min: min, max: max) }

    private func natures(
        minus: (SpeedSPRange?, SpeedSPRange?) = (nil, nil), neutral: (SpeedSPRange?, SpeedSPRange?) = (nil, nil),
        plus: (SpeedSPRange?, SpeedSPRange?) = (nil, nil)
    ) -> [SpeedNatureLine] {
        [
            SpeedNatureLine(nature: .minus, movesFirst: minus.0, tie: minus.1),
            SpeedNatureLine(nature: .neutral, movesFirst: neutral.0, tie: neutral.1),
            SpeedNatureLine(nature: .plus, movesFirst: plus.0, tie: plus.1),
        ]
    }

    // MARK: - カード(トリックルームなし)

    private var normalMatchup: SpeedMatchup {
        StubSpeed.matchup(
            lines: [
                StubSpeed.matchupLine(
                    .preset(.uninvested), speed: 150, current: .selfFirst,
                    natures: natures(neutral: (range(31, 32), nil), plus: (range(18, 32), range(17, 17)))),
                StubSpeed.matchupLine(.preset(.maxScarf), speed: 300, current: .opponentFirst, natures: natures()),
                StubSpeed.matchupLine(
                    .custom(SpeedCustomAssumption(sp: 20, nature: .plus)), speed: 167, current: .speedTie,
                    natures: natures(neutral: (nil, range(32, 32)))),
            ],
            selfSpeed: 167)
    }

    func testCardsKeepTheResponseOrderAndIdentifyThemByPresetOrCustomIndex() {
        let cards = SpeedMatchupDisplayBuilder.cards(normalMatchup)
        XCTAssertEqual(cards.map(\.id), ["uninvested", "max-scarf", "custom-0"])
        XCTAssertEqual(cards.map(\.assumptionLabel), ["無振り", "最速スカーフ", "カスタム(S20・上昇)"])
        XCTAssertEqual(cards.map(\.opponentSpeedLabel), ["相手の素早さ 150", "相手の素早さ 300", "相手の素早さ 167"])
    }

    func testCustomAssumptionsAreNumberedAmongThemselves() {
        let matchup = StubSpeed.matchup(
            lines: [
                StubSpeed.matchupLine(.preset(.max), speed: 200),
                StubSpeed.matchupLine(.custom(SpeedCustomAssumption(sp: 0, nature: .minus)), speed: 100),
                StubSpeed.matchupLine(.custom(SpeedCustomAssumption(sp: 4, nature: .neutral)), speed: 110),
            ])
        XCTAssertEqual(SpeedMatchupDisplayBuilder.cards(matchup).map(\.id), ["max", "custom-0", "custom-1"])
    }

    func testRowsAreNeutralAndPlusOnlyWithoutTrickRoom() {
        guard let card = SpeedMatchupDisplayBuilder.cards(normalMatchup)[safe: 0] else { return XCTFail("card が無い") }
        XCTAssertEqual(card.rows.map(\.id), ["neutral", "plus"], "下降の行はトリックルーム中だけ(ADR-0610 §6 Q5)")
        XCTAssertEqual(card.rows.map(\.natureLabel), ["補正なし", "上昇"])
        XCTAssertEqual(card.rows.map(\.text), ["S31以上で抜ける", "S18以上で抜ける"])
        XCTAssertEqual(card.rows.map(\.isReachable), [true, true])
        XCTAssertEqual(card.rows.map(\.tieText), [nil, "S17で同速"], "同速は最低ラインと別に出す")
        XCTAssertEqual(card.headline, "準速で抜ける")
    }

    func testUnreachableCardSaysSoInWordsOnEveryRow() {
        guard let card = SpeedMatchupDisplayBuilder.cards(normalMatchup)[safe: 1] else { return XCTFail("card が無い") }
        XCTAssertEqual(card.headline, "最速でも届かない")
        XCTAssertEqual(card.rows.map(\.text), ["届かない", "届かない"])
        XCTAssertEqual(card.rows.map(\.isReachable), [false, false], "届かないことを色だけで伝えないための判定")
        XCTAssertEqual(card.rows.map(\.tieText), [nil, nil])
    }

    /// 上を取れる SP は無いが、同速になる SP はある行(上を取るに同速は含めない。ADR-0610 §1)。
    func testTieOnlyRowIsNotCountedAsMovingFirst() {
        guard let card = SpeedMatchupDisplayBuilder.cards(normalMatchup)[safe: 2] else { return XCTFail("card が無い") }
        guard let neutral = card.rows[safe: 0] else { return XCTFail("行が無い") }
        XCTAssertEqual(neutral.id, "neutral")
        XCTAssertEqual(neutral.text, "届かない")
        XCTAssertFalse(neutral.isReachable)
        XCTAssertEqual(neutral.tieText, "S32で同速")
        XCTAssertEqual(card.headline, "最速でも届かない", "同速だけでは「抜ける」と言わない")
    }

    func testOrderBadgeComesFromTheCurrentTurnOrder() {
        let cards = SpeedMatchupDisplayBuilder.cards(normalMatchup)
        XCTAssertEqual(cards.map(\.order), [.selfFirst, .opponentFirst, .speedTie])
        XCTAssertEqual(cards.map(\.orderBadge), ["先に動く", "後に動く", "同速"])
    }

    func testNoCurrentMeansNoOrderBadge() {
        let matchup = StubSpeed.matchup(lines: [StubSpeed.matchupLine(.preset(.max), speed: 200)])
        guard let card = SpeedMatchupDisplayBuilder.cards(matchup)[safe: 0] else { return XCTFail("card が無い") }
        XCTAssertNil(card.order)
        XCTAssertNil(card.orderBadge)
    }

    func testNoLinesMeansNoCards() {
        XCTAssertEqual(SpeedMatchupDisplayBuilder.cards(StubSpeed.matchup(lines: [])), [])
    }

    // MARK: - カード(トリックルーム中)

    func testTrickRoomCardsShowTheMinusRowAndTheUpperLimit() {
        let matchup = StubSpeed.matchup(
            lines: [
                StubSpeed.matchupLine(
                    .preset(.max), speed: 200,
                    natures: natures(minus: (range(0, 5), nil), neutral: (range(0, 2), nil), plus: (range(0, 0), nil))),
                StubSpeed.matchupLine(.preset(.uninvested), speed: 150, natures: natures()),
            ],
            trickRoom: true)
        let cards = SpeedMatchupDisplayBuilder.cards(matchup)
        XCTAssertEqual(cards[safe: 0]?.rows.map(\.id), ["minus", "neutral", "plus"], "下降の行はトリックルーム中だけ出る。順は 下降・補正なし・上昇")
        XCTAssertEqual(cards[safe: 0]?.rows.map(\.text), ["S5まで下を取れる", "S2まで下を取れる", "S0まで下を取れる"])
        XCTAssertEqual(cards[safe: 0]?.headline, "補正なしなら S2まで下を取れる")
        XCTAssertEqual(cards[safe: 1]?.headline, "どう振っても下を取れない")
        XCTAssertEqual(cards[safe: 1]?.rows.map(\.text), ["下を取れない", "下を取れない", "下を取れない"])
        XCTAssertEqual(cards[safe: 1]?.rows.map(\.isReachable), [false, false, false])
    }

    // MARK: - 「これ以上」

    private var reachResponse: SpeedReach {
        SpeedReach(
            speed: 170,
            pokemon: nil,
            lines: [
                StubSpeed.reachLine(
                    .uninvested, slower: SpeedBaseSpeedRange(min: 1, max: 149), tie: SpeedBaseSpeedRange(min: 150, max: 150),
                    faster: SpeedBaseSpeedRange(min: 151, max: 255), counts: (3, 1, 2),
                    fastestSlower: StubSpeed.pokemonA, slowestFaster: StubSpeed.pokemonD),
                StubSpeed.reachLine(
                    .neutralMax, slower: SpeedBaseSpeedRange(min: 1, max: 117), faster: SpeedBaseSpeedRange(min: 118, max: 255),
                    counts: (4, 0, 0)),
                StubSpeed.reachLine(.max),
                StubSpeed.reachLine(.maxScarf),
                StubSpeed.reachLine(.maxPlus1),
                StubSpeed.reachLine(.maxPlus2),
            ])
    }

    func testReachRowsKeepThePresetOrderAndSayHowManyAndWho() {
        let rows = SpeedMatchupDisplayBuilder.reachRows(reachResponse, trickRoom: false)
        XCTAssertEqual(rows.map(\.id), SpeedPresetID.allCases.map(\.rawValue), "6 行・表のプリセットの順")
        XCTAssertEqual(rows.map(\.presetLabel), ["無振り", "準速", "最速", "最速スカーフ", "最速+1", "最速+2"])
        guard let first = rows[safe: 0] else { return XCTFail("行が無い") }
        XCTAssertEqual(first.slowerText, "無振り 149族まで抜ける")
        XCTAssertEqual(first.tieText, "無振り 150族と同速")
        XCTAssertEqual(first.fasterText, "無振り 151族以上に抜かれる")
        XCTAssertEqual([first.slowerCountLabel, first.tieCountLabel, first.fasterCountLabel], ["3体", "1体", "2体"])
        XCTAssertEqual(first.slowerExample, "例: テストカソウドリ")
        XCTAssertEqual(first.fasterExample, "例: テストデンキネズミ")
    }

    /// 要望の「準速○○族以上」: 準速の faster の最小がそのまま「○○族以上に抜かれる」になる。
    func testNeutralMaxRowReadsLikeTheRequestedPhrase() {
        guard let row = SpeedMatchupDisplayBuilder.reachRows(reachResponse, trickRoom: false)[safe: 1] else { return XCTFail("row が無い") }
        XCTAssertEqual(row.slowerText, "準速 117族まで抜ける")
        XCTAssertNil(row.tieText, "同速の族が無ければ出さない")
        XCTAssertEqual(row.fasterText, "準速 118族以上に抜かれる")
        XCTAssertNil(row.slowerExample)
        XCTAssertNil(row.fasterExample)
    }

    func testReachRowWithNoGroupsSaysSoInWords() {
        guard let row = SpeedMatchupDisplayBuilder.reachRows(reachResponse, trickRoom: false)[safe: 2] else { return XCTFail("row が無い") }
        XCTAssertEqual(row.slowerText, "最速 はどの族も抜けない")
        XCTAssertNil(row.tieText)
        XCTAssertEqual(row.fasterText, "最速 に抜かれる族なし")
    }

    /// reach の応答はトリックルームに依存しない。言い換えだけを画面側で切り替える(ADR-0610 §1)。
    func testReachRowsReadTheOtherWayUnderTrickRoom() {
        let rows = SpeedMatchupDisplayBuilder.reachRows(reachResponse, trickRoom: true)
        XCTAssertEqual(rows[safe: 0]?.slowerText, "無振り 149族までは後に動く")
        XCTAssertEqual(rows[safe: 0]?.tieText, "無振り 150族と同速", "同速はトリックルームでも同速")
        XCTAssertEqual(rows[safe: 0]?.fasterText, "無振り 151族以上には先に動ける")
        XCTAssertEqual(rows[safe: 2]?.slowerText, "最速 より後に動く族なし")
        XCTAssertEqual(rows[safe: 2]?.fasterText, "最速 には先に動けない")
        XCTAssertEqual(rows.map(\.slowerCountLabel), SpeedMatchupDisplayBuilder.reachRows(reachResponse, trickRoom: false).map(\.slowerCountLabel))
    }
}
