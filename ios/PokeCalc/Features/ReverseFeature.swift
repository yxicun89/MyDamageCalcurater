import SwiftUI

/// 逆算画面(P6-2b。ADR-0501「P6-2b」)の登録。
struct ReverseFeature: AppFeature {
    let id = "reverse"
    let order = 200
    let entry = FeatureEntry.rootButton(title: "逆算する", accessibilityIdentifier: "openReverseScreen")
    let openAtLaunchEnvironmentKey: String? = "POKECALC_OPEN_REVERSE_SCREEN_AT_LAUNCH"

    @MainActor func destination(in context: FeatureContext) -> AnyView {
        AnyView(
            ReverseScreenView(
                service: context.core.pokeCalc, teamStore: context.teamStore,
                backendDescription: context.core.backendDescription,
                frequentOpponentsService: context.core.frequentOpponents))
    }
}
