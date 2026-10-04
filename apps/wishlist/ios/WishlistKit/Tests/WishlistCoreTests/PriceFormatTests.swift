import XCTest

@testable import WishlistCore

// AC-IOS-EST-08: 金額・日付・理由の整形(純関数。web の src/lib/price.test.ts と同じ表)。
// JST・ja-JP は実行環境(CI のタイムゾーン)に依存させない。
final class PriceFormatTests: XCTestCase {
    func testYen() {
        let table: [(Int, String)] = [(0, "¥0"), (300, "¥300"), (3000, "¥3,000"), (1_234_567, "¥1,234,567")]
        for (amount, want) in table {
            XCTAssertEqual(PriceFormat.yen(amount), want, "\(amount)")
        }
    }

    func testRange() {
        let table: [(Int, Int?, String)] = [
            (3000, 4500, "¥3,000〜¥4,500"),
            (3000, nil, "¥3,000〜"),
            (3000, 3000, "¥3,000〜¥3,000"),
        ]
        for (low, mid, want) in table {
            XCTAssertEqual(PriceFormat.range(low: low, mid: mid), want, "low=\(low) mid=\(String(describing: mid))")
        }
    }

    /// JST の M/D(ゼロ詰めなし)。UTC では前日になる時刻・年またぎ・オフセット付き表記を含む。
    func testJSTDate() {
        let table: [(String, String)] = [
            ("2026-10-03T00:30:00Z", "10/3"),
            ("2026-10-02T15:30:00Z", "10/3"),  // UTC では 10/2。JST では 10/3 0:30
            ("2026-10-02T14:59:59Z", "10/2"),
            ("2026-12-31T15:00:00Z", "1/1"),  // 年またぎ
            ("2026-01-05T00:00:00Z", "1/5"),
            ("2026-10-03T08:00:00+09:00", "10/3"),
            ("2026-10-03T00:00:00+09:00", "10/3"),
        ]
        for (text, want) in table {
            XCTAssertEqual(PriceFormat.jstDate(iso(text)), want, text)
        }
    }

    /// JST の暦日の差(12 時間前でも暦日が前日なら 1 日前。未来は今日。月単位にしない)。
    func testAge() {
        let now = iso("2026-10-03T03:00:00Z")  // JST 10/3 12:00
        let table: [(String, String)] = [
            ("2026-10-03T03:00:00Z", "今日"),
            ("2026-10-02T15:00:00Z", "今日"),  // JST 0:00 ちょうど
            ("2026-10-02T14:59:00Z", "1日前"),
            ("2026-10-02T00:00:00Z", "1日前"),
            ("2026-09-30T00:00:00Z", "3日前"),
            ("2026-09-03T00:00:00Z", "30日前"),
            ("2026-10-04T00:00:00Z", "今日"),  // 未来(時計のずれ)
        ]
        for (text, want) in table {
            XCTAssertEqual(PriceFormat.age(of: iso(text), now: now), want, text)
        }
    }

    func testReasonLabels() {
        XCTAssertEqual(PriceFormat.reasonLabel(.titleMismatch), "商品名が一致しない")
        XCTAssertEqual(PriceFormat.reasonLabel(.tooCheap), "安すぎる")
        XCTAssertEqual(PriceFormat.reasonLabel(.belowMin), "下限価格未満")
    }
}
