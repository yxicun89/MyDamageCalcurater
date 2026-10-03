import XCTest

@testable import PokeCalcCore

// P6-20: `ShowdownTextParser.parse`(純粋関数・テーブル駆動。ADR-0501「P6-20」2章)。
// 名前は架空(テストアルファ等)。実在のポケモン・技・持ち物の名前は使わない。
final class ShowdownTextParserTests: XCTestCase {
    private func parse(_ text: String) -> ShowdownParseResult { ShowdownTextParser.parse(text) }

    // MARK: 先頭行

    func testHeaderLineTable() {
        struct Row {
            let input: String
            let species: String
            let nickname: String?
            let item: String?
        }
        let rows = [
            Row(input: "テストアルファ", species: "テストアルファ", nickname: nil, item: nil),
            Row(input: "テストアルファ @ テストどうぐA", species: "テストアルファ", nickname: nil, item: "テストどうぐA"),
            Row(input: "ニック (テストアルファ)", species: "テストアルファ", nickname: "ニック", item: nil),
            Row(input: "ニック (テストアルファ) @ テストどうぐA", species: "テストアルファ", nickname: "ニック", item: "テストどうぐA"),
            Row(input: "  テストアルファ  @  テストどうぐA  ", species: "テストアルファ", nickname: nil, item: "テストどうぐA"),
            Row(input: "\u{3000}テストアルファ\u{3000}", species: "テストアルファ", nickname: nil, item: nil),
            Row(input: "テストアルファ @ ", species: "テストアルファ", nickname: nil, item: nil),
        ]
        for row in rows {
            let result = parse(row.input)
            XCTAssertEqual(result.rejected, [], row.input)
            XCTAssertEqual(result.members.count, 1, row.input)
            let member = result.members.first
            XCTAssertEqual(member?.speciesName, row.species, row.input)
            XCTAssertEqual(member?.nickname, row.nickname, row.input)
            XCTAssertEqual(member?.itemName, row.item, row.input)
            XCTAssertEqual(member?.headerLineNumber, 1, row.input)
        }
    }

    func testEmptyAndBlankOnlyTextHasNoMembersAndNoRejections() {
        for input in ["", "\n", "  \n\u{3000}\n\n"] {
            XCTAssertEqual(parse(input), ShowdownParseResult(), "\(input.debugDescription)")
        }
    }

    // MARK: 全項目

    func testFullMemberAllFieldsAnyOrder() throws {
        let text = """
            ニック (テストアルファ) @ テストどうぐA
            - テストわざぶつり
            Nature: テストせいかく攻撃上昇
            Tera Type: ほのお
            SP: 32 Atk / 20 Spe
            Ability: テストとくせい
            - テストわざへんか
            """
        let result = parse(text)
        XCTAssertEqual(result.rejected, [])
        let member = try XCTUnwrap(result.members.first)
        XCTAssertEqual(member.abilityName, "テストとくせい")
        XCTAssertEqual(member.natureName, "テストせいかく攻撃上昇")
        XCTAssertEqual(member.teraTypeName, "ほのお")
        XCTAssertEqual(member.sp, [.atk: 32, .spe: 20])
        XCTAssertEqual(member.moveNames, ["テストわざぶつり", "テストわざへんか"])
        XCTAssertEqual(member.moveLines, [2, 7])
        XCTAssertEqual(member.natureLine, 3)
        XCTAssertEqual(member.teraTypeLine, 4)
        XCTAssertEqual(member.abilityLine, 6)
        XCTAssertEqual(member.itemLine, 1, "持ち物は先頭行に書かれる")
    }

    func testIdeographicSpaceAroundFieldLinesIsTrimmed() throws {
        let text = "\u{3000}テストアルファ\u{3000}\n\u{3000}Nature: テストせいかく攻撃上昇\u{3000}\n\u{3000}-\u{3000}テストわざぶつり\u{3000}"
        let result = parse(text)
        XCTAssertEqual(result.rejected, [])
        let member = try XCTUnwrap(result.members.first)
        XCTAssertEqual(member.natureName, "テストせいかく攻撃上昇")
        XCTAssertEqual(member.moveNames, ["テストわざぶつり"])
    }

    // MARK: ブロックの区切り・改行・行番号

    func testBlocksSeparatedByBlankLinesWithCRLFAndExtraBlanks() {
        let text = "テストアルファ\r\nAbility: テストとくせい\r\n\r\n   \r\n\r\nテストベータ\r\n- テストわざぶつり\r\n"
        let result = parse(text)
        XCTAssertEqual(result.rejected, [])
        XCTAssertEqual(result.members.map(\.speciesName), ["テストアルファ", "テストベータ"])
        XCTAssertEqual(result.members.map(\.headerLineNumber), [1, 6])
        XCTAssertEqual(result.members.last?.moveLines, [7])
    }

    func testLoneCarriageReturnAlsoSeparatesLines() {
        let result = parse("テストアルファ\rAbility: テストとくせい")
        XCTAssertEqual(result.members.first?.abilityName, "テストとくせい")
        XCTAssertEqual(result.members.first?.abilityLine, 2)
    }

    // MARK: 取り込めない行(黙って捨てない。行番号・理由つき)

    func testRejectedLineTable() {
        struct Row {
            let input: String
            let rejected: [ShowdownRejection]
            let file: StaticString
        }
        let rows = [
            Row(input: "テストアルファ\nEVs: 252 Atk", rejected: [.init(lineNumber: 2, text: "EVs: 252 Atk", reason: .unsupportedStatLine)], file: #filePath),
            Row(input: "テストアルファ\nIVs: 0 Atk", rejected: [.init(lineNumber: 2, text: "IVs: 0 Atk", reason: .unsupportedStatLine)], file: #filePath),
            Row(input: "テストアルファ\nLevel: 50", rejected: [.init(lineNumber: 2, text: "Level: 50", reason: .unrecognizedLine)], file: #filePath),
            Row(input: "テストアルファ\nAdamant Nature", rejected: [.init(lineNumber: 2, text: "Adamant Nature", reason: .unrecognizedLine)], file: #filePath),
            Row(input: "テストアルファ\n-", rejected: [.init(lineNumber: 2, text: "-", reason: .unrecognizedLine)], file: #filePath),
            Row(input: "\n\nテストアルファ\r\nFoo  ", rejected: [.init(lineNumber: 4, text: "Foo", reason: .unrecognizedLine)], file: #filePath),
            Row(input: "テストアルファ\nAbility: A\nAbility: B", rejected: [.init(lineNumber: 3, text: "Ability: B", reason: .duplicateField)], file: #filePath),
            Row(input: "テストアルファ\nNature: A\nNature: B", rejected: [.init(lineNumber: 3, text: "Nature: B", reason: .duplicateField)], file: #filePath),
            Row(input: "テストアルファ\nTera Type: A\nTera Type: B", rejected: [.init(lineNumber: 3, text: "Tera Type: B", reason: .duplicateField)], file: #filePath),
            Row(input: "テストアルファ\nSP: 1 HP\nSP: 2 Atk", rejected: [.init(lineNumber: 3, text: "SP: 2 Atk", reason: .duplicateField)], file: #filePath),
            Row(input: "テストアルファ\n- A\n- A", rejected: [.init(lineNumber: 3, text: "- A", reason: .duplicateMove)], file: #filePath),
            Row(input: "テストアルファ\n- A\n- B\n- C\n- D\n- E", rejected: [.init(lineNumber: 6, text: "- E", reason: .tooManyMoves)], file: #filePath),
        ]
        for row in rows {
            let result = parse(row.input)
            XCTAssertEqual(result.rejected, row.rejected, row.input.debugDescription, file: row.file)
            XCTAssertEqual(result.members.count, 1, "取り込める行があるメンバーは残る: \(row.input.debugDescription)")
        }
    }

    func testDuplicateFieldKeepsTheFirstValueAndMoveCapKeepsFirstFour() throws {
        let first = try XCTUnwrap(parse("テストアルファ\nAbility: A\nAbility: B").members.first)
        XCTAssertEqual(first.abilityName, "A")
        let moves = try XCTUnwrap(parse("テストアルファ\n- A\n- B\n- C\n- D\n- E").members.first)
        XCTAssertEqual(moves.moveNames, ["A", "B", "C", "D"])
        XCTAssertEqual(moves.moveNames.count, TeamLimits.maxMovesPerMember)
    }

    // MARK: SP(0〜32・合計66。EV ではない)

    func testSPLineTable() throws {
        struct Row {
            let line: String
            let sp: [StatKey: Int]
            let reason: ShowdownRejectionReason?
        }
        let rows = [
            Row(line: "SP: 32 HP", sp: [.hp: 32], reason: nil),
            Row(line: "SP: 0 HP / 0 Atk", sp: [.hp: 0, .atk: 0], reason: nil),
            Row(line: "SP: 2 Spe / 1 HP / 32 SpA", sp: [.spe: 2, .hp: 1, .spa: 32], reason: nil),
            Row(line: "SP: 32 HP / 32 Atk / 2 Def", sp: [.hp: 32, .atk: 32, .def: 2], reason: nil),
            Row(line: "SP: 33 HP", sp: [:], reason: .spOutOfRange),
            Row(line: "SP: 32 HP / 32 Atk / 3 Def", sp: [:], reason: .spTotalExceeded),
            Row(line: "SP: 5 HP / 6 HP", sp: [:], reason: .spMalformed),
            Row(line: "SP: 5 Foo", sp: [:], reason: .spMalformed),
            Row(line: "SP: abc", sp: [:], reason: .spMalformed),
            Row(line: "SP: -1 HP", sp: [:], reason: .spMalformed),
            Row(line: "SP: 252 HP", sp: [:], reason: .spOutOfRange),
            Row(line: "SP:", sp: [:], reason: .spMalformed),
            Row(line: "SP: 5HP", sp: [:], reason: .spMalformed),
        ]
        for row in rows {
            let result = parse("テストアルファ\n\(row.line)")
            let member = try XCTUnwrap(result.members.first, row.line)
            XCTAssertEqual(member.sp, row.sp, row.line)
            if let reason = row.reason {
                XCTAssertEqual(result.rejected, [.init(lineNumber: 2, text: row.line, reason: reason)], row.line)
            } else {
                XCTAssertEqual(result.rejected, [], row.line)
            }
        }
    }

    // MARK: 上限

    func testMoreThanSixBlocksReportOnlyTheSeventhHeader() {
        let blocks = (1...7).map { "テストモン\($0)\nAbility: テストとくせい" }
        let result = parse(blocks.joined(separator: "\n\n"))
        XCTAssertEqual(result.members.count, TeamLimits.maxMembers)
        XCTAssertEqual(result.rejected, [.init(lineNumber: 19, text: "テストモン7", reason: .memberLimitExceeded)])
    }
}
