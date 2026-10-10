// SpeedNeighborhood: 素早さ画面の「自分の周り」パネルの純粋な切り出し(G-04。ADR-0527。Web は ADR-0609)。
// 入力は `SpeedViewModel.tableRows`(表の並び。通常 = 速い順、トリックルーム = 遅い順 = 先に動く順)と自分の実数値。
// 素早さは計算し直さず、段の実数値と自分の実数値の直接比較だけで前後を決める(ADR-0503 §6)。

/// 近傍に出す1段(同じ実数値)。
public struct SpeedNeighborTier: Equatable, Identifiable, Sendable {
    public let speed: Int
    /// 代表のポケモン名(段の並びの先頭から)。
    public let names: [String]
    /// `names` に入らなかった体数(「ほか n 体」)。0 なら出さない。
    public let moreCount: Int
    /// この段の総体数。
    public let entryCount: Int

    public var id: Int { speed }

    public init(speed: Int, names: [String], moreCount: Int, entryCount: Int) {
        self.speed = speed
        self.names = names
        self.moreCount = moreCount
        self.entryCount = entryCount
    }
}

/// 自分の周り。`before`/`after` は表示順(`before`: 上 = 遠い → 下 = 自分に近い、`after`: 上 = 自分に近い → 下 = 遠い)。
public struct SpeedNeighborhood: Equatable, Sendable {
    public let ownSpeed: Int
    /// 自分と同じ実数値の段。無ければ nil(自分だけが入る境界)。
    public let tie: SpeedNeighborTier?
    /// 先に動く側(表の並びで自分より前)の直近の段。最大 steps 段。
    public let before: [SpeedNeighborTier]
    /// 後に動く側(表の並びで自分より後)の直近の段。最大 steps 段。
    public let after: [SpeedNeighborTier]
    /// 片側の全体の体数(表示した段の外も含む。同速の段は含めない)。
    public let beforeTotal: Int
    public let afterTotal: Int
    /// 表示した段の外にまだ段が残っているか。
    public let beforeHasMore: Bool
    public let afterHasMore: Bool

    public init(
        ownSpeed: Int, tie: SpeedNeighborTier?, before: [SpeedNeighborTier], after: [SpeedNeighborTier],
        beforeTotal: Int, afterTotal: Int, beforeHasMore: Bool, afterHasMore: Bool
    ) {
        self.ownSpeed = ownSpeed
        self.tie = tie
        self.before = before
        self.after = after
        self.beforeTotal = beforeTotal
        self.afterTotal = afterTotal
        self.beforeHasMore = beforeHasMore
        self.afterHasMore = afterHasMore
    }
}

public enum SpeedNeighborhoodBuilder {
    /// 自分の前後に出す段の既定数(Web の `DEFAULT_NEIGHBOR_STEPS` と同じ 3)。
    public static let defaultSteps = 3
    /// 近傍の段の代表名の数の既定(1段につき)。残りは「ほか n 体」。
    public static let defaultNamesPerTier = 1
    /// 同速の段で名前を並べる数の既定。残りは「ほか n 体」。
    public static let defaultTieNames = 4

    /// 自分の周りを切り出す。`rows` は `tableRows` の並びのまま(`.selfBoundary` は無視してよい)。
    /// 通常の場は speed が自分より大きい段が「先に動く側」、トリックルームは小さい段が「先に動く側」。
    /// `ownSpeed` が nil(自分が未決定)なら nil。
    public static func build(
        rows: [SpeedTableRow], ownSpeed: Int?, trickRoom: Bool,
        steps: Int = defaultSteps, namesPerTier: Int = defaultNamesPerTier, tieNames: Int = defaultTieNames
    ) -> SpeedNeighborhood? {
        guard let ownSpeed else { return nil }
        let tiers: [SpeedTierDisplay] = rows.compactMap {
            if case .tier(let tier) = $0 { return tier }
            return nil
        }
        // 表の並びで自分より前 = 先に動く側。通常(降順)は速い段、トリックルーム(昇順)は遅い段。
        let isBefore: (Int) -> Bool = { trickRoom ? $0 < ownSpeed : $0 > ownSpeed }
        let beforeAll = tiers.filter { isBefore($0.speed) }
        let tieTier = tiers.first { $0.speed == ownSpeed }
        let afterAll = tiers.filter { $0.speed != ownSpeed && !isBefore($0.speed) }

        func toTier(_ tier: SpeedTierDisplay, limit: Int) -> SpeedNeighborTier {
            let names = tier.entries.prefix(max(0, limit)).map(\.nameJa)
            return SpeedNeighborTier(
                speed: tier.speed, names: names, moreCount: tier.entries.count - names.count, entryCount: tier.entries.count)
        }
        func total(_ list: [SpeedTierDisplay]) -> Int { list.reduce(0) { $0 + $1.entries.count } }

        return SpeedNeighborhood(
            ownSpeed: ownSpeed,
            tie: tieTier.map { toTier($0, limit: tieNames) },
            before: beforeAll.suffix(max(0, steps)).map { toTier($0, limit: namesPerTier) },
            after: afterAll.prefix(max(0, steps)).map { toTier($0, limit: namesPerTier) },
            beforeTotal: total(beforeAll), afterTotal: total(afterAll),
            beforeHasMore: beforeAll.count > steps, afterHasMore: afterAll.count > steps)
    }
}

extension SpeedViewModel {
    /// 表と自分の位置から作る「自分の周り」。位置が未決定・表が未読み込みなら nil。
    public var neighborhood: SpeedNeighborhood? {
        guard case .loaded(let position) = positionState, case .loaded = tableState else { return nil }
        return SpeedNeighborhoodBuilder.build(rows: tableRows, ownSpeed: position.speed, trickRoom: trickRoom)
    }
}
