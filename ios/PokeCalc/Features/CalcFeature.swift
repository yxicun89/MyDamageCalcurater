import PokeCalcCore
import SwiftUI

/// 計算画面(P6-2a。ADR-0501「P6-2a」)の登録。
struct CalcFeature: AppFeature {
    let id = "calc"
    let order = 100
    let entry = FeatureEntry.tab(.damage, title: "計算", accessibilityIdentifier: "openCalcScreen")
    let openAtLaunchEnvironmentKey: String? = "POKECALC_OPEN_CALC_SCREEN_AT_LAUNCH"

    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws {
        switch backend {
        case .mock(let environment):
            // 連続技のモック(失敗シナリオは環境変数。ADR-0535)。
            try services.register(
                (any ComboCalcService).self,
                try MockPokeCalcService(
                    comboScenario: MockComboScenario(environmentValue: environment[MockComboScenario.environmentKey])))
        case .api(_, _, let pokeCalc):
            // 計算と同じ gateway・同じ契約(`APIPokeCalcService` の extension)。
            try services.register((any ComboCalcService).self, pokeCalc)
        }
    }

    @MainActor func destination(in context: FeatureContext) -> AnyView {
        AnyView(
            CalcScreenView(
                service: context.core.pokeCalc, teamStore: context.teamStore,
                backendDescription: context.core.backendDescription,
                frequentOpponentsService: context.core.frequentOpponents,
                favoritesService: context.services.resolve((any FavoritesService).self),
                comboService: context.services.resolve((any ComboCalcService).self),
                speedService: context.services.resolve((any SpeedService).self)))
    }
}
