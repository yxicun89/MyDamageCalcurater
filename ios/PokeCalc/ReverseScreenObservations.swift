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

    /// 単位の語(与えたダメージ = 「%」、受けたダメージ = 「HP」。単位はダメージの側で決まるので切り替えは無い。ADR-0539 §2)。
    private var unitLabel: String { ReverseLabels.unitLabel(viewModel.observationKind) }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            ForEach(Array(viewModel.observations.enumerated()), id: \.element.id) { index, row in
                ReverseObservationRowView(
                    viewModel: viewModel, row: row, index: index, unitLabel: unitLabel,
                    focusedObservationID: focusedObservationID
                )
            }
            // 単位が分からない欄の例外として、一覧の下に単位の 1 行を 1 つだけ置く(1 カード 1 行まで)。
            Text(ReverseLabels.observationHint(side: viewModel.side))
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("reverseObservationHint")
            Button {
                viewModel.addObservation()
            } label: {
                PopLabel(title: "ダメージを追加", systemImage: PopSymbol.add)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.brandPrimary.color)
                    .frame(minHeight: CalcScreenMetrics.minimumTapSide)
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .disabled(viewModel.observationsReachedLimit)
            .opacity(viewModel.observationsReachedLimit ? CalcScreenMetrics.disabledChipOpacity : 1)
            .accessibilityIdentifier("reverseAddObservationButton")
            if viewModel.observationsReachedLimit {
                PopNoticeView(kind: .info, message: RequestLimitLabels.observationsReachedLimit, identifier: "reverseObservationLimitHint")
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
            // 見える名前(行の頭)。欄の読み上げ名(「ダメージ1(%)」)は下の欄が持つ。
            Text(ReverseLabels.observationLabel(index + 1))
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("reverseObservationLabel-\(index)")
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
                .background(ColorToken.tableZebra.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
                .accessibilityIdentifier("reverseObservationField-\(index)")
                .accessibilityLabel("\(ReverseLabels.observationLabel(index + 1))(\(unitLabel))")

                Text(unitLabel)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier("reverseObservationUnit-\(index)")

                // 削除は通常の操作(エラーではない)ので danger にしない(PopIconButton の既定色)。
                PopIconButton(
                    systemImage: PopSymbol.minus, label: "このダメージを削除",
                    identifier: "reverseObservationRemove-\(index)"
                ) {
                    viewModel.scheduleLatest { await $0.removeObservation(id: row.id) }
                }
            }
            if let visibleErrorMessage {
                PopNoticeView(kind: .error, message: visibleErrorMessage, identifier: "reverseObservationError-\(index)")
            }
        }
    }
}
