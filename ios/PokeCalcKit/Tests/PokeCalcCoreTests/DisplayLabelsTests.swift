import XCTest

@testable import PokeCalcCore

/// `DisplayLabels.swift` の固定日本語ラベルが、18 タイプ・3分類・6段階の相性を漏れなく
/// カバーしていることを確かめる(P6-2a 批評対応「任意」項目)。
final class DisplayLabelsTests: XCTestCase {

    func testMoveCategoryLabelCoversAllThreeCategories() {
        let expected: [MoveCategory: String] = [
            .physical: "物理",
            .special: "特殊",
            .status: "変化",
        ]
        for category in MoveCategory.allCases {
            XCTAssertEqual(MoveCategoryLabel.japaneseName(for: category), expected[category], "\(category)")
        }
        XCTAssertEqual(MoveCategory.allCases.count, 3, "design.md の3分類から増減していないこと")
    }

    func testPokeTypeLabelCoversAll18Types() {
        // design.md「タイプ色(自作パレット)」と同じ18種・同じ日本語(PokeCalcDesign.TypeColorToken と対になる語彙)。
        let expected: [PokeType: String] = [
            .normal: "ノーマル", .fire: "ほのお", .water: "みず", .electric: "でんき",
            .grass: "くさ", .ice: "こおり", .fighting: "かくとう", .poison: "どく",
            .ground: "じめん", .flying: "ひこう", .psychic: "エスパー", .bug: "むし",
            .rock: "いわ", .ghost: "ゴースト", .dragon: "ドラゴン", .dark: "あく",
            .steel: "はがね", .fairy: "フェアリー",
        ]
        XCTAssertEqual(expected.count, 18, "テストの表自体が18タイプを持つこと")
        for type in PokeType.allCases {
            XCTAssertEqual(PokeTypeLabel.japaneseName(for: type), expected[type], "\(type)")
        }
        XCTAssertEqual(PokeType.allCases.count, 18)
    }

    func testEffectivenessLabelCoversAllSixMultipliers() {
        let cases: [(Double, String)] = [
            (0, "効果なし"),
            (0.25, "いまひとつ(×0.25)"),
            (0.5, "いまひとつ(×0.5)"),
            (1, "等倍"),
            (2, "ばつぐん(×2)"),
            (4, "ばつぐん(×4)"),
        ]
        for (value, expected) in cases {
            XCTAssertEqual(EffectivenessLabel.text(value), expected, "\(value)")
        }
    }

    func testIsSuperEffectiveIsTrueOnlyAtOrAboveDoubleDamage() {
        // 「ばつぐん」= 等倍(1)より効果が高い(2 以上)。View の色分けがこれ1つで判定できることを確かめる。
        let cases: [(Double, Bool)] = [
            (0, false),
            (0.25, false),
            (0.5, false),
            (1, false),
            (2, true),
            (4, true),
        ]
        for (value, expected) in cases {
            XCTAssertEqual(EffectivenessLabel.isSuperEffective(value), expected, "\(value)")
        }
    }
}
