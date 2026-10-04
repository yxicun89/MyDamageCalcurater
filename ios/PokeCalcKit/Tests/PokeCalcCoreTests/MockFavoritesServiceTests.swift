import XCTest

@testable import PokeCalcCore

/// `MockFavoritesService`(XCUITest 用。環境変数 `POKECALC_MOCK_FAVORITES`。ADR-0511)。
/// 既定は「空のストアで追加・削除が動く」(既存テストを壊さない)。
final class MockFavoritesServiceTests: XCTestCase {
    private let individual = Individual(
        speciesKey: "9001-000", natureId: "test-nature-neutral",
        sp: StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0))

    private func otherIndividual() -> Individual {
        var other = individual
        other.speciesKey = "9002-000"
        return other
    }

    func testEnvironmentValueMapsToScenario() {
        XCTAssertEqual(MockFavoritesService.scenarioEnvironmentKey, "POKECALC_MOCK_FAVORITES")
        XCTAssertEqual(MockFavoritesScenario(environmentValue: nil), .emptyStore)
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "unknown"), .emptyStore)
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "list"), .list)
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "fail"), .failure)
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "unavailable"), .unavailable)
        XCTAssertEqual(MockFavoritesScenario(environmentValue: "full"), .full)
    }

    func testDefaultIsEmptyStore() async throws {
        let list = try await MockFavoritesService(environment: [:]).favorites()
        XCTAssertEqual(list, [])
    }

    func testListScenarioReturnsTwoFictionalFavoritesNewestFirst() async throws {
        let list = try await MockFavoritesService(scenario: .list).favorites()
        XCTAssertEqual(list.map(\.id), ["102", "101"])
        XCTAssertEqual(list.map(\.label), ["HB特化", nil])
        XCTAssertEqual(list.map(\.individual.speciesKey), ["9002-000", "9003-000"])
        if list.count == 2 { XCTAssertGreaterThan(list[0].updatedAt, list[1].updatedAt) }
    }

    func testAddCreatesAndAppearsFirstThenSamePinIsAlreadyPinned() async throws {
        let service = MockFavoritesService(scenario: .emptyStore)
        let created = try await service.addFavorite(label: nil, individual: individual)
        guard case .created(let first) = created else { return XCTFail("新規は .created") }
        XCTAssertEqual(first.individual, individual)

        let second = try await service.addFavorite(label: nil, individual: otherIndividual())
        guard case .created(let secondFavorite) = second else { return XCTFail("別の内容は .created") }
        var list = try await service.favorites()
        XCTAssertEqual(list.map(\.id), [secondFavorite.id, first.id], "新しい順")

        let again = try await service.addFavorite(label: nil, individual: individual)
        guard case .alreadyPinned(let existing) = again else { return XCTFail("同じ内容は .alreadyPinned") }
        XCTAssertEqual(existing.id, first.id, "新しく作らず既存を返す")
        list = try await service.favorites()
        XCTAssertEqual(list.map(\.id), [first.id, secondFavorite.id], "再ピン留めで先頭へ(updatedAt が進む)")
    }

    func testRemoveDeletesAndUnknownIDIsNotFound() async throws {
        let service = MockFavoritesService(scenario: .list)
        try await service.removeFavorite(id: "102")
        let list = try await service.favorites()
        XCTAssertEqual(list.map(\.id), ["101"])
        let error = await assertThrowsPokeCalcError("2回目は 404") { try await service.removeFavorite(id: "102") }
        XCTAssertEqual(error?.code, "not_found")
    }

    func testFailureScenarioThrowsTransportOnEveryOperation() async {
        let service = MockFavoritesService(scenario: .failure)
        let listError = await assertThrowsPokeCalcError("list") { try await service.favorites() }
        let addError = await assertThrowsPokeCalcError("add") { try await service.addFavorite(label: nil, individual: self.individual) }
        let removeError = await assertThrowsPokeCalcError("remove") { try await service.removeFavorite(id: "1") }
        for error in [listError, addError, removeError] {
            XCTAssertEqual(error?.code, PokeCalcError.Code.transport)
        }
    }

    func testUnavailableScenarioThrowsStoreUnavailableOnEveryOperation() async {
        let service = MockFavoritesService(scenario: .unavailable)
        let listError = await assertThrowsPokeCalcError("list") { try await service.favorites() }
        let addError = await assertThrowsPokeCalcError("add") { try await service.addFavorite(label: nil, individual: self.individual) }
        let removeError = await assertThrowsPokeCalcError("remove") { try await service.removeFavorite(id: "1") }
        for error in [listError, addError, removeError] {
            XCTAssertEqual(error?.code, "store_unavailable")
        }
    }

    func testFullScenarioHasLimitItemsAndRejectsNewButAcceptsDuplicates() async throws {
        let service = MockFavoritesService(scenario: .full)
        let list = try await service.favorites()
        XCTAssertEqual(list.count, RequestLimits.maxFavorites)
        XCTAssertEqual(Set(list.map(\.id)).count, RequestLimits.maxFavorites, "id は重複しない")
        let error = await assertThrowsPokeCalcError("上限") { try await service.addFavorite(label: nil, individual: self.individual) }
        XCTAssertEqual(error?.code, "invalid_input", "上限到達は 400 invalid_input(契約どおり)")
        let head = try XCTUnwrap(list.first)
        try await service.removeFavorite(id: head.id)
        let afterRemove = try await service.addFavorite(label: nil, individual: individual)
        guard case .created = afterRemove else { return XCTFail("1件外せば追加できる") }
    }

    func testMockUsesFictionalKeysOnly() async throws {
        for scenario in [MockFavoritesScenario.list, .full] {
            for favorite in try await MockFavoritesService(scenario: scenario).favorites() {
                XCTAssertTrue(favorite.individual.speciesKey.hasPrefix("9"), "架空データ(9xxx)だけを使う")
            }
        }
    }
}
