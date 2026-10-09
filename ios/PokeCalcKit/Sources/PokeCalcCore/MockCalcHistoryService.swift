import Foundation

// MockCalcHistoryService: `CalcHistoryService` のモック(XCUITest 用。ADR-0519)。
// 挙動は起動時の環境変数 `POKECALC_MOCK_CALC_HISTORY` で切り替える(`POKECALC_MOCK_FAVORITES` と同じ流儀)。
// 架空の speciesKey(9001〜9004)・技(`test-move-*`)・性格(`test-nature-*`)だけを使う。実データは使わない。
// カーソルはこのモックだけの不透明な文字列(`mock-<開始位置>`)。画面は中身を見ない。

public enum MockCalcHistoryScenario: Equatable, Sendable {
    /// 3件を1ページで返す(続きなし)。環境変数なし・未知の値の既定。
    case list
    /// 4件を **1ページ2件** で返す(2ページ。`limit` が大きくても2件ずつ。UI で「もっと見る」を確かめるため)。
    case paged
    /// `paged` の1ページ目は成功し、続き(cursor つき)は常に 503 `store_unavailable`。
    case pagedFailMore
    /// 空のページ(`items: []`、`nextCursor: nil`)。
    case empty
    /// 常に transport エラー。
    case failure
    /// 常に 503 `store_unavailable`。
    case unavailable

    /// 環境変数の値: `paged` / `paged-fail-more` / `empty` / `fail` / `unavailable`。nil・未知の値は `.list`。
    public init(environmentValue: String?) {
        switch environmentValue {
        case "paged": self = .paged
        case "paged-fail-more": self = .pagedFailMore
        case "empty": self = .empty
        case "fail": self = .failure
        case "unavailable": self = .unavailable
        default: self = .list
        }
    }
}

public actor MockCalcHistoryService: CalcHistoryService {
    public static let scenarioEnvironmentKey = "POKECALC_MOCK_CALC_HISTORY"

    /// `paged` 系の1ページの件数(契約の `limit` とは無関係に固定する)。
    static let pagedPageSize = 2
    private static let cursorPrefix = "mock-"
    /// 固定の時刻(モックは決定的にする)。新しい順に1時間ずつ古くなる。
    private static let newestTime: TimeInterval = 1_790_000_000

    private let scenario: MockCalcHistoryScenario

    public init(scenario: MockCalcHistoryScenario = .list) {
        self.scenario = scenario
    }

    public init(environment: [String: String]) {
        self.init(scenario: MockCalcHistoryScenario(environmentValue: environment[Self.scenarioEnvironmentKey]))
    }

    /// 新しい順の全行(`list` は先頭3件、`paged` 系は4件)。
    private static let rows: [CalcHistoryEntry] = [
        make(0, attacker: "9002-000", defender: "9003-000", move: "test-move-special-b", min: 41.2, max: 48.9),
        make(1, attacker: "9003-000", defender: "9001-000", move: "test-move-special-a", min: 87.6, max: 103.4),
        make(2, attacker: "9001-000", defender: "9004-000", move: "test-move-physical-a", min: 12.5, max: 15.0),
        make(3, attacker: "9004-000", defender: "9002-000", move: "test-move-physical-a", min: 30.1, max: 36.8),
    ]

    private static func make(
        _ index: Int, attacker: String, defender: String, move: String, min: Double, max: Double
    ) -> CalcHistoryEntry {
        func individual(_ key: String) -> Individual {
            Individual(
                speciesKey: key, natureId: "test-nature-neutral",
                sp: StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 2, spe: 32))
        }
        return CalcHistoryEntry(
            occurredAt: Date(timeIntervalSince1970: newestTime - TimeInterval(index) * 3600),
            calc: CalcHistoryCalc(
                format: .single, attacker: individual(attacker), defender: individual(defender), moveId: move),
            minPercent: min, maxPercent: max)
    }

    public func calcHistory(limit: Int, cursor: String?) async throws -> CalcHistoryPage {
        switch scenario {
        case .empty:
            return CalcHistoryPage(items: [], nextCursor: nil)
        case .failure:
            throw PokeCalcError(code: PokeCalcError.Code.transport, message: "モックの通信エラー")
        case .unavailable:
            throw PokeCalcError(code: "store_unavailable", message: "モックの保存先の障害")
        case .list:
            let items = Array(Self.rows.prefix(3))
            return try page(of: items, pageSize: max(1, limit), cursor: cursor)
        case .paged:
            return try page(of: Self.rows, pageSize: Self.pagedPageSize, cursor: cursor)
        case .pagedFailMore:
            if cursor != nil {
                throw PokeCalcError(code: "store_unavailable", message: "モックの保存先の障害")
            }
            return try page(of: Self.rows, pageSize: Self.pagedPageSize, cursor: cursor)
        }
    }

    private func page(of items: [CalcHistoryEntry], pageSize: Int, cursor: String?) throws -> CalcHistoryPage {
        var start = 0
        if let cursor {
            guard cursor.hasPrefix(Self.cursorPrefix), let value = Int(cursor.dropFirst(Self.cursorPrefix.count)),
                value >= 0, value <= items.count
            else {
                throw PokeCalcError(code: PokeCalcError.Code.invalidInput, message: "モックのカーソルが読めない")
            }
            start = value
        }
        let end = min(items.count, start + pageSize)
        return CalcHistoryPage(
            items: Array(items[start..<end]), nextCursor: end < items.count ? "\(Self.cursorPrefix)\(end)" : nil)
    }
}
