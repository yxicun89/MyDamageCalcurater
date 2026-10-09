import Foundation
import Observation

// CalcHistory: 計算履歴(生の計算イベントの一覧)。ADR-0230・ADR-0519。
//
// `GET /api/record/calc-history`(openapi `listCalcHistory`)の境界と、画面の ViewModel。
// 新しい順・keyset のカーソル。続きは前のページの `nextCursor` をそのまま渡す(中身を解釈・加工しない)。
// 計算・逆算の `PokeCalcService` には混ぜない(絶対ルール5: ここが失敗しても計算は成功する)。
// 画面を開いたときに先頭から読み直す(ポーリングしない)。個別の行の削除・編集は契約に無い。

// MARK: - ドメイン型

/// 履歴の1行の計算の入力(openapi `CalcRequest` の写像)。お気に入りの `calc` と同じ形で、
/// 計算画面に入力を復元するのに使う(`CalcViewModel.loadHistoryCalc`)。
public struct CalcHistoryCalc: Equatable, Sendable {
    public var format: Format
    public var attacker: Individual
    public var defender: Individual
    public var moveId: String
    public var field: FieldState
    public var critical: Bool

    public init(
        format: Format, attacker: Individual, defender: Individual, moveId: String,
        field: FieldState = FieldState(), critical: Bool = false
    ) {
        self.format = format
        self.attacker = attacker
        self.defender = defender
        self.moveId = moveId
        self.field = field
        self.critical = critical
    }
}

/// 履歴の1行(openapi `CalcHistoryEntry` の写像)。
public struct CalcHistoryEntry: Equatable, Sendable {
    public var occurredAt: Date
    public var calc: CalcHistoryCalc
    public var minPercent: Double
    public var maxPercent: Double

    public init(occurredAt: Date, calc: CalcHistoryCalc, minPercent: Double, maxPercent: Double) {
        self.occurredAt = occurredAt
        self.calc = calc
        self.minPercent = minPercent
        self.maxPercent = maxPercent
    }
}

/// 1ページ分(openapi `CalcHistoryPage`)。`nextCursor` が nil なら続きは無い。
public struct CalcHistoryPage: Equatable, Sendable {
    public var items: [CalcHistoryEntry]
    public var nextCursor: String?

    public init(items: [CalcHistoryEntry], nextCursor: String?) {
        self.items = items
        self.nextCursor = nextCursor
    }
}

/// `listCalcHistory` の境界。`PokeCalcService` とは別のプロトコル(`FavoritesService` と同じ理由)。
/// 実装は `APIPokeCalcService`(extension)と `MockCalcHistoryService`。失敗は `PokeCalcError`。
public protocol CalcHistoryService: Sendable {
    /// 1回の HTTP 要求。`limit` はそのままクエリに載せる(範囲外はサーバーが 400。クライアントで丸めない)。
    /// `cursor` は前のページの `nextCursor` をそのまま渡す(nil は先頭〈最新〉から)。
    func calcHistory(limit: Int, cursor: String?) async throws -> CalcHistoryPage
}

/// 定数(coding-rules §2: 数値を散らさない)。
public enum CalcHistory {
    /// 1ページの件数(openapi の既定と同じ20。画面の1スクロール分。範囲は 1〜50)。
    public static let pageSize = 20
    /// 履歴の行から計算画面へ復元した攻撃側の出どころの印(`TeamIndividualSelection.teamID`・`memberID`)。
    public static let sourceTeamID = "history"
}

// MARK: - 文言

public enum CalcHistoryLabels {
    public static let sectionTitle = "計算履歴"
    public static let emptyHistory = "計算履歴はまだありません。計算すると、新しい順にここに並びます。"
    public static let loadMoreButton = "もっと見る"
    public static let loadingMore = "続きを読み込んでいます…"
    public static let unknownMove = "不明な技"
    public static let rowHint = "タップすると、この計算を計算画面に復元します。防御側の性格・能力ポイント・持ち物は復元されず、ダブルの計算もシングルで復元するため、結果が一覧と異なることがあります。"
    public static let note = "計算した直後の行は、少し遅れて載ることがあります。行を開いた結果は、防御側の性格・能力ポイント・持ち物を復元しないため、一覧の%と異なることがあります。"

    public static func occurredText(_ date: Date, now: Date, calendar: Calendar) -> String {
        let days = calendar.dateComponents(
            [.day], from: calendar.startOfDay(for: date), to: calendar.startOfDay(for: now)
        ).day ?? 0
        switch days {
        case ...0: return "今日"
        case 1: return "昨日"
        default: return "\(days)日前"
        }
    }

    /// 「攻撃側 → 防御側」(種族名。引けなければ `FavoritesLabels.unknownSpecies`)。
    public static func matchupText(attacker: String, defender: String) -> String {
        "\(attacker) → \(defender)"
    }
}

// MARK: - 行(表示用)

public struct CalcHistoryRow: Equatable, Sendable, Identifiable {
    /// 行の ID は契約に無いので、取得済みの並びの位置と `occurredAt` から作る(ページを足しても既存の行の ID は変わらない)。
    public var id: String
    public var entry: CalcHistoryEntry
    public var attackerName: String?
    public var defenderName: String?
    public var moveName: String?

    public init(id: String, entry: CalcHistoryEntry, attackerName: String?, defenderName: String?, moveName: String?) {
        self.id = id
        self.entry = entry
        self.attackerName = attackerName
        self.defenderName = defenderName
        self.moveName = moveName
    }

    public var title: String {
        CalcHistoryLabels.matchupText(
            attacker: attackerName ?? FavoritesLabels.unknownSpecies,
            defender: defenderName ?? FavoritesLabels.unknownSpecies)
    }

    public var moveText: String { moveName ?? CalcHistoryLabels.unknownMove }

    public var percentText: String {
        BulkRowDisplay.percentRangeText(minPercent: entry.minPercent, maxPercent: entry.maxPercent)
    }

    /// 「今日」「昨日」「3日前」。`calendar` の日付の差(時刻ではなく暦日)で決め、未来の時刻は「今日」にする。
    public func occurredText(now: Date, calendar: Calendar) -> String {
        CalcHistoryLabels.occurredText(entry.occurredAt, now: now, calendar: calendar)
    }
}

// MARK: - ViewModel

@MainActor
@Observable
public final class CalcHistoryViewModel {
    /// 取得済みの行(新しい順。続きは末尾に足す)。
    public private(set) var rows: [CalcHistoryRow] = []
    /// 先頭ページの読み込みの状態。失敗しても `rows`(前回の取得分)は残す。
    public private(set) var loadState: RecordLoadState = .idle
    /// 続き(「もっと見る」)を読み込み中。
    public private(set) var isLoadingMore = false
    /// 続きの読み込みの失敗(取得済みの行は残し、「もっと見る」を再試行に使う)。
    public private(set) var loadMoreError: RecordScreenError?
    /// 次のページのカーソル(サーバーの値そのまま)。nil なら続きは無い。
    public private(set) var nextCursor: String?

    let service: (any CalcHistoryService)?
    let resolver: any PokeCalcService
    let pageSize: Int

    private var entries: [CalcHistoryEntry] = []
    private var speciesNames: [String: String] = [:]
    private var moveNames: [String: String] = [:]
    /// 最新の `load()` の世代番号。進むと先行の `load()`・`loadMore()` は結果を捨てる。
    private var generation = 0

    /// - Parameter service: nil なら何もしない(`loadState` は `.idle` のまま)。
    public init(
        service: (any CalcHistoryService)?, resolver: any PokeCalcService, pageSize: Int = CalcHistory.pageSize
    ) {
        self.service = service
        self.resolver = resolver
        self.pageSize = pageSize
    }

    /// 「もっと見る」を出すか。
    public var hasMore: Bool { nextCursor != nil }

    /// 取得済みで0件(画面が `emptyHistory` を出す)。
    public var isEmpty: Bool { loadState == .loaded && rows.isEmpty }

    /// 先頭ページを取得して `rows` を置き換える。画面を開くたび・再読み込みで呼ぶ。
    public func load() async {
        guard let service else { return }
        generation += 1
        let mine = generation
        let previousState = loadState
        loadState = .loading
        isLoadingMore = false
        loadMoreError = nil
        let page: CalcHistoryPage
        do {
            page = try await service.calcHistory(limit: pageSize, cursor: nil)
        } catch {
            guard mine == generation else { return }
            loadState = Task.isCancelled || error is CancellationError ? previousState : .failed(RecordScreenError(error))
            return
        }
        guard await resolveNames(for: page.items, generation: mine) else {
            if mine == generation { loadState = previousState }
            return
        }
        entries = page.items
        nextCursor = page.nextCursor
        rebuildRows()
        loadState = .loaded
    }

    /// 次のページを取得して末尾に足す。続きが無い・読み込み中は何もしない。
    /// 400(カーソルが読めない)は先頭から読み直す。それ以外の失敗は `loadMoreError` を立て、取得済みの行を保つ。
    public func loadMore() async {
        guard let service, let cursor = nextCursor, !isLoadingMore, loadState != .loading else { return }
        let mine = generation
        isLoadingMore = true
        loadMoreError = nil
        let page: CalcHistoryPage
        do {
            page = try await service.calcHistory(limit: pageSize, cursor: cursor)
        } catch {
            guard mine == generation else { return }
            isLoadingMore = false
            if Task.isCancelled || error is CancellationError { return }
            let mapped = RecordScreenError(error)
            if mapped == .invalidInput {
                await load()
            } else {
                loadMoreError = mapped
            }
            return
        }
        guard await resolveNames(for: page.items, generation: mine) else {
            if mine == generation { isLoadingMore = false }
            return
        }
        entries += page.items
        nextCursor = page.nextCursor
        rebuildRows()
        isLoadingMore = false
    }

    /// 行に出す種族名・技名を引く(引けないものは辞書に入れず、行が「不明」を出す)。
    /// 応答を反映してよければ true。Task cancel・追い越されたら false。
    private func resolveNames(for items: [CalcHistoryEntry], generation mine: Int) async -> Bool {
        let speciesKeys = items.flatMap { [$0.calc.attacker.speciesKey, $0.calc.defender.speciesKey] }
            .filter { speciesNames[$0] == nil }
        let moveIds = items.map(\.calc.moveId).filter { moveNames[$0] == nil }
        let names = await SpeciesNameResolver.names(
            keys: speciesKeys, resolver: resolver, cap: Favorites.maxConcurrentResolutions)
        let moves = moveIds.isEmpty ? [] : ((try? await resolver.moves(ids: moveIds)) ?? [])
        guard mine == generation, let names, !Task.isCancelled else { return false }
        speciesNames.merge(names) { _, new in new }
        for move in moves { moveNames[move.id] = move.nameJa }
        return true
    }

    private func rebuildRows() {
        rows = entries.enumerated().map { index, entry in
            CalcHistoryRow(
                id: "\(index)-\(Int(entry.occurredAt.timeIntervalSince1970))", entry: entry,
                attackerName: speciesNames[entry.calc.attacker.speciesKey],
                defenderName: speciesNames[entry.calc.defender.speciesKey],
                moveName: moveNames[entry.calc.moveId])
        }
    }
}
