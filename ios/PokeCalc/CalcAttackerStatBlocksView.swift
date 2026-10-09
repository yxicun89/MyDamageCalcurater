import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// CalcAttackerStatBlocksView: 計算画面の攻撃側「攻撃」「特攻」の2ブロック(ADR-0518。F-01 / I-ios-1)。
//
// ロジックは持たない(ADR-0500 §1)。SP の文字列・性格補正・選べるかどうか・カスタムの判定は
// `CalcViewModel`(`CalcViewModel+AttackerStats`)が持ち、ここは描いて操作を async メソッドへつなぐだけ。
// 置き場所は「技セレクタ」と「詳細」の間。常に出す(design.md「数値の直接入力は詳細を開いたときだけ」の例外)。

/// 「攻撃」「特攻」の2ブロックと、計算しない理由の案内。
struct CalcAttackerStatBlocksView: View {
    let viewModel: CalcViewModel
    /// SP 欄のフォーカス。数値キーボードのツールバー(「完了」)が `CalcScreenView` にあるので、親から受け取る。
    var focus: FocusState<AttackStat?>.Binding

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            ForEach(AttackStat.allCases, id: \.self) { stat in
                AttackerStatBlockView(viewModel: viewModel, stat: stat, focus: focus)
            }
            notices
        }
        // `.contain`: 子(各ブロック)を個別の要素のまま、コンテナ自体も見つけられるようにする
        // (子が2つ以上あること。1つだけだと識別子が子に畳まれる)。
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("attackerStatBlocks")
    }

    @ViewBuilder
    private var notices: some View {
        if viewModel.attackerBuildSource.teamSelection != nil {
            notice(AttackerStatLabels.teamSourceNotice, identifier: "attackerStatTeamNotice")
        }
        if viewModel.isStatusMoveSelected {
            notice(AttackerStatLabels.statusMoveNotice, identifier: "calcStatusMoveNotice")
        }
        if viewModel.hasNoDamagingMoves {
            notice(AttackerStatLabels.noDamagingMovesNotice, identifier: "calcNoDamagingMovesNotice")
        }
    }

    private func notice(_ text: String, identifier: String) -> some View {
        Text(text)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityIdentifier(identifier)
    }
}

/// 1ブロック(見出し・SP の数値欄・性格補正の3択)。
private struct AttackerStatBlockView: View {
    let viewModel: CalcViewModel
    let stat: AttackStat
    var focus: FocusState<AttackStat?>.Binding
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    private var isUsed: Bool { viewModel.usedAttackStat == stat }

    private var hasUnselectableChoice: Bool {
        NatureChoice.allCases.contains { !viewModel.isModifierSelectable($0, for: stat) }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            headingRow
            AttackerSPField(viewModel: viewModel, stat: stat, focus: focus)
            if viewModel.isSPInvalid(stat) {
                Text(AttackerStatLabels.spInvalid(stat))
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.danger.color)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityIdentifier("attackerSPError-\(stat.rawValue)")
            }
            natureChoices
            if hasUnselectableChoice {
                Text(AttackerStatLabels.sameDirectionReason)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .accessibilityIdentifier("attackerSameDirectionReason-\(stat.rawValue)")
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard(cornerRadius: RadiusToken.input)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("attackerStatBlock-\(stat.rawValue)")
    }

    /// 見出し。選んだ技が使う側には「(この技で使用)」を文字で足す(色だけに頼らない)。
    private var headingRow: some View {
        HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
            Text(AttackerStatLabels.heading(for: stat, isUsed: isUsed))
                .font(isUsed ? TextStyleToken.heading.font : TextStyleToken.body.font)
                .foregroundStyle(isUsed ? ColorToken.textPrimary.color : ColorToken.textSecondary.color)
                .accessibilityIdentifier("attackerStatHeading-\(stat.rawValue)")
                .accessibilityAddTraits(.isHeader)
            if viewModel.isCustom(stat) {
                Text(AttackerStatLabels.custom)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier("attackerCustomMark-\(stat.rawValue)")
            }
            Spacer(minLength: 0)
        }
    }

    /// 性格補正の3択(上昇・補正なし・下降)。`Menu` は使わず、ボタンの並び。AX では縦に積む。
    @ViewBuilder
    private var natureChoices: some View {
        Group {
            if dynamicTypeSize.isAccessibilitySize {
                VStack(spacing: SpacingToken.x2) { choiceButtons }
            } else {
                HStack(spacing: SpacingToken.x2) { choiceButtons }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(AttackerStatLabels.natureGroupLabel(stat))
        .accessibilityIdentifier("attackerNatureGroup-\(stat.rawValue)")
    }

    @ViewBuilder
    private var choiceButtons: some View {
        ForEach(NatureChoice.allCases, id: \.self) { choice in
            let isSelected = viewModel.attackerStatInputs[stat].modifier == choice
            let isEnabled = viewModel.isModifierSelectable(choice, for: stat)
            Button {
                viewModel.scheduleLatest { await $0.setAttackerNatureModifier(choice, for: stat) }
            } label: {
                Text(AttackerStatLabels.modifierName(choice))
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(isSelected ? ColorToken.bgBase.color : ColorToken.textPrimary.color)
                    .frame(maxWidth: .infinity, minHeight: CalcScreenMetrics.minimumTapSide)
                    .background(Capsule().fill(isSelected ? ColorToken.textPrimary.color : ColorToken.bgGlass.color))
                    .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
            }
            .buttonStyle(.plain)
            .disabled(!isEnabled)
            .opacity(isEnabled ? 1 : CalcScreenMetrics.disabledChipOpacity)
            .accessibilityIdentifier("attackerNature-\(stat.rawValue)-\(choice.rawValue)")
            .accessibilityAddTraits(isSelected ? .isSelected : [])
        }
    }
}

/// SP の数値欄。入力途中の文字列を保つため、欄は自分の `draft` を同期で書き換え、VM へは後から反映する
/// (VM の更新を待つと `TextField` の反映が1フレーム遅れて、速く打つと文字が落ちる。`ReverseObservationRow` と同じ事情)。
/// プリセットなど外からの変更は `onChange` で `draft` に戻す。
private struct AttackerSPField: View {
    let viewModel: CalcViewModel
    let stat: AttackStat
    var focus: FocusState<AttackStat?>.Binding
    @State private var draft: String
    /// VM へ送ったが、まだ VM の値として戻ってきていない文字列。速く打つと VM の更新が遅れて古い値が戻ってくるので、
    /// 自分が送った値の戻りで `draft` を巻き戻さないために覚える(外からの変更だけを `draft` に反映する)。
    @State private var sentValues: Set<String> = []

    init(viewModel: CalcViewModel, stat: AttackStat, focus: FocusState<AttackStat?>.Binding) {
        self.viewModel = viewModel
        self.stat = stat
        self.focus = focus
        _draft = State(initialValue: viewModel.attackerStatInputs[stat].spText)
    }

    var body: some View {
        TextField("", text: $draft)
            .keyboardType(.numberPad)
            .focused(focus, equals: stat)
            .font(TextStyleToken.body.font.monospacedDigit())
            .foregroundStyle(ColorToken.textPrimary.color)
            .padding(.horizontal, SpacingToken.x3)
            .frame(minHeight: CalcScreenMetrics.minimumTapSide)
            .background(ColorToken.bgGlass.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
            .accessibilityLabel(AttackerStatLabels.spLabel(stat))
            .accessibilityIdentifier("attackerSPField-\(stat.rawValue)")
            .onChange(of: draft) { _, newValue in
                if newValue == viewModel.attackerStatInputs[stat].spText {
                    sentValues = []
                } else {
                    sentValues.insert(newValue)
                }
                viewModel.scheduleLatest { await $0.setAttackerSPText(newValue, for: stat) }
            }
            .onChange(of: viewModel.attackerStatInputs[stat].spText) { _, newValue in
                if newValue == draft {
                    sentValues = []
                } else if !sentValues.contains(newValue) {
                    draft = newValue
                    sentValues = []
                }
            }
    }
}
