import PokeCalcCore
import SwiftUI

/// 「このアプリについて」画面(P6-18。issue #328)の登録。右上の控えめな入口で、設定エラー時も開ける。
struct AboutFeature: AppFeature {
    let id = "about"
    let order = 10_000
    let entry = FeatureEntry.toolbarIcon(
        systemImage: "info.circle", accessibilityLabel: "このアプリについて", accessibilityIdentifier: "openAboutScreen")

    func registerServices(for backend: FeatureBackend, into services: inout FeatureServices) throws {
        switch backend {
        case .mock(let environment):
            try services.register((any DeviceDataService).self, MockDeviceDataService(environment: environment))
        case .api(_, _, let pokeCalc):
            // 端末データの削除は計算と同じ gateway(`APIPokeCalcService` の extension。ADR-0209 §8)。
            try services.register((any DeviceDataService).self, pokeCalc)
        }
    }

    @MainActor func destination(in context: FeatureContext) -> AnyView {
        AnyView(AboutView(deviceDataService: context.services.resolve((any DeviceDataService).self)))
    }

    @MainActor func destinationWithoutServices() -> AnyView? {
        AnyView(AboutView())
    }
}
