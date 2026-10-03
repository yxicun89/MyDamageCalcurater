import Foundation

// MockFrequentOpponentsService: `FrequentOpponentsService` のモック(P6-23。XCUITest 用)。
// 挙動は起動時の環境変数 `POKECALC_MOCK_FREQUENT_OPPONENTS` で切り替える(`POKECALC_MOCK_DEVICE_DATA` と同じ流儀)。
// 架空の speciesKey だけを返す(`Resources/species.json` の 9003-000・9001-000 と、どこにも無い 9999-000)。

public enum MockFrequentOpponentsScenario: Equatable, Sendable {
    /// 3件(9003-000, 9001-000, 9999-000〈マスタに無い〉)。環境変数なし・未知の値の既定。
    case list
    /// 空配列。
    case empty
    /// 常に transport エラー。
    case failure

    /// 環境変数の値: `empty` / `fail`。nil・未知の値は `.list`。
    public init(environmentValue: String?) {
        switch environmentValue {
        case "empty": self = .empty
        case "fail": self = .failure
        default: self = .list
        }
    }
}

public actor MockFrequentOpponentsService: FrequentOpponentsService {
    public static let scenarioEnvironmentKey = "POKECALC_MOCK_FREQUENT_OPPONENTS"

    private let scenario: MockFrequentOpponentsScenario

    public init(scenario: MockFrequentOpponentsScenario = .list) {
        self.scenario = scenario
    }

    public init(environment: [String: String]) {
        self.init(scenario: MockFrequentOpponentsScenario(environmentValue: environment[Self.scenarioEnvironmentKey]))
    }

    private static let listKeys = ["9003-000", "9001-000", "9999-000"]
    /// 固定の時刻(モックは決定的にする)。
    private static let fixedDate = Date(timeIntervalSince1970: 1_790_000_000)

    public func frequentOpponents(limit: Int) async throws -> [FrequentOpponent] {
        switch scenario {
        case .empty:
            return []
        case .failure:
            throw PokeCalcError(code: PokeCalcError.Code.transport, message: "モックの通信エラー")
        case .list:
            return Self.listKeys.prefix(max(0, limit)).enumerated().map { index, key in
                FrequentOpponent(
                    speciesKey: key, score: Double(Self.listKeys.count - index), count: Self.listKeys.count - index,
                    lastCalculatedAt: Self.fixedDate)
            }
        }
    }
}
