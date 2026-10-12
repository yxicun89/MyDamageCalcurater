import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// SpeedPokemonSheet: 自分 / 相手のポケモンを選ぶシート(P6-24。ADR-0503 §5。I-speed-4c で両方に共用)。
// speed の一覧を `load()` で 1 回取ったものを、名前の部分一致(ひらがな・カタカナ不問)で絞って出す。
// 検索欄は `.searchable` ではなく通常の `TextField`(`.searchable` の識別子はタップ・入力できる欄に渡らないため)。

struct SpeedPokemonSheet: View {
    let viewModel: SpeedMatchupViewModel
    let target: SpeedPickerTarget
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            List {
                searchField
                    .listRowSeparator(.hidden)
                Button {
                    select(nil)
                    dismiss()
                } label: {
                    Text(SpeedLabels.unselected)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                        .multilineTextAlignment(.leading)
                }
                .accessibilityIdentifier("speedPokemonRowNone")
                if viewModel.filteredPokemon.isEmpty {
                    Text(emptyMessage)
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                        .multilineTextAlignment(.leading)
                        .accessibilityIdentifier("speedPokemonNoMatch")
                }
                ForEach(viewModel.filteredPokemon) { pokemon in
                    Button {
                        select(pokemon.pokemonId)
                        dismiss()
                    } label: {
                        HStack(spacing: SpacingToken.x2) {
                            SpeedEmblem(name: pokemon.nameJa, primaryType: pokemon.types.first ?? "", speciesKey: pokemon.pokemonId)
                            Text(pokemon.nameJa)
                                .font(TextStyleToken.body.font)
                                .foregroundStyle(ColorToken.textPrimary.color)
                                .multilineTextAlignment(.leading)
                        }
                    }
                    .accessibilityLabel(pokemon.nameJa)
                    .accessibilityIdentifier("speedPokemonRow-\(pokemon.pokemonId)")
                }
            }
            .listStyle(.plain)
            .navigationTitle(SpeedLabels.pokemon)
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button(SpeedLabels.close) { dismiss() }
                        .accessibilityIdentifier("speedPokemonSheetClose")
                }
            }
        }
        .accessibilityIdentifier("speedPokemonSheet")
    }

    private func select(_ id: String?) {
        switch target {
        case .own: viewModel.selectSelfPokemon(id: id)
        case .opponent: viewModel.selectOpponentPokemon(id: id)
        }
    }

    private var searchField: some View {
        TextField(
            SpeedLabels.pokemonSearchPrompt,
            text: Binding(get: { viewModel.pokemonQuery }, set: { viewModel.setPokemonQuery($0) })
        )
        .font(TextStyleToken.body.font)
        .autocorrectionDisabled()
        .textInputAutocapitalization(.never)
        .padding(SpacingToken.x3)
        .background(ColorToken.tableZebra.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
        .accessibilityIdentifier("speedPokemonSearchField")
    }

    /// 一覧が空のとき: 読み込めていないのか、絞り込みで 0 件なのか。
    private var emptyMessage: String {
        if case .loaded = viewModel.pokemonState { return SpeedLabels.pokemonNoMatch }
        if case .failed(let failure) = viewModel.pokemonState { return failure.message }
        return SpeedLabels.loading
    }
}
