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
    /// 攻撃側の SP 欄(数値キーボード)のフォーカス。ツールバーの「完了」で外す(ADR-0518)。
    @FocusState private var focusedAttackerStat: AttackStat?
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @State private var frequentOpponents: FrequentOpponentsViewModel
    @State private var favoritePin: FavoritePinViewModel
    @State private var favoriteLoadTarget: FavoriteLoadTarget?
    private let service: any PokeCalcService
    private let favoritesService: (any FavoritesService)?
    /// 連続技の計算(CB-8。ADR-0535)。nil なら「連続技」の節を出さない。
    private let comboService: (any ComboCalcService)?
    /// 連続技の「素早さの基本値の例」を引く素早さ API。nil なら例を出さない。
    private let speedService: (any SpeedService)?
    private let backendDescription: String
    /// 計算履歴の行から開いたときの、復元する計算(ADR-0519)。nil は通常の起動(既定の入力)。
    private let restoring: CalcHistoryCalc?
    /// お気に入り(計算つき)から開いたときの、復元するお気に入りと一覧での見出し(F-09・ADR-0524)。履歴から開いたときは nil。
    private let restoringFavorite: RestoringFavorite?
    @State private var didRestore = false
    /// 素早さ画面から連続技を開く要求(ADR-0542)。nil は一度も要求されていない。
    private let comboOpenRequest: ComboOpenRequest?

    /// design.md「攻守入れ替え: カードが入れ替わる(0.35秒)」。
    private static let swapAnimationDuration: Double = 0.35
    /// 計算中の案内(アイコン + 一言)の枠の高さ。出たり消えたりで下の行が上下にずれないよう、
    /// 表示の有無に関わらず高さを固定で確保する(批評「任意」対応)。文字の大きさに合わせて広げる。
    @ScaledMetric(relativeTo: .body) private var loadingIndicatorHeight: CGFloat = 48

    /// 画面の持ち方(タブ・区切りは `.retained`。ADR-0531 §2)。離れたときに送信を取り消すかを決める。
    private let lifetime: ScreenLifetime

    init(
        service: any PokeCalcService, teamStore: any TeamStore, backendDescription: String,
        frequentOpponentsService: (any FrequentOpponentsService)? = nil,
        favoritesService: (any FavoritesService)? = nil, restoring: CalcHistoryCalc? = nil,
        restoringFavorite: RestoringFavorite? = nil, lifetime: ScreenLifetime = .retained,
        comboService: (any ComboCalcService)? = nil, speedService: (any SpeedService)? = nil,
        comboOpenRequest: ComboOpenRequest? = nil
    ) {
        self.comboOpenRequest = comboOpenRequest
        self.lifetime = lifetime
        _viewModel = State(
            initialValue: CalcViewModel(service: service, teamStore: teamStore))
        _frequentOpponents = State(
            initialValue: FrequentOpponentsViewModel(service: frequentOpponentsService, resolver: service))
        _favoritePin = State(initialValue: FavoritePinViewModel(service: favoritesService))
        self.service = service
        self.favoritesService = favoritesService
        self.comboService = comboService
        self.speedService = speedService
        self.backendDescription = backendDescription
        self.restoring = restoring
        self.restoringFavorite = restoringFavorite
    }

    /// ダメージバー・相性の色に使う、選択中の技のタイプ色。技が無い(読み込み中)ときは無彩色にする。
    private var moveTypeColor: Color {
        viewModel.selectedMove.flatMap { TypeColorToken.color(forTypeID: $0.type.rawValue) } ?? ColorToken.textSecondary.color
    }

    private var swapTransition: AnyTransition {
        reduceMotion
            ? .identity
            : .asymmetric(
                insertion: .move(edge: .trailing).combined(with: .opacity),
                removal: .move(edge: .leading).combined(with: .opacity)
            )
    }

    /// 連続技の節へ送るときの目印と、送る動きの長さ(視差効果を減らすでは動かさない)。
    private static let comboAnchorID = "calcComboSectionAnchor"
    private static let scrollAnimationDuration: Double = 0.3

    var body: some View {
        ScrollViewReader { proxy in
            scrollContent(proxy)
        }
    }

    private func scrollContent(_ proxy: ScrollViewProxy) -> some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                backendBadge
                if let error = viewModel.error {
                    ErrorBannerView(message: error.message)
                    if viewModel.canRetryLoad {
                        LoadRetryButton(identifier: "calcLoadRetry") { await viewModel.retryLoad() }
                    }
                }
                cardsRow
                presetSegmentedRow
                teamSourceRow
                if favoritesService != nil {
                    FavoriteLoadEntryRows { favoriteLoadTarget = FavoriteLoadTarget(side: $0) }
                }
                if let restoringFavorite, viewModel.error == nil {
                    FavoriteRestoreNoticeView(title: restoringFavorite.title)
                }
                if let notice = viewModel.favoriteLoadNotice {
                    FavoriteLoadNoticeView(notice: notice)
                }
                moveSelector
                CalcAttackerStatBlocksView(viewModel: viewModel, focus: $focusedAttackerStat)
                CalcConditionsSection(viewModel: viewModel)
                loadingSlot
                ResultsSectionView(viewModel: viewModel, barColor: moveTypeColor)
                if let comboService {
                    ComboSectionView(
                        calc: viewModel, combo: comboService, master: service, speed: speedService,
                        openRequest: comboOpenRequest,
                        openFromSpeed: { await viewModel.openFromSpeed($0) },
                        onHandoffApplied: {
                            withAnimation(reduceMotion ? nil : .easeInOut(duration: Self.scrollAnimationDuration)) {
                                proxy.scrollTo(Self.comboAnchorID, anchor: .top)
                            }
                        }
                    )
                    .id(Self.comboAnchorID)
                }
                if favoritePin.isAvailable {
                    FavoritePinSection(calc: viewModel, pin: favoritePin)
                }
            }
            .padding(SpacingToken.x4)
        }
        .popScreenBackground()
        .accessibilityIdentifier("calcScreen")
        .toolbar {
            // 数値キーボードには確定キーが無いので、閉じる手段をキーボードのツールバーに置く。
            ToolbarItemGroup(placement: .keyboard) {
                Spacer()
                Button(AttackerStatLabels.keyboardDone) { focusedAttackerStat = nil }
                    .accessibilityIdentifier("calcKeyboardDone")
            }
        }
        .sheet(item: $favoriteLoadTarget) { target in
            FavoriteLoadSheet(side: target.side, service: favoritesService, resolver: service) { favorite in
                viewModel.scheduleLatest { await $0.loadFavorite(favorite, side: target.side) }
            }
        }
        .task {
            if let restoringFavorite, !didRestore {
                didRestore = true
                await viewModel.loadFavoriteCalc(restoringFavorite.favorite)
            } else if let restoring, !didRestore {
                didRestore = true
                await viewModel.loadHistoryCalc(restoring)
            } else {
                await viewModel.load()
            }
        }
        // タブを移って戻ったら構築を読み直す(構築タブで作った構築を呼び出せるように。ADR-0531)。
        .onReappear { Task { await viewModel.loadTeams() } }
        // 戻ると破棄される画面(積んだ画面)だけ、離れたら入力・送信を止める(issue #113 A6・ADR-0531 §2)。
        .cancelsPendingWorkOnDisappear(lifetime) { viewModel.cancelPendingWork() }
    }

    private var backendBadge: some View {
        Text(backendDescription)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .padding(.horizontal, SpacingToken.x3)
            .padding(.vertical, SpacingToken.x1)
            .background(ColorToken.tableZebra.color, in: Capsule())
            .accessibilityIdentifier("calcBackendModeBadge")
    }

    private var cardsRow: some View {
        GlassEffectContainer(spacing: SpacingToken.x2) {
            // 文字サイズがアクセシビリティ域(`.accessibility1` 以上)まで大きいと、2枚のカードを
            // 横に並べる余白が無くなるため、縦に積む(攻撃側 → 入れ替えボタン → 防御側)。
            // 見出しはカードの外・上に置く(カードの入れ替えの動き〈`.id`〉に巻き込まない)。
            if dynamicTypeSize >= .accessibility1 {
                VStack(spacing: SpacingToken.x2) {
                    attackerColumn
                    swapButton
                    defenderColumn
                }
            } else {
                HStack(alignment: .top, spacing: SpacingToken.x2) {
                    attackerColumn
                    swapButton
                    defenderColumn
                }
            }
        }
        // `swapTick` は `swapSides()` だけが増やす(選択のやり直しや起動時の読み込みでは増えない)。
        // カードの入れ替えという「操作への反応」だけに動きを絞る(design.md「動き」)。
        .animation(reduceMotion ? nil : .easeInOut(duration: Self.swapAnimationDuration), value: viewModel.swapTick)
    }

    private var attackerColumn: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            PopHeading(title: CalcScreenText.attackerHeading, systemImage: PopSymbol.damage, identifier: "calcAttackerHeading")
            AttackerCardView(viewModel: viewModel)
                .id(viewModel.attackerSpeciesKey)
                .transition(swapTransition)
        }
    }

    private var defenderColumn: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            PopHeading(title: CalcScreenText.defenderHeading, systemImage: PopSymbol.judge, identifier: "calcDefenderHeading")
            DefenderCardView(viewModel: viewModel, frequentOpponents: frequentOpponents)
                .id(viewModel.defenderSpeciesKey)
                .transition(swapTransition)
        }
    }

    private var swapButton: some View {
        PopIconButton(
            systemImage: PopSymbol.swap, label: "攻守を入れ替える", identifier: "swapSidesButton", minSide: CalcScreenMetrics.iconButtonSide
        ) {
            viewModel.scheduleLatest { await $0.swapSides() }
        }
        .padding(.top, SpacingToken.x6)
    }

    /// 自分側のプリセット。カードの外、画面幅いっぱいの3等分のピルで並べる(批評 M3c:
    /// カード内に置くと幅が足りず「無振り」が見切れていた)。アクセシビリティの大きい文字サイズでは
    /// 3等分だとラベルが省略されるので縦に積む(P6-15。`cardsRow` と同じ分岐)。
    @ViewBuilder
    private var presetSegmentedRow: some View {
        if dynamicTypeSize >= .accessibility1 {
            VStack(spacing: SpacingToken.x2) {
                presetSegmentedRowPills
            }
        } else {
            HStack(spacing: SpacingToken.x2) {
                presetSegmentedRowPills
            }
        }
    }

    @ViewBuilder
    private var presetSegmentedRowPills: some View {
        ForEach(AttackerPreset.allCases, id: \.self) { preset in
            let isSelected = viewModel.attackerPreset == preset
            // もう一方のブロックが上昇のときの「特化」は同じ向きになるので選べない(理由はブロック側に出す。ADR-0518 §1)。
            let isEnabled = viewModel.isPresetSelectable(preset, for: viewModel.usedAttackStat ?? .atk)
            Button {
                viewModel.scheduleLatest { await $0.selectAttackerPreset(preset) }
            } label: {
                HStack(spacing: SpacingToken.x1) {
                    if isSelected { PopIcon(PopSymbol.done) }
                    Text(preset.label(for: viewModel.selectedMove?.category ?? .physical))
                        .font(TextStyleToken.body.font)
                        .multilineTextAlignment(.center)
                        .fixedSize(horizontal: false, vertical: true)
                }
                    .foregroundStyle(PopChipStyle.foreground(isSelected: isSelected))
                    .frame(maxWidth: .infinity, minHeight: CalcScreenMetrics.minimumTapSide)
                    .padding(.vertical, SpacingToken.x1)
                    .background(
                        Capsule().fill(PopChipStyle.fill(isSelected: isSelected))
                    )
                    .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
            }
            .buttonStyle(.plain)
            .disabled(!isEnabled)
            .opacity(isEnabled ? 1 : CalcScreenMetrics.disabledChipOpacity)
            .accessibilityIdentifier("attackerPreset-\(preset.rawValue)")
            .accessibilityAddTraits(isSelected ? .isSelected : [])
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
            viewModel.scheduleLatest { await $0.selectTeamIndividual(teamID: teamID, memberID: memberID) }
        }
    }

    /// issue #68: 技の一覧も数百件になりうるため、`Menu` ではなく検索シートで選ぶ(ADR-0501
    /// 「issue #68」1章「判断」)。入口の identifier(`movePicker`)は変えない。
    private var moveSelector: some View {
        Button {
            isMoveSearchPresented = true
        } label: {
            HStack(alignment: .center, spacing: SpacingToken.x2) {
                // タイプバッジ + 技名 + 分類のアイコン + 威力(Web の技のトリガーと同じ並び)。狭い幅では折り返す。
                ComboWrapLayout(spacing: SpacingToken.x2) {
                    if let move = viewModel.selectedMove {
                        TypeBadgeView(type: move.type)
                        Text(move.nameJa)
                            .font(TextStyleToken.heading.font)
                            .foregroundStyle(ColorToken.textPrimary.color)
                            .multilineTextAlignment(.leading)
                            .fixedSize(horizontal: false, vertical: true)
                        PopIcon(PopSymbol.moveCategory(move.category))
                            .foregroundStyle(ColorToken.textSecondary.color)
                        Text(CalcScreenLabels.movePower(move.power))
                            .font(TextStyleToken.body.font.monospacedDigit())
                            .foregroundStyle(ColorToken.textSecondary.color)
                            .fixedSize(horizontal: false, vertical: true)
                    } else {
                        Text("-")
                            .font(TextStyleToken.heading.font)
                            .foregroundStyle(ColorToken.textPrimary.color)
                    }
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                PopIcon(PopSymbol.picker)
                    .foregroundStyle(ColorToken.textSecondary.color)
            }
            .padding(SpacingToken.x3)
            .frame(minHeight: CalcScreenMetrics.minimumTapSide)
            .popCard(cornerRadius: RadiusToken.input)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        // 読み上げは技名、値に「威力37 / 物理 / ばつぐん(×2)」(文の要約は見える文字にしない。相性は結果の先頭のバッジ)。
        .accessibilityLabel(viewModel.selectedMove?.nameJa ?? "-")
        .accessibilityValue(viewModel.moveSummaryText)
        .accessibilityIdentifier("movePicker")
        .sheet(isPresented: $isMoveSearchPresented) {
            MoveSearchSheet(
                viewModel: viewModel, options: viewModel.displayedMoveOptions,
                onSelect: { move in viewModel.scheduleLatest { await $0.selectMove(id: move.id) } },
                groupsByType: true)
        }
    }

    private var loadingSlot: some View {
        Group {
            if viewModel.isLoading {
                PopNoticeView(kind: .loading, message: CalcScreenText.loading, identifier: "calcLoadingIndicator")
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .frame(height: loadingIndicatorHeight)
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
