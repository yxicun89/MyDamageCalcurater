import SwiftUI
import WishlistCore

/// 長押しで出るメニュー。背景を暗くし(ホームは 6px ぼかす)、押した商品の画像と、編集・元のページを開く・削除を出す。
/// 削除は確認のあとだけ(「削除」→「削除する」)。背景のタップで閉じる。
struct ItemContextMenu: View {
    let tile: HomeTile
    let onEdit: () -> Void
    let onDelete: () async -> Void
    let onClose: () -> Void

    @State private var confirmingDelete = false

    private var sourceURL: URL? {
        guard let text = tile.item.sourceURL, Deeplink.isHTTPURL(text) else { return nil }
        return URL(string: text)
    }

    var body: some View {
        ZStack {
            Palette.scrim
                .ignoresSafeArea()
                .contentShape(Rectangle())
                .onTapGesture(perform: onClose)
                .accessibilityElement()
                .accessibilityLabel("閉じる")
                .accessibilityAddTraits(.isButton)
                .accessibilityIdentifier("menuBackdrop")
            VStack(spacing: 18) {
                ItemImageView(item: tile.item, genreColor: tile.genreColor)
                    .frame(width: 210, height: 230)
                    .clipShape(RoundedRectangle(cornerRadius: Metrics.tileRadius, style: .continuous))
                    .genreRing(tile.genreColor)
                    .scaleEffect(1.04)
                    .accessibilityHidden(true)
                menu
            }
            .padding(.bottom, 40)
        }
    }

    private var menu: some View {
        VStack(spacing: 0) {
            if confirmingDelete {
                Text("削除しますか?").font(.wl(15)).foregroundStyle(Palette.muted).frame(height: 52)
                divider
                row("削除する", systemImage: "trash", color: Palette.danger, id: "deleteConfirm") { Task { await onDelete() } }
                divider
                row("やめる", systemImage: "xmark", color: Palette.fg, id: "deleteCancel") { confirmingDelete = false }
            } else {
                row("編集", systemImage: "pencil", color: Palette.fg, id: "menuEdit", action: onEdit)
                if let sourceURL {
                    divider
                    row("元のページを開く", systemImage: "arrow.up.right.square", color: Palette.fg, id: "menuOpenSource") {
                        UIApplication.shared.open(sourceURL)
                        onClose()
                    }
                }
                divider
                row("削除", systemImage: "trash", color: Palette.danger, id: "menuDelete") { confirmingDelete = true }
            }
        }
        .frame(width: 240)
        .background(Palette.surface.opacity(0.96), in: RoundedRectangle(cornerRadius: Metrics.cardRadius, style: .continuous))
        .clipShape(RoundedRectangle(cornerRadius: Metrics.cardRadius, style: .continuous))
    }

    private var divider: some View { Palette.lineSoft.frame(height: 1) }

    private func row(_ title: String, systemImage: String, color: Color, id: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack {
                Text(title).font(.wl(16, .heavy))
                Spacer()
                Image(systemName: systemImage).font(.system(size: 15, weight: .semibold))
            }
            .foregroundStyle(color)
            .padding(.horizontal, 18)
            .frame(height: 52)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier(id)
    }
}
