import Foundation

// フェーズ4-2 価格の推移のグラフ(Swift Charts)に渡す形と、その純関数(docs/phase4-spec.md AC-IOS-HIS-*)。
// 文言・規則は PWA(web/src/lib/history.ts)と同じ。日付は API の `YYYY-MM-DD`(JST の日付)を JST の 0:00 の Date にする。

/// グラフの 1 点。`segment` は線のまとまりの番号(0 始まり。前の点と 1 日より空いたら 1 増やす)。
/// Swift Charts の `LineMark(series:)` に「系列の id + segment」を渡して、出品の無い日で線を切る。
public struct HistoryChartPoint: Equatable, Sendable, Identifiable {
    public var day: String
    /// JST の `day` の 0:00
    public var date: Date
    public var value: Int
    public var segment: Int

    public var id: String { day }

    public init(day: String, date: Date, value: Int, segment: Int) {
        self.day = day
        self.date = date
        self.value = value
        self.segment = segment
    }
}

/// 1 本の線。`id` は `overall` か `site-<siteID>`。
public struct HistorySeries: Equatable, Sendable, Identifiable {
    public var id: String
    public var name: String
    public var points: [HistoryChartPoint]

    public init(id: String, name: String, points: [HistoryChartPoint]) {
        self.id = id
        self.name = name
        self.points = points
    }
}

/// 凡例のボタン 1 つ(サイトの線の表示切り替え)。
public struct HistoryLegendItem: Equatable, Sendable, Identifiable {
    public var siteID: Int
    public var name: String
    public var isSelected: Bool

    public var id: Int { siteID }

    public init(siteID: Int, name: String, isSelected: Bool) {
        self.siteID = siteID
        self.name = name
        self.isSelected = isSelected
    }
}

/// グラフ全体。`sites` は API の `sites` の順(点の無いサイトは除く)。
public struct PriceHistoryChart: Equatable, Sendable {
    public var overall: HistorySeries
    public var sites: [HistorySeries]
    /// VoiceOver・XCUITest 用の要約(`PriceHistoryFormat.accessibilityLabel`)
    public var accessibilityLabel: String

    public init(overall: HistorySeries, sites: [HistorySeries], accessibilityLabel: String) {
        self.overall = overall
        self.sites = sites
        self.accessibilityLabel = accessibilityLabel
    }
}

public enum PriceHistoryState: Equatable, Sendable {
    case notLoaded
    case loading
    /// 全体の最安の点が 2 未満 → `WishlistText.priceHistoryEmpty`
    case empty
    case loaded(PriceHistoryChart)
    /// 折りたたみの中に短い文を出す(通信できない → `WishlistText.offline`、それ以外 → `WishlistText.priceHistoryFailed`)
    case failed(String)
}

public enum PriceHistoryFormat {
    /// `2026-10-03` → JST 2026-10-03 0:00(= 2026-10-02T15:00:00Z)。形が違う・存在しない日付は nil
    public static func date(fromDay day: String) -> Date? {
        _ = day
        return nil  // TODO(implementer)
    }

    /// `2026-10-03` → `10/3`(ゼロ詰めしない)。形が違えば空文字
    public static func dayLabel(_ day: String) -> String {
        _ = day
        return ""  // TODO(implementer)
    }

    /// 点を day 昇順に並べ、`date(fromDay:)` が nil の点は捨て(値を作らない)、1 日より空いたところで `segment` を進める。
    public static func points(_ values: [(day: String, value: Int)]) -> [HistoryChartPoint] {
        _ = values
        return []  // TODO(implementer)
    }

    /// `価格の推移 7/6〜10/3 最安 ¥2,000 最高 ¥3,200`(1 日だけなら期間は `10/3`、空なら `価格の推移はまだありません`)
    public static func accessibilityLabel(overall: [DayLow]) -> String {
        _ = overall
        return ""  // TODO(implementer)
    }

    /// API の推移からグラフを作る。全体の最安の(有効な)点が 2 未満なら nil。
    /// サイト名は `sites` から引き、無いサイトは `サイト<ID>`。サイトの線は low の値で描く。
    public static func chart(_ history: PriceHistory, sites: [Site]) -> PriceHistoryChart? {
        _ = (history, sites)
        return nil  // TODO(implementer)
    }
}
