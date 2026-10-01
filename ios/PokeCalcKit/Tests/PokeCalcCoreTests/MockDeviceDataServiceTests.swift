import XCTest

@testable import PokeCalcCore

/// P6-7: モックの挙動切り替え(XCUITest が partial→completed・失敗→再試行を通せるようにする)。
final class MockDeviceDataServiceTests: XCTestCase {
    func testEnvironmentKey() {
        XCTAssertEqual(MockDeviceDataService.scenarioEnvironmentKey, "POKECALC_MOCK_DEVICE_DATA")
    }

    func testScenarioParsing() {
        XCTAssertEqual(MockDeviceDataScenario(environmentValue: nil), .immediate)
        XCTAssertEqual(MockDeviceDataScenario(environmentValue: "unknown"), .immediate)
        XCTAssertEqual(MockDeviceDataScenario(environmentValue: "partial"), .partialThenCompleted)
        XCTAssertEqual(MockDeviceDataScenario(environmentValue: "fail-once"), .failOnceThenCompleted)
    }

    func testImmediateCompletesBothTargets() async throws {
        let mock = MockDeviceDataService(scenario: .immediate)
        let record = try await mock.deleteRecordDeviceData()
        let team = try await mock.deleteTeamDeviceData()
        XCTAssertEqual(record, .completed)
        XCTAssertEqual(team, .completed)
    }

    func testPartialThenCompletedPerTargetAndIdempotentAfter() async throws {
        let mock = MockDeviceDataService(scenario: .partialThenCompleted)
        var record: [DeletionProgress] = []
        var team: [DeletionProgress] = []
        for _ in 0..<3 {
            record.append(try await mock.deleteRecordDeviceData())
            team.append(try await mock.deleteTeamDeviceData())
        }
        XCTAssertEqual(record, [.partial, .completed, .completed])
        XCTAssertEqual(team, [.partial, .completed, .completed])
    }

    func testFailOnceThenCompletedFailsOnlyTheFirstRecordCall() async throws {
        let mock = MockDeviceDataService(scenario: .failOnceThenCompleted)
        let error = await assertThrowsPokeCalcError("record 1回目") { try await mock.deleteRecordDeviceData() }
        XCTAssertEqual(error?.code, PokeCalcError.Code.transport)
        let second = try await mock.deleteRecordDeviceData()
        XCTAssertEqual(second, .completed)
        let team = try await mock.deleteTeamDeviceData()
        XCTAssertEqual(team, .completed, "team は常に成功")
    }
}
