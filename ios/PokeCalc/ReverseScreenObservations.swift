import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// ReverseScreenObservations: 観測の入力欄一覧(P6-2b・docs/design.md「画面: 逆算」)。
// 各行はテンキー入力(`.keyboardType(.numberPad)`)。テンキーには Return が無いので、キーボード上部に
// 「完了」ボタンを足す(ADR-0501「実装メモ」の「テンキーの「完了」ボタン」)。

/// 観測欄の一覧 + 「観測を追加」ボタン。
struct ReverseObservationListView: View {
    let viewModel: ReverseViewModel
    /// テンキーの「完了」ボタンで閉じるための共有フォーカス(行をまたいで1つ)。
    var focusedObservationID: FocusState<Int?>.Binding

    /// 単位の文言(% は観測%、実点数はダメージの単位が無いので短い接尾辞にする)。
    private var unitLabel: String {
        viewModel.observationKind == .percent ? "%" : "ダメージ"
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            ForEach(Array(viewModel.observations.enumerated()), id: \.element.id) { index, row in
                ReverseObservationRowView(
                    viewModel: viewModel, row: row, index: index, unitLabel: unitLabel,
                    focusedObservationID: focusedObservationID
                )
            }
            Button {
                viewModel.addObservation()
            } label: {
                Label("観測を追加", systemImage: "plus.circle")
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
            }
            .buttonStyle(.plain)
            .disabled(viewModel.observationsReachedLimit)
            .padding(.top, SpacingToken.x1)
            .accessibilityIdentifier("reverseAddObservationButton")
            if viewModel.observationsReachedLimit {
                Text(RequestLimitLabels.observationsReachedLimit)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier("reverseObservationLimitHint")
            }
        }
    }
}

/// 観測1行: テンキー入力欄・単位・エラー文言・削除ボタン。
private struct ReverseObservationRowView: View {
    let viewModel: ReverseViewModel
    let row: ObservationRow
    let index: Int
    let unitLabel: String
    var focusedObservationID: FocusState<Int?>.Binding

    /// 空の行(`.empty`)はエラーを表示しない(ADR-0501 の identifier 表「空の行では出さない」)。
    private var visibleErrorMessage: String? {
        guard let error = row.error, error != .empty else { return nil }
        return error.message(kind: viewModel.observationKind)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            HStack(spacing: SpacingToken.x2) {
                TextField(
                    "",
                    text: Binding(
                        get: { row.text },
                        set: { newValue in
                            // テキストの反映・検証は同期(批評対応: `Task { await ... }` だけで包むと
                            // `TextField` への反映が1フレーム遅れる)。逆算の呼び出しが要るときだけ
                            // debounce つきで予約する(issue #113 A2)。
                            if viewModel.setObservationText(id: row.id, text: newValue) {
                                viewModel.scheduleRecalculationAfterObservationEdit()
                            }
                        }
                    )
                )
                .keyboardType(.numberPad)
                .focused(focusedObservationID, equals: row.id)
                .font(TextStyleToken.body.font.monospacedDigit())
                .foregroundStyle(ColorToken.textPrimary.color)
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .background(ColorToken.bgGlass.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
                .accessibilityIdentifier("reverseObservationField-\(index)")
                .accessibilityLabel("観測\(index + 1)(\(unitLabel))")

                Text(unitLabel)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textSecondary.color)

                Button {
                    viewModel.scheduleLatest { await $0.removeObservation(id: row.id) }
                } label: {
                    // danger はエラー表示用の色(design.md「色を持つのはタイプだけ」の例外は danger のみ)。
                    // 削除はエラーではない通常の操作なので textSecondary にする。
                    Image(systemName: "minus.circle")
                        .foregroundStyle(ColorToken.textSecondary.color)
                }
                .buttonStyle(.plain)
                .accessibilityIdentifier("reverseObservationRemove-\(index)")
                .accessibilityLabel("この観測を削除")
            }
            if let visibleErrorMessage {
                Text(visibleErrorMessage)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.danger.color)
                    .accessibilityIdentifier("reverseObservationError-\(index)")
            }
        }
    }
}
