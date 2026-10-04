import PokeCalcCore
import SwiftUI

/// 計算画面(P6-2a。ADR-0501「P6-2a」)の登録。
struct CalcFeature: AppFeature {
    let id = "calc"
    let order = 100
    let entry = FeatureEntry.rootButton(title: "計算する", accessibilityIdentifier: "openCalcScreen")
    let openAtLaunchEnvironmentKey: String? = "POKECALC_OPEN_CALC_SCREEN_AT_LAUNCH"

    @MainActor func destination(in context: FeatureContext) -> AnyView {
        AnyView(
            CalcScreenView(
                service: context.core.pokeCalc, teamStore: context.teamStore,
                backendDescription: context.core.backendDescription,
                frequentOpponentsService: context.core.frequentOpponents,
                favoritesService: context.services.resolve((any FavoritesService).self)))
    }
}
