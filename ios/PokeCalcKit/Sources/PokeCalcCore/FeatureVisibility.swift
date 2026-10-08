import Foundation

/// 既定では入口を出さない画面(機能)の可視性(F-07。ADR-0507 の機能レジストリの上に載せる)。
///
/// ユーザー決定(2026-10-04): 判定は「今の機能はいらない・目的を作り直す」。再設計までルート画面の入口を**非表示**にする
/// (サービス・生成クライアント・画面のコード・テストは残す)。非表示の画面は、環境変数 `POKECALC_SHOW_<ID 大文字>=1`
/// (例 `POKECALC_SHOW_JUDGE=1`)を渡すと出る(XCUITest と `make ios-sim-run IOS_SCREEN=judge` が使う)。
public enum FeatureVisibility {
    /// 既定で入口を出さない機能の `id`。
    public static let hiddenByDefaultIDs: Set<String> = ["judge"]

    /// その機能を出すための環境変数のキー(`POKECALC_SHOW_JUDGE` など)。
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
