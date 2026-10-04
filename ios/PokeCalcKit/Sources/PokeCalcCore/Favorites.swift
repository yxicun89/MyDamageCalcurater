import Foundation
import Observation

// Favorites: お気に入り(手動ピン留め)の取得・追加・削除(ADR-0227・ADR-0511・ADR-0501「お気に入り・計算履歴」)。
//
// `GET/POST /api/record/favorites`・`DELETE /api/record/favorites/{favoriteId}`
// (openapi `listFavorites` / `createFavorite` / `deleteFavorite`)の境界と、画面の ViewModel。
// 計算・逆算の `PokeCalcService` には混ぜない(絶対ルール5: ここが失敗しても計算は成功する)。
//
// spec-writer が置いた足場。型・定数・文言は確定、挙動は `TODO(implementer` の箇所を実装者が書く。

// MARK: - ドメイン型

/// 保存済みのお気に入り1件(openapi `Favorite` の写像)。`individual.moveId` は契約に無いので常に nil。
public struct Favorite: Equatable, Sendable, Identifiable {
    /// サーバーが決めた10進の文字列(JSON の数値にしない。openapi `FavoriteId`)。
    public var id: String
    public var label: String?
    public var individual: Individual
    public var createdAt: Date
    public var updatedAt: Date

    public init(id: String, label: String?, individual: Individual, createdAt: Date, updatedAt: Date) {
        self.id = id
        self.label = label
        self.individual = individual
        self.createdAt = createdAt
        self.updatedAt = updatedAt
    }
}

/// `addFavorite` の結果。201 は新規、200 は「同じ内容がすでにあった」(`updatedAt` が進み一覧の先頭へ)。
public enum FavoriteSaveResult: Equatable, Sendable {
    case created(Favorite)
    case alreadyPinned(Favorite)
}

/// お気に入りの境界。`PokeCalcService` とは別のプロトコル(`DeviceDataService` と同じ理由)。
/// 実装は `APIPokeCalcService`(extension)と `MockFavoritesService`。失敗は `PokeCalcError`。
/// 各メソッドは1回の HTTP 要求に対応する。
public protocol FavoritesService: Sendable {
    /// サーバーの順(`updatedAt` 降順・同時刻は `id` 降順)のまま返す。無ければ空配列。
    func favorites() async throws -> [Favorite]
    /// `label` は `FavoriteLabel.normalize` を通して送る。
    func addFavorite(label: String?, individual: Individual) async throws -> FavoriteSaveResult
    /// 204 で正常終了。持っていない ID は `PokeCalcError(code: "not_found")`。
    func removeFavorite(id: String) async throws
}

// MARK: - 定数・ラベルの正規化

public enum Favorites {
    /// 名前を `species(key:)` で引くときの同時実行数の上限(`FrequentOpponents` と同じ値)。
    public static let maxConcurrentResolutions = 4
}

public enum FavoriteLabel {
    /// 前後の空白(改行含む)を落とし、空なら nil、`RequestLimits.maxFavoriteLabelLength` を超える分は
    /// Unicode コードポイント単位で切り落とす(サーバーはコードポイントで数える)。
    public static func normalize(_ raw: String?) -> String? {
        guard let trimmed = raw?.trimmingCharacters(in: .whitespacesAndNewlines), !trimmed.isEmpty else { return nil }
        let scalars = trimmed.unicodeScalars
        guard scalars.count > RequestLimits.maxFavoriteLabelLength else { return trimmed }
        var truncated = String.UnicodeScalarView()
        truncated.append(contentsOf: scalars.prefix(RequestLimits.maxFavoriteLabelLength))
        return String(truncated)
    }
}

// MARK: - 画面に出すエラー

/// 記録系(お気に入り・計算履歴)の操作。文言の出し分けに使う。
public enum RecordAction: Equatable, Sendable {
    case loadFavorites
    case addFavorite
    case removeFavorite
    case loadHistory
}

/// `PokeCalcError.code` を画面向けの種類に写したもの。サーバーの `message`(英語を含む)は画面に出さない。
public enum RecordScreenError: Equatable, Sendable {
    /// 通信できない(`PokeCalcError.Code.transport`)。
    case transport
    /// 応答の形が想定と違う(`decode`・`unexpectedStatus`・`PokeCalcError` 以外の Error)。
    case unexpectedResponse
    /// 503 `store_unavailable` / `upstream_unavailable`(保存・取得できないが計算はできる)。
    case storeUnavailable
    /// 404 `not_found`。
    case notFound
    /// 400 `invalid_input`(お気に入りの追加では上限到達を含む)。
    case invalidInput
    /// 上記以外のサーバーの code。
    case other(code: String)

    public init(_ error: any Error) {
        guard let error = error as? PokeCalcError else {
            self = .unexpectedResponse
            return
        }
        switch error.code {
        case PokeCalcError.Code.transport: self = .transport
        case PokeCalcError.Code.decode, PokeCalcError.Code.unexpectedStatus: self = .unexpectedResponse
        case "store_unavailable", "upstream_unavailable": self = .storeUnavailable
        case "not_found": self = .notFound
        case "invalid_input": self = .invalidInput
        default: self = .other(code: error.code)
        }
    }

    /// 画面に出す日本語。`FavoritesLabels` の文言だけを返し、`code`・サーバーの `message` を含めない。
    public func message(for action: RecordAction) -> String {
        switch self {
        case .transport:
            return FavoritesLabels.transportFailure
        case .storeUnavailable:
            switch action {
            case .loadFavorites: return FavoritesLabels.loadFavoritesUnavailable
            case .addFavorite: return FavoritesLabels.addFavoriteUnavailable
            case .removeFavorite: return FavoritesLabels.removeFavoriteUnavailable
            case .loadHistory: return FavoritesLabels.loadHistoryUnavailable
            }
        case .invalidInput where action == .addFavorite:
            return FavoritesLabels.addFavoriteInvalid
        case .unexpectedResponse, .notFound, .invalidInput, .other:
            return FavoritesLabels.genericFailure
        }
    }
}

// MARK: - 文言(マスタに無い表示専用。ここに1か所)

public enum FavoritesLabels {
    public static let screenTitle = "お気に入り・履歴"
    public static let rootButtonTitle = "お気に入り・履歴"
    public static let favoritesSectionTitle = "お気に入り"
    public static let historySectionTitle = "よく計算する相手"
    public static let emptyFavorites = "お気に入りはまだありません。計算画面の「お気に入りに追加」から追加できます。"
    public static let emptyHistory = "計算した相手がまだありません。計算すると、よく計算する相手がここに並びます。"
    /// 生の計算履歴の取得 API は契約に無い(ADR-0511)。契約ができるまで注記だけ出す。
    public static let pendingHistoryNote = "計算の履歴そのものの一覧は、サーバーの対応待ちです。"
    public static let removeButton = "外す"
    public static let retryButton = "再読み込み"
    public static let pinAttackerButton = "攻撃側をお気に入りに追加"
    public static let pinDefenderButton = "防御側をお気に入りに追加"
    public static let saving = "追加しています…"
    public static let pinned = "お気に入りに追加しました。"
    public static let alreadyPinned = "すでにお気に入りに追加済みです。"
    public static let unknownSpecies = "不明なポケモン"
    public static let limitNote = "お気に入りは最大\(RequestLimits.maxFavorites)件までです。"

    public static let transportFailure = "通信に失敗しました。接続を確認してもう一度お試しください。"
    public static let genericFailure = "うまくいきませんでした。時間をおいてもう一度お試しください。"
    public static let loadFavoritesUnavailable = "お気に入りを読み込めません。計算はそのまま使えます。しばらくしてからもう一度お試しください。"
    public static let addFavoriteUnavailable = "いまはお気に入りに保存できません。計算はそのまま使えます。"
    public static let removeFavoriteUnavailable = "いまはお気に入りを外せません。計算はそのまま使えます。"
    public static let loadHistoryUnavailable = "計算の履歴を読み込めません。計算はそのまま使えます。しばらくしてからもう一度お試しください。"
    public static let addFavoriteInvalid = "お気に入りに追加できませんでした。上限の\(RequestLimits.maxFavorites)件に達している可能性があります。"
}

// MARK: - お気に入り一覧の ViewModel

/// 一覧の1行(表示用)。
public struct FavoriteRow: Equatable, Sendable, Identifiable {
    public var favorite: Favorite
    /// `species(key:)` で引けた日本語名。引けなければ nil。
    public var speciesName: String?

    public init(favorite: Favorite, speciesName: String?) {
        self.favorite = favorite
        self.speciesName = speciesName
    }

    public var id: String { favorite.id }
    /// `label ?? speciesName ?? FavoritesLabels.unknownSpecies`。
    public var title: String {
        favorite.label ?? speciesName ?? FavoritesLabels.unknownSpecies
    }
    /// label があるときだけ、種族名(引けなければ `unknownSpecies`)。無ければ nil。
    public var subtitle: String? {
        favorite.label == nil ? nil : (speciesName ?? FavoritesLabels.unknownSpecies)
    }
}

/// 読み込みの状態。
public enum RecordLoadState: Equatable, Sendable {
    case idle
    case loading
    case loaded
    case failed(RecordScreenError)
}

/// 種族キーから日本語名を引く(お気に入り・計算履歴の一覧用)。引けない key・マスタの失敗は辞書に入れない。
/// 同時実行は `cap` まで。Task cancel されたら nil。
enum SpeciesNameResolver {
    static func names(keys: [String], resolver: any PokeCalcService, cap: Int) async -> [String: String]? {
        var seen = Set<String>()
        let unique = keys.filter { seen.insert($0).inserted }
        var names: [String: String] = [:]
        await withTaskGroup(of: (String, String?).self) { group in
            var next = 0
            func addNext() {
                let key = unique[next]
                next += 1
                group.addTask { (key, try? await resolver.species(key: key).nameJa) }
            }
            while next < min(max(1, cap), unique.count) { addNext() }
            while let (key, name) = await group.next() {
                if let name { names[key] = name }
                if Task.isCancelled {
                    group.cancelAll()
                } else if next < unique.count {
                    addNext()
                }
            }
        }
        return Task.isCancelled ? nil : names
    }
}

@MainActor
@Observable
public final class FavoritesViewModel {
    public private(set) var items: [FavoriteRow] = []
    public private(set) var loadState: RecordLoadState = .idle
    /// 外す操作の失敗(一覧は壊さず、画面の上に出す帯用)。
    public private(set) var actionError: RecordScreenError?
    /// 外す要求の最中の ID(ボタンの無効化に使う)。
    public private(set) var removingIDs: Set<String> = []

    let service: (any FavoritesService)?
    let resolver: any PokeCalcService

    /// 最新の `load()` の世代番号。進むと先行の呼び出しは結果を捨てる。
    private var generation = 0
    /// 外した ID と、そのとき進行中だった `load()` の世代(その応答に ID が残っていても捨てる)。
    private var removedAt: [String: Int] = [:]

    /// - Parameters:
    ///   - service: nil なら何もしない(`items` は空、`loadState` は `.idle` のまま)。
    ///   - resolver: 名前を引く先(計算と同じ `PokeCalcService`)。
    public init(service: (any FavoritesService)?, resolver: any PokeCalcService) {
        self.service = service
        self.resolver = resolver
    }

    /// 件数が `RequestLimits.maxFavorites` に達している(画面が上限の注記を出す)。
    public var isAtLimit: Bool { items.count >= RequestLimits.maxFavorites }

    /// 取得して名前を解決し `items` を置き換える。画面を開くたび・再読み込みで呼ぶ。
    public func load() async {
        guard let service else { return }
        generation += 1
        let mine = generation
        let previousState = loadState
        loadState = .loading
        let favorites: [Favorite]
        do {
            favorites = try await service.favorites()
        } catch {
            guard mine == generation else { return }
            if Task.isCancelled || error is CancellationError {
                loadState = previousState
            } else {
                loadState = .failed(RecordScreenError(error))
            }
            return
        }
        let names = await SpeciesNameResolver.names(
            keys: favorites.map(\.individual.speciesKey), resolver: resolver, cap: Favorites.maxConcurrentResolutions)
        guard mine == generation else { return }
        guard let names else {
            loadState = previousState
            return
        }
        items = favorites
            .filter { removedAt[$0.id] != mine }
            .map { FavoriteRow(favorite: $0, speciesName: names[$0.individual.speciesKey]) }
        loadState = .loaded
    }

    /// 1件外す。成功・404(すでに無い)は一覧から除く。それ以外の失敗は一覧を保ち `actionError` を立てる。
    public func remove(id: String) async {
        guard let service, !removingIDs.contains(id) else { return }
        removingIDs.insert(id)
        defer { removingIDs.remove(id) }
        do {
            try await service.removeFavorite(id: id)
        } catch {
            if Task.isCancelled || error is CancellationError { return }
            let mapped = RecordScreenError(error)
            guard mapped == .notFound else {
                actionError = mapped
                return
            }
        }
        removedAt[id] = generation
        items.removeAll { $0.id == id }
        actionError = nil
    }

    public func dismissActionError() {
        actionError = nil
    }
}

// MARK: - 計算画面から追加する ViewModel

@MainActor
@Observable
public final class FavoritePinViewModel {
    public enum Status: Equatable, Sendable {
        case idle
        case saving
        case pinned
        case alreadyPinned
        case failed(RecordScreenError)
    }

    public private(set) var status: Status = .idle
    let service: (any FavoritesService)?

    public init(service: (any FavoritesService)?) {
        self.service = service
    }

    /// サービスがあるときだけ、計算画面が追加ボタンを出す。
    public var isAvailable: Bool { service != nil }

    /// ラベル無しで個体をそのまま1回追加する。保存中の再呼び出しは無視する(要求は1回)。
    public func pin(_ individual: Individual) async {
        guard let service, status != .saving else { return }
        status = .saving
        do {
            switch try await service.addFavorite(label: nil, individual: individual) {
            case .created: status = .pinned
            case .alreadyPinned: status = .alreadyPinned
            }
        } catch {
            status = Task.isCancelled || error is CancellationError ? .idle : .failed(RecordScreenError(error))
        }
    }

    /// 表示中の個体が変わったときなどに `.idle` へ戻す(保存中は変えない)。
    public func reset() {
        if status != .saving { status = .idle }
    }
}
