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
    @State private var isSpeciesSearchPresented = false

    private var species: SpeciesSummary? { viewModel.attackerSpecies }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            // issue #68: 種族の一覧が数百件になりうるため、`Menu` ではなく検索シートで選ぶ
            // (ADR-0501「issue #68」1章「判断」)。入口の identifier は変えない。
            Button {
                isSpeciesSearchPresented = true
            } label: {
                speciesCardLabel(species: species, itemMark: viewModel.attackerItemMark)
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("attackerSpeciesPicker")
            // 子の `accessibilityIdentifier` は外から見えない(検索シートでも同じ制約を踏まえて
            // 別 identifier を振っている)。XCUITest が種族名の入れ替わりを検査できるよう、
            // ボタン自体のラベルを種族名にする(批評 M3d 対応の副作用)。
            .accessibilityLabel(species?.nameJa ?? SpeciesHeaderMenuLabel.placeholderName)
            .accessibilityHint("ポケモンを変える")
            .sheet(isPresented: $isSpeciesSearchPresented) {
                SpeciesSearchSheet(viewModel: viewModel) { option in
                    viewModel.scheduleLatest { await $0.selectAttacker(speciesKey: option.key) }
                }
            }

            attackerItemMenu
            attackerAbilityMenu
            if let reason = lockReason(for: viewModel.attackerItemLock) {
                Text(reason)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("attackerItemLockReason")
            }
            // 持ち物の一覧が上限に達していても黙って切り捨てない(ADR-0501「issue #68 の残り」6章)。
            if viewModel.itemOptionsReachedLimit {
                PopNoticeView(kind: .info, message: MasterSearchLabels.itemsTruncated, identifier: "attackerItemLimitHint")
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard(typeID: species?.types.first?.rawValue)
        // `.contain`: カード自体を1つの要素として見つけられるようにしつつ、中の名前・セレクタは
        // 個別の要素のまま残す(XCUITest が種族名の入れ替わりを検査できるように)。
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("attackerCard")
    }

    /// 持ち物の選択(攻撃側の役割の持ち物だけ。メガ種族ならメガストーンに固定して操作不可。ADR-0509 §8)。
    private var attackerItemMenu: some View {
        let lock = viewModel.attackerItemLock
        return Menu {
            Button(ItemDisplayName.noItemLabel) {
                viewModel.scheduleLatest { await $0.selectAttackerItem(id: nil) }
            }
            ForEach(viewModel.attackerItemOptions, id: \.id) { item in
                Button(item.nameJa) {
                    viewModel.scheduleLatest { await $0.selectAttackerItem(id: item.id) }
                }
            }
        } label: {
            MenuLabelChip(text: viewModel.itemLabel(for: viewModel.attackerItemId))
        }
        .disabled(lock.disablesItemField)
        .accessibilityHint(lockReason(for: lock) ?? "")
        .accessibilityIdentifier("attackerItemPicker")
    }
}

/// 特性の選択(「指定なし」+ 種族の特性。選んだ特性の名前が読み上げの値)。識別子は従来の `calcAttackerAbilityPicker`。
extension AttackerCardView {
    fileprivate var attackerAbilityMenu: some View {
        let selectedName = viewModel.attackerAbilityOptions
            .first(where: { $0.id == viewModel.attackerAbilityId })?.nameJa
            ?? CalcConditionLabels.abilityUnspecified
        return VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(CalcConditionLabels.abilityTitle)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
            Menu {
                Button(CalcConditionLabels.abilityUnspecified) {
                    viewModel.scheduleLatest { await $0.selectAttackerAbility(id: nil) }
                }
                ForEach(viewModel.attackerAbilityOptions, id: \.id) { ability in
                    Button(ability.nameJa) {
                        viewModel.scheduleLatest { await $0.selectAttackerAbility(id: ability.id) }
                    }
                }
            } label: {
                MenuLabelChip(text: selectedName)
            }
            .accessibilityValue(selectedName)
            .accessibilityIdentifier("calcAttackerAbilityPicker")
        }
    }
}

/// ポケモンのカードの顔(種族がある間は `PopPokemonCard`、無い間は「-」のチップ)。攻撃側・防御側で共有する。
@ViewBuilder
private func speciesCardLabel(species: SpeciesSummary?, itemMark: String?) -> some View {
    if let species {
        PopPokemonCard(speciesKey: species.key, name: species.nameJa, types: species.types, itemName: itemMark)
    } else {
        MenuLabelChip(text: SpeciesHeaderMenuLabel.placeholderName)
    }
}

/// メガ固定の理由の文(固定していなければ nil。ADR-0509 §7)。全画面で共有する。
func lockReason(for lock: MegaItemLock) -> String? {
    switch lock {
    case .none: return nil
    case .locked: return MegaItemText.lockedReason
    case .missing: return MegaItemText.missingReason
    }
}

/// 防御側カード: 攻撃側と違い、SP を入力しない(努力値は代表調整として行に出す。design.md)ので
/// 持ち物・プリセットは持たない。
struct DefenderCardView: View {
    let viewModel: CalcViewModel
    /// 種族シートの「よく使う相手」(P6-23。防御側だけに渡す)。
    var frequentOpponents: FrequentOpponentsViewModel?
    @State private var isSpeciesSearchPresented = false

    private var species: SpeciesSummary? { viewModel.defenderSpecies }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            Button {
                isSpeciesSearchPresented = true
            } label: {
                speciesCardLabel(species: species, itemMark: viewModel.defenderItemMark)
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("defenderSpeciesPicker")
            .accessibilityLabel(species?.nameJa ?? SpeciesHeaderMenuLabel.placeholderName)
            .accessibilityHint("ポケモンを変える")
            .sheet(isPresented: $isSpeciesSearchPresented) {
                SpeciesSearchSheet(viewModel: viewModel, frequentOpponents: frequentOpponents) { option in
                    viewModel.scheduleLatest { await $0.selectDefender(speciesKey: option.key) }
                }
            }
            defenderAbilityMenu
            // メガ種族の防御側は持ち物がメガストーンに固定される(持ち物欄は無いので名前だけ見せる。ADR-0509 §8)。
            if case .locked(_, let displayName) = viewModel.defenderItemLock {
                Text(MegaItemText.fixedItemName(displayName))
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("defenderItemLock")
            }
        }
        // 防御側の詳細(メガか)を種族が変わるたびに読む。固定が変わったときだけ計算し直す(ADR-0509 §4 L2)。
        .task(id: viewModel.defenderSpeciesKey) { await viewModel.loadDefenderAbilityOptions() }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard(typeID: species?.types.first?.rawValue)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("defenderCard")
    }
}

/// 防御側の特性の選択(「指定なし」+ 防御側の特性。識別子は従来の `calcDefenderAbilityPicker`)。
extension DefenderCardView {
    fileprivate var defenderAbilityMenu: some View {
        let selectedName = viewModel.defenderAbilityOptions
            .first(where: { $0.id == viewModel.defenderAbilityId })?.nameJa
            ?? AbilityPickerLabels.unspecified
        return VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(AbilityPickerLabels.defenderTitle)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
            Menu {
                Button(AbilityPickerLabels.unspecified) {
                    viewModel.scheduleLatest { await $0.selectDefenderAbility(id: nil) }
                }
                ForEach(viewModel.defenderAbilityOptions, id: \.id) { ability in
                    Button(ability.nameJa) {
                        viewModel.scheduleLatest { await $0.selectDefenderAbility(id: ability.id) }
                    }
                }
            } label: {
                MenuLabelChip(text: selectedName)
            }
            .accessibilityValue(selectedName)
            .accessibilityIdentifier("calcDefenderAbilityPicker")
        }
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

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            HStack(spacing: SpacingToken.x2) {
                SpeciesImageView(
                    speciesKey: species?.key, name: species?.nameJa ?? Self.placeholderName, types: species?.types ?? [])
                Spacer(minLength: 0)
                PopIcon(PopSymbol.picker)
                    .foregroundStyle(ColorToken.textSecondary.color)
            }
            Text(species?.nameJa ?? Self.placeholderName)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                // 名前だけの行でカード幅いっぱいを使い、折り返して全文を出す。
                // Menu のラベル内では複数行の既定が中央寄せになるので、2行目も左端にそろえる。
                .multilineTextAlignment(.leading)
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
    static let diameter: CGFloat = 40

    private var gradientColors: [Color] {
        let resolved = types.compactMap { TypeColorToken.color(forTypeID: $0.rawValue) }
        return resolved.isEmpty ? [ColorToken.textSecondary.color] : resolved
    }

    /// 頭文字の色。2タイプなら先頭タイプの ink(グラデーションの始点側に載るため)。タイプ無し・未知は従来の白。
    private var inkColor: Color {
        types.first.flatMap { TypeColorToken.inkColor(forTypeID: $0.rawValue) } ?? ColorToken.onPrimary.color
    }

    var body: some View {
        Circle()
            .fill(LinearGradient(colors: gradientColors, startPoint: .topLeading, endPoint: .bottomTrailing))
            .frame(width: Self.diameter, height: Self.diameter)
            .overlay(
                Text(name.prefix(1))
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(inkColor)
            )
            // 名前・タイプは隣のテキストが読み上げるので、エンブレムは装飾として隠す。
            .accessibilityHidden(true)
    }
}

/// タイプバッジ。design.md「用途: タイプバッジ、カードのふちの光、ダメージバー、効果抜群表示」。
/// 批評 M3a: 幅を詰められて1字ずつ縦に折り返らないよう、自然な幅で描く(行数の上限は付けない)。
struct TypeBadgeView: View {
    let type: PokeType

    var body: some View {
        Text(PokeTypeLabel.japaneseName(for: type))
            .font(TextStyleToken.caption.font)
            .foregroundStyle(TypeColorToken.inkColor(forTypeID: type.rawValue) ?? ColorToken.onPrimary.color)
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
                .multilineTextAlignment(.leading)
                .fixedSize(horizontal: false, vertical: true)
            PopIcon(PopSymbol.picker)
                .foregroundStyle(ColorToken.textSecondary.color)
        }
        .padding(.horizontal, SpacingToken.x3)
        .padding(.vertical, SpacingToken.x2)
        .frame(minHeight: CalcScreenMetrics.iconButtonSide)
        .background(ColorToken.tableZebra.color, in: Capsule())
    }
}
