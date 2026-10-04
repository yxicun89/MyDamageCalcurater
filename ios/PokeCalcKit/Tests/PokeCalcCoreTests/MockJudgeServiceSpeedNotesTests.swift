import XCTest

@testable import PokeCalcCore

/// `MockJudgeService` の `speed-notes` シナリオ(ADR-0512。XCUITest が頼る固定の事実)。位置 i = 候補の index(0 始まり)。
///  - 自分の Applied: 全行 [rank, tailwind]。自分の状態異常が paralysis なら末尾に paralysis(全行)。
///  - 自分の Ignored: i = 1 の行だけ [abilityId]。
///  - 候補の Applied: i = 0 は [choiceScarf]、それ以外は []。その候補の状態異常が paralysis なら末尾に paralysis(その行だけ)。
///  - 候補の Ignored: i = 1 の行だけ [itemId, fieldWeather]。
/// 既定(normal)・marks など他のシナリオでは4欄とも空(既存のテスト・画面を変えない)。
final class MockJudgeServiceSpeedNotesTests: XCTestCase {
    private static func individual(_ key: String, status: JudgeStatus? = nil) -> JudgeIndividual {
        JudgeIndividual(speciesKey: key, natureId: "mock-nature", sp: zeroSP, status: status)
    }

    private static func request(attackerStatus: JudgeStatus? = nil, defenderStatuses: [JudgeStatus?]) -> JudgeRequest {
        JudgeRequest(
            attacker: individual("9001-000", status: attackerStatus), moveId: "mock-move-self",
            defenders: defenderStatuses.enumerated().map {
                JudgeDefender(individual: individual("9002-000", status: $0.element), moveId: "mock-move-\($0.offset)")
            })
    }

    private func matchups(_ scenario: MockJudgeScenario, _ request: JudgeRequest) async throws -> [JudgeMatchup] {
        try await MockJudgeService(scenario: scenario).outspeedAndKo(request).matchups
    }

    func testScenarioFromEnvironmentValue() {
        XCTAssertEqual(MockJudgeScenario(environmentValue: "speed-notes"), .speedNotes)
        let service = MockJudgeService(environment: [MockJudgeService.scenarioEnvironmentKey: "speed-notes"])
        XCTAssertNotNil(service)
    }

    func testSpeedNotesScenarioReturnsFixedAppliedAndIgnoredPerSide() async throws {
        let rows = try await matchups(.speedNotes, Self.request(defenderStatuses: [nil, nil, nil]))
        XCTAssertEqual(rows.map(\.attackerSpeedApplied), [["rank", "tailwind"], ["rank", "tailwind"], ["rank", "tailwind"]])
        XCTAssertEqual(rows.map(\.attackerSpeedIgnored), [[], ["abilityId"], []])
        XCTAssertEqual(rows.map(\.defenderSpeedApplied), [["choiceScarf"], [], []])
        XCTAssertEqual(rows.map(\.defenderSpeedIgnored), [[], ["itemId", "fieldWeather"], []])
    }

    func testParalysisInTheRequestAddsParalysisAtTheEndOfThatSidesApplied() async throws {
        let attackerParalyzed = try await matchups(.speedNotes, Self.request(attackerStatus: .paralysis, defenderStatuses: [nil, nil]))
        XCTAssertEqual(attackerParalyzed.map(\.attackerSpeedApplied), [["rank", "tailwind", "paralysis"], ["rank", "tailwind", "paralysis"]])
        XCTAssertEqual(attackerParalyzed.map(\.defenderSpeedApplied), [["choiceScarf"], []], "自分のまひは候補側に出ない")

        let candidateParalyzed = try await matchups(.speedNotes, Self.request(defenderStatuses: [.paralysis, .paralysis]))
        XCTAssertEqual(candidateParalyzed.map(\.defenderSpeedApplied), [["choiceScarf", "paralysis"], ["paralysis"]])
        XCTAssertEqual(candidateParalyzed.map(\.attackerSpeedApplied), [["rank", "tailwind"], ["rank", "tailwind"]], "候補のまひは自分側に出ない")

        let onlySecond = try await matchups(.speedNotes, Self.request(defenderStatuses: [nil, .paralysis]))
        XCTAssertEqual(onlySecond.map(\.defenderSpeedApplied), [["choiceScarf"], ["paralysis"]], "候補ごと(位置で取り違えない)")
    }

    func testOtherStatusesAddNothing() async throws {
        for status in JudgeStatus.allCases where status != .paralysis {
            let rows = try await matchups(.speedNotes, Self.request(attackerStatus: status, defenderStatuses: [status]))
            XCTAssertEqual(rows[0].attackerSpeedApplied, ["rank", "tailwind"], status.rawValue)
            XCTAssertEqual(rows[0].defenderSpeedApplied, ["choiceScarf"], status.rawValue)
        }
    }

    func testOtherScenariosKeepAllFourFieldsEmpty() async throws {
        for scenario in [MockJudgeScenario.normal, .marks] {
            let rows = try await matchups(scenario, Self.request(attackerStatus: .paralysis, defenderStatuses: [.paralysis, nil]))
            for row in rows {
                XCTAssertEqual(row.attackerSpeedApplied, [])
                XCTAssertEqual(row.defenderSpeedApplied, [])
                XCTAssertEqual(row.attackerSpeedIgnored, [])
                XCTAssertEqual(row.defenderSpeedIgnored, [])
            }
        }
    }

    func testSpeedNotesScenarioKeepsTheNormalNumbers() async throws {
        let normal = try await matchups(.normal, Self.request(defenderStatuses: [nil, nil, nil]))
        let notes = try await matchups(.speedNotes, Self.request(defenderStatuses: [nil, nil, nil]))
        XCTAssertEqual(notes.map(\.attackerSpeed), normal.map(\.attackerSpeed))
        XCTAssertEqual(notes.map(\.defenderSpeed), normal.map(\.defenderSpeed))
        XCTAssertEqual(notes.map(\.attackerKo), normal.map(\.attackerKo))
        XCTAssertEqual(notes.map(\.defenderKo), normal.map(\.defenderKo))
    }
}
