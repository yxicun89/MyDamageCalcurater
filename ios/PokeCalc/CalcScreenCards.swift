import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// CalcScreenCards: 攻撃側・防御側カード(docs/design.md「画面: ダメージ計算」)。
// View はロジックを持たず、`CalcViewModel` の状態を描いて操作を async メソッドへつなぐだけ
// (ADR-0500 §1)。

/// 攻撃側カード: ヘッダー(エンブレム・名前・タイプ)が種族セレクタの Menu ラベルを兼ねる・持ち物セレクタ。
/// プリセットのチップはカードの外(`CalcScreenView` がカード行の直下に画面幅いっぱいで置く。批評 M3c)。
struct AttackerCardView: View {
    let viewModel: CalcViewModel

    private var species: SpeciesSummary? {
        viewModel.speciesOptions.first(where: { $0.key == viewModel.attackerSpeciesKey })
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Menu {
                ForEach(viewModel.speciesOptions, id: \.key) { option in
                    Button(option.nameJa) {
                        Task { await viewModel.selectAttacker(speciesKey: option.key) }
                    }
                }
            } label: {
                SpeciesHeaderMenuLabel(species: species)
            }
            .accessibilityIdentifier("attackerSpeciesPicker")
            // `Menu` はラベルの中身を個別の要素としてではなく、1つのボタンにまとめてしまう
            // (子の `accessibilityIdentifier` は外から見えない)。XCUITest が種族名の入れ替わりを
            // 検査できるよう、ボタン自体のラベルを種族名にする(批評 M3d 対応の副作用)。
            .accessibilityLabel(species?.nameJa ?? SpeciesHeaderMenuLabel.placeholderName)
            .accessibilityHint("ポケモンを変える")

            Menu {
                Button(BulkRowDisplay.itemLabel(itemId: nil, items: viewModel.itemOptions)) {
                    Task { await viewModel.selectAttackerItem(id: nil) }
                }
                ForEach(viewModel.itemOptions, id: \.id) { item in
                    Button(item.nameJa) {
                        Task { await viewModel.selectAttackerItem(id: item.id) }
                    }
                }
            } label: {
                MenuLabelChip(text: BulkRowDisplay.itemLabel(itemId: viewModel.attackerItemId, items: viewModel.itemOptions))
            }
            .accessibilityIdentifier("attackerItemPicker")
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard()
        // `.contain`: カード自体を1つの要素として見つけられるようにしつつ、中の名前・セレクタは
        // 個別の要素のまま残す(XCUITest が種族名の入れ替わりを検査できるように)。
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("attackerCard")
    }
}

/// 防御側カード: 攻撃側と違い、SP を入力しない(努力値は代表調整として行に出す。design.md)ので
/// 持ち物・プリセットは持たない。
struct DefenderCardView: View {
    let viewModel: CalcViewModel

    private var species: SpeciesSummary? {
        viewModel.speciesOptions.first(where: { $0.key == viewModel.defenderSpeciesKey })
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Menu {
                ForEach(viewModel.speciesOptions, id: \.key) { option in
                    Button(option.nameJa) {
                        Task { await viewModel.selectDefender(speciesKey: option.key) }
                    }
                }
            } label: {
                SpeciesHeaderMenuLabel(species: species)
            }
            .accessibilityIdentifier("defenderSpeciesPicker")
            .accessibilityLabel(species?.nameJa ?? SpeciesHeaderMenuLabel.placeholderName)
            .accessibilityHint("ポケモンを変える")
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard()
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("defenderCard")
    }
}

/// カードのヘッダーそのものを Menu のラベルにする(批評 M3d: 種族名の下に別行で
/// 「ポケモンを変える」を置くと縦に伸びて2行がちに見えるため、1つのタップ領域にまとめる)。
/// 3段: 1段目はエンブレムと末尾の chevron、2段目は名前だけをカード幅いっぱいに、
/// 3段目はタイプバッジ(批評 M3a・再レビュー対応: 名前を縮小(0.5)して1行に詰め込むと
/// 小さすぎたり(18 Pro で約9pt)、幅が足りず省略記号になったりした(17e・大きい文字)。
/// まず幅を確保してから、最後の手段として控えめに縮小する)。
struct SpeciesHeaderMenuLabel: View {
    let species: SpeciesSummary?
    /// 種族が (まだ) 無いときのプレースホルダ(読み込み中・0件時)。`Menu` の `accessibilityLabel`
    /// 側(呼び出し元)からも参照する。逆算画面(`ReverseScreenCards.swift`)も再利用するので internal にする。
    static let placeholderName = "-"
    /// 名前は2行まで折り返せるようにしたうえで、それでも収まらない極端なケースだけ控えめに
    /// 縮小する(最後の手段。design.md には数値指定が無いため実装側で決める)。
    private static let nameMinimumScaleFactor = 0.85

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            HStack(spacing: SpacingToken.x2) {
                SpeciesEmblemView(name: species?.nameJa ?? Self.placeholderName, types: species?.types ?? [])
                Spacer(minLength: 0)
                Image(systemName: "chevron.up.chevron.down")
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
            }
            Text(species?.nameJa ?? Self.placeholderName)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                // まず幅を確保する: 名前だけの行でカード幅いっぱいを使い、2行まで折り返せる。
                // 縮小は最後の手段(0.85)。
                .lineLimit(2)
                // Menu のラベル内では複数行の既定が中央寄せになるので、2行目も左端にそろえる。
                .multilineTextAlignment(.leading)
                .minimumScaleFactor(Self.nameMinimumScaleFactor)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityHidden(true)
            HStack(spacing: SpacingToken.x1) {
                ForEach(species?.types ?? [], id: \.self) { TypeBadgeView(type: $0) }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

/// 画像は必須にしない(CLAUDE.md ドメイン規約)。タイプ色のグラデーション + 名前の頭文字で成立させる。
struct SpeciesEmblemView: View {
    let name: String
    let types: [PokeType]
    /// 批評 M3a: ヘッダーが1行に収まるよう、名前・チェブロンと釣り合う小さめの直径にする
    /// (design.md に数値指定は無いため実装側で決める)。
    private static let diameter: CGFloat = 40

    private var gradientColors: [Color] {
        let resolved = types.compactMap { TypeColorToken.color(forTypeID: $0.rawValue) }
        return resolved.isEmpty ? [ColorToken.textSecondary.color] : resolved
    }

    var body: some View {
        Circle()
            .fill(LinearGradient(colors: gradientColors, startPoint: .topLeading, endPoint: .bottomTrailing))
            .frame(width: Self.diameter, height: Self.diameter)
            .overlay(
                Text(name.prefix(1))
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(.white)
            )
            // 名前・タイプは隣のテキストが読み上げるので、エンブレムは装飾として隠す。
            .accessibilityHidden(true)
    }
}

/// タイプバッジ。design.md「用途: タイプバッジ、カードのふちの光、ダメージバー、効果抜群表示」。
/// 批評 M3a: 幅を詰められて1字ずつ縦に折り返らないよう、常に1行の自然な幅で描く。
struct TypeBadgeView: View {
    let type: PokeType

    var body: some View {
        Text(PokeTypeLabel.japaneseName(for: type))
            .font(TextStyleToken.caption.font)
            .foregroundStyle(.white)
            .lineLimit(1)
            .fixedSize()
            .padding(.horizontal, SpacingToken.x2)
            .padding(.vertical, SpacingToken.x1)
            .background(
                TypeColorToken.color(forTypeID: type.rawValue) ?? ColorToken.textSecondary.color,
                in: Capsule()
            )
    }
}

/// セレクタ(Menu)のラベル共通見た目。背景は無彩色(色を持つのはタイプだけ、という方針)。
/// 攻撃側カードの持ち物セレクタで使う(種族セレクタは `SpeciesHeaderMenuLabel` がラベルを兼ねる)。
struct MenuLabelChip: View {
    let text: String

    var body: some View {
        HStack(spacing: SpacingToken.x1) {
            Text(text)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .lineLimit(1)
                .minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)
            Image(systemName: "chevron.up.chevron.down")
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
        }
        .padding(.horizontal, SpacingToken.x3)
        .padding(.vertical, SpacingToken.x2)
        .background(ColorToken.bgGlass.color, in: Capsule())
    }
}
