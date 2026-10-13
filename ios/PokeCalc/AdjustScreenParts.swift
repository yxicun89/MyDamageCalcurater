import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// AdjustScreenParts: 調整画面のカード・選択肢・欄の共通部品(AJ7。ADR-0502 §8)。
// 色は無彩色トークンだけ、アニメーションは入れない。

/// 調整画面のアイコンボタンのタップ領域の一辺(36pt に 1pt 足す。小数の座標で 36 ちょうどが 36 未満に測られないように)。
enum AdjustMetrics {
    static let iconButtonSide: CGFloat = CalcScreenMetrics.minimumTapSide + 1
}

/// 領域のカード: 見出し(`.isHeader`)+ 中身。中の `Text` を識別子で引けるよう `.contain` にする
/// (ADR-0501 P6-14 §2 の申し送り)。
struct AdjustCard<Content: View>: View {
    let title: String
    /// 見出しのアイコン(`PopSymbol`。Web の領域ごとのアイコンと同じ割り当て)。
    let systemImage: String
    let identifier: String
    @ViewBuilder let content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            PopHeading(title: title, systemImage: systemImage)
            content()
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard()
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier(identifier)
    }
}

/// 欄の見えるラベル(小さい補助の語)。
struct AdjustFieldCaption: View {
    let text: String

    var body: some View {
        Text(text)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// `Menu` で選ぶ欄: 見えるラベル + 現在の値のチップ。項目は見える名前の `Button`。
/// accessibilityLabel は「<領域>の<ラベル>」(見える語を含む)、値は現在の選択。
struct AdjustMenuRow<Items: View>: View {
    let title: String
    let accessibilityLabel: String
    let valueText: String
    let identifier: String
    /// 固定中の理由(メガ種族の持ち物。nil なら操作できる。ADR-0509 §8)。
    var lockReason: String?
    @ViewBuilder let items: () -> Items

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            AdjustFieldCaption(text: title)
            Menu {
                items()
            } label: {
                MenuLabelChip(text: valueText)
            }
            .disabled(lockReason != nil)
            .accessibilityLabel(accessibilityLabel)
            .accessibilityValue(valueText)
            .accessibilityHint(lockReason ?? "")
            .accessibilityIdentifier(identifier)
            if let lockReason {
                Text(lockReason)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("\(identifier)LockReason")
            }
        }
    }
}

/// 種族を検索シート(計算・逆算と同じ `SpeciesSearchSheet`)で選ぶ欄。未選択は「ポケモンを選ぶ」。
struct AdjustSpeciesRow: View {
    let viewModel: AdjustViewModel
    let species: SpeciesSummary?
    let title: String
    let accessibilityLabel: String
    let identifier: String
    /// ポケモンカードの持ち物の印(`AdjustViewModel.ownItemMark` など)。
    var itemMark: String?
    let onSelect: (SpeciesSummary) -> Void
    @State private var isSearchPresented = false

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            AdjustFieldCaption(text: title)
            Button {
                isSearchPresented = true
            } label: {
                if let species {
                    PopPokemonCard(speciesKey: species.key, name: species.nameJa, types: species.types, itemName: itemMark)
                } else {
                    MenuLabelChip(text: AdjustText.speciesPlaceholder)
                }
            }
            .buttonStyle(.plain)
            .accessibilityLabel(accessibilityLabel)
            .accessibilityValue(species?.nameJa ?? AdjustText.speciesPlaceholder)
            .accessibilityIdentifier(identifier)
            .sheet(isPresented: $isSearchPresented) {
                SpeciesSearchSheet(viewModel: viewModel, onSelect: onSelect)
            }
        }
    }
}

/// 縦に並ぶ全幅のチップ(モード・耐久の基準・攻撃の分類。segmented にしない。AX5 でも縦に積む)。
/// 選択中は塗り + チェック。選択は選択の特性で伝える(丸の列はやめた。ADR-0540 §6)。
struct AdjustChoiceButton: View {
    let title: String
    let isSelected: Bool
    let identifier: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
                if isSelected { PopIcon(PopSymbol.done) }
                Text(title)
                    .font(TextStyleToken.body.font)
                    .multilineTextAlignment(.leading)
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .foregroundStyle(PopChipStyle.foreground(isSelected: isSelected))
            .padding(.horizontal, SpacingToken.x3)
            .padding(.vertical, SpacingToken.x2)
            .frame(maxWidth: .infinity, minHeight: CalcScreenMetrics.minimumTapSide, alignment: .leading)
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
        .accessibilityIdentifier(identifier)
        .accessibilityAddTraits(isSelected ? .isSelected : [])
    }
}

/// プリセットのピル1つ(`ReverseScreenView` の `PresetPillButton` と同じ見た目)。
struct AdjustPillButton: View {
    let title: String
    let isSelected: Bool
    let identifier: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Text(title)
                .font(TextStyleToken.body.font)
                .foregroundStyle(PopChipStyle.foreground(isSelected: isSelected))
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, minHeight: CalcScreenMetrics.minimumTapSide)
                .padding(.vertical, SpacingToken.x1)
                .background(Capsule().fill(PopChipStyle.fill(isSelected: isSelected)))
                .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier(identifier)
        .accessibilityAddTraits(isSelected ? .isSelected : [])
    }
}

/// 欄の − / + の設定(範囲と識別子。読み上げ名は欄の名前から `AdjustText.stepperIncrease/Decrease`)。
struct AdjustStepConfig {
    let range: ClosedRange<Int>
    let downIdentifier: String
    let upIdentifier: String
}

/// 数字キーボードの欄(固定 SP・上限・素早さの目標)。文字で持ち、検査は送信時(ViewModel)。
/// `step` があれば左右に − / +(`PopIconButton`。丸めと可否は `AdjustNumberStep`)。大きい文字サイズでは見えるラベルを上に積む。
/// 説明文は `accessibilityHint`(見える補足は「説明」ボタンの奥)。
struct AdjustNumberFieldRow: View {
    let title: String
    let hint: String?
    let identifier: String
    let text: Binding<String>
    let focus: FocusState<AdjustFocus?>.Binding
    let focusValue: AdjustFocus
    var step: AdjustStepConfig?
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    var body: some View {
        if dynamicTypeSize >= .accessibility1 {
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                label
                controls
            }
        } else {
            HStack(spacing: SpacingToken.x2) {
                label
                controls
            }
        }
    }

    private var controls: some View {
        HStack(spacing: SpacingToken.x1) {
            if let step {
                PopIconButton(
                    systemImage: PopSymbol.minus, label: AdjustText.stepperDecrease(title), identifier: step.downIdentifier,
                    isEnabled: AdjustNumberStep.canDecrease(text: text.wrappedValue, range: step.range),
                    minSide: AdjustMetrics.iconButtonSide
                ) {
                    text.wrappedValue = AdjustNumberStep.stepped(text: text.wrappedValue, delta: -1, range: step.range)
                }
            }
            field
            if let step {
                PopIconButton(
                    systemImage: PopSymbol.add, label: AdjustText.stepperIncrease(title), identifier: step.upIdentifier,
                    isEnabled: AdjustNumberStep.canIncrease(text: text.wrappedValue, range: step.range),
                    minSide: AdjustMetrics.iconButtonSide
                ) {
                    text.wrappedValue = AdjustNumberStep.stepped(text: text.wrappedValue, delta: 1, range: step.range)
                }
            }
        }
    }

    private var label: some View {
        Text(title)
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityHidden(true)
    }

    private var field: some View {
        TextField("", text: text)
            .keyboardType(.numberPad)
            .focused(focus, equals: focusValue)
            .font(TextStyleToken.body.font.monospacedDigit())
            .multilineTextAlignment(step == nil ? .leading : .center)
            .foregroundStyle(ColorToken.textPrimary.color)
            .padding(.horizontal, SpacingToken.x3)
            .padding(.vertical, SpacingToken.x2)
            .frame(minHeight: CalcScreenMetrics.minimumTapSide)
            .frame(maxWidth: Self.fieldMaxWidth)
            .background(ColorToken.tableZebra.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
            .accessibilityLabel(title)
            .accessibilityHint(hint ?? "")
            .accessibilityIdentifier(identifier)
    }

    /// 数字の欄の幅(2〜3桁が入れば足りる)。
    private static let fieldMaxWidth: CGFloat = 120
}

/// 「説明」ボタンの奥に置く補足の文(吹き出しの中。文面は変えない。識別子は文に付ける)。
struct AdjustHelpText: View {
    let text: String
    let identifier: String

    var body: some View {
        Text(text)
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
            .accessibilityIdentifier(identifier)
    }
}

/// 固定する能力ポイントの「説明」ボタン(補足は常時の文にしない)。
struct AdjustFixedSPHelpButton: View {
    var body: some View {
        PopHelpButton(title: CommonPartsText.help, systemImage: PopSymbol.help, identifier: "adjustFixedSPHelpButton") {
            AdjustHelpText(text: AdjustText.fixedSPHint, identifier: "adjustFixedSPHint")
        }
    }
}

/// 最低素早さの「説明」ボタン。
struct AdjustMinSpeedHelpButton: View {
    var body: some View {
        PopHelpButton(title: CommonPartsText.help, systemImage: PopSymbol.help, identifier: "adjustMinSpeedHelpButton") {
            AdjustHelpText(text: AdjustText.minSpeedHint, identifier: "adjustMinSpeedHint")
        }
    }
}

/// `Menu` の Picker で選ぶ欄(発数・確率)。見えるラベルは上、大きい文字でも縦に積む。
struct AdjustPickerRow<Value: Hashable, Options: View>: View {
    let title: String
    let identifier: String
    let selection: Binding<Value>
    @ViewBuilder let options: () -> Options

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            AdjustFieldCaption(text: title)
            Picker(title, selection: selection, content: options)
                .pickerStyle(.menu)
                .labelsHidden()
                .tint(ColorToken.textPrimary.color)
                .accessibilityLabel(title)
                .accessibilityIdentifier(identifier)
        }
    }
}

/// 「覚えるポケモン」のボタン。技を選ぶまで無効。
struct AdjustLearnersButton: View {
    let accessibilityLabel: String
    let identifier: String
    let isEnabled: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Text(AdjustText.learnersButton)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .frame(minHeight: CalcScreenMetrics.minimumTapSide)
                .background(ColorToken.tableZebra.color, in: Capsule())
                .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        }
        .buttonStyle(.plain)
        .disabled(!isEnabled)
        .opacity(isEnabled ? 1 : CalcScreenMetrics.disabledChipOpacity)
        .accessibilityLabel(accessibilityLabel)
        .accessibilityIdentifier(identifier)
    }
}
