import XCTest

@testable import PokeCalcCore

/// メガ種族の持ち物の固定・解除・保存データの補正・文言・表示名(ADR-0509 §4・§6・§7)。
/// 文言は Web の `web/src/i18n/ja.ts` の `megaItemText` と同じ語(spec §4-3「iOS の文言は Web と同じ語」)。
final class MegaItemLockTests: XCTestCase {
    private static let stone = Item(id: "test-lock-stone", nameJa: "Test Mega Stone", roles: [], isMegaStone: true)
    private static let berry = Item(id: "test-lock-berry", nameJa: "テストきのみ", roles: [.defender], isMegaStone: false)
    private static let allItems = [berry, stone]

    private static func detail(
        key: String = "9601-001", isMega: Bool, requiredItemId: String? = nil, baseSpeciesNameJa: String? = nil
    ) -> SpeciesDetail {
        SpeciesDetail(
            key: key, dexNo: 9601, form: isMega ? 1 : 0, nameJa: "テストメガしゅぞく", types: [.fighting],
            baseStats: StatBlock(hp: 50, atk: 50, def: 50, spa: 50, spd: 50, spe: 50), abilities: [], learnset: [],
            isMega: isMega, requiredItemId: requiredItemId,
            baseSpeciesKey: isMega ? "9601-000" : nil, baseSpeciesNameJa: baseSpeciesNameJa)
    }

    // MARK: - 固定の判定

    func testMakeLock() {
        let cases: [(name: String, detail: SpeciesDetail?, expected: MegaItemLock)] = [
            ("詳細が無い", nil, .none),
            ("非メガ", Self.detail(isMega: false), .none),
            ("メガ + 全件にストーン", Self.detail(isMega: true, requiredItemId: Self.stone.id, baseSpeciesNameJa: "テストルカ"),
             .locked(itemId: Self.stone.id, displayName: "テストルカのメガストーン")),
            ("メガ + 基本種名が null", Self.detail(isMega: true, requiredItemId: Self.stone.id, baseSpeciesNameJa: nil),
             .locked(itemId: Self.stone.id, displayName: "メガストーン")),
            ("メガ + requiredItemId が null", Self.detail(isMega: true, requiredItemId: nil, baseSpeciesNameJa: "テストルカ"), .missing),
            ("メガ + 全件に無いストーン", Self.detail(isMega: true, requiredItemId: "test-lock-unknown", baseSpeciesNameJa: "テストルカ"),
             .missing),
        ]
        for testCase in cases {
            XCTAssertEqual(MegaItemLock.make(for: testCase.detail, allItems: Self.allItems), testCase.expected, testCase.name)
        }
    }

    /// 固定は絞り込む前の全件から引く(ストーンは役割が空で選択肢には出ないが、固定には要る)。
    func testLockFindsStoneOutsideFilteredOptions() {
        let mega = Self.detail(isMega: true, requiredItemId: Self.stone.id, baseSpeciesNameJa: "テストルカ")
        XCTAssertTrue(ItemRoleFilter.options(Self.allItems, for: .any).allSatisfy { $0.id != Self.stone.id }, "前提: 選択肢に無い")
        XCTAssertEqual(MegaItemLock.make(for: mega, allItems: Self.allItems).lockedItemId, Self.stone.id)
    }

    func testMakeFromInfoMatchesDetail() {
        let mega = Self.detail(isMega: true, requiredItemId: Self.stone.id, baseSpeciesNameJa: "テストルカ")
        XCTAssertEqual(
            MegaItemLock.make(for: MegaSpeciesInfo(detail: mega), allItems: Self.allItems),
            MegaItemLock.make(for: mega, allItems: Self.allItems))
        XCTAssertEqual(MegaItemLock.make(for: nil as MegaSpeciesInfo?, allItems: Self.allItems), .none)
    }

    func testLockedItemIdAndFieldDisabled() {
        let locked = MegaItemLock.locked(itemId: Self.stone.id, displayName: "テストルカのメガストーン")
        XCTAssertEqual(locked.lockedItemId, Self.stone.id)
        XCTAssertTrue(locked.disablesItemField)
        XCTAssertNil(MegaItemLock.missing.lockedItemId)
        XCTAssertTrue(MegaItemLock.missing.disablesItemField, "ストーンを引けなくても欄は操作できない(黙って別の持ち物にしない)")
        XCTAssertNil(MegaItemLock.none.lockedItemId)
        XCTAssertFalse(MegaItemLock.none.disablesItemField)
    }

    // MARK: - 種族を変えたときの持ち物

    func testItemIdAfterSpeciesChange() {
        let locked = MegaItemLock.locked(itemId: Self.stone.id, displayName: "テストルカのメガストーン")
        let cases: [(name: String, previous: MegaItemLock, next: MegaItemLock, current: String?, expected: String?)] = [
            ("非メガ → メガ: ストーンにする", .none, locked, Self.berry.id, Self.stone.id),
            ("非メガ(持ち物なし)→ メガ", .none, locked, nil, Self.stone.id),
            ("メガ → 非メガ: 未選択に戻す(ストーンを残さない)", locked, .none, Self.stone.id, nil),
            ("非メガ → missing: 空", .none, .missing, Self.berry.id, nil),
            ("missing → 非メガ: 空のまま", .missing, .none, nil, nil),
            ("非メガ → 非メガ: 持ち物を保つ", .none, .none, Self.berry.id, Self.berry.id),
            ("メガ → 別のメガ: 新しいストーン", locked, .locked(itemId: "test-lock-stone-2", displayName: "メガストーン"),
             Self.stone.id, "test-lock-stone-2"),
        ]
        for testCase in cases {
            XCTAssertEqual(
                MegaItemLock.itemIdAfterSpeciesChange(previous: testCase.previous, next: testCase.next, currentItemId: testCase.current),
                testCase.expected, testCase.name)
        }
    }

    // MARK: - 構築の保存データの補正(Web ADR-0320 PR-B と同じ方針)

    func testCorrection() {
        let locked = MegaItemLock.locked(itemId: Self.stone.id, displayName: "テストルカのメガストーン")
        let cases: [(name: String, current: String?, lock: MegaItemLock, expected: MegaItemCorrection)] = [
            ("メガ + 別の持ち物 → ストーンに直す", Self.berry.id, locked,
             .fixed(itemId: Self.stone.id, displayName: "テストルカのメガストーン")),
            ("メガ + 持ち物なし → ストーンに直す", nil, locked, .fixed(itemId: Self.stone.id, displayName: "テストルカのメガストーン")),
            ("メガ + ストーン → そのまま", Self.stone.id, locked, .unchanged),
            ("missing + 持ち物 → 空にする", Self.berry.id, .missing, .cleared),
            ("missing + 空 → 通知なし", nil, .missing, .unchanged),
            ("非メガがストーンを持つ → 直さない", Self.stone.id, .none, .unchanged),
            ("非メガ → そのまま", Self.berry.id, .none, .unchanged),
        ]
        for testCase in cases {
            XCTAssertEqual(MegaItemLock.correction(currentItemId: testCase.current, lock: testCase.lock), testCase.expected, testCase.name)
        }
    }

    // MARK: - 文言(Web と同じ語)

    func testStoneName() {
        XCTAssertEqual(MegaItemText.stoneName(baseSpeciesNameJa: "テストルカ"), "テストルカのメガストーン")
        XCTAssertEqual(MegaItemText.stoneName(baseSpeciesNameJa: nil), "メガストーン", "null なら名前を推測せず「メガストーン」だけ")
    }

    func testTextsMatchWeb() {
        XCTAssertEqual(MegaItemText.lockedReason, "メガシンカ: メガストーンを持ちます")
        XCTAssertEqual(MegaItemText.missingReason, "メガシンカ: メガストーンがマスタに見つかりません")
        XCTAssertEqual(MegaItemText.compareDisabledReason, "メガシンカ: 防御側の持ち物はメガストーンに固定されるため、候補は比較しません")
        XCTAssertEqual(MegaItemText.fixedItemName("テストルカのメガストーン"), "持ち物: テストルカのメガストーン")
        XCTAssertEqual(
            MegaItemText.correctedNotice("テストルカのメガストーン"),
            "メガシンカのため持ち物をテストルカのメガストーンに直しました。保存すると反映されます")
        XCTAssertEqual(MegaItemText.clearedNotice, "メガシンカのメガストーンがマスタに無いため、持ち物を空にしました。保存すると反映されます")
    }

    // MARK: - 表示名(持ち物はすべて日本語。ストーンの nameJa を出さない)

    func testDisplayName() {
        let names = [Self.stone.id: "テストルカのメガストーン"]
        let cases: [(name: String, itemId: String?, names: [String: String], expected: String)] = [
            ("持ち物なし", nil, [:], "持ち物なし"),
            ("ふつうの持ち物は nameJa", Self.berry.id, [:], "テストきのみ"),
            ("知っているメガのストーン", Self.stone.id, names, "テストルカのメガストーン"),
            ("知らないストーンは「メガストーン」", Self.stone.id, [:], "メガストーン"),
            ("マスタに無い ID はそのまま(既存の規則)", "test-lock-unknown", [:], "test-lock-unknown"),
        ]
        for testCase in cases {
            XCTAssertEqual(
                ItemDisplayName.text(itemId: testCase.itemId, items: Self.allItems, megaStoneNames: testCase.names),
                testCase.expected, testCase.name)
        }
    }

    /// 行・候補・チップが使う既存の `BulkRowDisplay.itemLabel` も、ストーンの英語名を出さない。
    func testBulkRowItemLabelHidesStoneName() {
        XCTAssertEqual(BulkRowDisplay.itemLabel(itemId: Self.stone.id, items: Self.allItems), "メガストーン")
        XCTAssertEqual(BulkRowDisplay.itemLabel(itemId: Self.berry.id, items: Self.allItems), "テストきのみ", "従来どおり")
    }
}
