import XCTest

@testable import PokeCalcCore

/// モックの架空データに持ち物の役割・メガストーン・メガ種族がある(ADR-0509 §9)。
/// XCUITest(モック強制)がメガの固定と役割の絞り込みを確かめられるようにするため。
final class MockPokeCalcServiceItemRolesTests: XCTestCase {
    private static let megaKey = "9001-001"
    private static let baseKey = "9001-000"
    private static let stoneID = "test-item-mega-stone"

    func testItemsCarryRolesAndMegaStoneFlag() async throws {
        let items = try await MockPokeCalcService().searchItems(query: "", limit: MasterSearch.pageLimit)
        let byID = Dictionary(uniqueKeysWithValues: items.map { ($0.id, $0) })
        // 既存の XCUITest が攻撃側・防御側の両方で使う持ち物は両方の役割。
        XCTAssertEqual(byID["test-item-berry"]?.roles, [.attacker, .defender])
        XCTAssertEqual(byID["test-item-unsupported"]?.roles, [.attacker, .defender])
        XCTAssertEqual(byID["test-item-attack-only"]?.roles, [.attacker])
        XCTAssertEqual(byID["test-item-defense-only"]?.roles, [.defender])
        XCTAssertEqual(byID["test-item-no-role"]?.roles, [])
        XCTAssertEqual(byID[Self.stoneID]?.roles, [])
        XCTAssertEqual(byID[Self.stoneID]?.isMegaStone, true)
        XCTAssertEqual(byID["test-item-berry"]?.isMegaStone, false)
    }

    func testMegaSpeciesDetail() async throws {
        let mock = try MockPokeCalcService()
        let mega = try await mock.species(key: Self.megaKey)
        XCTAssertTrue(mega.isMega)
        XCTAssertEqual(mega.requiredItemId, Self.stoneID)
        XCTAssertEqual(mega.baseSpeciesKey, Self.baseKey)
        let base = try await mock.species(key: Self.baseKey)
        XCTAssertEqual(mega.baseSpeciesNameJa, base.nameJa, "基本種名はモックの基本種の nameJa")
        XCTAssertFalse(base.isMega)
        XCTAssertNil(base.requiredItemId)
    }

    /// メガ種族は一覧の末尾(既定の攻撃側・防御側 = 先頭2件を変えない)。
    func testMegaSpeciesDoesNotChangeDefaultSelection() async throws {
        let species = try await MockPokeCalcService().searchSpecies(query: "", limit: MasterSearch.pageLimit)
        XCTAssertEqual(species.prefix(2).map(\.key), ["9001-000", "9002-000"])
        XCTAssertEqual(species.last?.key, Self.megaKey)
    }
}
