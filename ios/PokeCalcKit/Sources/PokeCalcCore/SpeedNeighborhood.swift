// SpeedNeighborhood: 素早さ画面の「自分の周り」パネルの純粋な切り出し(G-04。ADR-0527。Web は ADR-0609)。
// 入力は `SpeedViewModel.tableRows`(表の並び。通常 = 速い順、トリックルーム = 遅い順 = 先に動く順)と自分の実数値。
// 素早さは計算し直さず、段の実数値と自分の実数値の直接比較だけで前後を決める(ADR-0503 §6)。
//
// 【spec-writer の足場】本体は未実装。`build` は nil を返す。実装者は SpeedNeighborhoodTests を通すこと。

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
        // TODO(implementer): 実装する。
        nil
    }
}

extension SpeedViewModel {
    /// 表と自分の位置から作る「自分の周り」。位置が未決定・表が未読み込みなら nil。
    /// 【足場】常に nil。実装者は `SpeedNeighborhoodBuilder.build(rows: tableRows, ownSpeed:, trickRoom:)` につなぐ。
    public var neighborhood: SpeedNeighborhood? { nil }
}
