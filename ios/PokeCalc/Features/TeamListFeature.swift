import SwiftUI

/// 構築一覧画面(P6-2c。ADR-0501「P6-2c」)の登録。編集画面へはルートの path を共有して積む。
struct TeamListFeature: AppFeature {
    let id = "team"
    let order = 300
    let entry = FeatureEntry.rootButton(title: "構築", accessibilityIdentifier: "openTeamListScreen")
    let openAtLaunchEnvironmentKey: String? = "POKECALC_OPEN_TEAM_LIST_SCREEN_AT_LAUNCH"

    @MainActor func destination(in context: FeatureContext) -> AnyView {
        AnyView(TeamListView(store: context.teamStore, service: context.core.pokeCalc, path: context.path))
    }
}
