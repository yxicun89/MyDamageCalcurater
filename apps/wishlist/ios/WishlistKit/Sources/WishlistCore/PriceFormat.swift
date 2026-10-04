import Foundation

/// 目安価格の表示用の純関数(フェーズ3。docs/phase3-ios-spec.md AC-IOS-EST-08)。
/// 日付は **JST**、桁区切りは **ja-JP**(実行環境のロケール・タイムゾーンに依存しない)。
public enum PriceFormat {
    private static var jst: Calendar {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "Asia/Tokyo") ?? TimeZone(secondsFromGMT: 9 * 3600)!
        return calendar
    }

    /// `3000` → `¥3,000`
    public static func yen(_ amount: Int) -> String {
        let digits = String(abs(amount))
        var grouped = ""
        for (index, character) in digits.reversed().enumerated() {
            if index > 0 && index % 3 == 0 { grouped.append(",") }
            grouped.append(character)
        }
        return (amount < 0 ? "-¥" : "¥") + String(grouped.reversed())
    }

    /// `¥low〜¥mid`。`mid` が nil なら `¥low〜`
    public static func range(low: Int, mid: Int?) -> String {
        "\(yen(low))〜" + (mid.map(yen) ?? "")
    }

    /// JST の `M/D`(ゼロ詰めしない)
    public static func jstDate(_ date: Date) -> String {
        let parts = jst.dateComponents([.month, .day], from: date)
        return "\(parts.month ?? 0)/\(parts.day ?? 0)"
    }

    /// JST の暦日の差で `今日` / `N日前`(未来は `今日`)
    public static func age(of date: Date, now: Date) -> String {
        let calendar = jst
        let days = calendar.dateComponents([.day], from: calendar.startOfDay(for: date), to: calendar.startOfDay(for: now)).day ?? 0
        return days <= 0 ? "今日" : "\(days)日前"
    }

    /// `商品名が一致しない` / `安すぎる` / `下限価格未満`
    public static func reasonLabel(_ reason: SuspiciousReason) -> String {
        switch reason {
        case .titleMismatch: "商品名が一致しない"
        case .tooCheap: "安すぎる"
        case .belowMin: "下限価格未満"
        }
    }
}
