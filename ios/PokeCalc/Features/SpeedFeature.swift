import PokeCalcCore
import SwiftUI

/// 素早さ比較画面(P6-24。ADR-0503)の登録。
struct SpeedFeature: AppFeature {
    let id = "speed"
    let order = 600
    let entry = FeatureEntry.rootButton(title: SpeedLabels.openButton, accessibilityIdentifier: "openSpeedScreen")
    let openAtLaunchEnvironmentKey: String? = "POKECALC_OPEN_SPEED_SCREEN_AT_LAUNCH"

    let requiredServices = [ServiceKey((any SpeedService).self)]

    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws {
        switch backend {
        case .mock(let environment):
            try services.register((any SpeedService).self, MockSpeedService(environment: environment))
        case .api(let baseURL, let identity, _):
            // 同じ gateway(`/api/speed/*`)・同じ端末 ID/セッション ID(ADR-0503 §3)。
            try services.register((any SpeedService).self, APISpeedService(baseURL: baseURL, identity: identity))
        }
    }

    @MainActor func destination(in context: FeatureContext) -> AnyView {
        guard let speed = context.services.resolve((any SpeedService).self) else { return unavailableView() }
        return AnyView(SpeedScreenView(service: speed))
    }
}
