import XCTest

@testable import PokeCalcCore

/// P6-17(ADR-0501「P6-17」2章・3章): 未対応の印の日本語ラベル・名前の解決・置き場所(結果の上に1回 / 行ごと)・
/// 注記の文言。すべて純粋な整形なので、画面を起動せずにここで固定する。
///
/// テストは契約の文字列(rawValue)で書く(Swift の case 名は実装が決めてよい。`presets(_:)` と同じ方針)。
final class UnsupportedNoticeTests: XCTestCase {

    private func mark(_ target: String, _ reason: String, _ id: String,
                      file: StaticString = #filePath, line: UInt = #line) throws -> UnsupportedMark {
        UnsupportedMark(
            target: try XCTUnwrap(UnsupportedTarget(rawValue: target), "未知の target \(target)", file: file, line: line),
            reason: try XCTUnwrap(UnsupportedReason(rawValue: reason), "未知の reason \(reason)", file: file, line: line),
            id: id
        )
    }

    // MARK: - 2章: ラベル(対象5種・理由15種をこの表で固定する。Web レーンも同じ語。DECISIONS.md)

    private static let expectedTargetNames: [String: String] = [
        "move": "技",
        "attacker_item": "攻撃側の持ち物",
        "attacker_ability": "攻撃側の特性",
        "defender_item": "防御側の持ち物",
        "defender_ability": "防御側の特性",
    ]

    private static let expectedReasonNames: [String: String] = [
        "alt_defense_stat": "防御に使う能力値が通常と違う",
        "alt_offense_stat": "攻撃に使う能力値が通常と違う",
        "always_crit": "必ず急所",
        "effectiveness_change": "相性の求め方が通常と違う",
        "field_specific": "天候・フィールドで変化",
        "fixed_damage": "固定ダメージ",
        "ignore_defense_ranks": "防御側のランク変化を無視",
        "move_specific": "技固有の効果",
        "multi_hit": "多段技",
        "ohko": "一撃必殺",
        "priority_change": "優先度が変化",
        "type_change": "タイプが変化",
        "variable_power": "威力が変化",
        "zero_power": "威力が技の処理で決まる",
        "unsupported_effect": "効果を計算に反映していない",
    ]

    func testTargetNameCoversAllFiveTargets() {
        XCTAssertEqual(Self.expectedTargetNames.count, UnsupportedTarget.allCases.count, "表が全対象を持つこと")
        for target in UnsupportedTarget.allCases {
            XCTAssertEqual(UnsupportedMarkLabel.targetName(for: target), Self.expectedTargetNames[target.rawValue],
                           target.rawValue)
        }
    }

    func testReasonNameCoversAllFifteenReasons() {
        XCTAssertEqual(Self.expectedReasonNames.count, UnsupportedReason.allCases.count, "表が全理由を持つこと")
        for reason in UnsupportedReason.allCases {
            XCTAssertEqual(UnsupportedMarkLabel.reasonName(for: reason), Self.expectedReasonNames[reason.rawValue],
                           reason.rawValue)
        }
    }

    /// 同じ文言の理由が2つあると画面で区別できないので、すべて違う文言にする。
    func testReasonNamesAreDistinct() {
        let names = UnsupportedReason.allCases.map(UnsupportedMarkLabel.reasonName(for:))
        XCTAssertEqual(Set(names).count, names.count, "\(names)")
        XCTAssertFalse(names.contains(""), "空の文言がある")
    }

    /// 印1つの文言: `<対象>「<名前>」(<理由>)`。
    func testMarkTextForMoveIncludesReason() throws {
        let text = UnsupportedMarkLabel.text(for: try mark("move", "multi_hit", "test-move-x"), name: "テストわざX")
        XCTAssertEqual(text, "技「テストわざX」(多段技)")
    }

    /// 持ち物・特性(理由は常に `unsupported_effect`)は「(…)」を付けない(対象名で意味が通るため)。
    func testMarkTextForUnsupportedEffectOmitsReason() throws {
        XCTAssertEqual(
            UnsupportedMarkLabel.text(for: try mark("defender_item", "unsupported_effect", "test-item-x"), name: "テストどうぐX"),
            "防御側の持ち物「テストどうぐX」"
        )
        XCTAssertEqual(
            UnsupportedMarkLabel.text(for: try mark("attacker_ability", "unsupported_effect", "test-ability-x"), name: "テストとくせいX"),
            "攻撃側の特性「テストとくせいX」"
        )
    }

    // MARK: - 2章: 名前の解決(マスタから。無ければ ID)

    func testNamesResolveByTargetKindAndFallBackToID() throws {
        let names = UnsupportedMarkNames(
            moves: [Move(id: "test-move-x", nameJa: "テストわざX", type: .normal, category: .physical, power: 10)],
            items: [Item(id: "test-item-x", nameJa: "テストどうぐX")],
            abilities: [Ability(id: "test-ability-x", nameJa: "テストとくせいX")]
        )
        XCTAssertEqual(names.name(for: try mark("move", "multi_hit", "test-move-x")), "テストわざX")
        XCTAssertEqual(names.name(for: try mark("attacker_item", "unsupported_effect", "test-item-x")), "テストどうぐX")
        XCTAssertEqual(names.name(for: try mark("defender_item", "unsupported_effect", "test-item-x")), "テストどうぐX")
        XCTAssertEqual(names.name(for: try mark("attacker_ability", "unsupported_effect", "test-ability-x")), "テストとくせいX")
        XCTAssertEqual(names.name(for: try mark("defender_ability", "unsupported_effect", "test-ability-x")), "テストとくせいX")
        // 対象の種類で辞書を選ぶ(同じ ID が別の種類の辞書にあっても引かない)
        XCTAssertEqual(names.name(for: try mark("move", "multi_hit", "test-item-x")), "test-item-x")
        // マスタに無い ID は ID のまま(黙って消さない)
        XCTAssertEqual(names.name(for: try mark("move", "multi_hit", "test-move-unknown")), "test-move-unknown")
    }

    // MARK: - 3章: 置き場所

    /// 全行にある印は `common`(1行目の順)、残りは行ごと。
    func testPlacementSplitsCommonAndPerEntry() throws {
        let moveMark = try mark("move", "multi_hit", "test-move-x")
        let attackerItem = try mark("attacker_item", "unsupported_effect", "test-item-a")
        let defenderItem = try mark("defender_item", "unsupported_effect", "test-item-d")
        let placement = UnsupportedPlacement([
            [moveMark, attackerItem],
            [moveMark, attackerItem, defenderItem],
            [moveMark, attackerItem],
        ])
        XCTAssertEqual(placement.common, [moveMark, attackerItem])
        XCTAssertEqual(placement.perEntry, [[], [defenderItem], []])
    }

    /// 印が無い行が1つでもあれば、その印は共通にしない(全行が持つ印だけが共通)。
    func testPlacementKeepsMarksPerEntryWhenSomeRowLacksThem() throws {
        let defenderItem = try mark("defender_item", "unsupported_effect", "test-item-d")
        let placement = UnsupportedPlacement([[], [defenderItem], [], [defenderItem]])
        XCTAssertEqual(placement.common, [])
        XCTAssertEqual(placement.perEntry, [[], [defenderItem], [], [defenderItem]])
    }

    /// 行が1つなら、その行の印はすべて共通(結果の上に1回)。
    func testPlacementWithSingleEntryMakesEverythingCommon() throws {
        let moveMark = try mark("move", "fixed_damage", "test-move-x")
        let placement = UnsupportedPlacement([[moveMark]])
        XCTAssertEqual(placement.common, [moveMark])
        XCTAssertEqual(placement.perEntry, [[]])
    }

    /// 行が無ければ両方空。
    func testPlacementWithNoEntriesIsEmpty() {
        let placement = UnsupportedPlacement([])
        XCTAssertEqual(placement.common, [])
        XCTAssertEqual(placement.perEntry, [])
    }

    /// 同じ行に同じ印が重複していても1つにまとめる(同じ文言を2回出さない)。
    func testPlacementDeduplicatesWithinEntry() throws {
        let moveMark = try mark("move", "multi_hit", "test-move-x")
        let defenderItem = try mark("defender_item", "unsupported_effect", "test-item-d")
        let placement = UnsupportedPlacement([
            [moveMark, moveMark],
            [moveMark, defenderItem, defenderItem],
        ])
        XCTAssertEqual(placement.common, [moveMark])
        XCTAssertEqual(placement.perEntry, [[], [defenderItem]])
    }

    /// 1行目にだけある印(2行目以降には無い)は共通にしない。1行目の判定を「他の行にもあるか」まで見ずに
    /// 1行目の中身をそのまま `common` にしてしまう実装だと red になる(critic 指摘 2026-09-25)。
    func testPlacementDropsMarkOnlyPresentInFirstRow() throws {
        let moveMark = try mark("move", "multi_hit", "test-move-x")
        let defenderItem = try mark("defender_item", "unsupported_effect", "test-item-d")
        let placement = UnsupportedPlacement([
            [moveMark, defenderItem],
            [moveMark],
        ])
        XCTAssertEqual(placement.common, [moveMark])
        XCTAssertEqual(placement.perEntry, [[defenderItem], []])
    }

    /// `common` の並びは1行目の順(2行目以降の並びが違っても1行目を優先する)。
    func testPlacementCommonOrderFollowsFirstRow() throws {
        let moveMark = try mark("move", "multi_hit", "test-move-x")
        let attackerItem = try mark("attacker_item", "unsupported_effect", "test-item-a")
        let placement = UnsupportedPlacement([
            [attackerItem, moveMark],
            [moveMark, attackerItem],
        ])
        XCTAssertEqual(placement.common, [attackerItem, moveMark])
        XCTAssertEqual(placement.perEntry, [[], []])
    }

    // MARK: - 3章: 注記の文言

    private var names: UnsupportedMarkNames {
        UnsupportedMarkNames(
            moves: [Move(id: "test-move-x", nameJa: "テストわざX", type: .normal, category: .physical, power: 10)],
            items: [Item(id: "test-item-d", nameJa: "テストどうぐD")],
            abilities: []
        )
    }

    func testSummaryAndRowNoteAreNilWithoutMarks() {
        XCTAssertNil(UnsupportedNoticeText.summary([], names: names))
        XCTAssertNil(UnsupportedNoticeText.rowNote([], names: names))
    }

    func testSummaryText() throws {
        let text = UnsupportedNoticeText.summary(
            [try mark("move", "multi_hit", "test-move-x"), try mark("attacker_item", "unsupported_effect", "test-item-d")],
            names: names
        )
        XCTAssertEqual(text, "この結果は正確でない可能性があります(未対応: 技「テストわざX」(多段技)、攻撃側の持ち物「テストどうぐD」)")
    }

    func testRowNoteText() throws {
        let text = UnsupportedNoticeText.rowNote([try mark("defender_item", "unsupported_effect", "test-item-d")], names: names)
        XCTAssertEqual(text, "未対応: 防御側の持ち物「テストどうぐD」")
    }

    // MARK: - 3章: 一括計算の結果全体(`BulkResultDisplay`)

    private func bulkRow(preset: DefenderPreset, itemId: String?, marks: [UnsupportedMark]) -> BulkCalcRow {
        BulkCalcRow(
            preset: preset, presetLabel: "テスト調整", itemId: itemId, defender: testBulkDefender,
            result: CalcResult(
                rolls: Array(repeating: 10, count: 16), minDamage: 10, maxDamage: 10, minPercent: 10, maxPercent: 10,
                defenderHP: 100, effectiveness: 1, stab: false, category: .physical,
                ko: KOChance(hits: 10, guaranteed: true, chancePercent: 0, displayChancePercent: 100),
                unsupported: marks
            )
        )
    }

    /// 技の印(全行)は結果の上に1回、持ち物の比較で増えた行の防御側の持ち物の印はその行だけ。
    func testBulkResultDisplayPlacesMoveMarkAboveAndItemMarkOnRows() throws {
        let moveMark = try mark("move", "multi_hit", "test-move-x")
        let defenderItem = try mark("defender_item", "unsupported_effect", "test-item-d")
        let result = BulkCalcResult(defenderSpeciesKey: "9002-000", rows: [
            bulkRow(preset: .none, itemId: nil, marks: [moveMark]),
            bulkRow(preset: .none, itemId: "test-item-d", marks: [moveMark, defenderItem]),
            bulkRow(preset: .hp, itemId: nil, marks: [moveMark]),
            bulkRow(preset: .hp, itemId: "test-item-d", marks: [moveMark, defenderItem]),
        ])
        let display = BulkResultDisplay(result: result, items: [Item(id: "test-item-d", nameJa: "テストどうぐD")], names: names)

        XCTAssertEqual(display.unsupportedNotice, "この結果は正確でない可能性があります(未対応: 技「テストわざX」(多段技))")
        XCTAssertEqual(display.rows.map(\.id), ["none@-", "none@test-item-d", "hp@-", "hp@test-item-d"])
        XCTAssertEqual(display.rows.map(\.unsupportedNote), [
            nil, "未対応: 防御側の持ち物「テストどうぐD」", nil, "未対応: 防御側の持ち物「テストどうぐD」",
        ])
        // 行の他の値は `BulkRowDisplay(row:items:)` と同じ(印の追加で整形を変えない)
        XCTAssertEqual(display.rows.map(\.itemLabel), ["持ち物なし", "テストどうぐD", "持ち物なし", "テストどうぐD"])
    }

    /// 印が1つも無ければ注記は出ない(既存の画面のまま)。
    func testBulkResultDisplayWithoutMarksHasNoNotes() {
        let result = BulkCalcResult(defenderSpeciesKey: "9002-000", rows: [
            bulkRow(preset: .none, itemId: nil, marks: []),
            bulkRow(preset: .hp, itemId: nil, marks: []),
        ])
        let display = BulkResultDisplay(result: result, items: [], names: names)
        XCTAssertNil(display.unsupportedNotice)
        XCTAssertEqual(display.rows.map(\.unsupportedNote), [nil, nil])
        XCTAssertEqual(display.rows, result.rows.map { BulkRowDisplay(row: $0, items: []) })
    }

    // MARK: - 3章: 逆算の結果全体(`ReverseResultDisplay`)

    private func candidate(itemId: String?, marks: [UnsupportedMark]) -> ReverseCandidate {
        ReverseCandidate(
            natureClass: .neutral, nature: NatureModifier(), natureId: nil, itemId: itemId,
            ranges: [SPRange(min: 0, max: 3)], spCount: 4, exact: true, mismatch: 0, support: 4,
            minPercent: 10, maxPercent: 12, unsupported: marks
        )
    }

    func testReverseResultDisplayPlacesCommonMarkAboveAndItemMarkOnCards() throws {
        let moveMark = try mark("move", "variable_power", "test-move-x")
        let defenderItem = try mark("defender_item", "unsupported_effect", "test-item-d")
        let result = ReverseResult(side: .defender, stat: .def, assumedHPSP: 32, candidates: [
            candidate(itemId: nil, marks: [moveMark]),
            candidate(itemId: "test-item-d", marks: [moveMark, defenderItem]),
        ], exactCount: 2)
        let display = ReverseResultDisplay(result: result, items: [Item(id: "test-item-d", nameJa: "テストどうぐD")], names: names)

        XCTAssertEqual(display.unsupportedNotice, "この結果は正確でない可能性があります(未対応: 技「テストわざX」(威力が変化))")
        XCTAssertEqual(display.candidates.map(\.unsupportedNote), [nil, "未対応: 防御側の持ち物「テストどうぐD」"])
    }

    func testReverseResultDisplayWithoutMarksHasNoNotes() {
        let result = ReverseResult(side: .defender, stat: .def, assumedHPSP: 32, candidates: [
            candidate(itemId: nil, marks: []),
        ], exactCount: 1)
        let display = ReverseResultDisplay(result: result, items: [], names: names)
        XCTAssertNil(display.unsupportedNotice)
        XCTAssertEqual(display.candidates.map(\.unsupportedNote), [nil])
    }
}
