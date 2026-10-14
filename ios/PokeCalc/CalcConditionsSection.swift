import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// CalcConditionsSection: 計算画面の「詳細」(急所・やけど・天候・フィールド・防御側の壁・
// 攻撃側のランク。issue #274。ADR-0501「issue #274」6章・8章)。
//
// 既定は閉じる。ロジックは持たず、`CalcViewModel` の状態を描いて操作を async メソッドへ
// つなぐだけ(ADR-0500 §1)。開閉の折りたたみだけ View 側の `@State`(6章の identifier 表の備考)。

/// 「詳細」の折りたたみ。技セレクタの下・読み込み表示の上に置く(8章)。
struct CalcConditionsSection: View {
    let viewModel: CalcViewModel
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    /// 既定で閉じる(6章)。
    @State private var isExpanded = false
    /// 開いているときの開閉アイコンの回転(下向き → 上向き)。
    private static let openedIconRotation: Double = 180

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            toggleButton
            if isExpanded {
                conditionsPanel
            }
        }
    }

    private var toggleButton: some View {
        Button {
            // 開閉は操作したときだけ動かす(常時動くアニメーションは入れない。CLAUDE.md ドメイン規約)。
            withAnimation(reduceMotion ? nil : .easeInOut) {
                isExpanded.toggle()
            }
        } label: {
            HStack(spacing: SpacingToken.x2) {
                // 開いているときは向きを反転する(静止した表示。アイコンは装飾で、状態は読み上げの値が持つ)。
                PopIcon(PopSymbol.open)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .rotationEffect(.degrees(isExpanded ? Self.openedIconRotation : 0))
                Text(CalcConditionLabels.sectionTitle)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .fixedSize(horizontal: false, vertical: true)
                Spacer(minLength: 0)
            }
            .frame(minHeight: CalcScreenMetrics.minimumTapSide)
            .padding(SpacingToken.x3)
            .popCard(cornerRadius: RadiusToken.input)
        }
        .buttonStyle(.plain)
        .accessibilityValue(isExpanded ? CalcScreenText.disclosureOpened : CalcScreenText.disclosureClosed)
        .accessibilityIdentifier("calcConditionsToggle")
    }

    private var conditionsPanel: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            criticalAndBurnRow
            rankSection
            weatherSection
            terrainSection
            defenderScreensSection
            // 防御側の壁の次に置く(攻撃側のランクの前後に挟むと、前方スクロールしかしない既存 XCUITest
            // `CalcConditionsUITests` の壁→攻撃側ランクの到達が崩れるため。ADR-0501 末尾「実装結果」)。
            defenderRankSection
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard(cornerRadius: RadiusToken.input)
        // `.contain`: コンテナ自体を1つの要素として見つけられるようにしつつ、中の各入力は個別の要素の
        // ままにする(`AttackerCardView`/`DefenderCardView` と同じ理由。これが無いと、`popCard()` の
        // Liquid Glass コンテナの外にある子〈= 横スクロールの中でない子〉の `accessibilityIdentifier` が
        // このコンテナの id に飲まれる。XCUITest の要素ダンプで実際に踏んだ不具合)。
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("calcConditionsPanel")
    }

    /// 見出し(小見出し)+ 内容を1行にまとめる行(3章の各小見出しの節。批評: 見出しを内容の上に
    /// 別行で積むと6区画分の縦の高さが伸び、画面の下の方(天候より下)へ数値入力用に戻るスクロールが
    /// 効かない XCUITest の `scrollUntilHittable`〈前方スクロールしか行わない〉が届かなくなるため、
    /// 既定の文字サイズでは1行に収めて「詳細」パネル全体の高さを抑える)。
    ///
    /// アクセシビリティの文字サイズ(`dynamicTypeSize.isAccessibilitySize`)では、見出し
    /// (`.fixedSize()` で縮めない)と内容を並べると横に収まらないため、見出しを上に積む2行の
    /// レイアウトに切り替える(批評指摘)。既定サイズの挙動(1行・identifier)は変えないので、
    /// 既定サイズで動く `CalcConditionsUITests` の前提(前方スクロールしか行わない
    /// `scrollUntilHittable`)はそのまま保たれる。
    @ViewBuilder
    private func sectionRow<Content: View>(_ title: String, @ViewBuilder content: () -> Content) -> some View {
        if dynamicTypeSize.isAccessibilitySize {
            VStack(alignment: .leading, spacing: SpacingToken.x1) {
                sectionRowLabel(title)
                // 見出しだけを上の行に移し、内容(ランクの −/値/+ など)は横並びのまま保つ。
                HStack(alignment: .center, spacing: SpacingToken.x2) {
                    content()
                }
            }
        } else {
            HStack(alignment: .center, spacing: SpacingToken.x2) {
                sectionRowLabel(title).fixedSize()
                content()
            }
        }
    }

    private func sectionRowLabel(_ title: String) -> some View {
        Text(title)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(ColorToken.textSecondary.color)
            .fixedSize(horizontal: false, vertical: true)
    }

    /// 急所・攻撃側のやけど(横並びのトグル。3章の並び)。
    private var criticalAndBurnRow: some View {
        ComboWrapLayout(spacing: SpacingToken.x2) {
            ChipButton(
                title: CalcConditionLabels.critical,
                isSelected: viewModel.isCritical,
                identifier: "calcCondition-critical"
            ) {
                viewModel.scheduleLatest { await $0.setCritical(!$0.isCritical) }
            }
            ChipButton(
                title: CalcConditionLabels.burn,
                isSelected: viewModel.isAttackerBurned,
                identifier: "calcCondition-burn"
            ) {
                viewModel.scheduleLatest { await $0.setAttackerBurned(!$0.isAttackerBurned) }
            }
        }
    }

    /// 攻撃側のランク: メーターの対象(A/C)は `attackerRankStat` が決める(4章)。13 段のメーター(ADR-0533)。
    private var rankSection: some View {
        sectionRow(CalcConditionLabels.rankTitle) {
            RankMeterView(
                field: "calcAttacker", fieldName: CalcConditionLabels.rankTitle,
                statName: RankMeterLogic.statName(for: viewModel.attackerRankStat),
                valueLabel: viewModel.attackerRankText, value: viewModel.attackerRank
            ) { next in
                viewModel.scheduleLatest { await $0.setAttackerRank(next) }
            }
        }
    }

    /// 防御側のランク: メーターの対象(B/D)は `defenderRankStat` が決める(issue #274・ADR-0501・ADR-0533)。
    private var defenderRankSection: some View {
        sectionRow(CalcConditionLabels.defenderRankTitle) {
            RankMeterView(
                field: "calcDefender", fieldName: CalcConditionLabels.defenderRankTitle,
                statName: RankMeterLogic.statName(for: viewModel.defenderRankStat),
                valueLabel: viewModel.defenderRankText, value: viewModel.defenderRank
            ) { next in
                viewModel.scheduleLatest { await $0.setDefenderRank(next) }
            }
        }
    }

    /// 天候(1つ選ぶピル。既定 なし)。
    private var weatherSection: some View {
        sectionRow(CalcConditionLabels.weatherTitle) {
            ComboWrapLayout(spacing: SpacingToken.x2) {
                ForEach(Weather.allCases, id: \.self) { weather in
                    ChipButton(
                        title: WeatherLabel.japaneseName(for: weather),
                        isSelected: viewModel.weather == weather,
                        identifier: "calcWeather-\(weather.rawValue)"
                    ) {
                        viewModel.scheduleLatest { await $0.selectWeather(weather) }
                    }
                }
            }
        }
    }

    /// フィールド(1つ選ぶピル。既定 なし)。
    private var terrainSection: some View {
        sectionRow(CalcConditionLabels.terrainTitle) {
            ComboWrapLayout(spacing: SpacingToken.x2) {
                ForEach(Terrain.allCases, id: \.self) { terrain in
                    ChipButton(
                        title: TerrainLabel.japaneseName(for: terrain),
                        isSelected: viewModel.terrain == terrain,
                        identifier: "calcTerrain-\(terrain.rawValue)"
                    ) {
                        viewModel.scheduleLatest { await $0.selectTerrain(terrain) }
                    }
                }
            }
        }
    }

    /// 防御側の壁(3つ独立のトグル。重ねて張れる)。残り幅に収まらなければ折り返す。
    private var defenderScreensSection: some View {
        sectionRow(CalcConditionLabels.defenderScreensTitle) {
            ComboWrapLayout(spacing: SpacingToken.x2) {
                ForEach(ScreenKind.allCases, id: \.self) { kind in
                    let isOn = viewModel.defenderScreens.isOn(kind)
                    ChipButton(
                        title: ScreenKindLabel.japaneseName(for: kind),
                        isSelected: isOn,
                        identifier: "calcDefenderScreen-\(kind.rawValue)"
                    ) {
                        viewModel.scheduleLatest { await $0.setDefenderScreen(kind, isOn: !isOn) }
                    }
                }
            }
        }
    }
}
