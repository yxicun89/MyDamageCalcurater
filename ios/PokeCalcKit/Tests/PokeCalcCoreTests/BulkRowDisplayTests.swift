import XCTest

@testable import PokeCalcCore

/// 一括計算の結果行の整形(P6-2a。docs/design.md「画面: ダメージ計算」の行: 調整名・%幅・ダメージバー・確定数)。
///
/// 整形は純粋な関数で、サーバーの値を**丸め直さない**(%の丸めは engine の責務。ADR-0010 §3)。
/// 確定数は `displayChancePercent` を使い、`chancePercent`(engine の生値)は使わない(openapi `KOChance`)。
final class BulkRowDisplayTests: XCTestCase {

    private let itemA = Item(id: "test-item-a", nameJa: "テストどうぐA")
    private let itemB = Item(id: "test-item-b", nameJa: "テストどうぐB")

    private func result(minPercent: Double, maxPercent: Double, ko: KOChance, effectiveness: Double = 1) -> CalcResult {
        CalcResult(
            rolls: Array(repeating: 1, count: 16), minDamage: 1, maxDamage: 1,
            minPercent: minPercent, maxPercent: maxPercent, defenderHP: 100,
            effectiveness: effectiveness, stab: false, category: .physical, ko: ko
        )
    }

    private func ko(hits: Int, guaranteed: Bool, raw: Double = 0, display: Double) -> KOChance {
        KOChance(hits: hits, guaranteed: guaranteed, chancePercent: raw, displayChancePercent: display)
    }

    // MARK: - %幅

    func testPercentRangeText() {
        // 区切りは design.md と同じ U+301C(〜)。小数第1位で表示し、100% 超もそのまま出す。
        let cases: [(min: Double, max: Double, expected: String)] = [
            (72.1, 85.3, "72.1〜85.3%"),
            (0, 0, "0.0〜0.0%"),
            (44.0, 52.5, "44.0〜52.5%"),
            (100, 100, "100.0〜100.0%"),
            (98.7, 123.4, "98.7〜123.4%"),
            (5.5, 6.5, "5.5〜6.5%"),
        ]
        for c in cases {
            XCTAssertEqual(BulkRowDisplay.percentRangeText(minPercent: c.min, maxPercent: c.max), c.expected, "\(c)")
        }
    }

    // MARK: - ダメージバー

    func testBarFractionIsClampedToZeroOne() {
        let cases: [(percent: Double, expected: Double)] = [
            (0, 0),
            (50, 0.5),
            (72.1, 0.721),
            (100, 1),
            // 100% 超(倒しきる)はバーいっぱい
            (123.4, 1),
            // 負の値は来ない契約だが、来てもバーを壊さない
            (-5, 0),
        ]
        for c in cases {
            XCTAssertEqual(BulkRowDisplay.barFraction(percent: c.percent), c.expected, accuracy: 1e-9, "\(c)")
        }
    }

    // MARK: - 確定数

    func testKOText() {
        let cases: [(name: String, ko: KOChance, expected: String)] = [
            ("確定1発", ko(hits: 1, guaranteed: true, display: 100), "確定1発"),
            ("確定2発", ko(hits: 2, guaranteed: true, display: 100), "確定2発"),
            // 生値(chancePercent)は丸めずに来る。表示は displayChancePercent を使う
            ("乱数2発", ko(hits: 2, guaranteed: false, raw: 45.649, display: 45.6), "乱数2発(45.6%)"),
            ("乱数は小数第1位を常に出す", ko(hits: 3, guaranteed: false, raw: 12.0, display: 12.0), "乱数3発(12.0%)"),
            ("下限 0.1", ko(hits: 1, guaranteed: false, raw: 0.01, display: 0.1), "乱数1発(0.1%)"),
            ("上限 99.9", ko(hits: 2, guaranteed: false, raw: 99.99, display: 99.9), "乱数2発(99.9%)"),
            ("倒せない", ko(hits: 0, guaranteed: false, display: 0), "倒せない"),
        ]
        for c in cases {
            XCTAssertEqual(BulkRowDisplay.koText(c.ko), c.expected, c.name)
        }
    }

    func testKOTextIgnoresRawChancePercent() {
        // 同じ displayChancePercent なら、生値が違っても同じ表示
        let first = BulkRowDisplay.koText(ko(hits: 2, guaranteed: false, raw: 33.33, display: 33.3))
        let second = BulkRowDisplay.koText(ko(hits: 2, guaranteed: false, raw: 33.26, display: 33.3))
        XCTAssertEqual(first, "乱数2発(33.3%)")
        XCTAssertEqual(second, "乱数2発(33.3%)")
    }

    // MARK: - 確定数の段階(バッジを弾ませる判定)

    func testKOTier() {
        let cases: [(KOChance, KOTier)] = [
            (ko(hits: 0, guaranteed: false, display: 0), .cannotKO),
            (ko(hits: 2, guaranteed: true, display: 100), .guaranteed(hits: 2)),
            (ko(hits: 2, guaranteed: false, display: 45.6), .chance(hits: 2)),
        ]
        for (ko, expected) in cases {
            XCTAssertEqual(KOTier(ko), expected, "\(ko)")
        }
    }

    func testKOTierChangeDetection() {
        // 確定数が「変わった」= 段階(倒せない/確定n発/乱数n発)が変わったとき。確率の数字だけの変化は含めない。
        let chance2a = ko(hits: 2, guaranteed: false, display: 45.6)
        let chance2b = ko(hits: 2, guaranteed: false, display: 60.1)
        let sure2 = ko(hits: 2, guaranteed: true, display: 100)
        let chance3 = ko(hits: 3, guaranteed: false, display: 45.6)
        let none = ko(hits: 0, guaranteed: false, display: 0)
        let cases: [(name: String, previous: KOChance?, current: KOChance, changed: Bool)] = [
            ("初回表示は弾ませない", nil, chance2a, false),
            ("確率だけ変化", chance2a, chance2b, false),
            ("乱数2発 → 確定2発", chance2a, sure2, true),
            ("乱数2発 → 乱数3発", chance2a, chance3, true),
            ("確定2発 → 倒せない", sure2, none, true),
            ("同じ", sure2, sure2, false),
        ]
        for c in cases {
            XCTAssertEqual(KOTier.changed(from: c.previous, to: c.current), c.changed, c.name)
        }
    }

    // MARK: - 行全体

    func testRowDisplayFromBulkRow() throws {
        let row = BulkCalcRow(
            preset: .hbFull, presetLabel: "テスト特化ラベル", itemId: itemB.id, defender: testBulkDefender,
            result: result(minPercent: 44.3, maxPercent: 52.5, ko: ko(hits: 2, guaranteed: false, raw: 7.77, display: 7.8))
        )
        let display = BulkRowDisplay(row: row, items: [itemA, itemB])
        // 調整名はサーバーの presetLabel をそのまま使う(クライアントで名前を作らない)
        XCTAssertEqual(display.presetLabel, "テスト特化ラベル")
        XCTAssertEqual(display.itemLabel, "テストどうぐB")
        XCTAssertEqual(display.percentRangeText, "44.3〜52.5%")
        XCTAssertEqual(display.barMinFraction, 0.443, accuracy: 1e-9)
        XCTAssertEqual(display.barMaxFraction, 0.525, accuracy: 1e-9)
        XCTAssertEqual(display.koText, "乱数2発(7.8%)")
        XCTAssertEqual(display.koTier, .chance(hits: 2))
        XCTAssertEqual(display.preset, .hbFull)
        XCTAssertEqual(display.itemId, itemB.id)
        XCTAssertEqual(display.effectiveness, 1, "テストデータの既定値")
    }

    func testEffectivenessIsCopiedFromResult() {
        // M4: `moveEffectiveness`(CalcViewModel)が全行の一致を見られるよう、丸めずそのまま運ぶ。
        for value in [0.0, 0.25, 0.5, 1.0, 2.0, 4.0] {
            let row = BulkCalcRow(
                preset: .none, presetLabel: "x", itemId: nil, defender: testBulkDefender,
                result: result(minPercent: 10, maxPercent: 20, ko: ko(hits: 1, guaranteed: true, display: 100), effectiveness: value)
            )
            let display = BulkRowDisplay(row: row, items: [])
            XCTAssertEqual(display.effectiveness, value, "\(value)")
        }
    }

    func testItemLabel() {
        let cases: [(itemId: String?, expected: String)] = [
            (nil, "持ち物なし"),
            (itemA.id, "テストどうぐA"),
            // マスタに無い ID は ID をそのまま出す(黙って「持ち物なし」にしない)
            ("test-item-unknown", "test-item-unknown"),
        ]
        for c in cases {
            XCTAssertEqual(BulkRowDisplay.itemLabel(itemId: c.itemId, items: [itemA, itemB]), c.expected, "\(c)")
        }
    }

    func testRowIdIsStableAndUniquePerPresetAndItem() {
        // XCUITest の accessibilityIdentifier(`calcResultRow-<id>`)に使う。形は ADR-0501 の約束どおり
        // `<preset の rawValue>@<itemId。nil は ->`。
        let base = result(minPercent: 1, maxPercent: 2, ko: ko(hits: 0, guaranteed: false, display: 0))
        let plain = BulkRowDisplay(row: BulkCalcRow(preset: .hb, presetLabel: "x", itemId: nil, defender: testBulkDefender, result: base), items: [itemA])
        let withItem = BulkRowDisplay(row: BulkCalcRow(preset: .hb, presetLabel: "x", itemId: itemA.id, defender: testBulkDefender, result: base), items: [itemA])
        let other = BulkRowDisplay(row: BulkCalcRow(preset: .hp, presetLabel: "x", itemId: nil, defender: testBulkDefender, result: base), items: [itemA])
        XCTAssertEqual(plain.id, "hb@-")
        XCTAssertEqual(withItem.id, "hb@test-item-a")
        XCTAssertEqual(other.id, "hp@-")
    }
}
