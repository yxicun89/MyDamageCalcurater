import Foundation
import XCTest

@testable import WishlistCore

// AC-IOS-HIS-06〜08: 価格の推移の純関数(PWA の AC-HIS-07〜09 と同じ規則。docs/phase4-spec.md)。
final class PriceHistoryFormatTests: XCTestCase {
    func testDateFromDayIsJSTMidnight() {
        let table: [(String, Date?)] = [
            ("2026-10-03", iso("2026-10-02T15:00:00Z")),
            ("2027-01-01", iso("2026-12-31T15:00:00Z")),
            ("2026-02-30", nil),
            ("2026/10/03", nil),
            ("", nil),
        ]
        for (day, want) in table {
            XCTAssertEqual(PriceHistoryFormat.date(fromDay: day), want, day)
        }
    }

    func testDayLabel() {
        let table: [(String, String)] = [
            ("2026-10-03", "10/3"), ("2026-12-31", "12/31"), ("2027-01-01", "1/1"), ("2026-07-06", "7/6"), ("bad", ""),
        ]
        for (day, want) in table {
            XCTAssertEqual(PriceHistoryFormat.dayLabel(day), want, day)
        }
    }

    /// 1 日より空いたら segment を進める(線を切る)。日付順に並べ、読めない日付の点は捨てる(値を作らない)。年またぎは続いた日。
    func testPointsSortDropInvalidAndSplitSegments() {
        let table: [(name: String, input: [(day: String, value: Int)], want: [(String, Int, Int)])] = [
            ("続いた 3 日", [("2026-10-01", 3000), ("2026-10-02", 3100), ("2026-10-03", 3200)],
             [("2026-10-01", 3000, 0), ("2026-10-02", 3100, 0), ("2026-10-03", 3200, 0)]),
            ("10/3 が無い", [("2026-10-01", 3000), ("2026-10-02", 3000), ("2026-10-04", 3600)],
             [("2026-10-01", 3000, 0), ("2026-10-02", 3000, 0), ("2026-10-04", 3600, 1)]),
            ("並べ替え・2 か所で切る", [("2026-10-09", 1), ("2026-10-01", 2), ("2026-10-05", 3)],
             [("2026-10-01", 2, 0), ("2026-10-05", 3, 1), ("2026-10-09", 1, 2)]),
            ("読めない日付は捨てる", [("2026-10-01", 3000), ("2026-13-01", 1), ("2026-10-02", 3100)],
             [("2026-10-01", 3000, 0), ("2026-10-02", 3100, 0)]),
            ("年またぎ", [("2026-12-31", 5000), ("2027-01-01", 5000)], [("2026-12-31", 5000, 0), ("2027-01-01", 5000, 0)]),
            ("空", [], []),
        ]
        for row in table {
            let got = PriceHistoryFormat.points(row.input)
            XCTAssertEqual(got.map(\.day), row.want.map(\.0), row.name)
            XCTAssertEqual(got.map(\.value), row.want.map(\.1), row.name)
            XCTAssertEqual(got.map(\.segment), row.want.map(\.2), row.name)
            for p in got {
                XCTAssertEqual(p.date, PriceHistoryFormat.date(fromDay: p.day), row.name)
            }
        }
    }

    func testAccessibilityLabel() {
        let table: [(String, [DayLow], String)] = [
            ("期間・最安・最高",
             [DayLow(day: "2026-07-06", low: 2000), DayLow(day: "2026-10-01", low: 2800), DayLow(day: "2026-10-03", low: 3200)],
             "価格の推移 7/6〜10/3 最安 ¥2,000 最高 ¥3,200"),
            ("最安・最高は日付の順と関係ない", [DayLow(day: "2026-10-01", low: 3500), DayLow(day: "2026-10-02", low: 3000)],
             "価格の推移 10/1〜10/2 最安 ¥3,000 最高 ¥3,500"),
            ("1 日だけ", [DayLow(day: "2026-10-03", low: 3000)], "価格の推移 10/3 最安 ¥3,000 最高 ¥3,000"),
            ("空", [], "価格の推移はまだありません"),
        ]
        for (name, overall, want) in table {
            XCTAssertEqual(PriceHistoryFormat.accessibilityLabel(overall: overall), want, name)
        }
    }

    /// chart: 全体の最安の点が 2 未満なら nil。サイト名は sites から、無ければ `サイト<ID>`。サイトの線は low。
    func testChart() throws {
        let history = PriceHistory(
            itemID: 12,
            sites: [
                SitePriceHistory(siteID: 1, points: [PricePoint(day: "2026-10-01", low: 3000, mid: 3400), PricePoint(day: "2026-10-03", low: 3500)]),
                SitePriceHistory(siteID: 9, points: [PricePoint(day: "2026-10-02", low: 3300)]),
            ],
            overall: [DayLow(day: "2026-10-01", low: 3000), DayLow(day: "2026-10-02", low: 3300), DayLow(day: "2026-10-03", low: 3500)])
        let chart = try XCTUnwrap(PriceHistoryFormat.chart(history, sites: T.sites))
        XCTAssertEqual(chart.overall.id, "overall")
        XCTAssertEqual(chart.overall.name, "全体の最安")
        XCTAssertEqual(chart.overall.points.map(\.value), [3000, 3300, 3500])
        XCTAssertEqual(chart.sites.map(\.id), ["site-1", "site-9"])
        XCTAssertEqual(chart.sites.map(\.name), ["メルカリ", "サイト9"])
        XCTAssertEqual(chart.sites.first?.points.map(\.value), [3000, 3500])
        XCTAssertEqual(chart.sites.first?.points.map(\.segment), [0, 1], "10/2 が無いので線を切る")
        XCTAssertEqual(chart.accessibilityLabel, "価格の推移 10/1〜10/3 最安 ¥3,000 最高 ¥3,500")

        let one = PriceHistory(itemID: 12, sites: [SitePriceHistory(siteID: 1, points: [PricePoint(day: "2026-10-03", low: 3000)])],
                               overall: [DayLow(day: "2026-10-03", low: 3000)])
        XCTAssertNil(PriceHistoryFormat.chart(one, sites: T.sites), "点が 1 つならグラフにしない")
        XCTAssertNil(PriceHistoryFormat.chart(PriceHistory(itemID: 12), sites: T.sites))
        let invalid = PriceHistory(itemID: 12, overall: [DayLow(day: "2026-10-03", low: 3000), DayLow(day: "bad", low: 1)])
        XCTAssertNil(PriceHistoryFormat.chart(invalid, sites: T.sites), "読めない日付は点に数えない")
    }
}
