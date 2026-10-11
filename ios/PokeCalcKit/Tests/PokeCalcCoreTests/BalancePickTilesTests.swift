import XCTest

@testable import PokeCalcCore

// おすすめタイプの該当ポケモンのタイル(ADR-0422 §2)と相性の色の意味(§3)。
//
// 決めた形:
// - `BalancePickTiles.visibleCount == 8`。`visible(_:expanded:)` は閉じていれば先頭 8 件、開いていれば全件(応答の順のまま)。
//   `hiddenCount(_:expanded:)` は閉じているときの残りの件数(開いていれば 0。「もっと見る」を出すかどうか)。
// - `BalanceMatchupTone`: 防御相性は category から(弱点 ×2/×4 → `.weak`、耐性 ×1/2・×1/4 → `.resist`、無効 → `.immune`、等倍 → `.neutral`)。
//   攻撃範囲は倍率から(抜群 → `.weak`〈赤〉、いまひとつ → `.resist`、無効 → `.immune`、等倍・攻撃技なし → `.neutral`)。
//   値の範囲から判定し直さない(ADR-0303 §7)。色は View がトークン(danger・matchupResist・matchupImmune)で塗る。緑は使わない。
final class BalancePickTilesTests: XCTestCase {
    private func pokemon(_ count: Int) -> [BalanceCandidatePokemon] {
        (0..<count).map {
            BalanceCandidatePokemon(pokemonId: "9\($0)00-000", nameJa: "テスト\($0)", types: [.water], exactMatch: $0 == 0)
        }
    }

    func testShowsFirstEightAndCountsTheRest() {
        let list = pokemon(11)
        XCTAssertEqual(BalancePickTiles.visibleCount, 8)
        XCTAssertEqual(BalancePickTiles.visible(list, expanded: false).map(\.pokemonId), list.prefix(8).map(\.pokemonId))
        XCTAssertEqual(BalancePickTiles.hiddenCount(list, expanded: false), 3)
    }

    func testExpandedShowsAllInResponseOrder() {
        let list = pokemon(11)
        XCTAssertEqual(BalancePickTiles.visible(list, expanded: true).map(\.pokemonId), list.map(\.pokemonId))
        XCTAssertEqual(BalancePickTiles.hiddenCount(list, expanded: true), 0)
    }

    func testEightOrFewerHaveNothingHidden() {
        for count in [0, 3, 8] {
            let list = pokemon(count)
            XCTAssertEqual(BalancePickTiles.visible(list, expanded: false).count, count)
            XCTAssertEqual(BalancePickTiles.hiddenCount(list, expanded: false), 0, "\(count) 件なら「もっと見る」は出ない")
        }
    }

    // MARK: - 相性の色の意味

    func testDefenseToneComesFromCategory() {
        XCTAssertEqual(BalanceMatchupTone(defense: .quadWeak), .weak)
        XCTAssertEqual(BalanceMatchupTone(defense: .weak), .weak)
        XCTAssertEqual(BalanceMatchupTone(defense: .neutral), .neutral)
        XCTAssertEqual(BalanceMatchupTone(defense: .resist), .resist)
        XCTAssertEqual(BalanceMatchupTone(defense: .quadResist), .resist)
        XCTAssertEqual(BalanceMatchupTone(defense: .immune), .immune)
    }

    func testCoverageToneComesFromMultiplier() {
        XCTAssertEqual(BalanceMatchupTone(coverage: .double), .weak, "抜群は弱点と同じ赤")
        XCTAssertEqual(BalanceMatchupTone(coverage: .neutral), .neutral)
        XCTAssertEqual(BalanceMatchupTone(coverage: .half), .resist)
        XCTAssertEqual(BalanceMatchupTone(coverage: .zero), .immune)
        XCTAssertEqual(BalanceMatchupTone(coverage: nil), .neutral, "攻撃技なし")
    }
}
