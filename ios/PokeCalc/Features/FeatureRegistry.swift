import PokeCalcCore

/// ルート画面から開く画面の一覧(ADR-0507 §1・§3)。
enum FeatureRegistry {
    /// 登録されている画面。**画面を足すときは末尾に1行足すだけ**(1行1要素・末尾カンマ)。
    /// 並びは各機能の `order` で決まるので、ここの行の順は意味を持たない。
    private static let registered: [any AppFeature] = [
        CalcFeature(),
        ReverseFeature(),
        TeamListFeature(),
        AdjustFeature(),
        BalanceFeature(),
        SpeedFeature(),
        JudgeFeature(),
        AboutFeature(),
    ]

    /// `order` 順の画面(入口の並び。`order` の重複は起動時の設定エラー)。
    static let features: [any AppFeature] = registered.sorted { $0.order < $1.order }

    /// 検証・起動時に開く画面の選択に使う登録情報(`FeatureCatalog`)。
    static let specs: [FeatureSpec] = features.map(\.spec)

    static func feature(id: String) -> (any AppFeature)? {
        features.first { $0.id == id }
    }
}
