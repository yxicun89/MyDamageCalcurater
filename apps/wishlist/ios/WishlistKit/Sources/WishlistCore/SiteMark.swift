import Foundation

// フェーズ5-3 サイトのマーク(web/src/lib/siteMark.ts の移植。docs/phase5-spec.md 5-3「決めたこと」)。

public enum SiteMark {
    /// 名前が表に無いサイトの地の色(`--muted`)
    public static let fallbackColorHex = "#56677d"

    /// マークの地の色(`#rrggbb`)。`sites` に色の項目がまだ無いので名前から引く固定の表。
    /// メルカリ #ff4d6d・駿河屋 #e17055・Yahoo!フリマ #ef233c・Yahoo!ショッピング #2f9e44・あみあみ #00a884・Amazon #ff9f1c・魂ウェブ #5b4bd6。無い名前は `fallbackColorHex`
    public static func colorHex(for name: String) -> String {
        colors[name] ?? fallbackColorHex
    }

    private static let colors: [String: String] = [
        "メルカリ": "#ff4d6d", "駿河屋": "#e17055", "Yahoo!フリマ": "#ef233c", "Yahoo!ショッピング": "#2f9e44",
        "あみあみ": "#00a884", "Amazon": "#ff9f1c", "魂ウェブ": "#5b4bd6",
    ]

    private static let yahooPrefix = "Yahoo!"

    /// マークの 1 文字。通常はサイト名の先頭 1 文字。`Yahoo!` で始まる名前は、その後ろの最初の 1 文字
    /// (Yahoo!フリマ → 「フ」・Yahoo!ショッピング → 「シ」。同じ「Y」が並ばないため)。空の名前は ""。
    public static func mark(for name: String) -> String {
        var rest = Substring(name)
        if name.hasPrefix(yahooPrefix), name.count > yahooPrefix.count { rest = name.dropFirst(yahooPrefix.count) }
        return rest.first.map(String.init) ?? ""
    }
}
