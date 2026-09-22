import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// ReverseScreenView: 逆算(調整推定)画面(P6-2b・docs/design.md「画面: 逆算」)。
//
// ロジックは持たない。`ReverseViewModel`(PokeCalcCore)の状態を描き、操作を async メソッドへ
// つなぐだけ(ADR-0500 §1)。カード・チップ・エラー表示など計算画面の部品を再利用する。

/// 逆算画面。
struct ReverseScreenView: View {
    @State private var viewModel: ReverseViewModel
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    /// テンキーには Return が無いので、キーボード上部の「完了」で閉じる(README「実装メモ」の
    /// 「テンキーの「完了」ボタン」)。
    @FocusState private var focusedObservationID: Int?
    private let backendDescription: String

    /// 読み込み中インジケータの高さ(`CalcScreenView` と同じ理由で固定する)。
    private static let loadingIndicatorHeight: CGFloat = 24

    init(service: any PokeCalcService, backendDescription: String) {
        _viewModel = State(initialValue: ReverseViewModel(service: service))
        self.backendDescription = backendDescription
    }

    /// いまの技(未読み込みなら nil)。防御側プリセットの表示名(HB/HD)に使う。
    private var selectedMove: Move? {
        viewModel.moveOptions.first(where: { $0.id == viewModel.moveId })
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                backendBadge
                sideSwitch
                if let error = viewModel.error {
                    ErrorBannerView(message: error.message)
                }
                cardsRow
                presetSegmentedRow
                moveSelector
                opponentItemCandidateToggles
                ReverseObservationListView(viewModel: viewModel, focusedObservationID: $focusedObservationID)
                loadingSlot
                ReverseResultsSectionView(result: viewModel.result, observationCount: viewModel.observations.count)
            }
            .padding(SpacingToken.x4)
        }
        .background(ColorToken.bgBase.color.ignoresSafeArea())
        .accessibilityIdentifier("reverseScreen")
        .toolbar {
            // .numberPad のキーボードには Return が無いので、閉じる手段を別に用意する。
            ToolbarItemGroup(placement: .keyboard) {
                Spacer()
                Button("完了") { focusedObservationID = nil }
            }
        }
        .task { await viewModel.load() }
    }

    private var backendBadge: some View {
        Text(backendDescription)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .padding(.horizontal, SpacingToken.x3)
            .padding(.vertical, SpacingToken.x1)
            .background(ColorToken.bgGlass.color, in: Capsule())
            .accessibilityIdentifier("reverseBackendModeBadge")
    }

    /// 与えたダメージ / 受けたダメージ の切り替え(neutral colors のセグメントピル)。
    /// アクセシビリティの大きい文字サイズでは横に並べる幅が足りないので、`cardsRow` と同じ分岐で
    /// 縦積みに変える(批評対応)。
    private var sideSwitch: some View {
        let stack = ReverseSide.allCases.map { side -> (side: ReverseSide, isSelected: Bool) in
            (side, viewModel.side == side)
        }
        return Group {
            if dynamicTypeSize >= .accessibility1 {
                VStack(spacing: SpacingToken.x1) {
                    ForEach(stack, id: \.side) { entry in
                        sideSwitchButton(side: entry.side, isSelected: entry.isSelected)
                    }
                }
                .padding(SpacingToken.x1)
            } else {
                HStack(spacing: 0) {
                    ForEach(stack, id: \.side) { entry in
                        sideSwitchButton(side: entry.side, isSelected: entry.isSelected)
                    }
                }
                .padding(SpacingToken.x1)
            }
        }
        .background(ColorToken.bgGlass.color, in: RoundedRectangle(cornerRadius: RadiusToken.pill, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: RadiusToken.pill, style: .continuous)
                .stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth)
        )
    }

    /// どちらの並び(横に2分割/縦積み)でも、セグメントは常に幅いっぱいに広がる。
    private func sideSwitchButton(side: ReverseSide, isSelected: Bool) -> some View {
        Button {
            Task { await viewModel.selectSide(side) }
        } label: {
            Text(side == .defender ? "与えたダメージ" : "受けたダメージ")
                .font(TextStyleToken.body.font)
                .foregroundStyle(isSelected ? ColorToken.bgBase.color : ColorToken.textPrimary.color)
                .lineLimit(1)
                .minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)
                .frame(maxWidth: .infinity)
                .padding(.vertical, SpacingToken.x2)
                .background(Capsule().fill(isSelected ? ColorToken.textPrimary.color : Color.clear))
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("reverseSide-\(side.rawValue)")
        .accessibilityAddTraits(isSelected ? .isSelected : [])
    }

    private var cardsRow: some View {
        GlassEffectContainer(spacing: SpacingToken.x2) {
            // アクセシビリティの大きい文字サイズでは、2枚のカードを横に並べる幅が無いので縦に積む
            // (計算画面の `CalcScreenView.cardsRow` と同じ分岐)。
            if dynamicTypeSize >= .accessibility1 {
                VStack(spacing: SpacingToken.x2) {
                    ReverseMyCardView(viewModel: viewModel)
                    ReverseOpponentCardView(viewModel: viewModel)
                }
            } else {
                HStack(alignment: .top, spacing: SpacingToken.x2) {
                    ReverseMyCardView(viewModel: viewModel)
                    ReverseOpponentCardView(viewModel: viewModel)
                }
            }
        }
    }

    /// 自分のプリセット。与えたダメージ(自分が攻撃側)のときは `AttackerPreset`、受けたダメージ
    /// (自分が防御側)のときは `KnownDefenderPreset` を側で排他的に表示する。カードの外、画面幅いっぱいの
    /// 3等分のピルで並べる(`CalcScreenView.presetSegmentedRow` と同じ理由)。
    @ViewBuilder
    private var presetSegmentedRow: some View {
        switch viewModel.side {
        case .defender:
            HStack(spacing: SpacingToken.x2) {
                ForEach(AttackerPreset.allCases, id: \.self) { preset in
                    PresetPillButton(
                        title: preset.label, isSelected: viewModel.attackerPreset == preset,
                        identifier: "reverseAttackerPreset-\(preset.rawValue)"
                    ) {
                        Task { await viewModel.selectAttackerPreset(preset) }
                    }
                }
            }
        case .attacker:
            // ラベルは相手の技の分類(HB/HD)に依存する。技の読み込み前は物理扱いで暫定表示する。
            let category = selectedMove?.category ?? .physical
            HStack(spacing: SpacingToken.x2) {
                ForEach(KnownDefenderPreset.allCases, id: \.self) { preset in
                    PresetPillButton(
                        title: preset.label(for: category), isSelected: viewModel.knownDefenderPreset == preset,
                        identifier: "reverseKnownDefenderPreset-\(preset.rawValue)"
                    ) {
                        Task { await viewModel.selectKnownDefenderPreset(preset) }
                    }
                }
            }
        }
    }

    private var moveSelector: some View {
        Menu {
            ForEach(viewModel.moveOptions, id: \.id) { move in
                Button(move.nameJa) {
                    Task { await viewModel.selectMove(id: move.id) }
                }
            }
        } label: {
            HStack {
                VStack(alignment: .leading, spacing: SpacingToken.x1) {
                    Text(selectedMove?.nameJa ?? "-")
                        .font(TextStyleToken.heading.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .lineLimit(1)
                        .minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)
                    if let move = selectedMove {
                        Text("威力\(move.power) / \(MoveCategoryLabel.japaneseName(for: move.category))")
                            .font(TextStyleToken.caption.font)
                            .foregroundStyle(ColorToken.textSecondary.color)
                            .lineLimit(1)
                            .minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)
                    }
                }
                Spacer()
                Image(systemName: "chevron.up.chevron.down")
                    .foregroundStyle(ColorToken.textSecondary.color)
            }
            .padding(SpacingToken.x3)
            .glassCard(cornerRadius: RadiusToken.input)
        }
        .accessibilityIdentifier("reverseMovePicker")
    }

    private var opponentItemCandidateToggles: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text("相手の持ち物候補")
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: SpacingToken.x2) {
                    ForEach(viewModel.itemOptions, id: \.id) { item in
                        ChipButton(
                            title: item.nameJa,
                            isSelected: viewModel.opponentItemCandidateIds.contains(item.id),
                            identifier: "reverseOpponentItemToggle-\(item.id)"
                        ) {
                            Task { await viewModel.toggleOpponentItemCandidate(itemId: item.id) }
                        }
                    }
                }
            }
        }
    }

    private var loadingSlot: some View {
        Group {
            if viewModel.isLoading {
                ProgressView()
                    .accessibilityIdentifier("reverseLoadingIndicator")
            }
        }
        .frame(maxWidth: .infinity)
        .frame(height: Self.loadingIndicatorHeight)
    }
}

/// プリセットのピル1つ(`CalcScreenView.presetSegmentedRow` と同じ見た目)。
private struct PresetPillButton: View {
    let title: String
    let isSelected: Bool
    let identifier: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Text(title)
                .font(TextStyleToken.body.font)
                .foregroundStyle(isSelected ? ColorToken.bgBase.color : ColorToken.textPrimary.color)
                .lineLimit(1)
                .minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)
                .frame(maxWidth: .infinity)
                .padding(.vertical, SpacingToken.x2)
                .background(Capsule().fill(isSelected ? ColorToken.textPrimary.color : ColorToken.bgGlass.color))
                .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier(identifier)
        .accessibilityAddTraits(isSelected ? .isSelected : [])
    }
}

#Preview {
    if let mock = try? MockPokeCalcService() {
        NavigationStack {
            ReverseScreenView(service: mock, backendDescription: "モックデータで動作中")
        }
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
