import XCTest

@testable import PokeCalcCore

/// 技の選択肢の並び(F-02。Web の ADR-0335 と同じ語・同じ規則)。架空の技だけを使う。
final class MoveSortTests: XCTestCase {
    private func move(_ id: String, _ name: String, type: PokeType = .normal) -> Move {
        Move(id: id, nameJa: name, type: type, category: .physical, power: 40)
    }

    private func ids(_ moves: [Move]) -> [String] { moves.map(\.id) }

    // MARK: 五十音順

    func testKanaOrdersGojuon() {
        let input = [move("c", "うみ"), move("a", "あさ"), move("b", "いし")]
        XCTAssertEqual(ids(MoveSort.byType(input)), ["a", "b", "c"])
    }

    func testKanaTreatsHiraganaAndKatakanaAsSameLetter() {
        // 「いわ」(ひらがな)と「ウミ」(カタカナ)。カタカナだからといって末尾に回らない。
        let input = [move("c", "ウミ"), move("a", "あさ"), move("b", "いわ"), move("d", "エビ")]
        XCTAssertEqual(ids(MoveSort.byType(input)), ["a", "b", "c", "d"])
    }

    func testKanaVoicedComesAfterUnvoicedWithinSameLetter() {
        // 「かまう」「がまん」は「かみなり」より前(濁点は同じ字の中で後ろ)。「かまう」<「がまん」。
        let input = [move("c", "かみなり"), move("b", "がまん"), move("a", "かまう")]
        XCTAssertEqual(ids(MoveSort.byType(input)), ["a", "b", "c"])
    }

    func testKanaLongVowelIsNotTreatedAsLetterAfterEverything() {
        // 「スーパー」は「スパーク」より前(長音は照合器の扱いに従う)。
        let input = [move("b", "スパーク"), move("a", "スーパー")]
        XCTAssertEqual(ids(MoveSort.byType(input)), ["a", "b"])
    }

    func testKanaTieBreaksByIdAndIsStableRegardlessOfInputOrder() {
        // ひらがな/カタカナだけの違いは同順位。技 ID の昇順で決め、入力の順に依らない。
        let x = move("m2", "あさ")
        let y = move("m1", "アサ")
        XCTAssertEqual(ids(MoveSort.byType([x, y])), ["m1", "m2"])
        XCTAssertEqual(ids(MoveSort.byType([y, x])), ["m1", "m2"])
    }

    func testSortDoesNotMutateInputAndKeepsCount() {
        let input = [move("b", "いい"), move("a", "ああ")]
        _ = MoveSort.byType(input)
        XCTAssertEqual(ids(input), ["b", "a"])
        XCTAssertEqual(MoveSort.byType([]), [])
    }

    // MARK: タイプ順

    func testTypeOrderGroupsByTypeTableThenKanaInsideGroup() {
        let input = [
            move("w2", "みず2", type: .water), move("n1", "ふつう", type: .normal),
            move("f1", "ほのお", type: .fire), move("w1", "あわ", type: .water),
        ]
        // PokeType の並び: normal, fire, water ...
        XCTAssertEqual(ids(MoveSort.byType(input)), ["n1", "f1", "w1", "w2"])
    }

    func testTypeGroupsOnlyContainTypesWithMoves() {
        let input = [move("w1", "あわ", type: .water), move("n1", "ふつう", type: .normal)]
        let groups = MoveSort.typeGroups(input)
        XCTAssertEqual(groups.map(\.type), [.normal, .water])
        XCTAssertEqual(groups.map { ids($0.moves) }, [["n1"], ["w1"]])
        XCTAssertEqual(MoveSort.typeGroups([]).count, 0)
    }

    func testTypeGroupsMatchSortedTypeOrder() {
        let input = (0..<6).map { move("m\($0)", "わざ\(5 - $0)", type: $0 % 2 == 0 ? .grass : .ice) }
        XCTAssertEqual(MoveSort.typeGroups(input).flatMap(\.moves), MoveSort.byType(input))
    }
}
