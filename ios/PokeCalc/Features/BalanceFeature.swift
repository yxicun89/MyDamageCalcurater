import PokeCalcCore
import SwiftUI

/// タイプバランス画面(P6-21。ADR-0415)の登録。
struct BalanceFeature: AppFeature {
    let id = "balance"
    let order = 500
    let entry = FeatureEntry.rootButton(title: "タイプバランス", accessibilityIdentifier: "openBalanceScreen")

    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws {
        switch backend {
        case .mock:
            // タイプバランスはモックを持たない(架空の相性表を作らない。ADR-0415 §4)。
            try services.register((any BalanceService).self, UnavailableBalanceService())
        case .api(let baseURL, let identity, _):
            // gateway の `/api/balance/*`(計算・pokedex と同じ基点 URL。ADR-0415 §3)。
            try services.register((any BalanceService).self, APIBalanceService(baseURL: baseURL, identity: identity))
        }
    }

    @MainActor func destination(in context: FeatureContext) -> AnyView {
        guard let balance = context.services.resolve((any BalanceService).self) else { return AnyView(EmptyView()) }
        return AnyView(BalanceScreenView(balance: balance, service: context.core.pokeCalc, teamStore: context.teamStore))
    }
}
