import Foundation

/// 既定では入口を出さない画面(機能)の可視性(F-07。ADR-0507 の機能レジストリの上に載せる)。
///
/// 非表示にした画面は、環境変数 `POKECALC_SHOW_<ID 大文字>=1` を渡すと出る。
/// 判定の撤去(ADR-0715)で既定の非表示の画面は無くなった。次に画面を一時的に隠すときのために仕組みだけ残す。
public enum FeatureVisibility {
    /// 既定で入口を出さない機能の `id`。
    public static let hiddenByDefaultIDs: Set<String> = []

    /// その機能を出すための環境変数のキー(`POKECALC_SHOW_SPEED` など)。
    public static func showEnvironmentKey(forFeatureID id: String) -> String {
        "POKECALC_SHOW_\(id.uppercased())"
    }

    /// 入口・遷移先に出してよいか。既定で非表示の機能は、環境変数が `1` のときだけ出す。
    public static func isVisible(
        featureID: String,
        hiddenByDefaultIDs: Set<String> = FeatureVisibility.hiddenByDefaultIDs,
        environment: [String: String]
    ) -> Bool {
        guard hiddenByDefaultIDs.contains(featureID) else { return true }
        return environment[showEnvironmentKey(forFeatureID: featureID)] == "1"
    }
}
