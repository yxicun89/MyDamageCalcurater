import PokeCalcCore
import SwiftUI

/// お気に入り・計算履歴画面(ADR-0511・ADR-0501「お気に入り・計算履歴の受け入れ条件」)の登録。
struct FavoritesFeature: AppFeature {
    let id = "favorites"
    let order = 800
    let entry = FeatureEntry.rootButton(
        title: FavoritesLabels.rootButtonTitle, accessibilityIdentifier: "openFavoritesScreen")
    let openAtLaunchEnvironmentKey: String? = "POKECALC_OPEN_FAVORITES_SCREEN_AT_LAUNCH"
    let requiredServices = [ServiceKey((any FavoritesService).self), ServiceKey((any CalcHistoryService).self)]

    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws {
        switch backend {
        case .mock(let environment):
            try services.register((any FavoritesService).self, MockFavoritesService(environment: environment))
            try services.register((any CalcHistoryService).self, MockCalcHistoryService(environment: environment))
        case .api(_, _, let pokeCalc):
            // 計算と同じ gateway・同じ契約(`APIPokeCalcService` の extension)。
            try services.register((any FavoritesService).self, pokeCalc)
            try services.register((any CalcHistoryService).self, pokeCalc)
        }
    }

    @MainActor func destination(in context: FeatureContext) -> AnyView {
        guard let service = context.services.resolve((any FavoritesService).self),
            let historyService = context.services.resolve((any CalcHistoryService).self)
        else { return unavailableView() }
        return AnyView(
            FavoritesScreenView(
                favoritesService: service, calcHistoryService: historyService,
                frequentOpponentsService: context.core.frequentOpponents, resolver: context.core.pokeCalc,
                // 履歴の行・計算つきお気に入りの計算を、計算画面の入力に復元して開く(ADR-0519・ADR-0524)。
                calcScreen: { calc, favorite in
                    AnyView(
                        CalcScreenView(
                            service: context.core.pokeCalc, teamStore: context.teamStore,
                            backendDescription: context.core.backendDescription,
                            frequentOpponentsService: context.core.frequentOpponents, favoritesService: service,
                            restoring: favorite == nil ? calc : nil, restoringFavorite: favorite))
                }))
    }
}
