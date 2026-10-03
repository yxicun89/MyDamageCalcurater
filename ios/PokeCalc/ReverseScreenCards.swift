import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// ReverseScreenCards: 逆算画面の自分・相手カード(P6-2b・docs/design.md「画面: 逆算」)。
// 計算画面のカード部品(`SpeciesHeaderMenuLabel` / `MenuLabelChip`)をそのまま再利用する
// (ADR-0501「実装メモ」の「`SpeciesHeaderMenuLabel` / `MenuLabelChip` を internal に広げた」)。

/// 自分のカード: 種族セレクタ(ヘッダーがそのまま Menu ラベル。タイプバッジ込み)+ 持ち物セレクタ。
struct ReverseMyCardView: View {
    let viewModel: ReverseViewModel
    @State private var isSpeciesSearchPresented = false

    private var species: SpeciesSummary? { viewModel.mySpecies }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            // issue #68: `Menu` ではなく検索シートで選ぶ(`CalcScreenCards` と同じ理由)。
            Button {
                isSpeciesSearchPresented = true
            } label: {
                SpeciesHeaderMenuLabel(species: species)
            }
            .accessibilityIdentifier("reverseMySpeciesPicker")
            .accessibilityLabel(species?.nameJa ?? SpeciesHeaderMenuLabel.placeholderName)
            .accessibilityHint("ポケモンを変える")
            .sheet(isPresented: $isSpeciesSearchPresented) {
                SpeciesSearchSheet(viewModel: viewModel) { option in
                    viewModel.scheduleLatest { await $0.selectMySpecies(key: option.key) }
                }
            }

            myItemMenu
            if let reason = lockReason(for: viewModel.myItemLock) {
                Text(reason)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("reverseMyItemLockReason")
            }
            // 持ち物の一覧が上限に達していても黙って切り捨てない(ADR-0501「issue #68 の残り」6章)。
            if viewModel.itemOptionsReachedLimit {
                Text(MasterSearchLabels.itemsTruncated)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier("reverseMyItemLimitHint")
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard()
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("reverseMyCard")
        // 受けたダメージでは自分の変更で詳細を読まないので、View が種族の変更ごとに読む(ADR-0509 §4 L2)。
        .task(id: viewModel.mySpeciesKey) { await viewModel.loadMySpeciesDetail() }
    }

    /// 自分の持ち物の選択(側ごとの役割の持ち物だけ。メガ種族ならメガストーンに固定して操作不可。ADR-0509 §8)。
    private var myItemMenu: some View {
        let lock = viewModel.myItemLock
        return Menu {
            Button(ItemDisplayName.noItemLabel) {
                viewModel.scheduleLatest { await $0.selectMyItem(id: nil) }
            }
            ForEach(viewModel.myItemOptions, id: \.id) { item in
                Button(item.nameJa) {
                    viewModel.scheduleLatest { await $0.selectMyItem(id: item.id) }
                }
            }
        } label: {
            MenuLabelChip(text: viewModel.itemLabel(for: viewModel.myItemId))
        }
        .disabled(lock.disablesItemField)
        .accessibilityHint(lockReason(for: lock) ?? "")
        .accessibilityIdentifier("reverseMyItemPicker")
    }
}

/// 相手のカード: 種族セレクタだけ(相手の SP・性格は逆算の対象なので入力しない)。
struct ReverseOpponentCardView: View {
    let viewModel: ReverseViewModel
    /// 種族シートの「よく使う相手」(P6-23。相手だけに渡す。自分には出さない)。
    var frequentOpponents: FrequentOpponentsViewModel?
    @State private var isSpeciesSearchPresented = false

    private var species: SpeciesSummary? { viewModel.opponentSpecies }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Button {
                isSpeciesSearchPresented = true
            } label: {
                SpeciesHeaderMenuLabel(species: species)
            }
            .accessibilityIdentifier("reverseOpponentSpeciesPicker")
            .accessibilityLabel(species?.nameJa ?? SpeciesHeaderMenuLabel.placeholderName)
            .accessibilityHint("ポケモンを変える")
            .sheet(isPresented: $isSpeciesSearchPresented) {
                SpeciesSearchSheet(viewModel: viewModel, frequentOpponents: frequentOpponents) { option in
                    viewModel.scheduleLatest { await $0.selectOpponentSpecies(key: option.key) }
                }
            }
            // メガ種族の相手は持ち物がメガストーンに固定される(候補の欄は無いので名前だけ見せる。ADR-0509 §8)。
            if case .locked(_, let displayName) = viewModel.opponentItemLock {
                Text(MegaItemText.fixedItemName(displayName))
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("reverseOpponentItemLock")
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard()
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("reverseOpponentCard")
    }
}
