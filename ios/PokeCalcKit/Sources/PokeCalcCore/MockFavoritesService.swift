import Foundation

// MockFavoritesService: `FavoritesService` のモック(XCUITest 用。ADR-0511)。
// 起動時の環境変数 `POKECALC_MOCK_FAVORITES` で初期状態を切り替える(`POKECALC_MOCK_FREQUENT_OPPONENTS` と同じ流儀)。
// 既定(未設定・未知の値)は「空のストアで、追加・削除が実際に動く」(既存のテストに影響しない)。
// 架空の speciesKey(9001〜9004)と `test-nature-*` だけを使う。

public enum MockFavoritesScenario: Equatable, Sendable {
    /// 空のストア。追加・削除は状態に反映される(既定)。
    case emptyStore
    /// 2件: id "102"(label「HB特化」・9002-000・新しい方)、id "101"(label なし・9003-000)。
    case list
    /// 4件(読み込みの確認用。ADR-0513): id "304"(label「読み込める」・9002-000・攻撃上昇・特性/持ち物がマスタにある・最も新しい)、
    /// "303"(label「一部だけ」・9004-000・特性と持ち物がマスタに無い)、"302"(label「種族なし」・9999-000 = マスタに無い)、
    /// "301"(label なし・9003-000・特性あり・持ち物なし)。
    case loadable
    /// すべての操作が transport エラー。
    case failure
    /// すべての操作が 503 `store_unavailable`。
    case unavailable
    /// `RequestLimits.maxFavorites` 件が入っていて、新しい内容の追加は 400 `invalid_input`。
    case full

    /// 環境変数の値: `list` / `fail` / `unavailable` / `full`。nil・未知の値は `.emptyStore`。
    public init(environmentValue: String?) {
        switch environmentValue {
        case "list": self = .list
        case "loadable": self = .loadable
        case "fail": self = .failure
        case "unavailable": self = .unavailable
        case "full": self = .full
        default: self = .emptyStore
        }
    }
}

public actor MockFavoritesService: FavoritesService {
    public static let scenarioEnvironmentKey = "POKECALC_MOCK_FAVORITES"

    private let scenario: MockFavoritesScenario
    /// 新しい順。
    private var stored: [Favorite]
    private var nextID: Int
    private var clock: TimeInterval

    /// 固定の時刻(モックは決定的にする)。
    private static let baseTime: TimeInterval = 1_790_000_000

    public init(scenario: MockFavoritesScenario = .emptyStore) {
        self.scenario = scenario
        switch scenario {
        case .list:
            stored = [
                Self.make("102", label: "HB特化", key: "9002-000", nature: "test-nature-neutral", at: Self.baseTime + 100),
                Self.make("101", label: nil, key: "9003-000", nature: "test-nature-neutral", at: Self.baseTime),
            ]
            nextID = 103
            clock = Self.baseTime + 100
        case .loadable:
            let neutral = "test-nature-neutral"
            stored = [
                Self.make(
                    "304", label: "読み込める", key: "9002-000", nature: "test-nature-atk-up", at: Self.baseTime + 300,
                    sp: StatBlock(hp: 0, atk: 32, def: 0, spa: 0, spd: 2, spe: 32),
                    ability: "test-ability-beta", item: "test-item-berry"),
                Self.make(
                    "303", label: "一部だけ", key: "9004-000", nature: neutral, at: Self.baseTime + 200,
                    ability: "test-ability-gone", item: "test-item-gone"),
                Self.make("302", label: "種族なし", key: "9999-000", nature: neutral, at: Self.baseTime + 100),
                Self.make(
                    "301", label: nil, key: "9003-000", nature: neutral, at: Self.baseTime, ability: "test-ability-gamma"),
            ]
            nextID = 305
            clock = Self.baseTime + 300
        case .full:
            let count = RequestLimits.maxFavorites
            // id が大きいほど新しい。内容はすべて違う(natureId で区別)。
            stored = (1...count).reversed().map {
                Self.make(
                    "\($0)", label: nil, key: "900\(($0 % 4) + 1)-000", nature: "test-nature-full-\($0)",
                    at: Self.baseTime + TimeInterval($0))
            }
            nextID = count + 1
            clock = Self.baseTime + TimeInterval(count)
        case .emptyStore, .failure, .unavailable:
            stored = []
            nextID = 1
            clock = Self.baseTime
        }
    }

    public init(environment: [String: String]) {
        self.init(scenario: MockFavoritesScenario(environmentValue: environment[Self.scenarioEnvironmentKey]))
    }

    private static func make(
        _ id: String, label: String?, key: String, nature: String, at time: TimeInterval,
        sp: StatBlock = StatBlock(hp: 32, atk: 0, def: 32, spa: 0, spd: 2, spe: 0),
        ability: String? = nil, item: String? = nil
    ) -> Favorite {
        Favorite(
            id: id, label: label,
            individual: Individual(speciesKey: key, natureId: nature, sp: sp, abilityId: ability, itemId: item),
            createdAt: Date(timeIntervalSince1970: time), updatedAt: Date(timeIntervalSince1970: time))
    }

    private func failIfScripted() throws {
        switch scenario {
        case .failure:
            throw PokeCalcError(code: PokeCalcError.Code.transport, message: "モックの通信エラー")
        case .unavailable:
            throw PokeCalcError(code: "store_unavailable", message: "モックの保存先の障害")
        case .emptyStore, .list, .loadable, .full:
            return
        }
    }

    public func favorites() async throws -> [Favorite] {
        try failIfScripted()
        return stored
    }

    public func addFavorite(label: String?, individual: Individual) async throws -> FavoriteSaveResult {
        try failIfScripted()
        let label = FavoriteLabel.normalize(label)
        clock += 1
        if let index = stored.firstIndex(where: { $0.label == label && $0.individual == individual }) {
            var existing = stored.remove(at: index)
            existing.updatedAt = Date(timeIntervalSince1970: clock)
            stored.insert(existing, at: 0)
            return .alreadyPinned(existing)
        }
        guard stored.count < RequestLimits.maxFavorites else {
            throw PokeCalcError(code: "invalid_input", message: "モックのお気に入りの上限")
        }
        let now = Date(timeIntervalSince1970: clock)
        let favorite = Favorite(
            id: "\(nextID)", label: label, individual: individual, createdAt: now, updatedAt: now)
        nextID += 1
        stored.insert(favorite, at: 0)
        return .created(favorite)
    }

    public func removeFavorite(id: String) async throws {
        try failIfScripted()
        guard let index = stored.firstIndex(where: { $0.id == id }) else {
            throw PokeCalcError(code: "not_found", message: "モックにそのお気に入りは無い")
        }
        stored.remove(at: index)
    }
}
