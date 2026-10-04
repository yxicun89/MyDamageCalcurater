import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// AdjustScreenParts: 調整画面のカード・選択肢・欄の共通部品(AJ7。ADR-0502 §8)。
// 色は無彩色トークンだけ、アニメーションは入れない。

/// 領域のカード: 見出し(`.isHeader`)+ 中身。中の `Text` を識別子で引けるよう `.contain` にする
/// (ADR-0501 P6-14 §2 の申し送り)。
struct AdjustCard<Content: View>: View {
    let title: String
    let identifier: String
    @ViewBuilder let content: () -> Content

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            Text(title)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .accessibilityAddTraits(.isHeader)
            content()
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard()
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
    let onSelect: (SpeciesSummary) -> Void
    @State private var isSearchPresented = false

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            AdjustFieldCaption(text: title)
            Button {
                isSearchPresented = true
            } label: {
                if species != nil {
                    SpeciesHeaderMenuLabel(species: species)
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

/// 縦に並ぶ選択肢の1つ(モード・耐久の基準・攻撃の分類。segmented にしない。5つは AX5 の幅に収まらない)。
struct AdjustChoiceButton: View {
    let title: String
    let isSelected: Bool
    let identifier: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
                Image(systemName: isSelected ? "largecircle.fill.circle" : "circle")
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .accessibilityHidden(true)
                Text(title)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .multilineTextAlignment(.leading)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .padding(.vertical, SpacingToken.x1)
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
                .foregroundStyle(isSelected ? ColorToken.bgBase.color : ColorToken.textPrimary.color)
                .multilineTextAlignment(.center)
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

/// 数字キーボードの欄(固定 SP・素早さの目標)。文字で持ち、検査は送信時(ViewModel)。
/// 大きい文字サイズでは見えるラベルを上に積む。説明文は `accessibilityHint`。
struct AdjustNumberFieldRow: View {
    let title: String
    let hint: String?
    let identifier: String
    let text: Binding<String>
    let focus: FocusState<AdjustFocus?>.Binding
    let focusValue: AdjustFocus
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    var body: some View {
        if dynamicTypeSize >= .accessibility1 {
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                label
                field
            }
        } else {
            HStack(spacing: SpacingToken.x2) {
                label
                field
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
            .foregroundStyle(ColorToken.textPrimary.color)
            .padding(.horizontal, SpacingToken.x3)
            .padding(.vertical, SpacingToken.x2)
            .frame(maxWidth: Self.fieldMaxWidth)
            .background(ColorToken.bgGlass.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
            .accessibilityLabel(title)
            .accessibilityHint(hint ?? "")
            .accessibilityIdentifier(identifier)
    }

    /// 数字の欄の幅(2〜3桁が入れば足りる)。
    private static let fieldMaxWidth: CGFloat = 120
}

/// 0...32 などを `Menu` の Picker で選ぶ欄(上限・発数・確率)。見えるラベルは上、大きい文字でも縦に積む。
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
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .background(ColorToken.bgGlass.color, in: Capsule())
                .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        }
        .buttonStyle(.plain)
        .disabled(!isEnabled)
        .opacity(isEnabled ? 1 : CalcScreenMetrics.disabledChipOpacity)
        .accessibilityLabel(accessibilityLabel)
        .accessibilityIdentifier(identifier)
    }
}
