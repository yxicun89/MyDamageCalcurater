import PokeCalcCore
import SwiftUI

/// アプリのエントリポイント。View だけを持つ(ADR-0500 §1)。
@main
struct PokeCalcApp: App {
    // `@State` の初期値式は State の生存中に1回だけ評価される(SwiftUI の仕様)。
    // ここで作ることで、シーンフェーズの変化などで `body` が再評価されても
    // `AppEnvironment.makeAtLaunch()`(セッション ID の発行を含む)が起動時の1回だけになる。
    @State private var environment = AppEnvironment.makeAtLaunch()

    var body: some Scene {
        WindowGroup {
            RootView(environment: environment)
        }
    }
}
