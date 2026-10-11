import SwiftUI
import WishlistCore

/// アプリのエントリポイント。View だけを持つ(ロジックは WishlistKit)。
@main
struct WishlistApp: App {
    @State private var model = AppModel()

    var body: some Scene {
        WindowGroup {
            HomeView()
                .environment(model)
                .tint(Palette.accent)
        }
    }
}
