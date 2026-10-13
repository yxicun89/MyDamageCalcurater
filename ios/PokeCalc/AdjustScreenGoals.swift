import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// AdjustScreenGoals: 調整の「目標」方式(F-11。ADR-0331 §5・ADR-0525)の入力カード。
//
// 「相手を選んで目標の種類(素早さを上回る・この技を耐える・この技で倒す)を選ぶ」。ロジックは持たず、
// `AdjustViewModel`(+Goals)の状態を描いて操作をつなぐだけ。選択は sheet/チップ(Menu は使わない)、
// タップは 36pt 以上、行は折り返す(lineLimit なし)、色はトークンだけ、アニメーションは入れない。
// 識別子の番号 n は画面上の位置(1 始まり。Web の「目標 n」と同じ。外すと詰める)。

/// 技を選ぶ欄の役割(sheet の中身と選択先を決める)。
private enum GoalMoveRole: Hashable {
    /// survive: 相手の技。
    case opponent
    /// ko: 自分の技。
    case own
    /// outspeed: 先に使う技(任意)。
    case boost
}

private struct GoalMovePick: Identifiable, Equatable {
    let goalID: Int
    let role: GoalMoveRole
    var id: String { "\(goalID)-\(role)" }
}

/// 目標の領域: 目標のカードを追加順に並べ、「目標を追加」ボタンを置く。
struct AdjustGoalsCardView: View {
    let viewModel: AdjustViewModel
    @State private var movePick: GoalMovePick?

    var body: some View {
        AdjustCard(title: AdjustText.goalRegion, systemImage: PopSymbol.judge, identifier: "adjustGoalsCard") {
            if viewModel.goalDrafts.isEmpty {
                PopNoticeView(kind: .empty, message: AdjustText.noGoalsNotice, identifier: "adjustNoGoals")
            }
            ForEach(Array(viewModel.goalDrafts.enumerated()), id: \.element.id) { offset, draft in
                AdjustGoalItemView(viewModel: viewModel, draft: draft, number: offset + 1) { role in
                    movePick = GoalMovePick(goalID: draft.id, role: role)
                }
            }
            addButton
        }
        .sheet(item: $movePick) { pick in
            AdjustGoalMoveSheet(viewModel: viewModel, goalID: pick.goalID, role: pick.role)
        }
    }

    private var addButton: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Button {
                viewModel.addGoal()
            } label: {
                AdjustAddTile(title: AdjustText.addGoalButton)
            }
            .buttonStyle(.plain)
            .disabled(!viewModel.canAddGoal)
            .opacity(viewModel.canAddGoal ? 1 : CalcScreenMetrics.disabledChipOpacity)
            .accessibilityIdentifier("adjustAddGoalButton")
            if !viewModel.canAddGoal {
                Text(AdjustText.goalLimitHint(RequestLimits.maxAdjustGoals))
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("adjustGoalLimitHint")
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// 「+」のタイル(目標の追加。破線の枠。構築の空き枠と同じ作り)。
private struct AdjustAddTile: View {
    let title: String

    var body: some View {
        VStack(spacing: SpacingToken.x1) {
            PopIcon(PopSymbol.add)
            Text(title)
                .font(TextStyleToken.body.font)
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
        }
        .foregroundStyle(ColorToken.textSecondary.color)
        .padding(SpacingToken.x2)
        .frame(maxWidth: .infinity, minHeight: CalcScreenMetrics.minimumTapSide * 2)
        .overlay(
            RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                .stroke(ColorToken.borderHairline.color, style: StrokeStyle(lineWidth: 1.5, dash: [4, 4]))
        )
        .contentShape(Rectangle())
    }
}

/// 目標1件のカード(種類・相手・振り方・技・発数/確率・外す)。
private struct AdjustGoalItemView: View {
    let viewModel: AdjustViewModel
    let draft: AdjustGoalDraft
    let number: Int
    let onPickMove: (GoalMoveRole) -> Void
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    private var prefix: String { "adjustGoal-\(number)" }

    /// 発数の行(− 値 +)の最大幅。
    private static let hitsRowMaxWidth: CGFloat = 240

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            PopHeading(
                title: AdjustText.goalCardLegend(number), systemImage: kindSymbol(draft.kind), identifier: "\(prefix)-legend")
            kindChoices
            AdjustSpeciesRow(
                viewModel: viewModel, species: viewModel.goalOpponentSpecies(draft),
                title: AdjustText.goalOpponentSpeciesField,
                accessibilityLabel: AdjustText.goalFieldName(number, AdjustText.goalOpponentSpeciesField),
                identifier: "\(prefix)-opponentButton", itemMark: viewModel.goalOpponentItemMark(draft)
            ) { option in
                Task { await viewModel.selectGoalOpponent(id: draft.id, key: option.key) }
            }
            recentOpponents
            switch draft.kind {
            case .outspeed:
                speedPresets
                boostMove
            case .survive:
                moveButton(
                    role: .opponent, field: AdjustText.goalOpponentMoveField,
                    value: viewModel.goalOpponentMove(draft)?.nameJa, identifier: "\(prefix)-opponentMoveButton")
                attackerPresets
                hitsAndChance
            case .ko:
                moveButton(
                    role: .own, field: AdjustText.goalOwnMoveField,
                    value: viewModel.goalOwnMove(draft)?.nameJa, identifier: "\(prefix)-ownMoveButton")
                defenderPresets
                hitsAndChance
            }
            removeButton
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .overlay(
            RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                .stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth)
        )
        .accessibilityElement(children: .contain)
        .accessibilityLabel(AdjustText.goalCardLegend(number))
        .accessibilityIdentifier(prefix)
    }

    // MARK: - 種類

    /// 種類のアイコン(素早さ = 速度、耐える = 判定〈盾〉、倒す = 計算。Web と同じ)。
    private func kindSymbol(_ kind: AdjustGoalKind) -> String {
        switch kind {
        case .outspeed: return PopSymbol.speed
        case .survive: return PopSymbol.judge
        case .ko: return PopSymbol.calc
        }
    }

    /// 3 択の区切りボタン(アイコン + 短い名前。選択中は塗り + チェック。AX5 は縦に積む)。
    private var kindChoices: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            AdjustFieldCaption(text: AdjustText.goalKindField)
            if dynamicTypeSize >= .accessibility1 {
                VStack(spacing: SpacingToken.x2) { kindButtons }
            } else {
                HStack(alignment: .top, spacing: SpacingToken.x2) { kindButtons }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(AdjustText.goalFieldName(number, AdjustText.goalKindField))
        .accessibilityIdentifier("\(prefix)-kind")
    }

    @ViewBuilder
    private var kindButtons: some View {
        ForEach(AdjustGoalKind.allCases, id: \.self) { kind in
            let isSelected = draft.kind == kind
            Button {
                viewModel.setGoalKind(id: draft.id, kind)
            } label: {
                VStack(spacing: SpacingToken.x1) {
                    PopIcon(isSelected ? PopSymbol.done : kindSymbol(kind))
                    Text(AdjustText.goalKindOption(kind))
                        .font(TextStyleToken.body.font)
                        .multilineTextAlignment(.center)
                        .fixedSize(horizontal: false, vertical: true)
                }
                .foregroundStyle(PopChipStyle.foreground(isSelected: isSelected))
                .padding(.horizontal, SpacingToken.x2)
                .padding(.vertical, SpacingToken.x2)
                .frame(maxWidth: .infinity, minHeight: CalcScreenMetrics.minimumTapSide)
                .background(
                    PopChipStyle.fill(isSelected: isSelected),
                    in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                )
                .overlay(
                    RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous)
                        .stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth)
                )
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("\(prefix)-kind-\(kind.rawValue)")
            .accessibilityAddTraits(isSelected ? .isSelected : [])
        }
    }

    // MARK: - 選んだ相手から

    /// これまでに選んだ相手を絵つきのタイルで並べる(押すとその目標の相手になる。従来の選択・検索も残す)。
    @ViewBuilder
    private var recentOpponents: some View {
        let recent = viewModel.recentOpponents
        if !recent.isEmpty {
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                AdjustFieldCaption(text: AdjustText.goalRecentOpponents)
                AdjustWrapLayout(spacing: SpacingToken.x2) {
                    ForEach(recent, id: \.key) { species in
                        Button {
                            Task { await viewModel.selectGoalOpponent(id: draft.id, key: species.key) }
                        } label: {
                            PopPokemonCard(
                                speciesKey: species.key, name: species.nameJa, types: species.types, itemName: nil,
                                size: .compact)
                        }
                        .buttonStyle(.plain)
                        .accessibilityLabel(species.nameJa)
                        .accessibilityAddTraits(draft.opponentSpeciesKey == species.key ? .isSelected : [])
                        .accessibilityIdentifier("\(prefix)-recent-\(species.key)")
                    }
                }
            }
            .accessibilityElement(children: .contain)
            .accessibilityLabel(AdjustText.goalRecentOpponents)
            .accessibilityIdentifier("\(prefix)-recent")
        }
    }

    // MARK: - 振り方

    private var speedPresets: some View {
        presetRow {
            ForEach(AdjustSpeedPreset.allCases, id: \.self) { preset in
                AdjustPillButton(
                    title: preset.label, isSelected: draft.speedPreset == preset,
                    identifier: "\(prefix)-preset-\(preset.rawValue)"
                ) {
                    viewModel.selectGoalSpeedPreset(id: draft.id, preset)
                }
            }
        }
    }

    /// 耐える: 相手が攻撃する。A / C は相手の技の分類(技を選ぶまでは物理扱いで暫定表示)。
    private var attackerPresets: some View {
        let category = viewModel.goalOpponentMove(draft)?.category ?? .physical
        return presetRow {
            ForEach(AttackerPreset.allCases, id: \.self) { preset in
                AdjustPillButton(
                    title: preset.label(for: category), isSelected: draft.attackerPreset == preset,
                    identifier: "\(prefix)-preset-\(preset.rawValue)"
                ) {
                    viewModel.selectGoalAttackerPreset(id: draft.id, preset)
                }
            }
        }
    }

    /// 倒す: 相手が受ける。HB / HD は自分の技の分類(技を選ぶまでは物理扱いで暫定表示)。
    private var defenderPresets: some View {
        let category = viewModel.goalOwnMove(draft)?.category ?? .physical
        return presetRow {
            ForEach(KnownDefenderPreset.allCases, id: \.self) { preset in
                AdjustPillButton(
                    title: preset.label(for: category), isSelected: draft.defenderPreset == preset,
                    identifier: "\(prefix)-preset-\(preset.rawValue)"
                ) {
                    viewModel.selectGoalDefenderPreset(id: draft.id, preset)
                }
            }
        }
    }

    /// 3等分のピル。大きい文字サイズでは縦に積む(従来の相手カードと同じ分岐)。
    @ViewBuilder
    private func presetRow<Content: View>(@ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            AdjustFieldCaption(text: AdjustText.goalPresetField)
            if dynamicTypeSize >= .accessibility1 {
                VStack(spacing: SpacingToken.x2) { content() }
            } else {
                HStack(spacing: SpacingToken.x2) { content() }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(AdjustText.goalFieldName(number, AdjustText.goalPresetField))
        .accessibilityIdentifier("\(prefix)-presets")
    }

    // MARK: - 技

    private var boostMove: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            moveButton(
                role: .boost, field: AdjustText.goalBoostMoveField,
                value: viewModel.goalBoostMove(draft)?.nameJa ?? AdjustText.goalBoostMoveNone,
                identifier: "\(prefix)-boostMoveButton", hint: AdjustText.goalBoostMoveHint)
            PopHelpButton(title: CommonPartsText.help, systemImage: PopSymbol.help, identifier: "\(prefix)-boostMoveHelpButton") {
                AdjustHelpText(text: AdjustText.goalBoostMoveHint, identifier: "\(prefix)-boostMoveHint")
            }
        }
    }

    /// 技を選ぶ欄(押すと sheet)。未選択は「技を選ぶ」。
    private func moveButton(
        role: GoalMoveRole, field: String, value: String?, identifier: String, hint: String? = nil
    ) -> some View {
        let shown = value ?? AdjustText.movePlaceholder
        return VStack(alignment: .leading, spacing: SpacingToken.x1) {
            AdjustFieldCaption(text: field)
            Button {
                onPickMove(role)
            } label: {
                MenuLabelChip(text: shown)
                    .frame(minHeight: CalcScreenMetrics.minimumTapSide)
            }
            .buttonStyle(.plain)
            .accessibilityLabel(AdjustText.goalFieldName(number, field))
            .accessibilityValue(shown)
            .accessibilityHint(hint ?? "")
            .accessibilityIdentifier(identifier)
        }
    }

    // MARK: - 発数・確率

    private var hitsAndChance: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            AdjustFieldCaption(text: AdjustText.goalHitsAndChanceField)
            // − と + は行の両端に置く(端は整数の位置になり、タップの大きさが 36pt ちょうどで測れる)。
            HStack(spacing: SpacingToken.x3) {
                PopIconButton(
                    systemImage: PopSymbol.minus, label: AdjustText.goalFieldName(number, AdjustText.hitsDecrease),
                    identifier: "\(prefix)-hitsMinus", isEnabled: draft.hits > 1,
                    minSide: AdjustMetrics.iconButtonSide
                ) {
                    viewModel.selectGoalHits(id: draft.id, draft.hits - 1)
                }
                Text(AdjustText.hitsOption(draft.hits))
                    .font(TextStyleToken.body.font.monospacedDigit())
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .multilineTextAlignment(.center)
                    .frame(maxWidth: .infinity)
                    .accessibilityIdentifier("\(prefix)-hits")
                PopIconButton(
                    systemImage: PopSymbol.add, label: AdjustText.goalFieldName(number, AdjustText.hitsIncrease),
                    identifier: "\(prefix)-hitsPlus", isEnabled: draft.hits < RequestLimits.maxAdjustHits,
                    minSide: AdjustMetrics.iconButtonSide
                ) {
                    viewModel.selectGoalHits(id: draft.id, draft.hits + 1)
                }
            }
            .frame(maxWidth: Self.hitsRowMaxWidth, alignment: .leading)
            AdjustWrapLayout(spacing: SpacingToken.x2) {
                ForEach(AdjustViewModel.thresholdOptions, id: \.self) { percent in
                    AdjustPillButton(
                        title: AdjustText.thresholdOption(percent), isSelected: draft.thresholdPercent == percent,
                        identifier: "\(prefix)-threshold-\(Int(percent))"
                    ) {
                        viewModel.selectGoalThreshold(id: draft.id, percent)
                    }
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(AdjustText.goalFieldName(number, AdjustText.goalHitsAndChanceField))
        .accessibilityIdentifier("\(prefix)-hitsAndChance")
    }

    // MARK: - 外す

    private var removeButton: some View {
        Button {
            viewModel.removeGoal(id: draft.id)
        } label: {
            PopLabel(title: AdjustText.removeGoalVisible, systemImage: PopSymbol.delete)
        }
        .buttonStyle(PillButtonStyle(kind: .secondary))
        .accessibilityLabel(AdjustText.removeGoalName(number))
        .accessibilityIdentifier("\(prefix)-remove")
    }
}

/// 技を選ぶ sheet(一覧から1つ選ぶ。選ぶと閉じる)。先頭に「未選択」(先に使う技は「使わない」)。
private struct AdjustGoalMoveSheet: View {
    let viewModel: AdjustViewModel
    let goalID: Int
    let role: GoalMoveRole
    @Environment(\.dismiss) private var dismiss

    private var draft: AdjustGoalDraft? { viewModel.goalDrafts.first { $0.id == goalID } }

    private var title: String {
        switch role {
        case .opponent: return AdjustText.goalOpponentMoveField
        case .own: return AdjustText.goalOwnMoveField
        case .boost: return AdjustText.goalBoostMoveField
        }
    }

    private var noneLabel: String { role == .boost ? AdjustText.goalBoostMoveNone : AdjustText.unselectedOption }

    private var moves: [Move] {
        switch role {
        case .opponent: return draft?.opponentMoves ?? []
        case .own, .boost: return viewModel.ownMoveOptions
        }
    }

    private var selectedID: String? {
        switch role {
        case .opponent: return draft?.opponentMoveId
        case .own: return draft?.ownMoveId
        case .boost: return draft?.boostMoveId
        }
    }

    private func choose(_ id: String?) {
        switch role {
        case .opponent: viewModel.selectGoalOpponentMove(id: goalID, moveId: id)
        case .own: viewModel.selectGoalOwnMove(id: goalID, moveId: id)
        case .boost: viewModel.selectGoalBoostMove(id: goalID, moveId: id)
        }
        dismiss()
    }

    var body: some View {
        NavigationStack {
            List {
                row(title: noneLabel, move: nil, isSelected: selectedID == nil, identifier: "adjustGoalMoveOption-none") { choose(nil) }
                ForEach(moves, id: \.id) { move in
                    row(title: move.nameJa, move: move, isSelected: selectedID == move.id, identifier: "adjustGoalMoveOption-\(move.id)") {
                        choose(move.id)
                    }
                }
            }
            .listStyle(.plain)
            .accessibilityIdentifier("adjustGoalMoveSheet")
            .navigationTitle(title)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    PopIconButton(
                        systemImage: PopSymbol.close, label: AdjustText.goalMoveSheetClose,
                        identifier: "adjustGoalMoveSheetClose", minSide: AdjustMetrics.iconButtonSide
                    ) { dismiss() }
                }
            }
        }
    }

    /// 行: タイプバッジ + 技名 + 威力(「未選択」の行はバッジ・威力なし)。選択中はチェック。
    private func row(title: String, move: Move?, isSelected: Bool, identifier: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(alignment: .center, spacing: SpacingToken.x2) {
                if let move { TypeBadgeView(type: move.type) }
                Text(title)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .multilineTextAlignment(.leading)
                    .frame(maxWidth: .infinity, alignment: .leading)
                if let move, move.power > 0 {
                    Text("\(move.power)")
                        .font(TextStyleToken.caption.font.monospacedDigit())
                        .foregroundStyle(ColorToken.textSecondary.color)
                        .accessibilityHidden(true)
                }
                if isSelected { PopIcon(PopSymbol.done).foregroundStyle(ColorToken.brandPrimary.color) }
            }
            .frame(minHeight: CalcScreenMetrics.minimumTapSide)
        }
        .buttonStyle(.plain)
        .accessibilityLabel(title)
        .accessibilityAddTraits(isSelected ? .isSelected : [])
        .accessibilityIdentifier(identifier)
    }
}

/// 子を左から並べ、入りきらなければ次の行へ折り返すレイアウト(確率のピル用)。
/// `LazyVGrid` は画面外の子を作らず、スクロール外の要素を引けなくなる(XCUITest・VoiceOver の巡回)ので、非 Lazy で作る。
/// 子の幅は理想の幅(1行で収まる幅)を上限 = 行の幅として決め、狭ければ文字が折り返す。
private struct AdjustWrapLayout: Layout {
    let spacing: CGFloat

    func sizeThatFits(proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) -> CGSize {
        let rows = arrange(width: proposal.width ?? .infinity, subviews: subviews)
        return CGSize(width: rows.map(\.width).max() ?? 0, height: rows.last.map { $0.originY + $0.height } ?? 0)
    }

    func placeSubviews(in bounds: CGRect, proposal: ProposedViewSize, subviews: Subviews, cache: inout ()) {
        for row in arrange(width: bounds.width, subviews: subviews) {
            for item in row.items {
                subviews[item.index].place(
                    at: CGPoint(x: bounds.minX + item.originX, y: bounds.minY + row.originY),
                    proposal: ProposedViewSize(width: item.width, height: row.height))
            }
        }
    }

    private struct Item {
        let index: Int
        let originX: CGFloat
        let width: CGFloat
    }

    private struct Row {
        var items: [Item] = []
        var originY: CGFloat = 0
        var width: CGFloat = 0
        var height: CGFloat = 0
    }

    private func arrange(width: CGFloat, subviews: Subviews) -> [Row] {
        var rows: [Row] = [Row()]
        var originY: CGFloat = 0
        for index in subviews.indices {
            let ideal = subviews[index].sizeThatFits(.unspecified)
            let itemWidth = min(ideal.width, width)
            let fitted = subviews[index].sizeThatFits(ProposedViewSize(width: itemWidth, height: nil))
            var row = rows[rows.count - 1]
            let startX = row.items.isEmpty ? 0 : row.width + spacing
            if !row.items.isEmpty && startX + itemWidth > width {
                originY += row.height + spacing
                rows.append(Row(originY: originY))
                row = rows[rows.count - 1]
            }
            let x = row.items.isEmpty ? 0 : row.width + spacing
            row.items.append(Item(index: index, originX: x, width: itemWidth))
            row.width = x + itemWidth
            row.height = max(row.height, fitted.height)
            rows[rows.count - 1] = row
        }
        return rows
    }
}
