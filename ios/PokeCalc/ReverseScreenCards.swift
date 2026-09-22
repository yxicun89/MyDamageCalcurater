import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// ReverseScreenCards: 逆算画面の自分・相手カード(P6-2b・docs/design.md「画面: 逆算」)。
// 計算画面のカード部品(`SpeciesHeaderMenuLabel` / `MenuLabelChip`)をそのまま再利用する
// (ADR-0501「実装メモ」の「`SpeciesHeaderMenuLabel` / `MenuLabelChip` を internal に広げた」)。

/// 自分のカード: 種族セレクタ(ヘッダーがそのまま Menu ラベル。タイプバッジ込み)+ 持ち物セレクタ。
struct ReverseMyCardView: View {
    let viewModel: ReverseViewModel

    private var species: SpeciesSummary? {
        viewModel.speciesOptions.first(where: { $0.key == viewModel.mySpeciesKey })
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Menu {
                ForEach(viewModel.speciesOptions, id: \.key) { option in
                    Button(option.nameJa) {
                        Task { await viewModel.selectMySpecies(key: option.key) }
                    }
                }
            } label: {
                SpeciesHeaderMenuLabel(species: species)
            }
            .accessibilityIdentifier("reverseMySpeciesPicker")
            .accessibilityLabel(species?.nameJa ?? SpeciesHeaderMenuLabel.placeholderName)
            .accessibilityHint("ポケモンを変える")

            Menu {
                Button(BulkRowDisplay.itemLabel(itemId: nil, items: viewModel.itemOptions)) {
                    Task { await viewModel.selectMyItem(id: nil) }
                }
                ForEach(viewModel.itemOptions, id: \.id) { item in
                    Button(item.nameJa) {
                        Task { await viewModel.selectMyItem(id: item.id) }
                    }
                }
            } label: {
                MenuLabelChip(text: BulkRowDisplay.itemLabel(itemId: viewModel.myItemId, items: viewModel.itemOptions))
            }
            .accessibilityIdentifier("reverseMyItemPicker")
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard()
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("reverseMyCard")
    }
}

/// 相手のカード: 種族セレクタだけ(相手の SP・性格は逆算の対象なので入力しない)。
struct ReverseOpponentCardView: View {
    let viewModel: ReverseViewModel

    private var species: SpeciesSummary? {
        viewModel.speciesOptions.first(where: { $0.key == viewModel.opponentSpeciesKey })
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Menu {
                ForEach(viewModel.speciesOptions, id: \.key) { option in
                    Button(option.nameJa) {
                        Task { await viewModel.selectOpponentSpecies(key: option.key) }
                    }
                }
            } label: {
                SpeciesHeaderMenuLabel(species: species)
            }
            .accessibilityIdentifier("reverseOpponentSpeciesPicker")
            .accessibilityLabel(species?.nameJa ?? SpeciesHeaderMenuLabel.placeholderName)
            .accessibilityHint("ポケモンを変える")
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard()
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("reverseOpponentCard")
    }
}
