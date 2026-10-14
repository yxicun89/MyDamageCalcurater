import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// SpeedScreenView: 素早さ画面(相手を選んで上を取る最低ライン。I-speed-4c。ADR-0610 追記)。
//
// ロジックは持たない。`SpeedMatchupViewModel`(PokeCalcCore)の状態を描き、操作をメソッドへつなぐだけ(ADR-0500 §1)。
// 縦の 1 本のスクロール: 入力(自分 / 相手 / 場)→ 出力 1(想定ごとのカード)→ 出力 2(「これ以上」)→ 連続技へのリンク。
// 表と周辺パネルは出さない。固定高にしない(長い文言は折り返す。AX5 でも横にはみ出さない)。
// 色だけに頼らない(バッジ・行は文字でも区別する)。常時動くアニメーションは入れない。

struct SpeedScreenView: View {
    @State private var viewModel: SpeedMatchupViewModel
    @State private var pickerTarget: SpeedPickerTarget?
    /// 連続技(計算画面)へ渡す先。nil の間は連続技へのリンクを出さない(連続技 CB-8 が入るまで)。
    private let onOpenCombo: ((SpeedComboHandoff) -> Void)?

    /// 画面の持ち方(タブ・区切りは `.retained`。ADR-0531 §2)。離れたときに送信を取り消すかを決める。
    private let lifetime: ScreenLifetime

    init(
        service: any SpeedService, lifetime: ScreenLifetime = .retained,
        onOpenCombo: ((SpeedComboHandoff) -> Void)? = nil
    ) {
        self.lifetime = lifetime
        _viewModel = State(
            initialValue: SpeedMatchupViewModel(service: service, comboAvailable: onOpenCombo != nil))
        self.onOpenCombo = onOpenCombo
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: SpacingToken.x4) {
                Text(SpeedLabels.screenTitle)
                    .font(TextStyleToken.heading.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                SpeedSelfSection(viewModel: viewModel, pickerTarget: $pickerTarget)
                SpeedOpponentSection(viewModel: viewModel, pickerTarget: $pickerTarget)
                SpeedFieldSection(viewModel: viewModel)
                SpeedMatchupSection(viewModel: viewModel, onOpenCombo: onOpenCombo)
                SpeedReachSection(viewModel: viewModel)
            }
            .padding(SpacingToken.x4)
        }
        .popScreenBackground()
        .accessibilityIdentifier("speedScreen")
        .task { await viewModel.load() }
        // 戻ると破棄される画面(積んだ画面)だけ、離れたら入力・送信を止める(issue #113 A6・ADR-0531 §2)。
        .cancelsPendingWorkOnDisappear(lifetime) { viewModel.cancelPendingWork() }
        .sheet(item: $pickerTarget) { target in
            SpeedPokemonSheet(viewModel: viewModel, target: target)
        }
    }
}

/// ポケモンのシートの対象(自分 / 相手)。
enum SpeedPickerTarget: String, Identifiable {
    case own
    case opponent

    var id: String { rawValue }
}

/// ポケモンを選ぶボタン(選んだ名前とエンブレム。タップでシートを開く)。
private struct SpeedPokemonButton: View {
    let title: String
    let pokemon: SpeedPokemon?
    let identifier: String
    let action: () -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            SpeedCaption(text: title)
            Button(action: action) {
                HStack(spacing: SpacingToken.x2) {
                    if let pokemon {
                        SpeedEmblem(name: pokemon.nameJa, primaryType: pokemon.types.first ?? "", speciesKey: pokemon.pokemonId)
                    }
                    Text(pokemon?.nameJa ?? SpeedLabels.unselected)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .multilineTextAlignment(.leading)
                    Image(systemName: "chevron.right")
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                }
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .background(ColorToken.tableZebra.color, in: Capsule())
                .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier(identifier)
        }
    }
}

/// 「自分のポケモン」: ポケモン・ランク・スカーフ/追い風/まひ・今の振り(任意)。
private struct SpeedSelfSection: View {
    let viewModel: SpeedMatchupViewModel
    @Binding var pickerTarget: SpeedPickerTarget?

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            PopHeading(title: SpeedLabels.selfRegion, systemImage: PopSymbol.speed)
            SpeedPokemonButton(
                title: SpeedLabels.pokemon, pokemon: viewModel.selfPokemon, identifier: "speedPokemonButton"
            ) { pickerTarget = .own }
            if case .failed(let failure) = viewModel.pokemonState {
                ErrorBannerView(message: failure.message, identifier: "speedPokemonError")
                if viewModel.canRetryLoad {
                    LoadRetryButton(identifier: "speedPokemonRetry") { await viewModel.retryLoad() }
                }
            }
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                SpeedCaption(text: SpeedLabels.rank)
                RankMeterView(
                    field: "speed", fieldName: SpeedLabels.rank,
                    statName: RankMeterLogic.statName(for: .spe), value: viewModel.rank
                ) { viewModel.setRank($0) }
            }
            SpeedFlowLayout {
                SpeedPill(title: SpeedLabels.scarf, isSelected: viewModel.scarf, identifier: "speedScarf") {
                    viewModel.setScarf(!viewModel.scarf)
                }
                SpeedPill(title: SpeedLabels.selfTailwind, isSelected: viewModel.tailwind, identifier: "speedSelfTailwind") {
                    viewModel.setTailwind(!viewModel.tailwind)
                }
                SpeedPill(title: SpeedMatchupLabels.selfParalysis, isSelected: viewModel.paralysis, identifier: "speedSelfParalysis") {
                    viewModel.setParalysis(!viewModel.paralysis)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            currentGroup
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard(cornerRadius: RadiusToken.card)
        .modifier(SpeedAccentBand(color: speedAccentColor(viewModel.selfAccentTypeID)))
        .accessibilityElement(children: .contain)
    }

    /// 今の振り(任意)。入れると行動順のバッジと「これ以上」が出る。
    private var currentGroup: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            SpeedPillGroup(title: SpeedMatchupLabels.currentGroup) {
                SpeedPill(
                    title: SpeedMatchupLabels.currentToggle, isSelected: viewModel.current != nil,
                    identifier: "speedCurrentToggle"
                ) { viewModel.setCurrentEnabled(viewModel.current == nil) }
            }
            SpeedHint(text: SpeedMatchupLabels.currentHint, identifier: "speedCurrentHint")
            if let current = viewModel.current {
                SpeedPillGroup(title: SpeedMatchupLabels.currentPresetGroup) {
                    ForEach(SpeedMinimalPreset.allCases, id: \.self) { preset in
                        SpeedPill(
                            title: SpeedLabels.preset(preset), isSelected: current == Self.investment(preset),
                            identifier: "speedCurrentPreset-\(preset.rawValue)"
                        ) { viewModel.applyCurrentPreset(preset) }
                    }
                }
                SpeedStepper(
                    title: SpeedLabels.sp, valueText: String(current.sp),
                    decrementLabel: SpeedLabels.spDecrement, incrementLabel: SpeedLabels.spIncrement,
                    canDecrement: current.sp > 0, canIncrement: current.sp < SPLimits.maxPerStat,
                    identifierPrefix: "speedSP",
                    onDecrement: { viewModel.setCurrentSP(current.sp - 1) },
                    onIncrement: { viewModel.setCurrentSP(current.sp + 1) })
                SpeedPillGroup(title: SpeedLabels.natureGroup) {
                    ForEach(SpeedNature.allCases, id: \.self) { nature in
                        SpeedPill(
                            title: SpeedLabels.nature(nature), isSelected: current.nature == nature,
                            identifier: "speedNature-\(nature.rawValue)"
                        ) { viewModel.setCurrentNature(nature) }
                    }
                }
            }
        }
    }

    /// かんたん設定が指す SP・性格(`SpeedMatchupViewModel.applyCurrentPreset` と同じ対応)。
    private static func investment(_ preset: SpeedMinimalPreset) -> SpeedInvestment {
        switch preset {
        case .uninvested: return SpeedInvestment(sp: 0, nature: .neutral)
        case .neutralMax: return SpeedInvestment(sp: SPLimits.maxPerStat, nature: .neutral)
        case .max: return SpeedInvestment(sp: SPLimits.maxPerStat, nature: .plus)
        }
    }
}

/// 「相手のポケモン」: ポケモン・想定のチップ・追い風/まひ。
private struct SpeedOpponentSection: View {
    let viewModel: SpeedMatchupViewModel
    @Binding var pickerTarget: SpeedPickerTarget?

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            PopHeading(title: SpeedMatchupLabels.opponentRegion, systemImage: PopSymbol.speed)
            SpeedPokemonButton(
                title: SpeedLabels.pokemon, pokemon: viewModel.opponentPokemon, identifier: "speedOpponentPokemonButton"
            ) { pickerTarget = .opponent }
            assumptionGroup
            SpeedFlowLayout {
                SpeedPill(title: SpeedMatchupLabels.opponentTailwind, isSelected: viewModel.opponentTailwind, identifier: "speedOpponentTailwind") {
                    viewModel.setOpponentTailwind(!viewModel.opponentTailwind)
                }
                SpeedPill(
                    title: SpeedMatchupLabels.opponentParalysis, isSelected: viewModel.opponentParalysis,
                    identifier: "speedOpponentParalysis"
                ) { viewModel.setOpponentParalysis(!viewModel.opponentParalysis) }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard(cornerRadius: RadiusToken.card)
        .modifier(SpeedAccentBand(color: speedAccentColor(viewModel.opponentAccentTypeID)))
        .accessibilityElement(children: .contain)
    }

    private var assumptionGroup: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            SpeedPillGroup(title: SpeedMatchupLabels.assumptionGroup) {
                ForEach(SpeedPresetID.allCases, id: \.self) { id in
                    SpeedPill(
                        title: SpeedLabels.preset(id), isSelected: viewModel.selectedPresets.contains(id),
                        identifier: "speedAssumption-\(id.rawValue)"
                    ) { viewModel.togglePreset(id) }
                }
            }
            SpeedHint(text: SpeedMatchupLabels.assumptionHint, identifier: "speedAssumptionHint")
            if viewModel.assumptionMinimumNoticeVisible {
                SpeedHint(text: SpeedMatchupLabels.assumptionMinimumNotice, identifier: "speedAssumptionMinimumNotice")
            }
        }
    }
}

/// 「場の状態」: トリックルーム。
private struct SpeedFieldSection: View {
    let viewModel: SpeedMatchupViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            SpeedPillGroup(title: SpeedLabels.fieldGroup) {
                SpeedPill(title: SpeedLabels.trickRoom, isSelected: viewModel.trickRoom, identifier: "speedTrickRoom") {
                    viewModel.setTrickRoom(!viewModel.trickRoom)
                }
            }
            SpeedHint(text: SpeedMatchupLabels.trickRoomHint, identifier: "speedTrickRoomHint")
        }
    }
}
