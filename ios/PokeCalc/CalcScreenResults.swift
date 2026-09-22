import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// CalcScreenResults: 計算結果の行と、防御側の持ち物比較トグル(docs/design.md「画面: ダメージ計算」)。

/// 結果の行の一覧 + 持ち物の比較トグル。
struct ResultsSectionView: View {
    let viewModel: CalcViewModel
    /// ダメージバーの色(技のタイプ色。design.md「用途」)。
    let barColor: Color
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// 直前に表示した行ごとの確定数の段階(id → `KOTier`)。新しい行と比較し、段階が変わった行だけ
    /// バッジを弾ませる(design.md「動き」)。判定は一覧側でまとめて1回だけ行い、触覚もここで
    /// 1回だけ鳴らす(批評: 行ごとの `onChange` と `sensoryFeedback` の二重判定にしない)。
    @State private var previousTiers: [String: KOTier] = [:]
    /// 直前の反映で段階が変わった行の id(この行だけバッジを弾ませる)。
    @State private var changedRowIDs: Set<String> = []
    /// `.sensoryFeedback` のトリガ。値そのものに意味は無く、変わるたびに1回だけ触覚を鳴らす。
    @State private var hapticTick = 0

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            ForEach(viewModel.rows) { row in
                ResultRowView(display: row, barColor: barColor, tierChanged: changedRowIDs.contains(row.id))
            }
            if !viewModel.itemOptions.isEmpty {
                itemComparisonToggles
            }
        }
        .onChange(of: viewModel.rows) { _, newRows in
            updateChangedTiers(for: newRows)
        }
        .sensoryFeedback(.impact(weight: .light), trigger: hapticTick) { _, _ in !reduceMotion }
    }

    /// `KOTier.changed(from:to:)` と同じ規則(前回の表示が無い = 初回は弾ませない、段階が
    /// 変わったときだけ true)。ここでは前回・今回とも既に `KOTier` を持っているので、
    /// その等価比較がそのまま同じ判定になる。
    private func updateChangedTiers(for rows: [BulkRowDisplay]) {
        var changed: Set<String> = []
        for row in rows {
            if let previous = previousTiers[row.id], previous != row.koTier {
                changed.insert(row.id)
            }
        }
        changedRowIDs = changed
        previousTiers = Dictionary(uniqueKeysWithValues: rows.map { ($0.id, $0.koTier) })
        if !changed.isEmpty {
            hapticTick += 1
        }
    }

    private var itemComparisonToggles: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text("持ち物で比較")
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
            // チップが折り返せるよう横スクロールにする(design.md「+持ち物のトグル」)。
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: SpacingToken.x2) {
                    ForEach(viewModel.itemOptions, id: \.id) { item in
                        ChipButton(
                            title: item.nameJa,
                            isSelected: viewModel.comparedDefenderItemIds.contains(item.id),
                            identifier: "defenderItemToggle-\(item.id)"
                        ) {
                            Task { await viewModel.toggleDefenderItemComparison(itemId: item.id) }
                        }
                    }
                }
            }
        }
    }
}

/// 結果1行: 1段目に調整名(+持ち物名)と%幅、2段目にカード幅いっぱいのダメージバー、
/// 3段目に右寄せの確定数バッジ(批評 M3b: 全行でバーの物差しをそろえる)。
struct ResultRowView: View {
    let display: BulkRowDisplay
    let barColor: Color
    /// `ResultsSectionView` が `KOTier` の変化から判定した「このタイミングで弾ませるか」。
    let tierChanged: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var badgeScale: CGFloat = 1

    private static let bounceScale: CGFloat = 1.18
    private static let bounceAnimation = Animation.spring(response: 0.28, dampingFraction: 0.45)
    private static let settleAnimation = Animation.spring(response: 0.3, dampingFraction: 0.75)
    private static let settleDelay: Double = 0.12

    private var subtitle: String {
        display.itemId == nil ? display.presetLabel : "\(display.presetLabel)(\(display.itemLabel))"
    }

    private var subtitleText: some View {
        Text(subtitle)
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .lineLimit(1)
            .minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)
    }

    private var percentRangeTextView: some View {
        Text(display.percentRangeText)
            .font(TextStyleToken.resultPercent.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .lineLimit(1)
            .fixedSize()
            .accessibilityIdentifier("calcResultPercent-\(display.id)")
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            // 大きい文字サイズだと横に並べる余白が無くなるので、収まらなければ縦積みに落ちる
            // (批評「任意」対応)。
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
                    subtitleText
                    Spacer(minLength: SpacingToken.x2)
                    percentRangeTextView
                }
                VStack(alignment: .leading, spacing: SpacingToken.x1) {
                    subtitleText
                    percentRangeTextView
                }
            }
            DamageBarView(minFraction: display.barMinFraction, maxFraction: display.barMaxFraction, color: barColor)
                .frame(maxWidth: .infinity)
            HStack {
                Spacer(minLength: 0)
                Text(display.koText)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .lineLimit(1)
                    .fixedSize()
                    .padding(.horizontal, SpacingToken.x2)
                    .padding(.vertical, SpacingToken.x1)
                    .background(ColorToken.bgGlass.color, in: Capsule())
                    .scaleEffect(badgeScale)
                    .accessibilityIdentifier("calcResultKO-\(display.id)")
            }
        }
        .padding(SpacingToken.x3)
        .glassCard(cornerRadius: RadiusToken.input)
        .accessibilityIdentifier("calcResultRow-\(display.id)")
        .onChange(of: tierChanged) { _, changed in
            guard changed, !reduceMotion else { return }
            withAnimation(Self.bounceAnimation) { badgeScale = Self.bounceScale }
            withAnimation(Self.settleAnimation.delay(Self.settleDelay)) { badgeScale = 1 }
        }
    }
}

/// ダメージバー: min〜max の幅を帯で示す。色は技のタイプ色。spring で動く(design.md「動き」)。
struct DamageBarView: View {
    let minFraction: Double
    let maxFraction: Double
    let color: Color
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    private static let height: CGFloat = 10
    private static let motion = Animation.spring(response: 0.4, dampingFraction: 0.8)
    /// % が小さいと帯が消えて見えなくなるため、常に視認できる最小幅を確保する(design.md
    /// 「min〜maxの幅が分かる」)。
    private static let minimumVisibleWidth: CGFloat = 6

    var body: some View {
        GeometryReader { proxy in
            let width = proxy.size.width
            let fillWidth = max(Self.minimumVisibleWidth, CGFloat(maxFraction - minFraction) * width)
            ZStack(alignment: .leading) {
                Capsule()
                    .fill(ColorToken.bgGlass.color)
                    .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
                Capsule()
                    .fill(color)
                    .frame(width: fillWidth)
                    .offset(x: min(CGFloat(minFraction) * width, width - fillWidth))
            }
        }
        .frame(height: Self.height)
        .animation(reduceMotion ? nil : Self.motion, value: minFraction)
        .animation(reduceMotion ? nil : Self.motion, value: maxFraction)
    }
}

/// トグル/選択チップ。design.md「背景は無彩色。色を持つのはタイプだけ」。
struct ChipButton: View {
    let title: String
    let isSelected: Bool
    let identifier: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            Text(title)
                .font(TextStyleToken.body.font)
                // チップは横スクロールできる行の中にあるので、幅を詰められて1字ずつ縦に
                // 折り返ることが無いよう、常に自然な1行の幅で描く。
                .lineLimit(1)
                .fixedSize()
                .foregroundStyle(isSelected ? ColorToken.bgBase.color : ColorToken.textPrimary.color)
                .padding(.horizontal, SpacingToken.x3)
                .padding(.vertical, SpacingToken.x2)
                .background(
                    Capsule().fill(isSelected ? ColorToken.textPrimary.color : ColorToken.bgGlass.color)
                )
                .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier(identifier)
        .accessibilityAddTraits(isSelected ? .isSelected : [])
    }
}

/// エラー表示。design.md「danger」トークン。
struct ErrorBannerView: View {
    let message: String

    var body: some View {
        Text(message)
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.danger.color)
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(SpacingToken.x3)
            .background(ColorToken.bgGlass.color, in: RoundedRectangle(cornerRadius: RadiusToken.input, style: .continuous))
            .accessibilityIdentifier("calcErrorMessage")
    }
}
