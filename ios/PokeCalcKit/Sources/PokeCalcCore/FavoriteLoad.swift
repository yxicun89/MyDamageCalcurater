import Foundation
import Observation

// FavoriteLoad: お気に入りを計算画面の攻撃側・防御側に読み込む(ADR-0513。ADR-0511 が「別タスク」としていた続き)。
//
// 型・定数・文言は確定。挙動は `TODO(implementer` の箇所を実装者が書く(spec-writer の足場)。
// `FavoritesService`(取得)は ADR-0511 のまま。計算画面の `CalcViewModel` は `Favorite` を1件受け取るだけで、
// 取得は入口のシートの ViewModel(`FavoriteLoadPickerViewModel`)が持つ(絶対ルール5: 取得の失敗は計算を壊さない)。

// MARK: - 側・落とした項目・案内

/// どちらのカードに読み込むか。
public enum FavoriteLoadSide: Equatable, Sendable {
    case attacker
    case defender
}

/// マスタに無い・規則に合わず、読み込めなかった項目(表示順はこの宣言順)。
public enum FavoriteLoadDropped: Equatable, Sendable, CaseIterable {
    /// 性格と能力ポイント(2つで1組。性格がマスタに無ければ SP も使わず、出どころはいまのプリセットのまま)。
    case nature
    /// 持ち物(マスタに無い・メガ固定に反する・非メガ種族のメガストーン)。
    case item
    /// 特性(種族の特性に無い)。
    case ability
}

/// 読み込みの結果の案内(計算画面の `favoriteLoadNotice`)。全部読めたときは案内なし(nil)。
public enum FavoriteLoadNotice: Equatable, Sendable {
    /// 読めた分だけ設定した。`dropped` は空でなく、`FavoriteLoadDropped` の宣言順。
    case partial([FavoriteLoadDropped])
    /// 種族がマスタに無い(`species(key:)` が `not_found`)。何も変えていない。
    case speciesMissing
    /// 種族を引けなかった(通信・503 など)。何も変えていない。
    case unavailable

    /// 画面に出す日本語。`PokeCalcError` の code・サーバーの message を含めない。
    public var text: String {
        switch self {
        case .partial(let dropped):
            return FavoritesLabels.loadPartialNotice(dropped)
        case .speciesMissing:
            return FavoritesLabels.loadSpeciesMissingNotice
        case .unavailable:
            return FavoritesLabels.loadUnavailableNotice
        }
    }
}

// MARK: - 文言(マスタに無い表示専用。既存の `FavoritesLabels` は変えず、読み込み用を足す)

extension FavoritesLabels {
    /// 入口のボタンとシートの見出し(同じ文言)。
    public static func loadTitle(for side: FavoriteLoadSide) -> String {
        switch side {
        case .attacker: "お気に入りから攻撃側を選ぶ"
        case .defender: "お気に入りから防御側を選ぶ"
        }
    }

    /// シートの上に出す注記(何を読み込むか)。
    public static func loadNote(for side: FavoriteLoadSide) -> String {
        switch side {
        case .attacker: "攻撃側には、種族・性格・能力ポイント・特性・持ち物を読み込みます。技は変わりません。"
        case .defender: "防御側には、種族と特性だけを読み込みます。性格・能力ポイント・持ち物は使いません。"
        }
    }

    public static let loadCloseButton = "閉じる"
    public static let loadLoading = "お気に入りを読み込んでいます…"

    public static func loadDroppedName(_ dropped: FavoriteLoadDropped) -> String {
        switch dropped {
        case .nature: "性格と能力ポイント"
        case .item: "持ち物"
        case .ability: "特性"
        }
    }

    /// 例: 「一部は読み込めませんでした(持ち物、特性)。読み込めた分だけ設定しました。」
    public static func loadPartialNotice(_ dropped: [FavoriteLoadDropped]) -> String {
        "一部は読み込めませんでした(\(dropped.map(loadDroppedName).joined(separator: "、")))。読み込めた分だけ設定しました。"
    }

    public static let loadSpeciesMissingNotice = "このお気に入りのポケモンはいまのデータに無いため、読み込めませんでした。何も変えていません。"
    public static let loadUnavailableNotice = "お気に入りを読み込めませんでした。何も変えていません。もう一度お試しください。"
}

// MARK: - お気に入り → 計算画面の個体(純粋関数)

/// 1件のお気に入りを、いまのマスタに照らして画面に設定できる形にしたもの。
public struct FavoriteLoadPlan: Equatable, Sendable {
    /// 攻撃側のみ。nil は「使わない」(防御側・性格がマスタに無い)。`sp` と同時に nil になる。
    public var natureId: String?
    public var sp: StatBlock?
    /// 種族の特性に無ければ nil。
    public var abilityId: String?
    /// 攻撃側のみ。防御側は常に nil(使わない。落としたことにもしない)。
    public var itemId: String?
    public var dropped: [FavoriteLoadDropped]

    public init(
        natureId: String? = nil, sp: StatBlock? = nil, abilityId: String? = nil, itemId: String? = nil,
        dropped: [FavoriteLoadDropped] = []
    ) {
        self.natureId = natureId
        self.sp = sp
        self.abilityId = abilityId
        self.itemId = itemId
        self.dropped = dropped
    }
}

public enum FavoriteLoad {
    /// 構築から呼ぶ処理の `.team` スナップショットに写すときの `teamID`(お気に入り由来の印。`memberID` はお気に入りの id)。
    public static let sourceTeamID = "favorite"

    /// 規則は ADR-0513 §3。
    /// - 攻撃側: 性格(`natures` に無ければ性格と SP を落とす)・特性(`species.abilities` に無ければ落とす)・
    ///   持ち物(`items` に無い、メガ種族でストーンと違う、非メガ種族でメガストーンなら落とす。
    ///   メガ種族で保存が nil ならストーンに固定して案内なし。役割(ItemRoleFilter)に反するだけの持ち物は残して案内なし)。
    /// - 防御側: 特性だけ(性格・SP・持ち物は使わず、落としたことにもしない)。
    /// - `dropped` は `FavoriteLoadDropped` の宣言順。
    public static func plan(
        for favorite: Favorite, side: FavoriteLoadSide, species: SpeciesDetail, natures: [Nature], items: [Item]
    ) -> FavoriteLoadPlan {
        let saved = favorite.individual
        var plan = FavoriteLoadPlan()
        var droppedNature = false
        var droppedItem = false
        var droppedAbility = false

        if side == .attacker {
            if natures.contains(where: { $0.id == saved.natureId }) {
                plan.natureId = saved.natureId
                plan.sp = saved.sp
            } else {
                droppedNature = true
            }
            let (itemId, itemDropped) = attackerItem(saved: saved.itemId, species: species, items: items)
            plan.itemId = itemId
            droppedItem = itemDropped
        }

        if let abilityId = saved.abilityId {
            if species.abilities.contains(where: { $0.id == abilityId }) {
                plan.abilityId = abilityId
            } else {
                droppedAbility = true
            }
        }

        plan.dropped = [
            droppedNature ? FavoriteLoadDropped.nature : nil,
            droppedItem ? .item : nil,
            droppedAbility ? .ability : nil,
        ].compactMap { $0 }
        return plan
    }

    /// 攻撃側の持ち物(ADR-0509: メガ種族はストーンに固定、非メガ種族のメガストーンは落とす。役割に反するだけなら残す)。
    private static func attackerItem(saved: String?, species: SpeciesDetail, items: [Item]) -> (itemId: String?, dropped: Bool) {
        switch MegaItemLock.make(for: species, allItems: items) {
        case .locked(let stoneId, _):
            return (stoneId, saved != nil && saved != stoneId)
        case .missing:
            return (nil, saved != nil)
        case .none:
            guard let saved else { return (nil, false) }
            guard let item = items.first(where: { $0.id == saved }), item.isMegaStone != true else { return (nil, true) }
            return (saved, false)
        }
    }
}

// MARK: - 入口のシートの ViewModel

/// 「お気に入りから呼ぶ」シートの状態。シートを開くたびに作り、開いたとき `load()` を1回呼ぶ(`.task`)。
/// 一覧の取得・名前の解決・古い応答の破棄は `FavoritesViewModel` と同じ規則(実装は包んでよい)。
@MainActor
@Observable
public final class FavoriteLoadPickerViewModel {
    public private(set) var rows: [FavoriteRow] = []
    public private(set) var loadState: RecordLoadState = .idle

    let service: (any FavoritesService)?
    let resolver: any PokeCalcService

    /// 最新の `load()` の世代番号。進むと先行の呼び出しは結果を捨てる。
    private var generation = 0

    /// - Parameters:
    ///   - service: nil なら入口を出さない(`isAvailable == false`)。`load()` は何もしない。
    ///   - resolver: 種族名を引く先(計算と同じ `PokeCalcService`)。
    public init(service: (any FavoritesService)?, resolver: any PokeCalcService) {
        self.service = service
        self.resolver = resolver
    }

    /// サービスがあるときだけ、計算画面が入口を出す(`FavoritePinViewModel.isAvailable` と同じ流儀)。
    public var isAvailable: Bool { service != nil }

    /// 取得済みで0件(シートが `emptyFavorites` を出す)。
    public var isEmpty: Bool { loadState == .loaded && rows.isEmpty }

    /// `favorites()` を1回呼び、種族名を解決して `rows` を置き換える(サーバーの順のまま。マスタに無い種族の行も残す)。
    /// 再呼び出し(再読み込み)では、新しい方の応答だけを反映する。Task cancel では状態を変えない。
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
        rows = favorites.map { FavoriteRow(favorite: $0, speciesName: names[$0.individual.speciesKey]) }
        loadState = .loaded
    }
}
