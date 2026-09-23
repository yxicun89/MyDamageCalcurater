import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// CalcScreenView: ダメージ計算画面(P6-2a・docs/design.md「画面: ダメージ計算」)。
//
// ロジックは持たない。`CalcViewModel`(PokeCalcCore)の状態を描き、操作を async メソッドへ
// つなぐだけ(ADR-0500 §1)。

/// ダメージ計算画面。
struct CalcScreenView: View {
    @State private var viewModel: CalcViewModel
    @State private var isMoveSearchPresented = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    private let backendDescription: String

    /// design.md「攻守入れ替え: カードが入れ替わる(0.35秒)」。
    private static let swapAnimationDuration: Double = 0.35
    /// 読み込み中インジケータの高さ。出たり消えたりで下の行が上下にずれないよう、
    /// 表示の有無に関わらず高さを固定で確保する(批評「任意」対応)。
    private static let loadingIndicatorHeight: CGFloat = 24

    init(service: any PokeCalcService, teamStore: any TeamStore, backendDescription: String) {
        _viewModel = State(initialValue: CalcViewModel(service: service, teamStore: teamStore))
        self.backendDescription = backendDescription
    }

    /// ダメージバー・相性の色に使う、選択中の技のタイプ色。技が無い(読み込み中)ときは無彩色にする。
    private var moveTypeColor: Color {
        viewModel.selectedMove.flatMap { TypeColorToken.color(forTypeID: $0.type.rawValue) } ?? ColorToken.textSecondary.color
    }

    /// 「ばつぐん」のときだけ技のタイプ色を使う(design.md「色を持つのはタイプだけ」)。
    /// 「ばつぐん」の判定自体は Core の `EffectivenessLabel.isSuperEffective(_:)` を使う。
    private var moveSummaryColor: Color {
        guard let effectiveness = viewModel.moveEffectiveness, EffectivenessLabel.isSuperEffective(effectiveness) else {
            return ColorToken.textSecondary.color
        }
        return moveTypeColor
    }

    private var swapTransition: AnyTransition {
        reduceMotion
            ? .identity
            : .asymmetric(
                insertion: .move(edge: .trailing).combined(with: .opacity),
                removal: .move(edge: .leading).combined(with: .opacity)
            )
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                backendBadge
                if let error = viewModel.error {
                    ErrorBannerView(message: error.message)
                }
                cardsRow
                presetSegmentedRow
                teamSourceRow
                moveSelector
                loadingSlot
                ResultsSectionView(viewModel: viewModel, barColor: moveTypeColor)
            }
            .padding(SpacingToken.x4)
        }
        .background(ColorToken.bgBase.color.ignoresSafeArea())
        .accessibilityIdentifier("calcScreen")
        .task { await viewModel.load() }
    }

    private var backendBadge: some View {
        Text(backendDescription)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .padding(.horizontal, SpacingToken.x3)
            .padding(.vertical, SpacingToken.x1)
            .background(ColorToken.bgGlass.color, in: Capsule())
            .accessibilityIdentifier("calcBackendModeBadge")
    }

    private var cardsRow: some View {
        GlassEffectContainer(spacing: SpacingToken.x2) {
            // 文字サイズがアクセシビリティ域(`.accessibility1` 以上)まで大きいと、2枚のカードを
            // 横に並べる余白が無くなるため、縦に積む(攻撃側 → 入れ替えボタン → 防御側)。
            if dynamicTypeSize >= .accessibility1 {
                VStack(spacing: SpacingToken.x2) {
                    AttackerCardView(viewModel: viewModel)
                        .id(viewModel.attackerSpeciesKey)
                        .transition(swapTransition)
                    swapButton
                    DefenderCardView(viewModel: viewModel)
                        .id(viewModel.defenderSpeciesKey)
                        .transition(swapTransition)
                }
            } else {
                HStack(alignment: .top, spacing: SpacingToken.x2) {
                    AttackerCardView(viewModel: viewModel)
                        .id(viewModel.attackerSpeciesKey)
                        .transition(swapTransition)
                    swapButton
                    DefenderCardView(viewModel: viewModel)
                        .id(viewModel.defenderSpeciesKey)
                        .transition(swapTransition)
                }
            }
        }
        // `swapTick` は `swapSides()` だけが増やす(選択のやり直しや起動時の読み込みでは増えない)。
        // カードの入れ替えという「操作への反応」だけに動きを絞る(design.md「動き」)。
        .animation(reduceMotion ? nil : .easeInOut(duration: Self.swapAnimationDuration), value: viewModel.swapTick)
    }

    private var swapButton: some View {
        Button {
            Task { await viewModel.swapSides() }
        } label: {
            Image(systemName: "arrow.left.arrow.right")
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .padding(SpacingToken.x2)
                .background(ColorToken.bgGlass.color, in: Circle())
        }
        .padding(.top, SpacingToken.x6)
        .accessibilityIdentifier("swapSidesButton")
        .accessibilityLabel("攻守を入れ替える")
    }

    /// 自分側のプリセット。カードの外、画面幅いっぱいの3等分のピルで並べる(批評 M3c:
    /// カード内に置くと幅が足りず「無振り」が見切れていた)。
    private var presetSegmentedRow: some View {
        HStack(spacing: SpacingToken.x2) {
            ForEach(AttackerPreset.allCases, id: \.self) { preset in
                let isSelected = viewModel.attackerPreset == preset
                Button {
                    Task { await viewModel.selectAttackerPreset(preset) }
                } label: {
                    Text(preset.label)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(isSelected ? ColorToken.bgBase.color : ColorToken.textPrimary.color)
                        .lineLimit(1)
                        .minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, SpacingToken.x2)
                        .background(
                            Capsule().fill(isSelected ? ColorToken.textPrimary.color : ColorToken.bgGlass.color)
                        )
                        .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
                }
                .buttonStyle(.plain)
                .accessibilityIdentifier("attackerPreset-\(preset.rawValue)")
                .accessibilityAddTraits(isSelected ? .isSelected : [])
            }
        }
    }

    /// 「構築から選ぶ」の入口(P6-2d)。プリセットのピル行の直下、幅いっぱいの独立した1行
    /// (ADR-0501「P6-2d」5章・7章)。
    private var teamSourceRow: some View {
        TeamSourceMenuRow(
            teamOptions: viewModel.teamOptions,
            selection: viewModel.attackerBuildSource.teamSelection,
            identifierPrefix: "attackerTeam"
        ) { teamID, memberID in
            await viewModel.selectTeamIndividual(teamID: teamID, memberID: memberID)
        }
    }

    /// issue #68: 技の一覧も数百件になりうるため、`Menu` ではなく検索シートで選ぶ(ADR-0501
    /// 「issue #68」1章「判断」)。入口の identifier(`movePicker`)は変えない。
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
                    // 「威力37 / 物理 / ばつぐん(×2)」(design.md「技セレクタ 威力・分類・相性」)。
                    // 文言の組み立ては Core の `moveSummaryText` が持つ(View にロジックを置かない)。
                    Text(viewModel.moveSummaryText)
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(moveSummaryColor)
                        .lineLimit(1)
                        .minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)
                }
                Spacer()
                Image(systemName: "chevron.up.chevron.down")
                    .foregroundStyle(ColorToken.textSecondary.color)
            }
            .padding(SpacingToken.x3)
            .glassCard(cornerRadius: RadiusToken.input)
        }
        .accessibilityIdentifier("movePicker")
        .sheet(isPresented: $isMoveSearchPresented) {
            MoveSearchSheet(viewModel: viewModel, options: viewModel.moveOptions) { move in
                Task { await viewModel.selectMove(id: move.id) }
            }
        }
    }

    private var loadingSlot: some View {
        Group {
            if viewModel.isLoading {
                ProgressView()
                    .accessibilityIdentifier("calcLoadingIndicator")
            }
        }
        .frame(maxWidth: .infinity)
        .frame(height: Self.loadingIndicatorHeight)
    }
}

#Preview {
    if let mock = try? MockPokeCalcService() {
        NavigationStack {
            CalcScreenView(service: mock, teamStore: LocalTeamStore(), backendDescription: "モックデータで動作中")
        }
    } else {
        Text("プレビュー用モックの読み込みに失敗")
    }
}
