import SwiftUI
import WishlistCore

/// 長押しで出る編集・削除のメニュー。削除は確認のあとだけ(「削除」→「削除する」)。
struct ItemActionsSheet: View {
    let item: Item
    let onEdit: () -> Void
    let onDelete: () async -> Void

    @State private var confirmingDelete = false

    var body: some View {
        VStack(spacing: 12) {
            Text(item.name).font(.headline).lineLimit(2)
            if confirmingDelete {
                Text("削除しますか?").foregroundStyle(.secondary)
                actionButton("削除する", systemImage: "trash", role: .destructive, id: "deleteConfirm") {
                    Task { await onDelete() }
                }
                actionButton("やめる", systemImage: "xmark", id: "deleteCancel") { confirmingDelete = false }
            } else {
                actionButton("編集", systemImage: "pencil", id: "menuEdit", action: onEdit)
                actionButton("削除", systemImage: "trash", role: .destructive, id: "menuDelete") { confirmingDelete = true }
            }
        }
        .padding(20)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
        .presentationDetents([.height(260)])
    }

    private func actionButton(
        _ title: String, systemImage: String, role: ButtonRole? = nil, id: String, action: @escaping () -> Void
    ) -> some View {
        Button(role: role, action: action) {
            Label(title, systemImage: systemImage)
                .frame(maxWidth: .infinity)
                .padding(.vertical, 14)
        }
        .buttonStyle(.plain)
        .glassEffect(.regular, in: RoundedRectangle(cornerRadius: 16, style: .continuous))
        .accessibilityIdentifier(id)
    }
}
