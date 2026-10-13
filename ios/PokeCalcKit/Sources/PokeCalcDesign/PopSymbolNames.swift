// PopSymbolNames: G-05 で Web が足したアイコン(close/open/history/help/up/down/minus)に対応する SF Symbols の名前。
//
// spec-writer の骨組み: 値は実装者が ADR-0501「G-05 …」章の表どおりに入れる(今は空。`PopSymbolNamesTests` が失敗する)。
// アプリ側の `PopSymbol`(ios/PokeCalc/PopComponents.swift)はこの値を参照して同じ名前を持つ。
public enum PopSymbolNames {
    public static let close = "xmark"
    public static let open = "chevron.down"
    public static let history = "clock.arrow.circlepath"
    public static let help = "questionmark.circle"
    public static let up = "arrow.up"
    public static let down = "arrow.down"
    public static let minus = "minus"
    /// 調整の「耐える」目標と「目標」の見出し(Web のアイコン名 `judge`。盾の形)。G-05 第 4 弾(ADR-0540)。
    /// spec-writer の骨組み: 値は実装者が入れる(今は空。`PopSymbolNamesTests` が失敗する)。
    public static let judge = "shield"
}
