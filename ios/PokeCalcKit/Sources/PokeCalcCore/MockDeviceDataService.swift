// MockDeviceDataService: `DeviceDataService` のモック(P6-7。XCUITest 用)。
// 挙動は起動時の環境変数 `POKECALC_MOCK_DEVICE_DATA` で切り替える(`POKECALC_USE_MOCK` と同じ流儀)。

public enum MockDeviceDataScenario: Equatable, Sendable {
    /// 各対象が最初の要求で `completed`(環境変数なし・未知の値の既定)。
    case immediate
    /// 各対象が1回目 `partial`、2回目 `completed`。
    case partialThenCompleted
    /// record の1回目だけ transport エラー、以後は成功。team は常に成功。
    case failOnceThenCompleted

    /// 環境変数の値: `partial` / `fail-once`。nil・未知の値は `.immediate`。
    public init(environmentValue: String?) {
        switch environmentValue {
        case "partial": self = .partialThenCompleted
        case "fail-once": self = .failOnceThenCompleted
        default: self = .immediate
        }
    }
}

public actor MockDeviceDataService: DeviceDataService {
    public static let scenarioEnvironmentKey = "POKECALC_MOCK_DEVICE_DATA"

    private let scenario: MockDeviceDataScenario
    private var recordCalls = 0
    private var teamCalls = 0

    public init(scenario: MockDeviceDataScenario = .immediate) {
        self.scenario = scenario
    }

    /// 環境変数 `POKECALC_MOCK_DEVICE_DATA` から作る(`POKECALC_USE_MOCK` と同じ流儀)。
    public init(environment: [String: String]) {
        self.init(scenario: MockDeviceDataScenario(environmentValue: environment[Self.scenarioEnvironmentKey]))
    }

    public func deleteRecordDeviceData() async throws -> DeletionProgress {
        recordCalls += 1
        switch scenario {
        case .immediate: return .completed
        case .partialThenCompleted: return recordCalls == 1 ? .partial : .completed
        case .failOnceThenCompleted:
            if recordCalls == 1 {
                throw PokeCalcError(code: PokeCalcError.Code.transport, message: "モック: 1回目だけ失敗")
            }
            return .completed
        }
    }

    public func deleteTeamDeviceData() async throws -> DeletionProgress {
        teamCalls += 1
        switch scenario {
        case .immediate, .failOnceThenCompleted: return .completed
        case .partialThenCompleted: return teamCalls == 1 ? .partial : .completed
        }
    }
}
