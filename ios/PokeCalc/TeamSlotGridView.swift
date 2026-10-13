import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// TeamSlotGridView: 構築編集画面の先頭に置く 6 枠のタイルの格子(G-05 第 2 弾。ADR-0537 決定 2)。
//
// 埋まった枠はポケモンの小さなカード(画像かエンブレム・名前・タイプバッジ・持ち物の印)、空きは破線の「+ N体目を追加」。
// 押すとその枠の欄まで画面を送る(`onSelect`。送る処理は `TeamEditView` の `ScrollViewReader`)。Web の「欄へ移る近道」と同じ。
// ロジックは持たない(`TeamEditViewModel` の状態を描くだけ)。タイルは 36pt 以上、押下の縮みは視差効果を減らすで 0 秒。

struct TeamSlotGridView: View {
    let viewModel: TeamEditViewModel
    /// 押された枠(0 始まり)。
    let onSelect: (Int) -> Void

    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    /// 通常は 3 列、文字を大きくしたときは 1 列(名前とタイプが欠けない幅を確保する)。
    private var columns: [GridItem] {
        let count = dynamicTypeSize.isAccessibilitySize ? 1 : 3
        return Array(repeating: GridItem(.flexible(), spacing: SpacingToken.x2, alignment: .top), count: count)
    }

    var body: some View {
        LazyVGrid(columns: columns, alignment: .leading, spacing: SpacingToken.x2) {
            ForEach(0..<TeamLimits.maxMembers, id: \.self) { index in
                tile(index: index)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(TeamLabels.slotsGroup)
        .accessibilityIdentifier("teamSlotGrid")
    }

    @ViewBuilder
    private func tile(index: Int) -> some View {
        let number = index + 1
        if let member = viewModel.member(atSlot: index) {
            let species = viewModel.speciesSummary(forKey: member.speciesKey)
            let name = species?.nameJa ?? TeamLabels.slotTitle(number)
            Button {
                onSelect(index)
            } label: {
                FilledTile(
                    speciesKey: member.speciesKey, name: name, types: species?.types ?? [],
                    itemName: member.itemId == nil ? nil : viewModel.itemLabel(for: member.itemId))
            }
            .buttonStyle(TileButtonStyle())
            .accessibilityLabel(name)
            .accessibilityIdentifier("teamTile-\(number)")
        } else {
            Button {
                onSelect(index)
            } label: {
                EmptyTile(title: TeamLabels.slotAdd(number))
            }
            .buttonStyle(TileButtonStyle())
            .accessibilityLabel(TeamLabels.slotAdd(number))
            .accessibilityIdentifier("teamTile-\(number)")
        }
    }
}

/// 埋まった枠のカード。名前・持ち物は折り返して全文を出す(行数を制限しない)。
private struct FilledTile: View {
    let speciesKey: String
    let name: String
    let types: [PokeType]
    let itemName: String?

    var body: some View {
        VStack(spacing: SpacingToken.x1) {
            SpeciesImageView(speciesKey: speciesKey, name: name, types: types)
            Text(name)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(.center)
                .fixedSize(horizontal: false, vertical: true)
            HStack(spacing: SpacingToken.x1) {
                ForEach(types, id: \.self) { TypeBadgeView(type: $0) }
            }
            if let itemName {
                Text(itemName)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .multilineTextAlignment(.center)
                    .fixedSize(horizontal: false, vertical: true)
            }
        }
        .padding(SpacingToken.x2)
        .frame(maxWidth: .infinity, minHeight: CalcScreenMetrics.minimumTapSide)
        .popCard(typeID: types.first?.rawValue, cornerRadius: RadiusToken.input)
        .contentShape(Rectangle())
    }
}

/// 空きの枠: 破線の枠に「+ N体目を追加」。
private struct EmptyTile: View {
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

/// タイルの押下の見た目(縮みは操作時のみ。視差効果を減らすで 0 秒)。
private struct TileButtonStyle: ButtonStyle {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? MotionToken.pressScale : 1)
            .animation(
                .easeOut(duration: MotionToken.pressDuration(reduceMotion: reduceMotion)),
                value: configuration.isPressed)
    }
}
