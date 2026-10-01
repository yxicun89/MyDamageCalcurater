import Foundation
import XCTest

@testable import PokeCalcCore

/// `MockPokeCalcService` の特性の候補(issue #272。ADR-0501「P6-19」5章。サーバーの規則は ADR-0126・ADR-0214)。
///
/// モックは計算しないので、特性で行・候補が分かれる状況はフィクスチャの任意項目 `nullifiesMoveType` で決め打ちに
/// 再現する: 防御側(逆算は相手が防御側 = side=defender のとき)の特性がその技のタイプを無効にするなら、その特性の
/// 行・候補はダメージ 0。結果が同じ特性は1つにまとめ、違うときだけ分ける(ADR-0126)。
/// 既存の種族(9001〜9003)は特性が1つずつで、行・候補の数と ID は変わらない(既存の XCUITest を変えない)。
final class MockPokeCalcServiceAbilityTests: XCTestCase {

    /// `Resources/*.json` の架空データ(ADR-0002)。
    private static let attackerKey = "9001-000"
    private static let plainDefenderKey = "9002-000"
    private static let plainDefenderAbility = "test-ability-beta"
    /// 特性を2つ持ち、2つ目がかくとう技を無効にする架空の種族。
    private static let splitSpeciesKey = "9004-000"
    private static let deltaAbility = "test-ability-delta"
    private static let immuneAbility = "test-ability-fighting-immune"
    /// かくとうタイプの物理技(無効の特性が効く)。
    private static let fightingMoveID = "test-move-physical-a"
    /// どくタイプの特殊技(無効の特性が効かない)。
    private static let poisonMoveID = "test-move-special-a"
    private static let itemID = "test-item-berry"
    private static let neutralNatureID = "test-nature-neutral"
    private static let physicalPresets = ["none", "hp", "hb_boost", "hb", "hb_full"]

    private var mock: MockPokeCalcService!

    override func setUpWithError() throws {
        mock = try MockPokeCalcService()
    }

    private func individual(_ speciesKey: String) -> Individual {
        Individual(speciesKey: speciesKey, natureId: Self.neutralNatureID, sp: zeroSP)
    }

    private func bulk(
        defender: String, moveId: String = fightingMoveID, itemVariants: [String?] = [], defenderAbilityId: String? = nil
    ) async throws -> BulkCalcResult {
        try await mock.calcBulk(BulkCalcRequest(
            format: .single, attacker: individual(Self.attackerKey), defenderSpeciesKey: defender, moveId: moveId,
            itemVariants: itemVariants, defenderAbilityId: defenderAbilityId
        ))
    }

    private func reverse(
        side: ReverseSide, known: String, unknown: String, moveId: String = fightingMoveID,
        unknownAbilityId: String? = nil
    ) async throws -> ReverseResult {
        try await mock.reverse(ReverseRequest(
            format: .single, side: side, known: individual(known), unknownSpeciesKey: unknown, moveId: moveId,
            observations: [.percent(30), .percent(31)], unknownAbilityId: unknownAbilityId
        ))
    }

    private func assertZeroDamage(_ result: CalcResult, _ context: String, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertEqual(result.rolls, Array(repeating: 0, count: 16), context, file: file, line: line)
        XCTAssertEqual(result.minDamage, 0, context, file: file, line: line)
        XCTAssertEqual(result.maxDamage, 0, context, file: file, line: line)
        XCTAssertEqual(result.minPercent, 0, context, file: file, line: line)
        XCTAssertEqual(result.maxPercent, 0, context, file: file, line: line)
        XCTAssertEqual(result.ko.hits, 0, "倒せない: \(context)", file: file, line: line)
        XCTAssertFalse(result.ko.guaranteed, context, file: file, line: line)
    }

    // MARK: - フィクスチャ

    func testSplitSpeciesFixtureHasTwoAbilities() async throws {
        let detail = try await mock.species(key: Self.splitSpeciesKey)
        XCTAssertEqual(detail.abilities.map(\.id), [Self.deltaAbility, Self.immuneAbility])
        XCTAssertTrue(detail.abilities.allSatisfy { $0.nameJa.hasPrefix("テスト") })
        let move = try await mock.move(id: Self.fightingMoveID)
        XCTAssertEqual(move.type, .fighting, "無効の特性が効く技")
        let other = try await mock.move(id: Self.poisonMoveID)
        XCTAssertNotEqual(other.type, .fighting, "無効の特性が効かない技")
    }

    // MARK: - 一括計算

    func testExistingSpeciesRowsCarryTheirSingleAbilityAndKeepShape() async throws {
        let result = try await bulk(defender: Self.plainDefenderKey)
        XCTAssertEqual(result.rows.map(\.preset.rawValue), Self.physicalPresets, "行の数・順は変わらない")
        XCTAssertEqual(result.rows.map(\.abilityId), Array(repeating: Self.plainDefenderAbility, count: 5),
                       "契約どおり abilityId は常に入る(種族の特性)")
        XCTAssertEqual(result.rows.map(\.abilityIds), Array(repeating: [Self.plainDefenderAbility], count: 5))
    }

    func testNullifyingAbilitySplitsRowsPresetThenAbilityThenItem() async throws {
        let result = try await bulk(defender: Self.splitSpeciesKey, itemVariants: [nil, Self.itemID])
        let plain = try await bulk(defender: Self.plainDefenderKey, itemVariants: [nil, Self.itemID])

        XCTAssertEqual(result.rows.count, 5 * 2 * 2, "プリセット × 特性の組(2)× 持ち物")
        guard result.rows.count == 5 * 2 * 2 else { return }
        var index = 0
        for preset in Self.physicalPresets {
            for (ability, ids) in [(Self.deltaAbility, [Self.deltaAbility]), (Self.immuneAbility, [Self.immuneAbility])] {
                for itemId in [nil, Self.itemID] as [String?] {
                    let row = result.rows[index]
                    let context = "\(preset)/\(ability)/\(itemId ?? "-")"
                    XCTAssertEqual(row.preset.rawValue, preset, context)
                    XCTAssertEqual(row.itemId, itemId, context)
                    XCTAssertEqual(row.abilityId, ability, context)
                    XCTAssertEqual(row.abilityIds, ids, context)
                    if ability == Self.immuneAbility {
                        assertZeroDamage(row.result, context)
                    } else {
                        let plainRow = try XCTUnwrap(plain.rows.first { $0.preset == row.preset && $0.itemId == itemId })
                        XCTAssertEqual(row.result.maxDamage, plainRow.result.maxDamage, "効かない特性の行は既存の決め打ちの結果: \(context)")
                    }
                    index += 1
                }
            }
        }
    }

    func testMoveTheAbilityDoesNotNullifyMergesAllAbilitiesIntoOneRow() async throws {
        let result = try await bulk(defender: Self.splitSpeciesKey, moveId: Self.poisonMoveID)
        XCTAssertEqual(result.rows.count, 5, "結果が同じ特性は1行にまとめる(行数は増えない)")
        XCTAssertEqual(result.rows.map(\.abilityId), Array(repeating: Self.deltaAbility, count: 5), "代表は種族の特性の先頭")
        XCTAssertEqual(result.rows.map(\.abilityIds), Array(repeating: [Self.deltaAbility, Self.immuneAbility], count: 5))
    }

    func testPinnedDefenderAbilityReturnsOnlyThatAbility() async throws {
        let immune = try await bulk(defender: Self.splitSpeciesKey, defenderAbilityId: Self.immuneAbility)
        XCTAssertEqual(immune.rows.count, 5)
        XCTAssertEqual(immune.rows.map(\.abilityIds), Array(repeating: [Self.immuneAbility], count: 5))
        for row in immune.rows { assertZeroDamage(row.result, row.preset.rawValue) }

        let delta = try await bulk(defender: Self.splitSpeciesKey, defenderAbilityId: Self.deltaAbility)
        XCTAssertEqual(delta.rows.map(\.abilityIds), Array(repeating: [Self.deltaAbility], count: 5))
        XCTAssertTrue(delta.rows.allSatisfy { $0.result.maxDamage > 0 })
    }

    func testPinnedAbilityTheDefenderLacksIsInvalidInput() async throws {
        let error = await assertThrowsPokeCalcError("種族が持たない特性") {
            try await self.bulk(defender: Self.splitSpeciesKey, defenderAbilityId: self.otherSpeciesAbility)
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.invalidInput, "サーバーと同じく 400 invalid_input")
    }

    private var otherSpeciesAbility: String { Self.plainDefenderAbility }

    // MARK: - 逆算

    func testReverseExistingSpeciesCandidatesCarryTheirSingleAbility() async throws {
        let result = try await reverse(side: .defender, known: Self.attackerKey, unknown: Self.plainDefenderKey)
        XCTAssertEqual(result.candidates.count, 2, "候補の数は変わらない")
        XCTAssertEqual(result.candidates.map(\.abilityIds), [[Self.plainDefenderAbility], [Self.plainDefenderAbility]])
        XCTAssertEqual(result.candidates.map(\.abilityId), [Self.plainDefenderAbility, Self.plainDefenderAbility])
    }

    func testReverseSplitsCandidatesWhenOpponentDefenderAbilityNullifiesTheMove() async throws {
        let result = try await reverse(side: .defender, known: Self.attackerKey, unknown: Self.splitSpeciesKey)

        XCTAssertEqual(result.candidates.count, 4, "性格クラス × 特性の組(2)")
        XCTAssertEqual(result.candidates.map(\.natureClass), [.neutral, .plus, .neutral, .plus],
                       "一致する候補(mismatch 0)が先。定義順(性格クラス → 特性)は mismatch の昇順で安定に並べ替える")
        XCTAssertEqual(result.candidates.map(\.abilityIds), [
            [Self.deltaAbility], [Self.deltaAbility], [Self.immuneAbility], [Self.immuneAbility],
        ])
        XCTAssertEqual(result.candidates.map(\.exact), [true, true, false, false])
        XCTAssertEqual(result.candidates.map(\.mismatch), [0, 0, 2, 2], "無効の候補は観測(2件)をどれも説明できない")
        XCTAssertEqual(result.candidates.suffix(2).map(\.maxPercent), [0, 0])
        XCTAssertEqual(result.exactCount, 2)
    }

    func testReverseDoesNotSplitWhenOpponentIsTheAttacker() async throws {
        // 受けたダメージ(side=attacker): 相手 = 攻撃側。無効の特性は防御側の効果なので結果は変わらず、まとまる。
        let result = try await reverse(side: .attacker, known: Self.attackerKey, unknown: Self.splitSpeciesKey)
        XCTAssertEqual(result.candidates.count, 2)
        XCTAssertEqual(result.candidates.map(\.abilityIds), Array(repeating: [Self.deltaAbility, Self.immuneAbility], count: 2))
    }

    func testReversePinnedUnknownAbility() async throws {
        let result = try await reverse(
            side: .defender, known: Self.attackerKey, unknown: Self.splitSpeciesKey, unknownAbilityId: Self.immuneAbility
        )
        XCTAssertEqual(result.candidates.count, 2)
        XCTAssertEqual(result.candidates.map(\.abilityIds), [[Self.immuneAbility], [Self.immuneAbility]])
        XCTAssertEqual(result.exactCount, 0)

        let error = await assertThrowsPokeCalcError("相手が持たない特性") {
            try await self.reverse(
                side: .defender, known: Self.attackerKey, unknown: Self.splitSpeciesKey,
                unknownAbilityId: self.otherSpeciesAbility
            )
        }
        XCTAssertEqual(error?.code, PokeCalcError.Code.invalidInput)
    }
}
