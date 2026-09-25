import Foundation

// BulkRowDisplay: 一括計算の結果行を画面に出す形へ整形する(P6-2a・docs/design.md「画面: ダメージ計算」)。
//
// サーバーの値を丸め直さない(%の丸めは engine の責務。ADR-0010 §3)。純粋な整形だけを持ち、
// View はここが作った文字列・数値をそのまま描く。

/// 確定数の「段階」(倒せない / 確定n発 / 乱数n発)。バッジを弾ませるかどうかは段階が変わったかで決める
/// (確率の数字だけの変化では弾ませない。docs/design.md「動き」)。
public enum KOTier: Equatable, Sendable {
    case cannotKO
    case guaranteed(hits: Int)
    case chance(hits: Int)

    public init(_ ko: KOChance) {
        if ko.hits == 0 {
            self = .cannotKO
        } else if ko.guaranteed {
            self = .guaranteed(hits: ko.hits)
        } else {
            self = .chance(hits: ko.hits)
        }
    }

    /// `previous`(直前の表示。初回は nil)から `current` へ、段階が変わったときだけ true。
    public static func changed(from previous: KOChance?, to current: KOChance) -> Bool {
        guard let previous else { return false }
        return KOTier(previous) != KOTier(current)
    }
}

/// 一括計算の1行(`BulkCalcRow`)を画面向けに整形した値。
public struct BulkRowDisplay: Identifiable, Equatable, Sendable {
    /// XCUITest の `calcResultRow-<id>` に使う安定な ID(`<preset の rawValue>@<itemId。nil は "-">`)。
    public let id: String
    public let preset: DefenderPreset
    /// 調整名。サーバーの `presetLabel` をそのまま使う(クライアントで名前を作らない)。
    public let presetLabel: String
    public let itemId: String?
    public let itemLabel: String
    /// 「72.1〜85.3%」のような%幅。
    public let percentRangeText: String
    /// ダメージバーの左端(0...1 に収めた minPercent / 100)。
    public let barMinFraction: Double
    /// ダメージバーの右端(0...1 に収めた maxPercent / 100)。
    public let barMaxFraction: Double
    /// 「確定2発」「乱数2発(45.6%)」「倒せない」のような文言。
    public let koText: String
    public let koTier: KOTier
    /// タイプ相性(0, 0.25, 0.5, 1, 2, 4)。`CalcViewModel.moveEffectiveness` が全行の一致を見るのに使う。
    public let effectiveness: Double
    /// この行だけに付いた未対応の印の注記(`UnsupportedNoticeText.rowNote`。無ければ nil)。
    /// 全行に共通する印は行ではなく `BulkResultDisplay.unsupportedNotice` に出す(ADR-0501「P6-17」3章)。
    public let unsupportedNote: String?

    public init(row: BulkCalcRow, items: [Item], unsupportedNote: String? = nil) {
        self.unsupportedNote = unsupportedNote
        preset = row.preset
        presetLabel = row.presetLabel
        itemId = row.itemId
        itemLabel = Self.itemLabel(itemId: row.itemId, items: items)
        percentRangeText = Self.percentRangeText(minPercent: row.result.minPercent, maxPercent: row.result.maxPercent)
        barMinFraction = Self.barFraction(percent: row.result.minPercent)
        barMaxFraction = Self.barFraction(percent: row.result.maxPercent)
        koText = Self.koText(row.result.ko)
        effectiveness = row.result.effectiveness
        koTier = KOTier(row.result.ko)
        id = "\(row.preset.rawValue)@\(row.itemId ?? Self.noItemIDPlaceholder)"
    }

    /// `id` で「持ち物なし」を表す記号。
    private static let noItemIDPlaceholder = "-"
    private static let noItemLabel = "持ち物なし"
    private static let cannotKOLabel = "倒せない"
    /// %幅・確率の区切り・小数点の書式はロケールに依存させない(端末の言語設定でカンマ小数点になる等を避ける)。
    private static let numberLocale = Locale(identifier: "en_US_POSIX")
    /// design.md の%幅の区切り文字(U+301C 波ダッシュ)。
    private static let rangeSeparator = "\u{301C}"

    /// `itemId` が `nil` なら「持ち物なし」、マスタに無い ID はその ID をそのまま返す
    /// (黙って「持ち物なし」に丸めない)。
    public static func itemLabel(itemId: String?, items: [Item]) -> String {
        guard let itemId else { return noItemLabel }
        return items.first(where: { $0.id == itemId })?.nameJa ?? itemId
    }

    /// 「72.1〜85.3%」のような文字列。小数第1位固定・100% 超もそのまま出す。
    public static func percentRangeText(minPercent: Double, maxPercent: Double) -> String {
        "\(formatOneDecimal(minPercent))\(rangeSeparator)\(formatOneDecimal(maxPercent))%"
    }

    /// ダメージバーの割合(0...1)。100% 超はバーいっぱい、負の値(来ない契約だが)は 0 にする。
    public static func barFraction(percent: Double) -> Double {
        min(max(percent / 100, 0), 1)
    }

    /// 確定数の文言。`displayChancePercent` を使い、`chancePercent`(engine の生値)は使わない
    /// (openapi `KOChance` の説明のとおり)。
    public static func koText(_ ko: KOChance) -> String {
        guard ko.hits > 0 else { return cannotKOLabel }
        if ko.guaranteed {
            return "確定\(ko.hits)発"
        }
        return "乱数\(ko.hits)発(\(formatOneDecimal(ko.displayChancePercent))%)"
    }

    private static func formatOneDecimal(_ value: Double) -> String {
        String(format: "%.1f", locale: numberLocale, value)
    }
}
