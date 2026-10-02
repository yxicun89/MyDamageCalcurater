import XCTest

@testable import PokeCalcCore

// P6-20: 名前の戦略(英語名への拡張点。ADR-0502 §4)。既定は日本語名。
final class ShowdownNamingTests: XCTestCase {
    func testJapaneseNamingUsesNameJaAndTypeLabels() {
        let naming = JapaneseShowdownNaming()
        XCTAssertEqual(naming.name(of: StubMaster.alpha), "テストアルファ")
        XCTAssertEqual(naming.name(of: StubMaster.physicalMove), "テストわざぶつり")
        XCTAssertEqual(naming.name(of: StubMaster.itemA), "テストどうぐA")
        XCTAssertEqual(naming.name(of: StubMaster.ability), "テストとくせい")
        XCTAssertEqual(naming.name(of: StubMaster.atkUpNature), "テストせいかく攻撃上昇")
        for type in PokeType.allCases {
            XCTAssertEqual(naming.name(of: type), PokeTypeLabel.japaneseName(for: type))
            XCTAssertEqual(naming.type(forImportedName: PokeTypeLabel.japaneseName(for: type)), type)
        }
        XCTAssertNil(naming.type(forImportedName: "そんなタイプ"))
        XCTAssertEqual(naming.searchQuery(forImportedName: "テストアルファ"), "テストアルファ")
    }
}
