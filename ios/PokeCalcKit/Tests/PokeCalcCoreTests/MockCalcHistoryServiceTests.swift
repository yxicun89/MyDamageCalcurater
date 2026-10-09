import XCTest

@testable import PokeCalcCore

/// `MockCalcHistoryService`(XCUITest 用。環境変数 `POKECALC_MOCK_CALC_HISTORY`)。架空の key だけを返す。
final class MockCalcHistoryServiceTests: XCTestCase {
    func testEnvironmentValueMapsToScenario() {
        XCTAssertEqual(MockCalcHistoryService.scenarioEnvironmentKey, "POKECALC_MOCK_CALC_HISTORY")
        XCTAssertEqual(MockCalcHistoryScenario(environmentValue: nil), .list)
        XCTAssertEqual(MockCalcHistoryScenario(environmentValue: "unknown"), .list)
        XCTAssertEqual(MockCalcHistoryScenario(environmentValue: "paged"), .paged)
        XCTAssertEqual(MockCalcHistoryScenario(environmentValue: "paged-fail-more"), .pagedFailMore)
        XCTAssertEqual(MockCalcHistoryScenario(environmentValue: "empty"), .empty)
        XCTAssertEqual(MockCalcHistoryScenario(environmentValue: "fail"), .failure)
        XCTAssertEqual(MockCalcHistoryScenario(environmentValue: "unavailable"), .unavailable)
    }

    func testListScenarioReturnsThreeNewestFirstInOnePage() async throws {
        let page = try await MockCalcHistoryService(scenario: .list).calcHistory(limit: 20, cursor: nil)
        XCTAssertEqual(page.items.count, 3)
        XCTAssertNil(page.nextCursor)
        XCTAssertEqual(page.items.map(\.occurredAt), page.items.map(\.occurredAt).sorted(by: >), "新しい順")
    }

    func testPagedScenarioHasTwoPagesAndFollowsCursor() async throws {
        let service = MockCalcHistoryService(scenario: .paged)
        let first = try await service.calcHistory(limit: 20, cursor: nil)
        XCTAssertEqual(first.items.count, MockCalcHistoryService.pagedPageSize)
        let cursor = try XCTUnwrap(first.nextCursor)
        let second = try await service.calcHistory(limit: 20, cursor: cursor)
        XCTAssertEqual(second.items.count, 2)
        XCTAssertNil(second.nextCursor)
        XCTAssertNotEqual(first.items.first?.occurredAt, second.items.first?.occurredAt)
        XCTAssertGreaterThan(first.items.last!.occurredAt, second.items.first!.occurredAt, "続きはより古い")
    }

    func testBadCursorIsInvalidInput() async throws {
        let service = MockCalcHistoryService(scenario: .paged)
        let error = await assertThrowsPokeCalcError("cursor") { try await service.calcHistory(limit: 20, cursor: "garbage") }
        XCTAssertEqual(error?.code, "invalid_input")
    }

    func testEmptyFailureAndUnavailable() async throws {
        let empty = try await MockCalcHistoryService(scenario: .empty).calcHistory(limit: 20, cursor: nil)
        XCTAssertEqual(empty, CalcHistoryPage(items: [], nextCursor: nil))
        let failing = MockCalcHistoryService(scenario: .failure)
        let transport = await assertThrowsPokeCalcError("fail") { try await failing.calcHistory(limit: 20, cursor: nil) }
        XCTAssertEqual(transport?.code, PokeCalcError.Code.transport)
        let unavailable = MockCalcHistoryService(scenario: .unavailable)
        let down = await assertThrowsPokeCalcError("503") { try await unavailable.calcHistory(limit: 20, cursor: nil) }
        XCTAssertEqual(down.map(RecordScreenError.init), .storeUnavailable)
    }

    func testPagedFailMoreFailsOnlyContinuation() async throws {
        let service = MockCalcHistoryService(scenario: .pagedFailMore)
        let first = try await service.calcHistory(limit: 20, cursor: nil)
        let cursor = try XCTUnwrap(first.nextCursor)
        let error = await assertThrowsPokeCalcError("more") { try await service.calcHistory(limit: 20, cursor: cursor) }
        XCTAssertEqual(error.map(RecordScreenError.init), .storeUnavailable)
    }

    func testUsesFictionalKeysResolvableInMockMaster() async throws {
        let master = try MockPokeCalcService()
        let page = try await MockCalcHistoryService(scenario: .paged).calcHistory(limit: 50, cursor: nil)
        let second = try await MockCalcHistoryService(scenario: .paged).calcHistory(limit: 50, cursor: page.nextCursor)
        for entry in page.items + second.items {
            for key in [entry.calc.attacker.speciesKey, entry.calc.defender.speciesKey] {
                XCTAssertTrue(key.hasPrefix("900"), "架空の key だけ: \(key)")
                _ = try await master.species(key: key)
            }
            _ = try await master.move(id: entry.calc.moveId)
        }
    }
}
