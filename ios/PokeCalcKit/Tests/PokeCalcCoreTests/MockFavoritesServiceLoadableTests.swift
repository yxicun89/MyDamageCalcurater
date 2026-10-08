import XCTest

@testable import PokeCalcCore

/// `MockFavoritesService` の読み込み用シナリオ `loadable`(ADR-0513 §9。XCUITest が使う)。
/// 既存のシナリオの既定(未設定=空のストア・`list` の2件)は変えない。
final class MockFavoritesServiceLoadableTests: XCTestCase {

    func testEnvironmentValueSelectsLoadableAndOthersAreUnchanged() {
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "loadable"), .loadable)
        XCTAssertEqual(MockFavoritesScenario(environmentValue: nil), .emptyStore, "既定は変えない")
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "list"), .list)
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "fail"), .failure)
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "unavailable"), .unavailable)
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "full"), .full)
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "loadable-typo"), .emptyStore)
    }

    func testLoadableReturnsFourFavoritesNewestFirst() async throws {
        let favorites = try await MockFavoritesService(scenario: .loadable).favorites()
        XCTAssertEqual(favorites.map(\.id), ["304", "303", "302", "301"])
        XCTAssertEqual(favorites.map(\.label), ["読み込める", "一部だけ", "種族なし", nil])
        XCTAssertEqual(favorites.map(\.individual.speciesKey), ["9002-000", "9004-000", "9999-000", "9003-000"])
    }

    func testLoadableFirstIsFullyLoadableAgainstTheMockMaster() async throws {
        let first = try await MockFavoritesService(scenario: .loadable).favorites()[0].individual
        XCTAssertEqual(first.natureId, "test-nature-atk-up")
        XCTAssertEqual(first.abilityId, "test-ability-beta")
        XCTAssertEqual(first.itemId, "test-item-berry")
        XCTAssertEqual(first.sp, StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 2, spe: 32))
        let sp = first.sp
        XCTAssertLessThanOrEqual(sp.hp + sp.atk + sp.def + sp.spa + sp.spd + sp.spe, SPLimits.maxTotal)
    }

    func testLoadableSecondHasAbilityAndItemMissingFromMock() async throws {
        let second = try await MockFavoritesService(scenario: .loadable).favorites()[1].individual
        XCTAssertEqual(second.abilityId, "test-ability-gone")
        XCTAssertEqual(second.itemId, "test-item-gone")
    }

    func testLoadableStillSupportsAddAndRemove() async throws {
        let service = MockFavoritesService(scenario: .loadable)
        let individual = Individual(
            speciesKey: "9001-000", natureId: "test-nature-neutral",
            sp: StatBlock(hp: 1, atk: 0, def: 0, spa: 0, spd: 0, spe: 0))
        guard case .created(let created) = try await service.addFavorite(label: nil, individual: individual) else {
            return XCTFail("新規は created")
        }
        XCTAssertEqual(created.id, "305", "id は既存の続き")
        try await service.removeFavorite(id: "301")
        let ids = try await service.favorites().map(\.id)
        XCTAssertEqual(ids, ["305", "304", "303", "302"])
    }

    func testDefaultStoreIsStillEmptyAndListUnchanged() async throws {
        let empty = try await MockFavoritesService(environment: [:]).favorites()
        XCTAssertTrue(empty.isEmpty)
        let list = try await MockFavoritesService(environment: ["POKECALC_MOCK_FAVORITES": "list"]).favorites()
        XCTAssertEqual(list.map(\.id), ["102", "101"])
    }
}
