import Foundation

// フェーズ5-3 詳細の価格の帯(web/src/lib/band.ts の移植。docs/phase5-spec.md 5-2「詳細」)。

/// 帯の上の 1 点(1 サイト)。
public struct BandDot: Equatable, Sendable {
    public var siteID: Int
    public var low: Int
    /// 整数 0〜100(Web の `data-position` と同じ契約): 点にするサイトの最安を 0、最高を 100 として `round((low - min) / (max - min) * 100)`。全部同額なら 0
    public var position: Int
    /// 帯の線の上での描画位置 0...1。通常は `position / 100`。**全部同額(min == max)のときは 0.5(中央)**。iOS だけの決まり
    public var fraction: Double

    public init(siteID: Int, low: Int, position: Int, fraction: Double) {
        self.siteID = siteID
        self.low = low
        self.position = position
        self.fraction = fraction
    }
}

public struct PriceBand: Equatable, Sendable {
    /// `sites` の順。`low` を持つサイト(取得失敗の前回値も含む)だけ
    public var dots: [BandDot]
    /// `low` の最小のサイト(同額は全部)。点が無ければ空
    public var bestSiteIDs: Set<Int>
    /// 「安い〜中央」の太線の範囲(帯の線の上での 0...1)。通常は `0...1`。点が 0・1 個、または全部同額のときは **nil(伸ばした太線を描かない)**
    public var rangeBar: ClosedRange<Double>?

    public init(dots: [BandDot], bestSiteIDs: Set<Int>, rangeBar: ClosedRange<Double>?) {
        self.dots = dots
        self.bestSiteIDs = bestSiteIDs
        self.rangeBar = rangeBar
    }

    /// `estimates` から `sites`(このジャンルのサイトを表示順に)の `low` を引いて組む。
    public static func make(sites: [Site], estimates: [SiteEstimate]) -> PriceBand {
        let lows: [(siteID: Int, low: Int)] = sites.compactMap { site in
            guard let low = estimates.first(where: { $0.siteID == site.id })?.low else { return nil }
            return (site.id, low)
        }
        guard let minLow = lows.map(\.low).min(), let maxLow = lows.map(\.low).max() else {
            return PriceBand(dots: [], bestSiteIDs: [], rangeBar: nil)
        }
        let equal = minLow == maxLow
        let dots = lows.map { entry -> BandDot in
            let position = equal ? 0 : Int((Double(entry.low - minLow) / Double(maxLow - minLow) * 100).rounded())
            let fraction = equal ? 0.5 : Double(position) / 100
            return BandDot(siteID: entry.siteID, low: entry.low, position: position, fraction: fraction)
        }
        return PriceBand(
            dots: dots, bestSiteIDs: Set(lows.filter { $0.low == minLow }.map(\.siteID)),
            rangeBar: equal ? nil : 0.0...1.0)
    }
}
