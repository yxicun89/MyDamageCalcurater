// SpeedNeighborhoodLabels: 「自分の周り」パネルの固定文言(G-04。ADR-0527)。
// Web の `speedScreenText.neighborhood*`(web/src/i18n/speed.ts。ADR-0609)と同じ文言にそろえる。
// やさしい言葉(F-13)。通常の場は「速い側/遅い側」、トリックルームは「先に動く側/後に動く側」(ADR-0607)。

extension SpeedLabels {
    public static let neighborhoodRegion = "自分の周り"
    public static let neighborhoodHeading = "自分の周り"
    public static let neighborhoodEmpty = "自分のポケモンを選ぶと、ここに前後のポケモンが出ます"
    public static let neighborhoodFasterList = "自分より速い側"
    public static let neighborhoodSlowerList = "自分より遅い側"
    public static let neighborhoodBeforeList = "先に動く側"
    public static let neighborhoodAfterList = "後に動く側"
    public static let neighborhoodSelfBadge = "自分"
    public static let neighborhoodTie = "同速"
    public static let neighborhoodBoundary = "ここに自分が入ります(同速なし)"
    public static let neighborhoodNone = "いません"
    /// 色だけに頼らない目印の文字(速い/先に動く側 = 上向き、遅い/後に動く側 = 下向き)。
    public static let neighborhoodUpArrow = "↑"
    public static let neighborhoodDownArrow = "↓"

    public static func neighborhoodMore(_ n: Int) -> String { "ほか \(n) 体" }
    public static func neighborhoodFasterTotal(_ n: Int) -> String { "これより速いポケモンは計 \(n) 体" }
    public static func neighborhoodSlowerTotal(_ n: Int) -> String { "これより遅いポケモンは計 \(n) 体" }
    public static func neighborhoodBeforeTotal(_ n: Int) -> String { "これより先に動くポケモンは計 \(n) 体" }
    public static func neighborhoodAfterTotal(_ n: Int) -> String { "これより後に動くポケモンは計 \(n) 体" }
}
