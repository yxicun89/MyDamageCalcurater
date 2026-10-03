import XCTest

@testable import WishlistCore

// AC-IOS-ALI-01: 表記揺れの辞書の 1 行 = 1 グループの読み書き(PWA の AC-A14 と同じ規則。docs/phase4-spec.md 4-1 iOS)。
final class AliasLinesTests: XCTestCase {
    func testParseLine() {
        let table: [(String, [String])] = [
            ("HG, ハイグレード", ["HG", "ハイグレード"]),
            ("HG，ハイグレード", ["HG", "ハイグレード"]),
            ("HG、ハイグレード", ["HG", "ハイグレード"]),
            ("　HG　,  ハイグレード\t", ["HG", "ハイグレード"]),
            ("MG, マスター グレード", ["MG", "マスター グレード"]),
            ("HG,,ハイグレード,", ["HG", "ハイグレード"]),
            ("  ", []),
            ("", []),
            ("HG", ["HG"]),
        ]
        for (line, want) in table { XCTAssertEqual(AliasLines.parseLine(line), want, line) }
    }

    func testFormat() {
        XCTAssertEqual(AliasLines.format(["S.H.Figuarts", "SHフィギュアーツ"]), "S.H.Figuarts, SHフィギュアーツ")
        XCTAssertEqual(AliasLines.format([]), "")
    }

    func testParseGroups() {
        let ok = AliasLines.parseGroups(["HG, ハイグレード", "  ", "MG、マスターグレード", ""])
        XCTAssertEqual(ok.groups, [["HG", "ハイグレード"], ["MG", "マスターグレード"]])
        XCTAssertNil(ok.invalidRow)

        let bad = AliasLines.parseGroups(["HG, ハイグレード", "MG", "RG"])
        XCTAssertEqual(bad.invalidRow, 2, "1 語だけの最初の行(1 始まり)")
        XCTAssertEqual(bad.groups, [["HG", "ハイグレード"], ["MG"], ["RG"]])

        XCTAssertEqual(AliasLines.parseGroups([]).groups, [])
        XCTAssertEqual(AliasLines.invalidRowMessage(2), "別名グループ2は2語以上をカンマで区切って入力してください")
    }
}
