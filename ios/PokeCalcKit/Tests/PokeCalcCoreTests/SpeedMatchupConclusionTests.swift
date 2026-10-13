import XCTest

@testable import PokeCalcCore

/// 素早さ画面の「結論 1 行 + 想定カード + 折りたたみ」(I-ios-25。ADR-0538 / Web I-web-34・ADR-0610 §9)。
/// 文言は `web/src/i18n/speed.ts` の `speedScreenText` が正。値と既存の文言の語は変えない(既存の
/// SpeedMatchupDisplayTests・SpeedMatchupLabelsTests はそのまま通ること)。
final class SpeedMatchupConclusionTests: XCTestCase {
    private func range(_ min: Int, _ max: Int) -> SpeedSPRange { SpeedSPRange(min: min, max: max) }

    // MARK: - 文言(テーブル駆動)

    func testMatchupConclusionWording() {
        struct Row { let reachable: Int; let total: Int; let trickRoom: Bool; let expected: String }
        let table: [Row] = [
            Row(reachable: 3, total: 4, trickRoom: false, expected: "4 つの想定のうち 3 つで上を取れます"),
            Row(reachable: 1, total: 4, trickRoom: false, expected: "4 つの想定のうち 1 つで上を取れます"),
            Row(reachable: 4, total: 4, trickRoom: false, expected: "4 つの想定すべてで上を取れます"),
            Row(reachable: 1, total: 1, trickRoom: false, expected: "1 つの想定すべてで上を取れます"),
            Row(reachable: 0, total: 4, trickRoom: false, expected: "どの想定でも上を取れません"),
            Row(reachable: 2, total: 5, trickRoom: true, expected: "5 つの想定のうち 2 つで先に動けます"),
            Row(reachable: 5, total: 5, trickRoom: true, expected: "5 つの想定すべてで先に動けます"),
            Row(reachable: 0, total: 5, trickRoom: true, expected: "どの想定でも先に動けません"),
        ]
        for row in table {
            XCTAssertEqual(
                SpeedMatchupLabels.matchupConclusion(reachable: row.reachable, total: row.total, trickRoom: row.trickRoom),
                row.expected, "\(row)")
        }
    }

    func testCardHeadingAndSummaryWording() {
        XCTAssertEqual(SpeedMatchupLabels.cardHeading(assumption: "最速", speed: 145), "最速 ・ 素早さ 145")
        XCTAssertEqual(
            SpeedMatchupLabels.cardHeading(assumption: "カスタム(S20・上昇)", speed: 167), "カスタム(S20・上昇) ・ 素早さ 167")
        XCTAssertEqual(SpeedMatchupLabels.naturesSummary, "性格補正ごとの最小 SP")
        XCTAssertEqual(SpeedMatchupLabels.naturesSummaryTrickRoom, "性格補正ごとの上限 SP")
        XCTAssertEqual(SpeedMatchupLabels.reachListSummary(count: 6), "6 つの振り方ごとの一覧を見る")
    }

    // MARK: - 結論の数え方(Display builder)

    private func natures(
        minus: SpeedSPRange? = nil, neutral: SpeedSPRange? = nil, plus: SpeedSPRange? = nil, tie: SpeedSPRange? = nil
    ) -> [SpeedNatureLine] {
        [
            SpeedNatureLine(nature: .minus, movesFirst: minus, tie: nil),
            SpeedNatureLine(nature: .neutral, movesFirst: neutral, tie: tie),
            SpeedNatureLine(nature: .plus, movesFirst: plus, tie: nil),
        ]
    }

    private func matchup(_ lines: [SpeedMatchupLine], trickRoom: Bool = false) -> SpeedMatchup {
        StubSpeed.matchup(lines: lines, trickRoom: trickRoom)
    }

    /// 上を取れる想定 = どれかの性格に movesFirst がある。同速だけの想定は数えない。
    func testConclusionCountsOnlyAssumptionsWhereSomeNatureMovesFirst() {
        let lines = [
            StubSpeed.matchupLine(.preset(.uninvested), speed: 150, natures: natures(neutral: range(31, 32))),
            StubSpeed.matchupLine(.preset(.neutralMax), speed: 170, natures: natures(plus: range(18, 32))),
            StubSpeed.matchupLine(.preset(.max), speed: 200, natures: natures(tie: range(32, 32))),
            StubSpeed.matchupLine(.preset(.maxScarf), speed: 300),
        ]
        XCTAssertEqual(SpeedMatchupDisplayBuilder.matchupConclusion(matchup(lines)), "4 つの想定のうち 2 つで上を取れます")
    }

    func testConclusionAllReachableAndNoneReachable() {
        let all = [
            StubSpeed.matchupLine(.preset(.uninvested), speed: 150, natures: natures(neutral: range(0, 0))),
            StubSpeed.matchupLine(.preset(.max), speed: 200, natures: natures(plus: range(5, 32))),
        ]
        XCTAssertEqual(SpeedMatchupDisplayBuilder.matchupConclusion(matchup(all)), "2 つの想定すべてで上を取れます")
        let none = [StubSpeed.matchupLine(.preset(.max), speed: 200), StubSpeed.matchupLine(.preset(.maxScarf), speed: 300)]
        XCTAssertEqual(SpeedMatchupDisplayBuilder.matchupConclusion(matchup(none)), "どの想定でも上を取れません")
    }

    func testConclusionUnderTrickRoomUsesTheMoveFirstWording() {
        let lines = [
            StubSpeed.matchupLine(.preset(.max), speed: 200, natures: natures(minus: range(0, 5))),
            StubSpeed.matchupLine(.preset(.uninvested), speed: 150),
            StubSpeed.matchupLine(.preset(.maxScarf), speed: 300),
        ]
        XCTAssertEqual(
            SpeedMatchupDisplayBuilder.matchupConclusion(matchup(lines, trickRoom: true)), "3 つの想定のうち 1 つで先に動けます")
        XCTAssertEqual(
            SpeedMatchupDisplayBuilder.matchupConclusion(matchup([lines[1], lines[2]], trickRoom: true)), "どの想定でも先に動けません")
    }

    func testNoAssumptionsMeansNoConclusion() {
        XCTAssertNil(SpeedMatchupDisplayBuilder.matchupConclusion(matchup([])))
    }

    // MARK: - カードの見出し・行は従来のまま

    func testCardKeepsExistingFieldsAndAddsTheHeading() {
        let lines = [
            StubSpeed.matchupLine(
                .preset(.max), speed: 145, current: .selfFirst, natures: natures(neutral: range(3, 32), plus: range(0, 32)))
        ]
        guard let card = SpeedMatchupDisplayBuilder.cards(matchup(lines)).first else { return XCTFail("card が無い") }
        XCTAssertEqual(card.heading, "最速 ・ 素早さ 145")
        XCTAssertEqual(card.assumptionLabel, "最速")
        XCTAssertEqual(card.opponentSpeedLabel, "相手の素早さ 145")
        XCTAssertEqual(card.headline, "準速で抜ける")
        XCTAssertEqual(card.rows.map(\.id), ["neutral", "plus"], "折りたたみの中身は従来の行(下降はトリックルーム中だけ)")
        XCTAssertEqual(card.orderBadge, "今の振りなら先に動く")
    }

    func testCustomCardHeadingUsesTheCustomLabel() {
        let lines = [StubSpeed.matchupLine(.custom(SpeedCustomAssumption(sp: 20, nature: .plus)), speed: 167)]
        XCTAssertEqual(SpeedMatchupDisplayBuilder.cards(matchup(lines)).first?.heading, "カスタム(S20・上昇) ・ 素早さ 167")
    }

    // MARK: - 「これ以上」の結論

    private func reach(maxLine: SpeedReachLine?) -> SpeedReach {
        var lines = [
            StubSpeed.reachLine(.uninvested, slower: SpeedBaseSpeedRange(min: 1, max: 40)),
            StubSpeed.reachLine(.neutralMax, slower: SpeedBaseSpeedRange(min: 1, max: 50)),
        ]
        if let maxLine { lines.append(maxLine) }
        return SpeedReach(speed: 170, pokemon: nil, lines: lines)
    }

    func testReachConclusionIsTheMaxRowSlowerText() {
        let response = reach(
            maxLine: StubSpeed.reachLine(.max, slower: SpeedBaseSpeedRange(min: 1, max: 60), faster: SpeedBaseSpeedRange(min: 61, max: 255)))
        let normal = SpeedMatchupDisplayBuilder.reachRows(response, trickRoom: false)
        XCTAssertEqual(SpeedMatchupDisplayBuilder.reachConclusion(normal), "最速 60 族まで抜ける")
        let trick = SpeedMatchupDisplayBuilder.reachRows(response, trickRoom: true)
        XCTAssertEqual(SpeedMatchupDisplayBuilder.reachConclusion(trick), "最速 60 族までは後に動く")
    }

    func testReachConclusionWhenMaxHasNoGroupsOrIsMissing() {
        let none = SpeedMatchupDisplayBuilder.reachRows(reach(maxLine: StubSpeed.reachLine(.max)), trickRoom: false)
        XCTAssertEqual(SpeedMatchupDisplayBuilder.reachConclusion(none), "抜ける族はありません")
        let missing = SpeedMatchupDisplayBuilder.reachRows(reach(maxLine: nil), trickRoom: false)
        XCTAssertNil(SpeedMatchupDisplayBuilder.reachConclusion(missing), "最速の行が無ければ結論は出さない")
        XCTAssertNil(SpeedMatchupDisplayBuilder.reachConclusion([]))
    }

    // MARK: - 帯の色(タイプ ID。未選択は nil = brandPrimary)

    func testAccentTypeIsTheFirstTypeOrNil() {
        XCTAssertEqual(SpeedMatchupDisplayBuilder.accentTypeID(StubSpeed.pokemonA), "fire", "2 タイプは先頭")
        XCTAssertEqual(SpeedMatchupDisplayBuilder.accentTypeID(StubSpeed.pokemonD), "electric")
        XCTAssertNil(SpeedMatchupDisplayBuilder.accentTypeID(nil), "未選択は nil(画面は brandPrimary)")
        XCTAssertNil(SpeedMatchupDisplayBuilder.accentTypeID(SpeedPokemon(pokemonId: "x", nameJa: "テスト", types: [], baseSpeed: 1)))
    }
}
