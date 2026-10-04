import Foundation
import Observation

// OpponentHistory: 「よく計算する相手」の一覧画面(ADR-0511)。
//
// データは `FrequentOpponentsService`(P6-23 で実装済みの `GET /api/record/frequent-opponents`)をそのまま使う。
// 種族ピッカーの `FrequentOpponentsViewModel` は名前を引けた種族だけを出して件数・時刻を捨てるので、
// こちらは件数・最終計算時刻を含む行を作り、名前を引けない種族も(不明として)残す。
// 生の計算履歴(イベント1件ずつ)の取得 API は契約に無い。推測で足さず、画面は注記だけを出す。
//
// spec-writer が置いた足場。挙動は `TODO(implementer` の箇所を実装者が書く。

public enum OpponentHistory {
    /// 1回に取る件数(openapi `limit` の最大。一覧画面は絞らず全部見せる)。
    public static let listLimit = FrequentOpponents.limitRange.upperBound
}

/// 一覧の1行。
public struct OpponentHistoryRow: Equatable, Sendable, Identifiable {
    public var speciesKey: String
    /// 引けた日本語名。引けなければ nil。
    public var speciesName: String?
    public var count: Int
    public var lastCalculatedAt: Date

    public init(speciesKey: String, speciesName: String?, count: Int, lastCalculatedAt: Date) {
        self.speciesKey = speciesKey
        self.speciesName = speciesName
        self.count = count
        self.lastCalculatedAt = lastCalculatedAt
    }

    public var id: String { speciesKey }
    /// `speciesName ?? FavoritesLabels.unknownSpecies`。
    public var title: String {
        speciesName ?? FavoritesLabels.unknownSpecies
    }
}

public enum OpponentHistoryLabels {
    /// 「3回」。
    public static func countText(_ count: Int) -> String {
        "\(count)回"
    }

    /// 「最後: 今日」「最後: 昨日」「最後: 3日前」。`calendar` の日付の差(時刻ではなく暦日)で決め、
    /// 未来の時刻は「今日」にする。
    public static func lastCalculatedText(_ date: Date, now: Date, calendar: Calendar) -> String {
        let days = calendar.dateComponents(
            [.day], from: calendar.startOfDay(for: date), to: calendar.startOfDay(for: now)
        ).day ?? 0
        switch days {
        case ...0: return "最後: 今日"
        case 1: return "最後: 昨日"
        default: return "最後: \(days)日前"
        }
    }
}

@MainActor
@Observable
public final class OpponentHistoryViewModel {
    public private(set) var items: [OpponentHistoryRow] = []
    public private(set) var loadState: RecordLoadState = .idle

    let service: (any FrequentOpponentsService)?
    let resolver: any PokeCalcService
    let limit: Int
    /// 最新の `load()` の世代番号。進むと先行の呼び出しは結果を捨てる。
    private var generation = 0

    /// - Parameter service: nil なら何もしない(`loadState` は `.idle` のまま)。
    public init(
        service: (any FrequentOpponentsService)?, resolver: any PokeCalcService, limit: Int = OpponentHistory.listLimit
    ) {
        self.service = service
        self.resolver = resolver
        self.limit = limit
    }

    /// 取得して名前を解決し `items` を置き換える。サーバーの順を保ち、同じ `speciesKey` の2件目以降は捨てる。
    public func load() async {
        guard let service else { return }
        generation += 1
        let mine = generation
        let previousState = loadState
        loadState = .loading
        let opponents: [FrequentOpponent]
        do {
            opponents = try await service.frequentOpponents(limit: limit)
        } catch {
            guard mine == generation else { return }
            if Task.isCancelled || error is CancellationError {
                loadState = previousState
            } else {
                loadState = .failed(RecordScreenError(error))
            }
            return
        }
        var seen = Set<String>()
        let unique = opponents.filter { seen.insert($0.speciesKey).inserted }
        let names = await SpeciesNameResolver.names(
            keys: unique.map(\.speciesKey), resolver: resolver, cap: Favorites.maxConcurrentResolutions)
        guard mine == generation else { return }
        guard let names else {
            loadState = previousState
            return
        }
        items = unique.map {
            OpponentHistoryRow(
                speciesKey: $0.speciesKey, speciesName: names[$0.speciesKey], count: $0.count,
                lastCalculatedAt: $0.lastCalculatedAt)
        }
        loadState = .loaded
    }
}
