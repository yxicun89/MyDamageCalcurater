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
    @State private var isMoveSearchPresented = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    /// テンキーには Return が無いので、キーボード上部の「完了」で閉じる(ADR-0501「実装メモ」の
    /// 「テンキーの「完了」ボタン」)。
    @FocusState private var focusedObservationID: Int?
    private let backendDescription: String

    /// 読み込み中インジケータの高さ(`CalcScreenView` と同じ理由で固定する)。
    private static let loadingIndicatorHeight: CGFloat = 24

    init(service: any PokeCalcService, teamStore: any TeamStore, backendDescription: String) {
        _viewModel = State(initialValue: ReverseViewModel(service: service, teamStore: teamStore))
        self.backendDescription = backendDescription
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
                teamSourceRow
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
        // 画面破棄で保持中の入力 Task を止める(issue #113 A6)。
        .onDisappear { viewModel.cancelPendingWork() }
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
            viewModel.scheduleLatest { await $0.selectSide(side) }
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
            // ラベルは自分の技の分類(A/C)に依存する。技の読み込み前は物理扱いで暫定表示する
            // (`.attacker` 側の HB/HD 表示と同じ理由)。
            let category = viewModel.selectedMove?.category ?? .physical
            HStack(spacing: SpacingToken.x2) {
                ForEach(AttackerPreset.allCases, id: \.self) { preset in
                    PresetPillButton(
                        title: preset.label(for: category), isSelected: viewModel.attackerPreset == preset,
                        identifier: "reverseAttackerPreset-\(preset.rawValue)"
                    ) {
                        viewModel.scheduleLatest { await $0.selectAttackerPreset(preset) }
                    }
                }
            }
        case .attacker:
            // ラベルは相手の技の分類(HB/HD)に依存する。技の読み込み前は物理扱いで暫定表示する。
            let category = viewModel.selectedMove?.category ?? .physical
            HStack(spacing: SpacingToken.x2) {
                ForEach(KnownDefenderPreset.allCases, id: \.self) { preset in
                    PresetPillButton(
                        title: preset.label(for: category), isSelected: viewModel.knownDefenderPreset == preset,
                        identifier: "reverseKnownDefenderPreset-\(preset.rawValue)"
                    ) {
                        viewModel.scheduleLatest { await $0.selectKnownDefenderPreset(preset) }
                    }
                }
            }
        }
    }

    /// 「構築から選ぶ」の入口(P6-2d)。側ごとに出し分けないので identifier は1つ(いまの側は
    /// `reverseSide-*` で分かる。ADR-0501「P6-2d」7章)。いま表示している側の出どころだけを見せる。
    private var teamSourceRow: some View {
        let currentSideSelection: TeamIndividualSelection? = {
            switch viewModel.side {
            case .defender: return viewModel.attackerBuildSource.teamSelection
            case .attacker: return viewModel.knownDefenderBuildSource.teamSelection
            }
        }()
        return TeamSourceMenuRow(
            teamOptions: viewModel.teamOptions,
            selection: currentSideSelection,
            identifierPrefix: "reverseTeam"
        ) { teamID, memberID in
            viewModel.scheduleLatest { await $0.selectTeamIndividual(teamID: teamID, memberID: memberID) }
        }
    }

    /// issue #68: `Menu` ではなく検索シートで選ぶ(`CalcScreenView.moveSelector` と同じ理由)。
    private var moveSelector: some View {
        Button {
            isMoveSearchPresented = true
        } label: {
            HStack {
                VStack(alignment: .leading, spacing: SpacingToken.x1) {
                    Text(viewModel.selectedMove?.nameJa ?? "-")
                        .font(TextStyleToken.heading.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .lineLimit(1)
                        .minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)
                    if let move = viewModel.selectedMove {
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
        .sheet(isPresented: $isMoveSearchPresented) {
            MoveSearchSheet(viewModel: viewModel, options: viewModel.moveOptions) { move in
                viewModel.scheduleLatest { await $0.selectMove(id: move.id) }
            }
        }
    }

    private var opponentItemCandidateToggles: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text("相手の持ち物候補")
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: SpacingToken.x2) {
                    ForEach(viewModel.itemOptions, id: \.id) { item in
                        let isSelected = viewModel.opponentItemCandidateIds.contains(item.id)
                        ChipButton(
                            title: item.nameJa,
                            isSelected: isSelected,
                            identifier: "reverseOpponentItemToggle-\(item.id)",
                            // 選択済みのチップは上限に達していても常に有効(解除できる。issue #110 A6・9章)。
                            isEnabled: isSelected || !viewModel.opponentItemCandidatesReachedLimit
                        ) {
                            viewModel.scheduleLatest { await $0.toggleOpponentItemCandidate(itemId: item.id) }
                        }
                    }
                }
            }
            if viewModel.opponentItemCandidatesReachedLimit {
                Text(RequestLimitLabels.itemCandidatesReachedLimit)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier("reverseItemCandidateLimitHint")
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
            ReverseScreenView(service: mock, teamStore: LocalTeamStore(), backendDescription: "モックデータで動作中")
        }
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
