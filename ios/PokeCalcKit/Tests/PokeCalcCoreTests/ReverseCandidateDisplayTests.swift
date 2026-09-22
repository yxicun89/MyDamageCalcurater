import XCTest

@testable import PokeCalcCore

/// 逆算候補の整形(P6-2b。純粋関数)。サーバーの値を丸め直さず、並べ替えもしない(ADR-0010 §R4 の順のまま)。
final class ReverseCandidateDisplayTests: XCTestCase {

    private let items = [StubMaster.itemA, StubMaster.itemB]

    private func candidate(
        natureClass: NatureClass = .neutral, itemId: String? = nil, ranges: [SPRange] = [SPRange(min: 20, max: 23)],
        exact: Bool = true, minPercent: Double = 12.3, maxPercent: Double = 15.6
    ) -> ReverseCandidate {
        let spCount = ranges.reduce(0) { $0 + $1.max - $1.min + 1 }
        return ReverseCandidate(
            natureClass: natureClass, nature: NatureModifier(), natureId: nil, itemId: itemId,
            ranges: ranges, spCount: spCount, exact: exact, mismatch: exact ? 0 : 7, support: 1,
            minPercent: minPercent, maxPercent: maxPercent
        )
    }

    // MARK: - 部品

    func testStatLetter() {
        let expected: [StatKey: String] = [.hp: "H", .atk: "A", .def: "B", .spa: "C", .spd: "D", .spe: "S"]
        for stat in StatKey.allCases {
            XCTAssertEqual(ReverseCandidateDisplay.statLetter(stat), expected[stat], "\(stat)")
        }
    }

    func testNatureClassLabel() {
        let cases: [(NatureClass, StatKey, String)] = [
            (.neutral, .def, "補正なし"),
            (.neutral, .atk, "補正なし"),
            (.plus, .def, "B上昇"),
            (.plus, .spd, "D上昇"),
            (.plus, .atk, "A上昇"),
            (.plus, .spa, "C上昇"),
        ]
        for (natureClass, stat, expected) in cases {
            XCTAssertEqual(ReverseCandidateDisplay.natureClassLabel(natureClass, stat: stat), expected, "\(natureClass)/\(stat)")
        }
    }

    func testSPRangeText() {
        let cases: [([SPRange], StatKey, String)] = [
            ([SPRange(min: 20, max: 23)], .def, "B 20\u{301C}23"),
            // 非連続は1区間に畳まない(ADR-0010 §R3)
            ([SPRange(min: 17, max: 17), SPRange(min: 19, max: 32)], .def, "B 17, 19\u{301C}32"),
            ([SPRange(min: 20, max: 20)], .spd, "D 20"),
            ([SPRange(min: 0, max: 32)], .atk, "A 0\u{301C}32"),
            ([SPRange(min: 0, max: 3), SPRange(min: 5, max: 5), SPRange(min: 8, max: 9)], .spa, "C 0\u{301C}3, 5, 8\u{301C}9"),
        ]
        for (ranges, stat, expected) in cases {
            XCTAssertEqual(ReverseCandidateDisplay.spRangeText(ranges, stat: stat), expected)
        }
    }

    func testGuideNames() {
        // 範囲に SP 0 / 32 が入っているときだけ、性格クラスとの組で目安の名前を付ける(Web の ADR-0300 §7 と同じ規則。
        // ADR-0010 §R3「表示層が付けてよい」)。0 側の名前が先。
        let cases: [([SPRange], NatureClass, StatKey, [String])] = [
            ([SPRange(min: 0, max: 5)], .neutral, .def, ["H振り"]),
            ([SPRange(min: 0, max: 5)], .plus, .def, ["H振り+B補正"]),
            ([SPRange(min: 28, max: 32)], .neutral, .def, ["HB振り"]),
            ([SPRange(min: 28, max: 32)], .plus, .def, ["HB特化"]),
            ([SPRange(min: 28, max: 32)], .plus, .spd, ["HD特化"]),
            ([SPRange(min: 0, max: 0), SPRange(min: 32, max: 32)], .neutral, .spd, ["H振り", "HD振り"]),
            ([SPRange(min: 0, max: 32)], .plus, .def, ["H振り+B補正", "HB特化"]),
            ([SPRange(min: 0, max: 3)], .neutral, .atk, ["無振り"]),
            ([SPRange(min: 0, max: 3)], .plus, .atk, ["A補正のみ"]),
            ([SPRange(min: 30, max: 32)], .neutral, .spa, ["C振り"]),
            ([SPRange(min: 30, max: 32)], .plus, .atk, ["A特化"]),
            ([SPRange(min: 1, max: 31)], .neutral, .def, []),   // 端を含まなければ名前なし
            ([SPRange(min: 0, max: 32)], .neutral, .hp, []),    // 逆算の関連ステータスにならない値は名前なし
        ]
        for (ranges, natureClass, stat, expected) in cases {
            XCTAssertEqual(
                ReverseCandidateDisplay.guideNames(ranges: ranges, natureClass: natureClass, stat: stat), expected,
                "\(ranges) / \(natureClass) / \(stat)"
            )
        }
    }

    func testMatchLabel() {
        XCTAssertEqual(ReverseCandidateDisplay.matchLabel(exact: true), "観測と一致")
        XCTAssertEqual(ReverseCandidateDisplay.matchLabel(exact: false), "一致なし(最も近い SP)")
    }

    // MARK: - 1件の整形

    func testCandidateDisplay() {
        let display = ReverseCandidateDisplay(
            candidate: candidate(natureClass: .plus, itemId: StubMaster.itemA.id), stat: .def, items: items
        )
        XCTAssertEqual(display.id, "plus@\(StubMaster.itemA.id)")
        XCTAssertEqual(display.natureClass, .plus)
        XCTAssertEqual(display.natureClassLabel, "B上昇")
        XCTAssertEqual(display.itemId, StubMaster.itemA.id)
        XCTAssertEqual(display.itemLabel, StubMaster.itemA.nameJa)
        XCTAssertEqual(display.spRangeText, "B 20\u{301C}23")
        XCTAssertEqual(display.guideNames, [])
        XCTAssertTrue(display.exact)
        XCTAssertEqual(display.matchLabel, "観測と一致")
        XCTAssertEqual(display.percentRangeText, "12.3\u{301C}15.6%")
    }

    func testCandidateDisplayWithoutItemAndNotExact() {
        let display = ReverseCandidateDisplay(
            candidate: candidate(itemId: nil, ranges: [SPRange(min: 0, max: 0)], exact: false), stat: .atk, items: items
        )
        XCTAssertEqual(display.id, "neutral@-")
        XCTAssertEqual(display.itemLabel, "持ち物なし")
        XCTAssertEqual(display.spRangeText, "A 0")
        XCTAssertEqual(display.guideNames, ["無振り"])
        XCTAssertFalse(display.exact)
        XCTAssertEqual(display.matchLabel, "一致なし(最も近い SP)")
    }

    func testUnknownItemIdIsShownAsIs() {
        let display = ReverseCandidateDisplay(candidate: candidate(itemId: "stub-item-unknown"), stat: .def, items: items)
        XCTAssertEqual(display.itemLabel, "stub-item-unknown", "マスタに無い ID は黙って「持ち物なし」にしない")
    }

    // MARK: - 結果全体

    func testResultDisplayKeepsServerOrderAndStatesPremise() {
        // 並べ替えれば順が変わる並び(support が後ろの方が大きい)を渡し、そのまま出ることを確かめる。
        let first = ReverseCandidate(
            natureClass: .plus, nature: NatureModifier(plus: .def, minus: .atk), natureId: nil, itemId: StubMaster.itemB.id,
            ranges: [SPRange(min: 4, max: 7)], spCount: 4, exact: true, mismatch: 0, support: 1,
            minPercent: 20.0, maxPercent: 24.0
        )
        let second = ReverseCandidate(
            natureClass: .neutral, nature: NatureModifier(), natureId: nil, itemId: nil,
            ranges: [SPRange(min: 20, max: 32)], spCount: 13, exact: true, mismatch: 0, support: 99,
            minPercent: 18.0, maxPercent: 22.0
        )
        let third = ReverseCandidate(
            natureClass: .neutral, nature: NatureModifier(), natureId: nil, itemId: StubMaster.itemA.id,
            ranges: [SPRange(min: 32, max: 32)], spCount: 1, exact: false, mismatch: 12, support: 0,
            minPercent: 30.0, maxPercent: 33.0
        )
        let result = ReverseResult(side: .defender, stat: .def, assumedHPSP: 32, candidates: [first, second, third], exactCount: 2)
        let display = ReverseResultDisplay(result: result, items: items)

        XCTAssertEqual(display.candidates.map(\.id),
                       ["plus@\(StubMaster.itemB.id)", "neutral@-", "neutral@\(StubMaster.itemA.id)"],
                       "サーバーの順(ADR-0010 §R4)を並べ替えない")
        XCTAssertEqual(display.side, .defender)
        XCTAssertEqual(display.stat, .def)
        XCTAssertEqual(display.assumedHPSP, 32)
        XCTAssertEqual(display.exactCount, 2)
        XCTAssertEqual(display.exactCountText, "観測と一致: 2 件 / 候補 3 件")
        // ADR-0010 §R1・§R7: 「H32 を仮定した結果」であることを画面に出す(値はサーバーの assumedHpSp から作る)
        XCTAssertEqual(display.premiseText, "相手の HP の SP を 32(H32)と仮定した結果です")
    }

    func testPremiseUsesServerValueAndIsAbsentForAttackerSide() {
        let defender = ReverseResult(side: .defender, stat: .spd, assumedHPSP: 20, candidates: [], exactCount: 0)
        XCTAssertEqual(ReverseResultDisplay(result: defender, items: items).premiseText,
                       "相手の HP の SP を 20(H20)と仮定した結果です", "32 を直書きしない")

        // 攻撃側の逆算は相手の H を計算に使わない(ADR-0010 §R1。assumedHpSp = 0)ので前提の文言を出さない。
        let attacker = ReverseResult(side: .attacker, stat: .atk, assumedHPSP: 0, candidates: [], exactCount: 0)
        let display = ReverseResultDisplay(result: attacker, items: items)
        XCTAssertNil(display.premiseText)
        XCTAssertEqual(display.exactCountText, "観測と一致: 0 件 / 候補 0 件")
    }
}
