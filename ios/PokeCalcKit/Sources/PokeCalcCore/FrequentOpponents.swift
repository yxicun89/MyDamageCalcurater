import Foundation
import Observation

// FrequentOpponents: 種族ピッカーの「よく使う相手」候補(P6-23。ADR-0501「P6-23」)。
//
// `GET /api/record/frequent-opponents`(openapi `listFrequentOpponents`)が返すのは種族キー・スコア・件数・
// 最終計算時刻だけで、名前・タイプは返さない。名前・タイプは `PokeCalcService.species(key:)`(pokedex)で
// 1件ずつ引く。頻度は calc-svc が計算のたびに非同期で貯める(クライアントは記録しない)。
// 計算・逆算の `PokeCalcService` には混ぜない(絶対ルール5: ここが失敗しても計算と検索は成功する)。

/// 1件の「よく使う相手」(openapi `FrequentOpponent` の写像)。並びはサーバーの順(スコア降順・同点は
/// `speciesKey` 昇順)のまま保つ。
public struct FrequentOpponent: Equatable, Sendable {
    public var speciesKey: String
    public var score: Double
    public var count: Int
    public var lastCalculatedAt: Date

    public init(speciesKey: String, score: Double, count: Int, lastCalculatedAt: Date) {
        self.speciesKey = speciesKey
        self.score = score
        self.count = count
        self.lastCalculatedAt = lastCalculatedAt
    }
}

/// `listFrequentOpponents` の境界。`PokeCalcService` とは別のプロトコル(`DeviceDataService` と同じ理由)。
/// 実装は `APIPokeCalcService`(extension)と `MockFrequentOpponentsService`。失敗は `PokeCalcError`。
public protocol FrequentOpponentsService: Sendable {
    /// 1回の HTTP 要求。`limit` はそのままクエリに載せる(範囲外はサーバーが 400。クライアントで丸めない)。
    func frequentOpponents(limit: Int) async throws -> [FrequentOpponent]
}

/// 定数(coding-rules §2: 数値を散らさない)。
public enum FrequentOpponents {
    /// 1回に出す件数(openapi の既定と同じ10。ピッカー先頭の1セクションに収まる量)。
    public static let defaultLimit = 10
    /// openapi `limit` の範囲(1〜50)。
    public static let limitRange = 1...50
    /// `species(key:)` で名前を引くときの同時実行数の上限(ADR-0501「P6-23」5章)。
    public static let maxConcurrentResolutions = 4
}

/// 表示専用の文言(マスタに無い文言なので `MasterSearchLabels` と同じくここに1か所)。
public enum FrequentOpponentsLabels {
    /// セクション見出し。
    public static let sectionTitle = "よく使う相手"
}

/// 「よく使う相手」セクションの状態(`@MainActor @Observable`)。計算系 ViewModel には持たせない
/// (View が `SpeciesSearchSheet` に渡す。ADR-0501「P6-23」6章)。
@MainActor
@Observable
public final class FrequentOpponentsViewModel {
    /// 名前を解決できた相手(サーバーの順。解決できない key は含めない)。
    public private(set) var items: [SpeciesSummary] = []
    public private(set) var isLoading = false

    private let service: (any FrequentOpponentsService)?
    private let resolver: any PokeCalcService
    private let limit: Int
    private let maxConcurrentResolutions: Int

    /// - Parameters:
    ///   - service: nil なら何もしない(`items` は常に空)。
    ///   - resolver: `species(key:)` で名前・タイプを引く先(計算と同じ `PokeCalcService`)。
    public init(
        service: (any FrequentOpponentsService)?,
        resolver: any PokeCalcService,
        limit: Int = FrequentOpponents.defaultLimit,
        maxConcurrentResolutions: Int = FrequentOpponents.maxConcurrentResolutions
    ) {
        self.service = service
        self.resolver = resolver
        self.limit = limit
        self.maxConcurrentResolutions = maxConcurrentResolutions
    }

    /// 最新の `refresh()` の世代番号。進むと先行の呼び出しは結果を捨てる。
    private var generation = 0
    /// 進行中の取得・解決(新しい `refresh()` が cancel する)。
    private var currentTask: Task<[SpeciesSummary]?, Never>?

    /// 取得して名前を解決し、`items` を置き換える。シートを開くたびに呼ぶ(ADR-0501「P6-23」7章)。
    /// 新しい呼び出しは先行の呼び出しを無効にする(古い応答は捨てる)。取得失敗・空配列は `items = []`
    /// (エラーは出さない)。Task cancel では `items` を変えず、解決中の `species(key:)` も cancel する。
    public func refresh() async {
        guard let service else { return }
        generation += 1
        let myGeneration = generation
        currentTask?.cancel()
        let limit = limit
        let cap = max(1, maxConcurrentResolutions)
        let resolver = resolver
        let task = Task<[SpeciesSummary]?, Never> {
            await Self.load(service: service, resolver: resolver, limit: limit, cap: cap)
        }
        currentTask = task
        isLoading = true
        // 呼び出し元の Task が cancel されたら内側の Task も cancel する。
        let result = await withTaskCancellationHandler {
            await task.value
        } onCancel: {
            task.cancel()
        }
        guard myGeneration == generation else { return }
        isLoading = false
        currentTask = nil
        // nil は cancel(候補を変えない)。
        if let result { items = result }
    }

    /// 空クエリのときだけ `items`、検索中(前後空白を除いて空でない)は空配列。
    public func visibleItems(forQuery query: String) -> [SpeciesSummary] {
        query.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ? items : []
    }

    /// 取得と名前解決。cancel されたら nil、失敗・空は `[]`。
    private nonisolated static func load(
        service: any FrequentOpponentsService, resolver: any PokeCalcService, limit: Int, cap: Int
    ) async -> [SpeciesSummary]? {
        let opponents: [FrequentOpponent]
        do {
            opponents = try await service.frequentOpponents(limit: limit)
        } catch {
            return Task.isCancelled ? nil : []
        }
        if Task.isCancelled { return nil }
        var seen = Set<String>()
        let keys = opponents.prefix(limit).map(\.speciesKey).filter { seen.insert($0).inserted }
        var resolved = [SpeciesSummary?](repeating: nil, count: keys.count)
        await withTaskGroup(of: ResolvedSpecies.self) { group in
            var next = 0
            while next < min(cap, keys.count) {
                let index = next
                group.addTask { await Self.resolve(resolver, key: keys[index], index: index) }
                next += 1
            }
            while let item = await group.next() {
                resolved[item.index] = item.summary
                if Task.isCancelled {
                    group.cancelAll()
                } else if next < keys.count {
                    let index = next
                    group.addTask { await Self.resolve(resolver, key: keys[index], index: index) }
                    next += 1
                }
            }
        }
        return Task.isCancelled ? nil : resolved.compactMap { $0 }
    }

    private struct ResolvedSpecies: Sendable {
        let index: Int
        let summary: SpeciesSummary?
    }

    private nonisolated static func resolve(_ resolver: any PokeCalcService, key: String, index: Int) async -> ResolvedSpecies {
        let detail: SpeciesDetail? = try? await resolver.species(key: key)
        return ResolvedSpecies(index: index, summary: detail.map(SpeciesSummary.init(detail:)))
    }
}
