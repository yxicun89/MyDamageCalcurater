import XCTest

@testable import PokeCalcCore

/// 特性で分かれた行・候補の ID と副題(issue #272。ADR-0501「P6-19」1〜2章)。
///
/// サーバーは防御側(逆算は相手)の特性を省略すると種族の特性をすべて試し、結果が違うときだけ同じ
/// (プリセット, 持ち物)・(性格クラス, 持ち物)の行・候補を特性ごとに分ける(ADR-0126・ADR-0214)。
/// 画面の ID が衝突しないこと・既存の ID(`none@-` 等)が分かれないときは変わらないこと・分かれたときだけ
/// 「特性: A / B」を出すことを、純粋な整形(`ResultEntryIdentity`・`AbilityGroupLabel`・
/// `BulkResultDisplay`・`ReverseResultDisplay`)で固定する。フィクスチャは架空(ADR-0002)。
final class AbilitySplitDisplayTests: XCTestCase {

    private let abilityA = "test-ability-a"
    private let abilityB = "test-ability-b"
    private let abilityC = "test-ability-c"
    private lazy var abilityNames = [abilityA: "テストとくせいA", abilityB: "テストとくせいB", abilityC: "テストとくせいC"]

    private let itemA = Item(id: "test-item-a", nameJa: "テストどうぐA")

    private func calcResult(percent: Double, unsupported: [UnsupportedMark] = []) -> CalcResult {
        CalcResult(
            rolls: Array(repeating: 10, count: 16), minDamage: 10, maxDamage: 10,
            minPercent: percent, maxPercent: percent, defenderHP: 100, effectiveness: 1, stab: false,
            category: .physical, ko: KOChance(hits: 10, guaranteed: true, chancePercent: 0, displayChancePercent: 100),
            unsupported: unsupported
        )
    }

    private func row(
        _ preset: DefenderPreset, itemId: String? = nil, abilityIds: [String], percent: Double = 10,
        unsupported: [UnsupportedMark] = []
    ) -> BulkCalcRow {
        BulkCalcRow(
            preset: preset, presetLabel: "テスト調整-\(preset.rawValue)", itemId: itemId,
            defender: testBulkDefender, result: calcResult(percent: percent, unsupported: unsupported),
            abilityId: abilityIds.first, abilityIds: abilityIds
        )
    }

    private func candidate(
        _ natureClass: NatureClass, itemId: String? = nil, abilityIds: [String], mismatch: Int = 0,
        unsupported: [UnsupportedMark] = []
    ) -> ReverseCandidate {
        ReverseCandidate(
            natureClass: natureClass, nature: NatureModifier(), natureId: nil, itemId: itemId,
            ranges: [SPRange(min: 0, max: 32)], spCount: 33,
            exact: mismatch == 0, mismatch: mismatch, support: 1, minPercent: 10, maxPercent: 12,
            unsupported: unsupported, abilityId: abilityIds.first, abilityIds: abilityIds
        )
    }

    private func bulkDisplay(_ rows: [BulkCalcRow]) -> BulkResultDisplay {
        BulkResultDisplay(
            result: BulkCalcResult(defenderSpeciesKey: "9001-000", rows: rows),
            items: [itemA], names: UnsupportedMarkNames(), abilityNames: abilityNames
        )
    }

    private func reverseDisplay(_ candidates: [ReverseCandidate]) -> ReverseResultDisplay {
        ReverseResultDisplay(
            result: ReverseResult(
                side: .defender, stat: .def, assumedHPSP: 32, candidates: candidates,
                exactCount: candidates.filter(\.exact).count
            ),
            items: [itemA], names: UnsupportedMarkNames(), abilityNames: abilityNames
        )
    }

    // MARK: - 1. ID の規則(`ResultEntryIdentity`)

    func testUniqueIDsKeepBaseIDsWhenNothingIsSplit() {
        let bases = ["none@-", "hp@-", "none@test-item-a"]
        XCTAssertEqual(
            ResultEntryIdentity.uniqueIDs(baseIDs: bases, abilityIds: [abilityA, abilityA, abilityA]), bases,
            "特性で分かれない(base が1回ずつ)なら既存の ID のまま(既存の XCUITest の identifier を変えない)"
        )
        XCTAssertEqual(ResultEntryIdentity.uniqueIDs(baseIDs: bases, abilityIds: [nil, nil, nil]), bases,
                       "特性の情報が無い行も既存の ID のまま")
        XCTAssertEqual(ResultEntryIdentity.splitBaseIDs(bases), [])
        XCTAssertEqual(ResultEntryIdentity.uniqueIDs(baseIDs: [], abilityIds: []), [])
    }

    func testUniqueIDsAppendAbilityOnlyToSplitBases() {
        // 同じ (none, 持ち物なし) が特性 A と B で2行に分かれ、hp は1行だけ(実際のサーバーでは全組が同じように
        // 分かれるが、規則は base ごとに決める)。
        let bases = ["none@-", "none@-", "hp@-"]
        let ids = ResultEntryIdentity.uniqueIDs(baseIDs: bases, abilityIds: [abilityA, abilityB, abilityA])
        XCTAssertEqual(ids, ["none@-@\(abilityA)", "none@-@\(abilityB)", "hp@-"])
        XCTAssertEqual(ResultEntryIdentity.splitBaseIDs(bases), ["none@-"])
    }

    func testUniqueIDsStayUniqueEvenForContractViolatingDuplicates() {
        // 同じ base・同じ特性が2回(契約違反)と、特性の情報なしで分かれた行。どちらでも衝突させない。
        let bases = ["none@-", "none@-", "none@-", "hp@-", "hp@-"]
        let ids = ResultEntryIdentity.uniqueIDs(
            baseIDs: bases, abilityIds: [abilityA, abilityA, abilityB, nil, nil]
        )
        XCTAssertEqual(ids.count, bases.count)
        XCTAssertEqual(Set(ids).count, ids.count, "ID は必ず一意: \(ids)")
        XCTAssertEqual(ids, ["none@-@\(abilityA)", "none@-@\(abilityA)#2", "none@-@\(abilityB)", "hp@-@-", "hp@-@-#2"])
    }

    // MARK: - 2. 副題の文言(`AbilityGroupLabel`)

    func testAbilityGroupLabelJoinsNamesInGivenOrderAndFallsBackToID() {
        XCTAssertEqual(AbilityGroupLabel.text(abilityIds: [abilityB, abilityA], names: abilityNames),
                       "特性: テストとくせいB / テストとくせいA", "順序は abilityIds のまま(並べ替えない)")
        XCTAssertEqual(AbilityGroupLabel.text(abilityIds: [abilityA], names: abilityNames), "特性: テストとくせいA")
        XCTAssertEqual(AbilityGroupLabel.text(abilityIds: ["test-ability-unknown"], names: abilityNames),
                       "特性: test-ability-unknown", "名前が無い ID は ID のまま(黙って消さない)")
        XCTAssertNil(AbilityGroupLabel.text(abilityIds: [], names: abilityNames))
    }

    func testPickerLabels() {
        XCTAssertEqual(AbilityPickerLabels.defenderTitle, "防御側の特性", "「攻撃側の特性」の対")
        XCTAssertEqual(AbilityPickerLabels.opponentTitle, "相手の特性")
        XCTAssertEqual(AbilityPickerLabels.unspecified, "指定なし", "攻撃側の特性と同じ語")
        XCTAssertEqual(AbilityPickerLabels.unspecified, CalcConditionLabels.abilityUnspecified)
    }

    // MARK: - 3. 一括計算の結果(`BulkResultDisplay`)

    func testBulkRowsWithSingleAbilityGroupKeepExistingIDsAndNoSubtitle() {
        let display = bulkDisplay([
            row(.none, abilityIds: [abilityA, abilityB]),
            row(.hp, abilityIds: [abilityA, abilityB]),
            row(.none, itemId: itemA.id, abilityIds: [abilityA, abilityB]),
        ])
        XCTAssertEqual(display.rows.map(\.id), ["none@-", "hp@-", "none@test-item-a"])
        XCTAssertEqual(display.rows.map(\.abilityText), [nil, nil, nil], "全行が同じ組なら副題を出さない(ノイズにしない)")
        XCTAssertEqual(display.rows.map(\.abilityIds), Array(repeating: [abilityA, abilityB], count: 3),
                       "abilityIds はそのまま持つ")
    }

    func testBulkRowsSplitByAbilityGetUniqueIDsAndSubtitles() {
        // サーバーの並び: プリセット → 特性 → 持ち物(ADR-0126)。特性 A と C は同じ結果、B だけ違う。
        let display = bulkDisplay([
            row(.none, abilityIds: [abilityA, abilityC], percent: 10),
            row(.none, itemId: itemA.id, abilityIds: [abilityA, abilityC], percent: 9),
            row(.none, abilityIds: [abilityB], percent: 0),
            row(.none, itemId: itemA.id, abilityIds: [abilityB], percent: 0),
            row(.hp, abilityIds: [abilityA, abilityC], percent: 8),
            row(.hp, abilityIds: [abilityB], percent: 0),
        ])
        let ids = display.rows.map(\.id)
        XCTAssertEqual(ids, [
            "none@-@\(abilityA)", "none@test-item-a@\(abilityA)", "none@-@\(abilityB)", "none@test-item-a@\(abilityB)",
            "hp@-@\(abilityA)", "hp@-@\(abilityB)",
        ])
        XCTAssertEqual(Set(ids).count, ids.count, "SwiftUI の ForEach が衝突しない")
        XCTAssertEqual(display.rows.map(\.abilityText), [
            "特性: テストとくせいA / テストとくせいC", "特性: テストとくせいA / テストとくせいC",
            "特性: テストとくせいB", "特性: テストとくせいB",
            "特性: テストとくせいA / テストとくせいC", "特性: テストとくせいB",
        ])
        XCTAssertEqual(display.rows.map(\.percentRangeText).count, 6, "行を落とさない・並べ替えない")
        XCTAssertEqual(display.rows.map(\.presetLabel), [
            "テスト調整-none", "テスト調整-none", "テスト調整-none", "テスト調整-none", "テスト調整-hp", "テスト調整-hp",
        ])
    }

    func testBulkRowsWithoutAbilityInfoKeepExistingBehaviour() {
        // P6-19 より前に書かれた呼び出し側・テストの行(abilityIds 空)。ID も副題も今までどおり。
        let rows = [
            BulkCalcRow(preset: .none, presetLabel: "テスト調整", defender: testBulkDefender, result: calcResult(percent: 1)),
            BulkCalcRow(preset: .hp, presetLabel: "テスト調整", defender: testBulkDefender, result: calcResult(percent: 1)),
        ]
        let display = bulkDisplay(rows)
        XCTAssertEqual(display.rows.map(\.id), ["none@-", "hp@-"])
        XCTAssertEqual(display.rows.map(\.abilityText), [nil, nil])
    }

    func testUnsupportedPlacementWorksPerSplitRow() {
        // 技の印は全行共通(上に1回)、防御側の特性の印は特性 B の行だけ(その行の注記)。
        let moveMark = UnsupportedMark(target: .move, reason: .multiHit, id: "test-move-x")
        let abilityMark = UnsupportedMark(target: .defenderAbility, reason: .unsupportedEffect, id: abilityB)
        let display = BulkResultDisplay(
            result: BulkCalcResult(defenderSpeciesKey: "9001-000", rows: [
                row(.none, abilityIds: [abilityA], unsupported: [moveMark]),
                row(.none, abilityIds: [abilityB], unsupported: [moveMark, abilityMark]),
            ]),
            items: [itemA],
            names: UnsupportedMarkNames(moveNames: ["test-move-x": "テストわざX"], abilityNames: abilityNames),
            abilityNames: abilityNames
        )
        XCTAssertEqual(display.rows.map(\.id), ["none@-@\(abilityA)", "none@-@\(abilityB)"])
        XCTAssertNotNil(display.unsupportedNotice, "全行共通の技の印は結果の上に1回")
        XCTAssertNil(display.rows[0].unsupportedNote, "特性 A の行には行だけの印が無い")
        XCTAssertEqual(display.rows[1].unsupportedNote, "未対応: 防御側の特性「テストとくせいB」",
                       "特性 B の行だけに注記(行の ID が分かれているので置き場所も衝突しない)")
    }

    // MARK: - 4. 逆算の結果(`ReverseResultDisplay`)

    func testReverseCandidatesWithSingleAbilityGroupKeepExistingIDs() {
        let display = reverseDisplay([
            candidate(.neutral, abilityIds: [abilityA]),
            candidate(.plus, abilityIds: [abilityA]),
        ])
        XCTAssertEqual(display.candidates.map(\.id), ["neutral@-", "plus@-"])
        XCTAssertEqual(display.candidates.map(\.abilityText), [nil, nil])
    }

    func testReverseCandidatesSplitByAbilityGetUniqueIDsAndSubtitles() {
        // サーバーは mismatch 昇順に並べるので、同じ base の候補が離れて並ぶこともある。並べ替えない。
        let display = reverseDisplay([
            candidate(.neutral, abilityIds: [abilityA]),
            candidate(.plus, abilityIds: [abilityA]),
            candidate(.neutral, abilityIds: [abilityB, abilityC], mismatch: 2),
            candidate(.plus, abilityIds: [abilityB, abilityC], mismatch: 2),
        ])
        let ids = display.candidates.map(\.id)
        XCTAssertEqual(ids, [
            "neutral@-@\(abilityA)", "plus@-@\(abilityA)", "neutral@-@\(abilityB)", "plus@-@\(abilityB)",
        ])
        XCTAssertEqual(Set(ids).count, ids.count)
        XCTAssertEqual(display.candidates.map(\.abilityText), [
            "特性: テストとくせいA", "特性: テストとくせいA",
            "特性: テストとくせいB / テストとくせいC", "特性: テストとくせいB / テストとくせいC",
        ])
        XCTAssertEqual(display.exactCountText, "入力したダメージと一致: 2 件 / 候補 4 件", "件数は分かれた候補をそのまま数える")
    }
}
