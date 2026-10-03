import PokeCalcCore
import SwiftUI

/// 調整画面(ADR-0502)の登録。
struct AdjustFeature: AppFeature {
    let id = "adjust"
    let order = 400
    let entry = FeatureEntry.rootButton(title: AdjustText.screenTitle, accessibilityIdentifier: "openAdjustScreen")
    let openAtLaunchEnvironmentKey: String? = "POKECALC_OPEN_ADJUST_SCREEN_AT_LAUNCH"

    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws {
        switch backend {
        case .mock:
            try services.register((any AdjustService).self, try MockAdjustService())
        case .api(_, _, let pokeCalc):
            // 調整は計算と同じ gateway・同じ契約(`APIPokeCalcService` の extension)。
            try services.register((any AdjustService).self, pokeCalc)
        }
    }

    @MainActor func destination(in context: FeatureContext) -> AnyView {
        guard let adjust = context.services.resolve((any AdjustService).self) else { return AnyView(EmptyView()) }
        return AnyView(
            AdjustScreenView(
                service: context.core.pokeCalc, adjust: adjust, backendDescription: context.core.backendDescription))
    }
}
