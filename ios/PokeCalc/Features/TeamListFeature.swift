import PokeCalcCore
import SwiftUI

/// 構築一覧画面(P6-2c。ADR-0501「P6-2c」)の登録。編集画面へは構築タブの path を共有して積む。
/// 構築編集の中の「タイプバランス」(ADR-0421・ADR-0532)が使う `BalanceService` もここで登録する(別画面は廃止)。
/// `requiredServices` には入れない: balance が無くても構築の編集・保存は止めない(ADR-0421 §2)。
struct TeamListFeature: AppFeature {
    let id = "team"
    let order = 300
    let entry = FeatureEntry.tab(.team, title: "構築", accessibilityIdentifier: "openTeamListScreen")
    let openAtLaunchEnvironmentKey: String? = "POKECALC_OPEN_TEAM_LIST_SCREEN_AT_LAUNCH"

    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws {
        switch backend {
        case .mock(let environment):
            // 既定は接続先なし(架空の相性表を作らない。ADR-0415 §4)。UI テストだけ `POKECALC_MOCK_BALANCE=fixture` で架空の応答。
            try services.register((any BalanceService).self, MockBalanceService.make(environment: environment))
        case .api(let baseURL, let identity, _):
            // gateway の `/api/balance/*`(計算・pokedex と同じ基点 URL。ADR-0415 §3)。
            try services.register((any BalanceService).self, APIBalanceService(baseURL: baseURL, identity: identity))
        }
    }

    @MainActor func destination(in context: FeatureContext) -> AnyView {
        AnyView(
            TeamListView(
                store: context.teamStore, service: context.core.pokeCalc,
                balance: context.services.resolve((any BalanceService).self), path: context.path))
    }
}
