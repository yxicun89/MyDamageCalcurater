import Foundation
import XCTest

@testable import PokeCalcCore

/// P6-17(ADR-0501「P6-17」4章): `MockPokeCalcService` の未対応の印。
///
/// モックは印を**フィクスチャから**決める(技の `mechanisms`・持ち物の `unsupportedEffect`)。既定の技・持ち物では
/// 印なし(`[]`)のまま(既存の画面・XCUITest を変えない)。XCUITest(`UnsupportedMarksUITests`)は、この規則で
/// 印の付く架空の技・持ち物を選んで表示を確かめる。印の条件(急所・天候等。ADR-0123 §3)は再現しない。
final class MockPokeCalcServiceUnsupportedTests: XCTestCase {

    /// `Resources/*.json` の架空データ(ADR-0002)。
    private static let attackerKey = "9001-000"
    private static let defenderKey = "9002-000"
    private static let plainMoveID = "test-move-physical-a"
    private static let statusMoveID = "test-move-status-a"
    /// `mechanisms: ["multi_hit"]` を持つ架空の技(9001-000 の learnset の末尾)。
    private static let multiHitMoveID = "test-move-multi-hit"
    private static let plainItemID = "test-item-berry"
    /// `unsupportedEffect: true` の架空の持ち物。
    private static let unsupportedItemID = "test-item-unsupported"
    private static let neutralNatureID = "test-nature-neutral"

    private var mock: MockPokeCalcService!

    override func setUpWithError() throws {
        mock = try MockPokeCalcService()
    }

    private func attacker(itemId: String? = nil) -> Individual {
        Individual(speciesKey: Self.attackerKey, natureId: Self.neutralNatureID, sp: zeroSP, itemId: itemId)
    }

    private var multiHitMark: UnsupportedMark {
        UnsupportedMark(target: .move, reason: .multiHit, id: Self.multiHitMoveID)
    }

    // MARK: - フィクスチャ

    /// 前提: 印の付く技・持ち物がフィクスチャにあり、攻撃側(先頭の種族)が技を覚える。架空の名前(「テスト」始まり)。
    func testFixtureHasMarkedMoveAndItem() async throws {
        let move = try await mock.move(id: Self.multiHitMoveID)
        XCTAssertTrue(move.nameJa.hasPrefix("テスト"))
        XCTAssertNotEqual(move.category, .status, "変化技には印が付かないので攻撃技にする")
        let items = try await mock.searchItems(query: "", limit: 200)
        XCTAssertEqual(items.first?.id, Self.plainItemID, "既存の XCUITest が使う持ち物は先頭のまま")
        XCTAssertTrue(items.contains { $0.id == Self.unsupportedItemID })
        let attackerDetail = try await mock.species(key: Self.attackerKey)
        XCTAssertTrue(attackerDetail.learnset.contains(Self.multiHitMoveID))
        XCTAssertNotEqual(attackerDetail.learnset.first, Self.multiHitMoveID, "既定の技(先頭のダメージ技)を変えない")
    }

    // MARK: - 既定は印なし

    func testPlainMoveAndItemsHaveNoMarks() async throws {
        let calc = try await mock.calcDamage(CalcRequest(
            format: .single, attacker: attacker(itemId: Self.plainItemID),
            defender: Individual(speciesKey: Self.defenderKey, natureId: Self.neutralNatureID, sp: zeroSP),
            moveId: Self.plainMoveID))
        XCTAssertEqual(calc.unsupported, [])

        let bulk = try await mock.calcBulk(BulkCalcRequest(
            format: .single, attacker: attacker(), defenderSpeciesKey: Self.defenderKey, moveId: Self.plainMoveID,
            itemVariants: [nil, Self.plainItemID]))
        XCTAssertFalse(bulk.rows.isEmpty)
        XCTAssertTrue(bulk.rows.allSatisfy { $0.result.unsupported.isEmpty })

        let reverse = try await mock.reverse(ReverseRequest(
            format: .single, side: .defender, known: attacker(), unknownSpeciesKey: Self.defenderKey,
            moveId: Self.plainMoveID, itemCandidates: [nil, Self.plainItemID], observations: [.percent(40)]))
        XCTAssertTrue(reverse.candidates.allSatisfy { $0.unsupported.isEmpty })
    }

    /// 変化技には印を付けない(ADR-0123 §2。ダメージを持たない)。
    func testStatusMoveHasNoMoveMark() async throws {
        let bulk = try await mock.calcBulk(BulkCalcRequest(
            format: .single, attacker: attacker(), defenderSpeciesKey: Self.defenderKey, moveId: Self.statusMoveID))
        XCTAssertTrue(bulk.rows.allSatisfy { $0.result.unsupported.isEmpty })
    }

    // MARK: - 技の印(全行・全候補)

    func testMultiHitMoveMarksEveryResult() async throws {
        let calc = try await mock.calcDamage(CalcRequest(
            format: .single, attacker: attacker(),
            defender: Individual(speciesKey: Self.defenderKey, natureId: Self.neutralNatureID, sp: zeroSP),
            moveId: Self.multiHitMoveID))
        XCTAssertEqual(calc.unsupported, [multiHitMark])

        let bulk = try await mock.calcBulk(BulkCalcRequest(
            format: .single, attacker: attacker(), defenderSpeciesKey: Self.defenderKey, moveId: Self.multiHitMoveID))
        XCTAssertEqual(bulk.rows.count, 5, "前提: 物理技の既定5行")
        XCTAssertTrue(bulk.rows.allSatisfy { $0.result.unsupported == [multiHitMark] })

        let reverse = try await mock.reverse(ReverseRequest(
            format: .single, side: .defender, known: attacker(), unknownSpeciesKey: Self.defenderKey,
            moveId: Self.multiHitMoveID, observations: [.percent(40)]))
        XCTAssertFalse(reverse.candidates.isEmpty)
        XCTAssertTrue(reverse.candidates.allSatisfy { $0.unsupported == [multiHitMark] })
    }

    // MARK: - 持ち物の印(持つ側で target が決まる)

    /// 一括計算: 比較した防御側の持ち物の行だけ `defender_item`。並びは 技 → 防御側の持ち物(ADR-0123 §2)。
    func testBulkMarksOnlyRowsWithUnsupportedDefenderItem() async throws {
        let bulk = try await mock.calcBulk(BulkCalcRequest(
            format: .single, attacker: attacker(), defenderSpeciesKey: Self.defenderKey, moveId: Self.multiHitMoveID,
            presets: [.none], itemVariants: [nil, Self.plainItemID, Self.unsupportedItemID]))
        XCTAssertEqual(bulk.rows.map(\.itemId), [nil, Self.plainItemID, Self.unsupportedItemID])
        XCTAssertEqual(bulk.rows.map(\.result.unsupported), [
            [multiHitMark],
            [multiHitMark],
            [multiHitMark, UnsupportedMark(target: .defenderItem, reason: .unsupportedEffect, id: Self.unsupportedItemID)],
        ])
    }

    /// 攻撃側が持てば全行に `attacker_item`(技の印の後)。
    func testBulkMarksEveryRowWhenAttackerHoldsUnsupportedItem() async throws {
        let bulk = try await mock.calcBulk(BulkCalcRequest(
            format: .single, attacker: attacker(itemId: Self.unsupportedItemID),
            defenderSpeciesKey: Self.defenderKey, moveId: Self.multiHitMoveID))
        let expected = [multiHitMark,
                        UnsupportedMark(target: .attackerItem, reason: .unsupportedEffect, id: Self.unsupportedItemID)]
        XCTAssertTrue(bulk.rows.allSatisfy { $0.result.unsupported == expected }, "\(bulk.rows.map(\.result.unsupported))")
    }

    /// 1対1: 防御側の持ち物は `defender_item`。
    func testCalcDamageMarksDefenderItem() async throws {
        let calc = try await mock.calcDamage(CalcRequest(
            format: .single, attacker: attacker(),
            defender: Individual(speciesKey: Self.defenderKey, natureId: Self.neutralNatureID, sp: zeroSP,
                                 itemId: Self.unsupportedItemID),
            moveId: Self.plainMoveID))
        XCTAssertEqual(calc.unsupported,
                       [UnsupportedMark(target: .defenderItem, reason: .unsupportedEffect, id: Self.unsupportedItemID)])
    }

    /// 逆算: side=defender(与えたダメージ)では相手 = 防御側なので、候補の持ち物は `defender_item`。
    /// side=attacker(受けたダメージ)では相手 = 攻撃側なので `attacker_item`。
    func testReverseMarksCandidateItemBySide() async throws {
        let defenderSide = try await mock.reverse(ReverseRequest(
            format: .single, side: .defender, known: attacker(), unknownSpeciesKey: Self.defenderKey,
            moveId: Self.plainMoveID, itemCandidates: [nil, Self.unsupportedItemID], observations: [.percent(40)]))
        for candidate in defenderSide.candidates {
            let expected = candidate.itemId == Self.unsupportedItemID
                ? [UnsupportedMark(target: .defenderItem, reason: .unsupportedEffect, id: Self.unsupportedItemID)]
                : []
            XCTAssertEqual(candidate.unsupported, expected, "\(candidate.natureClass)@\(candidate.itemId ?? "-")")
        }

        let known = Individual(speciesKey: Self.defenderKey, natureId: Self.neutralNatureID, sp: zeroSP)
        let attackerSide = try await mock.reverse(ReverseRequest(
            format: .single, side: .attacker, known: known, unknownSpeciesKey: Self.attackerKey,
            moveId: Self.plainMoveID, itemCandidates: [nil, Self.unsupportedItemID], observations: [.percent(40)]))
        for candidate in attackerSide.candidates {
            let expected = candidate.itemId == Self.unsupportedItemID
                ? [UnsupportedMark(target: .attackerItem, reason: .unsupportedEffect, id: Self.unsupportedItemID)]
                : []
            XCTAssertEqual(candidate.unsupported, expected, "\(candidate.natureClass)@\(candidate.itemId ?? "-")")
        }
    }

    /// 逆算: 既知側(自分)の持ち物の印は side で target が決まる。side=defender は既知側 = 攻撃側なので
    /// `attacker_item`、side=attacker は既知側 = 防御側なので `defender_item`(4章(2))。すべての候補に付く。
    func testReverseMarksKnownSideItemByRole() async throws {
        let defenderSide = try await mock.reverse(ReverseRequest(
            format: .single, side: .defender, known: attacker(itemId: Self.unsupportedItemID),
            unknownSpeciesKey: Self.defenderKey, moveId: Self.plainMoveID, observations: [.percent(40)]))
        let expectedAttackerItemMark = [UnsupportedMark(target: .attackerItem, reason: .unsupportedEffect, id: Self.unsupportedItemID)]
        XCTAssertFalse(defenderSide.candidates.isEmpty)
        XCTAssertTrue(defenderSide.candidates.allSatisfy { $0.unsupported == expectedAttackerItemMark },
                      "\(defenderSide.candidates.map(\.unsupported))")

        let known = Individual(speciesKey: Self.defenderKey, natureId: Self.neutralNatureID, sp: zeroSP,
                               itemId: Self.unsupportedItemID)
        let attackerSide = try await mock.reverse(ReverseRequest(
            format: .single, side: .attacker, known: known, unknownSpeciesKey: Self.attackerKey,
            moveId: Self.plainMoveID, observations: [.percent(40)]))
        let expectedDefenderItemMark = [UnsupportedMark(target: .defenderItem, reason: .unsupportedEffect, id: Self.unsupportedItemID)]
        XCTAssertFalse(attackerSide.candidates.isEmpty)
        XCTAssertTrue(attackerSide.candidates.allSatisfy { $0.unsupported == expectedDefenderItemMark },
                      "\(attackerSide.candidates.map(\.unsupported))")
    }
}
