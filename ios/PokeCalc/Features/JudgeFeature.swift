import PokeCalcCore
import SwiftUI

/// 判定画面(P6-25。ADR-0504)の登録。
struct JudgeFeature: AppFeature {
    let id = "judge"
    let order = 700
    let entry = FeatureEntry.rootButton(title: JudgeLabels.openButton, accessibilityIdentifier: "openJudgeScreen")
    let openAtLaunchEnvironmentKey: String? = "POKECALC_OPEN_JUDGE_SCREEN_AT_LAUNCH"

    let requiredServices = [ServiceKey((any JudgeService).self)]

    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws {
        switch backend {
        case .mock(let environment):
            try services.register((any JudgeService).self, MockJudgeService(environment: environment))
        case .api(let baseURL, let identity, _):
            // 判定は gateway の `/api/judge/*`。ホストと端末 ID/セッション ID は他の機能と同じ(ADR-0504 §1)。
            try services.register((any JudgeService).self, APIJudgeService(baseURL: baseURL, identity: identity))
        }
    }

    @MainActor func destination(in context: FeatureContext) -> AnyView {
        guard let judge = context.services.resolve((any JudgeService).self) else { return unavailableView() }
        return AnyView(JudgeScreenView(service: judge, master: context.core.pokeCalc, teamStore: context.teamStore))
    }
}
