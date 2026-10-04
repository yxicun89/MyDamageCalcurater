import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// SpeedScreenView: 素早さ比較画面(P6-24。ADR-0503 §7)。
//
// ロジックは持たない。`SpeedViewModel`(PokeCalcCore)の状態を描き、操作をメソッドへつなぐだけ(ADR-0500 §1)。
// 縦の 1 本のスクロール: 上から「自分のポケモン」(入力と結果)、「素早さの表」。
// 色を持つのはタイプのエンブレムだけ(design.md)。常時動くアニメーションは入れない。

struct SpeedScreenView: View {
    @State private var viewModel: SpeedViewModel
    @State private var isPokemonSheetPresented = false

    init(service: any SpeedService) {
        _viewModel = State(initialValue: SpeedViewModel(service: service))
    }

    var body: some View {
        ScrollViewReader { proxy in
            ScrollView {
                VStack(alignment: .leading, spacing: SpacingToken.x4) {
                    Text(SpeedLabels.screenTitle)
                        .font(TextStyleToken.heading.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                    SpeedSelfSection(viewModel: viewModel, isPokemonSheetPresented: $isPokemonSheetPresented)
                    SpeedTableSection(viewModel: viewModel, scrollToRow: { proxy.scrollTo($0, anchor: .center) })
                }
                .padding(SpacingToken.x4)
            }
        }
        .background(ColorToken.bgBase.color.ignoresSafeArea())
        .accessibilityIdentifier("speedScreen")
        .task { await viewModel.load() }
        .onDisappear { viewModel.cancelPendingWork() }
        .sheet(isPresented: $isPokemonSheetPresented) {
            SpeedPokemonSheet(viewModel: viewModel)
        }
    }
}

/// 「自分のポケモン」: 入力の方法・ポケモン・調整と、結果。
private struct SpeedSelfSection: View {
    let viewModel: SpeedViewModel
    @Binding var isPokemonSheetPresented: Bool

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            Text(SpeedLabels.selfRegion)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
            modePills
            pokemonRow
            switch viewModel.mode {
            case .preset: presetControls
            case .custom: customControls
            case .raw: rawControls
            }
            SpeedResultView(viewModel: viewModel)
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard(cornerRadius: RadiusToken.card)
        .accessibilityElement(children: .contain)
    }

    private var modePills: some View {
        SpeedPillGroup(title: SpeedLabels.modeGroup) {
            ForEach(SpeedInputMode.allCases, id: \.self) { mode in
                SpeedPill(
                    title: SpeedLabels.mode(mode), isSelected: viewModel.mode == mode,
                    identifier: "speedMode-\(mode.rawValue)"
                ) { viewModel.setMode(mode) }
            }
        }
    }

    private var pokemonRow: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            SpeedCaption(text: SpeedLabels.pokemon)
            Button {
                isPokemonSheetPresented = true
            } label: {
                HStack(spacing: SpacingToken.x2) {
                    if let selected = viewModel.selectedPokemon {
                        SpeedEmblem(name: selected.nameJa, primaryType: selected.types.first ?? "")
                    }
                    Text(viewModel.selectedPokemon?.nameJa ?? SpeedLabels.unselected)
                        .font(TextStyleToken.body.font)
                        .foregroundStyle(ColorToken.textPrimary.color)
                        .multilineTextAlignment(.leading)
                    Image(systemName: "chevron.right")
                        .font(TextStyleToken.caption.font)
                        .foregroundStyle(ColorToken.textSecondary.color)
                }
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .background(ColorToken.bgGlass.color, in: Capsule())
                .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("speedPokemonButton")
            if case .failed(let failure) = viewModel.pokemonState {
                ErrorBannerView(message: failure.message, identifier: "speedPokemonError")
            }
        }
    }

    private var presetControls: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            SpeedPillGroup(title: SpeedLabels.presetGroup) {
                ForEach(SpeedMinimalPreset.allCases, id: \.self) { preset in
                    SpeedPill(
                        title: SpeedLabels.preset(preset), isSelected: viewModel.preset == preset,
                        identifier: "speedPreset-\(preset.rawValue)"
                    ) { viewModel.setPreset(preset) }
                }
            }
            fieldFlags
        }
    }

    private var customControls: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x3) {
            SpeedStepper(
                title: SpeedLabels.sp, valueText: String(viewModel.sp),
                decrementLabel: SpeedLabels.spDecrement, incrementLabel: SpeedLabels.spIncrement,
                canDecrement: viewModel.sp > 0, canIncrement: viewModel.sp < SPLimits.maxPerStat,
                identifierPrefix: "speedSP",
                onDecrement: { viewModel.setSP(viewModel.sp - 1) },
                onIncrement: { viewModel.setSP(viewModel.sp + 1) })
            SpeedPillGroup(title: SpeedLabels.natureGroup) {
                ForEach(SpeedNature.allCases, id: \.self) { nature in
                    SpeedPill(
                        title: SpeedLabels.nature(nature), isSelected: viewModel.nature == nature,
                        identifier: "speedNature-\(nature.rawValue)"
                    ) { viewModel.setNature(nature) }
                }
            }
            SpeedStepper(
                title: SpeedLabels.rank, valueText: Self.rankText(viewModel.rank),
                decrementLabel: CalcConditionLabels.rankDecrement, incrementLabel: CalcConditionLabels.rankIncrement,
                canDecrement: viewModel.rank > RankLimits.min, canIncrement: viewModel.rank < RankLimits.max,
                identifierPrefix: "speedRank",
                onDecrement: { viewModel.setRank(viewModel.rank - 1) },
                onIncrement: { viewModel.setRank(viewModel.rank + 1) })
            fieldFlags
        }
    }

    private static func rankText(_ rank: Int) -> String { rank > 0 ? "+\(rank)" : String(rank) }

    /// スカーフ・追い風・まひ(preset と custom で共通。raw は補正済みの値を入れるので出さない)。
    private var fieldFlags: some View {
        SpeedFlowLayout {
            SpeedPill(title: SpeedLabels.scarf, isSelected: viewModel.scarf, identifier: "speedScarf") {
                viewModel.setScarf(!viewModel.scarf)
            }
            SpeedPill(title: SpeedLabels.selfTailwind, isSelected: viewModel.tailwind, identifier: "speedSelfTailwind") {
                viewModel.setTailwind(!viewModel.tailwind)
            }
            SpeedPill(title: SpeedLabels.paralysis, isSelected: viewModel.paralysis, identifier: "speedParalysis") {
                viewModel.setParalysis(!viewModel.paralysis)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var rawControls: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            SpeedCaption(text: SpeedLabels.rawValue)
            TextField(
                SpeedLabels.rawValue,
                text: Binding(get: { viewModel.rawValueText }, set: { viewModel.setRawValueText($0) })
            )
            .keyboardType(.numberPad)
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .padding(SpacingToken.x3)
            .background(ColorToken.bgGlass.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
            .accessibilityIdentifier("speedRawValueField")
            if let message = viewModel.rawValueError {
                Text(message)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.danger.color)
                    .multilineTextAlignment(.leading)
                    .accessibilityIdentifier("speedRawValueError")
            }
        }
    }
}

/// 自分の結果(実数値・速い/遅い行数・同速)。トリックルーム中は行動順の読み替えを足す。
private struct SpeedResultView: View {
    let viewModel: SpeedViewModel

    var body: some View {
        switch viewModel.positionState {
        case .idle:
            EmptyView()
        case .loading:
            Text(SpeedLabels.positionLoading)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .accessibilityIdentifier("speedPositionLoading")
        case .failed(let failure):
            ErrorBannerView(message: failure.message, identifier: "speedPositionError")
        case .loaded:
            if let display = viewModel.positionDisplay {
                resultBody(display)
            }
        }
    }

    private func resultBody(_ display: SpeedPositionDisplay) -> some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            if let name = display.pokemonName {
                line(name, style: .body, id: "speedResultPokemon")
            }
            line(display.speedLabel, style: .heading, id: "speedResultSpeed")
            line(display.fasterLabel, style: .body, id: "speedResultFaster")
            line(display.slowerLabel, style: .body, id: "speedResultSlower")
            if let before = display.movesBeforeLabel {
                line(before, style: .body, id: "speedResultMovesBefore")
            }
            if let after = display.movesAfterLabel {
                line(after, style: .body, id: "speedResultMovesAfter")
            }
            if display.hasTie {
                line(SpeedLabels.tie, style: .caption, id: "speedResultTie", secondary: true)
                ForEach(Array(display.tieEntries.enumerated()), id: \.offset) { index, entry in
                    line(entry.label, style: .body, id: "speedResultTieEntry-\(index)")
                }
            } else {
                line(SpeedLabels.noTie, style: .body, id: "speedResultNoTie")
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("speedResult")
    }

    private func line(_ text: String, style: TextStyleToken, id: String, secondary: Bool = false) -> some View {
        Text(text)
            .font(style.font)
            .foregroundStyle(secondary ? ColorToken.textSecondary.color : ColorToken.textPrimary.color)
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
            .accessibilityIdentifier(id)
    }
}
