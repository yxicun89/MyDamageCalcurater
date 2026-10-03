import Foundation

@testable import PokeCalcCore

/// `StubPokeCalcService` を包み、マスタ参照の同時実行数の最大値を記録する(P6-20: 名前解決の並行数の上限の検査)。
/// 各呼び出しは少し待ってから転送するので、上限なしで並べて投げれば同時実行数が積み上がる。
actor ConcurrencyProbeService: PokeCalcService {
    private let inner: StubPokeCalcService
    private let delay: Duration
    private var inFlight = 0
    private(set) var maxInFlight = 0
    private(set) var totalCalls = 0

    init(inner: StubPokeCalcService, delay: Duration = .milliseconds(30)) {
        self.inner = inner
        self.delay = delay
    }

    private func track<T: Sendable>(_ body: @Sendable () async throws -> T) async throws -> T {
        inFlight += 1
        totalCalls += 1
        maxInFlight = max(maxInFlight, inFlight)
        defer { inFlight -= 1 }
        try await Task.sleep(for: delay)
        return try await body()
    }

    func searchSpecies(query: String, limit: Int) async throws -> [SpeciesSummary] {
        try await track { try await inner.searchSpecies(query: query, limit: limit) }
    }
    func species(key: String) async throws -> SpeciesDetail { try await track { try await inner.species(key: key) } }
    func searchMoves(query: String, limit: Int) async throws -> [Move] {
        try await track { try await inner.searchMoves(query: query, limit: limit) }
    }
    func move(id: String) async throws -> Move { try await track { try await inner.move(id: id) } }
    func moves(ids: [String]) async throws -> [Move] { try await track { try await inner.moves(ids: ids) } }
    func searchItems(query: String, limit: Int) async throws -> [Item] {
        try await track { try await inner.searchItems(query: query, limit: limit) }
    }
    func natures() async throws -> [Nature] { try await track { try await inner.natures() } }
    func calcDamage(_ request: CalcRequest) async throws -> CalcResult { try await inner.calcDamage(request) }
    func calcBulk(_ request: BulkCalcRequest) async throws -> BulkCalcResult { try await inner.calcBulk(request) }
    func reverse(_ request: ReverseRequest) async throws -> ReverseResult { try await inner.reverse(request) }
}
