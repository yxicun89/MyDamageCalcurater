import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// PopPokemonCard: ポケモンのカード(画像かタイプ色エンブレム + 名前 + タイプバッジ + 持ち物の印。G-05 第 4 弾=調整。ADR-0540 §1)。
// 押す操作は呼び出し側のボタンが持つ(読み上げ名・識別子も呼び出し側)。名前・持ち物は折り返して全文を出す(lineLimit なし)。

struct PopPokemonCard: View {
    /// 大: 自分・相手・目標の相手の欄(横並び)。小: 「選んだ相手から」のタイル(縦並び)。
    enum Size { case regular, compact }

    let speciesKey: String
    let name: String
    let types: [PokeType]
    /// 持ち物の印(選んだ持ち物・メガストーン。無ければ nil)。
    let itemName: String?
    var size: Size = .regular

    var body: some View {
        Group {
            switch size {
            case .regular:
                HStack(spacing: SpacingToken.x3) {
                    SpeciesImageView(speciesKey: speciesKey, name: name, types: types)
                    VStack(alignment: .leading, spacing: SpacingToken.x1) {
                        nameText
                        badges
                        itemText
                    }
                    .frame(maxWidth: .infinity, alignment: .leading)
                }
            case .compact:
                VStack(spacing: SpacingToken.x1) {
                    SpeciesImageView(speciesKey: speciesKey, name: name, types: types)
                    nameText.multilineTextAlignment(.center)
                    itemText
                }
            }
        }
        .padding(SpacingToken.x2)
        .frame(maxWidth: size == .regular ? .infinity : nil, minHeight: CalcScreenMetrics.minimumTapSide, alignment: .leading)
        .popCard(typeID: types.first?.rawValue, cornerRadius: RadiusToken.input)
        .contentShape(Rectangle())
    }

    private var nameText: some View {
        Text(name)
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .multilineTextAlignment(.leading)
            .fixedSize(horizontal: false, vertical: true)
    }

    private var badges: some View {
        // 狭い幅(大きい文字)では縦に積む。
        ViewThatFits(in: .horizontal) {
            HStack(spacing: SpacingToken.x1) { ForEach(types, id: \.self) { TypeBadgeView(type: $0) } }
            VStack(alignment: .leading, spacing: SpacingToken.x1) { ForEach(types, id: \.self) { TypeBadgeView(type: $0) } }
        }
    }

    @ViewBuilder
    private var itemText: some View {
        if let itemName {
            Text(itemName)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .multilineTextAlignment(size == .compact ? .center : .leading)
                .fixedSize(horizontal: false, vertical: true)
        }
    }
}
