import SwiftUI

/// 2 回目以降の表示(タブを移って戻ったとき・積んだ画面から戻ったとき)だけ `action` を呼ぶ。初回は各画面の `.task` の読み込みに任せる。
/// ナビゲーション案 B(ADR-0531)で画面を破棄しなくなったため、ほかのタブで変えたデータ(構築・お気に入り・計算履歴)を
/// 反映するのに使う(以前は画面を開き直すたびに `load()` で読み直していた)。
private struct OnReappearModifier: ViewModifier {
    let action: () -> Void
    @State private var hasAppeared = false

    func body(content: Content) -> some View {
        content.onAppear {
            if hasAppeared { action() }
            hasAppeared = true
        }
    }
}

extension View {
    func onReappear(perform action: @escaping () -> Void) -> some View {
        modifier(OnReappearModifier(action: action))
    }
}
