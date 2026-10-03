import SwiftUI
import WishlistCore

/// アプリのエントリポイント。View だけを持つ(ロジックは WishlistKit)。
@main
struct WishlistApp: App {
    @State private var model = AppModel()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(model)
        }
    }
}

/// 接続設定が無ければ設定画面から始め(API を呼ばない)、あればホームを出す。
struct RootView: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        if model.home.needsSettings {
            SettingsView(isInitialSetup: true)
        } else {
            HomeView()
        }
    }
}
