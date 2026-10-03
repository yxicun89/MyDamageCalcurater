import XCTest

@testable import PokeCalcCore

/// P6-23: `MockFrequentOpponentsService`(XCUITest 用)。環境変数 `POKECALC_MOCK_FREQUENT_OPPONENTS`。
final class MockFrequentOpponentsServiceTests: XCTestCase {
    func testEnvironmentValueMapsToScenario() {
        XCTAssertEqual(MockFrequentOpponentsService.scenarioEnvironmentKey, "POKECALC_MOCK_FREQUENT_OPPONENTS")
        XCTAssertEqual(MockFrequentOpponentsScenario(environmentValue: nil), .list)
        XCTAssertEqual(MockFrequentOpponentsScenario(environmentValue: "unknown"), .list)
        XCTAssertEqual(MockFrequentOpponentsScenario(environmentValue: "empty"), .empty)
        XCTAssertEqual(MockFrequentOpponentsScenario(environmentValue: "fail"), .failure)
    }

    func testListScenarioReturnsFictionalKeysInOrderWithOneUnresolvable() async throws {
        let service = MockFrequentOpponentsService(environment: [:])
        let list = try await service.frequentOpponents(limit: FrequentOpponents.defaultLimit)
        XCTAssertEqual(list.map(\.speciesKey), ["9003-000", "9001-000", "9999-000"])
        XCTAssertEqual(list.map(\.score), list.map(\.score).sorted(by: >), "スコア降順")
    }

    func testListScenarioHonorsLimit() async throws {
        let list = try await MockFrequentOpponentsService(scenario: .list).frequentOpponents(limit: 2)
        XCTAssertEqual(list.map(\.speciesKey), ["9003-000", "9001-000"])
    }

    func testEmptyScenarioReturnsEmptyArray() async throws {
        let list = try await MockFrequentOpponentsService(environment: [MockFrequentOpponentsService.scenarioEnvironmentKey: "empty"])
            .frequentOpponents(limit: 10)
        XCTAssertEqual(list, [])
    }

    func testFailScenarioThrowsTransportError() async throws {
        let service = MockFrequentOpponentsService(environment: [MockFrequentOpponentsService.scenarioEnvironmentKey: "fail"])
        let error = await assertThrowsPokeCalcError("fail") { try await service.frequentOpponents(limit: 10) }
        XCTAssertEqual(error?.code, PokeCalcError.Code.transport)
    }

    func testMockKeysUseFictionalRangeOnly() async throws {
        for key in try await MockFrequentOpponentsService(scenario: .list).frequentOpponents(limit: 50).map(\.speciesKey) {
            XCTAssertTrue(key.hasPrefix("9"), "架空データ(9xxx)だけを使う: \(key)")
        }
    }
}
