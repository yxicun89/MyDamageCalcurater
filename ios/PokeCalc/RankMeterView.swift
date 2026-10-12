import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// 能力ランク(-6...+6)の入力メーター(ADR-0533。Web の ADR-0357 と段数・色の意味・読み上げをそろえる)。
// 左 6 段 = 下降(warning)・中央 = 0(太い目印)・右 6 段 = 上昇(brandPrimary)。赤は弱点と紛れるので使わない。
// 値は +2 / -3 / ±0 の文字でも出し、色だけに頼らない。タップ・ドラッグで最寄りの段、− / + ボタン、
// VoiceOver では全体を 1 つの adjustable 要素にする(上下スワイプで増減)。常時アニメーションは無い。
// 段の値↔添字・ドラッグ位置→段・文言は `RankMeterLogic`(PokeCalcCore。XCTest 済み)。

struct RankMeterView: View {
    /// 欄の識別子の後ろ(`rankMeter-<field>` など。欄ごとに安定。例 `calcAttacker`)。
    let field: String
    /// 欄の名前(− / + の名前に使う。例「攻撃側のランク」)。
    let fieldName: String
    /// 読み上げ用のステータス名(「攻撃」)。VoiceOver の label。
    let statName: String
    /// 見える値の表記(「A +2」など)。省くと符号付きの数だけ。
    var valueLabel: String?
    let value: Int
    let onChange: (Int) -> Void

    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// ボタンとトラックのタップ領域の最小の一辺(受け入れ条件: 36pt 以上)。
    private static let minTapSide: CGFloat = 36
    private static let segmentSpacing: CGFloat = 2
    private static let segmentHeight: CGFloat = 20
    private static let zeroMarkerWidth: CGFloat = 4
    private static let trackMinWidth: CGFloat = 96
    private static let valueMinWidth: CGFloat = 36

    var body: some View {
        if dynamicTypeSize.isAccessibilitySize {
            // AX 文字サイズ: メーターを幅いっぱいに置き、− / 値 / + を下の行へ(横にはみ出さない)。
            VStack(alignment: .leading, spacing: SpacingToken.x2) {
                track
                HStack(spacing: SpacingToken.x2) {
                    stepButton(systemName: "minus", plus: false)
                    valueText
                    stepButton(systemName: "plus", plus: true)
                }
            }
        } else {
            HStack(spacing: SpacingToken.x2) {
                stepButton(systemName: "minus", plus: false)
                track
                stepButton(systemName: "plus", plus: true)
                valueText
            }
        }
    }

    private var current: Int { RankMeterLogic.clamp(value) }

    private var valueText: some View {
        Text(valueLabel ?? RankMeterLogic.signedText(current))
            .font(TextStyleToken.body.font)
            .foregroundStyle(ColorToken.textPrimary.color)
            .lineLimit(1)
            .fixedSize()
            .frame(minWidth: Self.valueMinWidth)
            // 読み上げは track の value(「+2」)が受け持つ。二重に読まない。
            .accessibilityHidden(true)
    }

    // MARK: トラック

    private var track: some View {
        GeometryReader { proxy in
            HStack(spacing: Self.segmentSpacing) {
                ForEach(RankMeterLogic.values, id: \.self) { step in
                    segment(step)
                }
            }
            .frame(width: proxy.size.width, height: proxy.size.height)
            .contentShape(Rectangle())
            .gesture(
                DragGesture(minimumDistance: 0)
                    .onChanged { gesture in select(atX: gesture.location.x, width: proxy.size.width) }
            )
        }
        .frame(minWidth: Self.trackMinWidth, maxWidth: .infinity)
        .frame(height: Self.minTapSide)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(statName)
        .accessibilityValue(RankMeterLogic.signedText(current))
        .accessibilityIdentifier(RankMeterLogic.identifier(field: field))
        .accessibilityAdjustableAction { direction in
            switch direction {
            case .increment: move(by: 1)
            case .decrement: move(by: -1)
            @unknown default: break
            }
        }
    }

    @ViewBuilder
    private func segment(_ step: Int) -> some View {
        let state = RankMeterLogic.segmentState(step: step, value: current)
        switch state {
        case .zero:
            // 中央の太い目印(常に見える)。
            Rectangle()
                .fill(ColorToken.textPrimary.color)
                .frame(width: Self.zeroMarkerWidth, height: Self.minTapSide - SpacingToken.x2)
                .frame(maxWidth: .infinity)
        case .up:
            filledSegment(ColorToken.brandPrimary.color)
        case .down:
            filledSegment(ColorToken.warning.color)
        case .off:
            Capsule()
                .strokeBorder(ColorToken.textSecondary.color, lineWidth: 1)
                .frame(height: Self.segmentHeight)
                .frame(maxWidth: .infinity)
        }
    }

    private func filledSegment(_ color: Color) -> some View {
        Capsule()
            .fill(color)
            .frame(height: Self.segmentHeight)
            .frame(maxWidth: .infinity)
    }

    // MARK: ボタン

    private func stepButton(systemName: String, plus: Bool) -> some View {
        let enabled = plus ? RankMeterLogic.canIncrement(current) : RankMeterLogic.canDecrement(current)
        return Button {
            move(by: plus ? 1 : -1)
        } label: {
            Image(systemName: systemName)
                .font(TextStyleToken.body.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .frame(minWidth: Self.minTapSide, minHeight: Self.minTapSide)
                .background(ColorToken.tableZebra.color, in: Circle())
                .contentShape(Circle())
        }
        .buttonStyle(.plain)
        // 行が狭くても 36pt を割らない(縮むのはトラックだけ)。
        .fixedSize()
        .disabled(!enabled)
        .opacity(enabled ? 1 : CalcScreenMetrics.disabledChipOpacity)
        .accessibilityLabel(plus ? RankMeterLogic.increaseLabel(fieldName: fieldName) : RankMeterLogic.decreaseLabel(fieldName: fieldName))
        .accessibilityIdentifier(plus ? RankMeterLogic.plusIdentifier(field: field) : RankMeterLogic.minusIdentifier(field: field))
    }

    // MARK: 操作

    private func select(atX x: CGFloat, width: CGFloat) {
        guard let next = RankMeterLogic.value(atX: Double(x), width: Double(width)) else { return }
        commit(next)
    }

    private func move(by delta: Int) { commit(RankMeterLogic.stepped(current, by: delta)) }

    private func commit(_ next: Int) {
        guard next != current else { return }
        // 操作時だけの短い反応。「視差効果を減らす」では動かさない(常時アニメーションは入れない)。
        withAnimation(reduceMotion ? nil : .easeOut(duration: MotionToken.pressDuration)) {
            onChange(next)
        }
    }
}
