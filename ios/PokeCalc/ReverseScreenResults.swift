import PokeCalcCore
import PokeCalcDesign
import SwiftUI

// ReverseScreenResults: 逆算結果(前提・一致件数・候補カード。P6-2b・docs/design.md「画面: 逆算」)。
// 候補はサーバーの順のまま描く(並べ替え・グループ化しない。ADR-0010 §R4)。

/// 前提・一致件数・候補カードの一覧。
struct ReverseResultsSectionView: View {
    let result: ReverseResultDisplay?
    /// design.md「画面: 逆算」の「「観測を追加」で2回目以降を入力すると候補が絞られるアニメーション」。
    /// 2件目以降の観測がある状態で候補の並びが変わったときだけ動かす(操作への反応だけに絞る。design.md「動き」)。
    let observationCount: Int
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private static let narrowingAnimation = Animation.spring(response: 0.4, dampingFraction: 0.8)

    /// 一致する候補が 1 件も無い(各候補の SP の範囲に「参考」を添える)。
    private var isReference: Bool {
        guard let result else { return false }
        return ReverseLabels.hasNoExactCandidate(exactCount: result.exactCount, candidateCount: result.candidates.count)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            // 厳密に合う候補が 1 件も無いときは先頭に案内(判定は engine の `exactCount` のまま)。
            if let result,
                ReverseLabels.hasNoExactCandidate(exactCount: result.exactCount, candidateCount: result.candidates.count)
            {
                PopNoticeView(kind: .info, message: ReverseLabels.noExactNotice, identifier: "reverseNoExactNotice")
            }
            if let premiseText = result?.premiseText {
                Text(premiseText)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier("reversePremise")
            }
            if let result {
                Text(result.exactCountText)
                    .font(TextStyleToken.body.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .accessibilityIdentifier("reverseExactCount")
            }
            if let unsupportedNotice = result?.unsupportedNotice {
                Text(unsupportedNotice)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .accessibilityIdentifier("reverseUnsupportedNotice")
            }
            VStack(alignment: .leading, spacing: SpacingToken.x2) {
                ForEach(result?.candidates ?? []) { candidate in
                    ReverseCandidateCardView(candidate: candidate, isReference: isReference)
                }
            }
            // 値(候補の id 列)そのものが変わったときにアニメーションする(批評対応: 以前は別の
            // `@State` の「ティック」を `onChange` で1手遅れて進めていたため、SwiftUI が描画を終えた
            // *後* にアニメーションの発火条件が成立し、実際には動いていなかった)。
            // 観測欄が1つ(起動時・側の切り替え直後)のときと「視差効果を減らす」時は動かさない。
            .animation(reduceMotion || observationCount <= 1 ? nil : Self.narrowingAnimation, value: result?.candidates.map(\.id))
        }
    }
}

/// 候補1件のカード(Web の ADR-0346 と同じ順): 型の名前 + 目安の名前・「ほぼ合う候補」のバッジ、予測%の大きい数字、
/// 予測%の上限のバー(装飾)、SP の範囲(+「参考」)、持ち物、特性。
private struct ReverseCandidateCardView: View {
    let candidate: ReverseCandidateDisplay
    /// 一致する候補が 1 件も無いとき true(SP の範囲に「参考」を添える)。
    let isReference: Bool
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
            header
            ReverseCandidateBar(fraction: BulkRowDisplay.barFraction(percent: candidate.maxPercent))
                .accessibilityHidden(true)
            HStack(alignment: .firstTextBaseline, spacing: SpacingToken.x2) {
                Text(candidate.spRangeText)
                    .font(TextStyleToken.body.font.monospacedDigit())
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("reverseCandidateRange-\(candidate.id)")
                if isReference {
                    ReverseBadge(text: ReverseLabels.referenceRange, isWarning: false)
                }
            }
            Text(candidate.itemLabel)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("reverseCandidateItem-\(candidate.id)")
            if let abilityText = candidate.abilityText {
                Text(abilityText)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("reverseCandidateAbility-\(candidate.id)")
            }
            if let unsupportedNote = candidate.unsupportedNote {
                Text(unsupportedNote)
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityIdentifier("reverseCandidateUnsupported-\(candidate.id)")
            }
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .popCard(cornerRadius: RadiusToken.input)
        // `.contain`: `ResultRowView`(`CalcScreenResults.swift`)・`CalcConditionsSection.calcConditionsPanel`
        // と同じ理由(コンテナに飲まれて `reverseCandidateRange-*`/`reverseCandidateMatch-*` が
        // `reverseCandidateRow-*` に潰れるのを防ぐ)。
        .accessibilityElement(children: .contain)
        .accessibilityIdentifier("reverseCandidateRow-\(candidate.id)")
        .transition(.opacity.combined(with: .scale(scale: 0.96)))
    }

    /// 1 行目: 左に型の名前とバッジ、右に予測%(大きい数字)と「予測」。大きい文字では縦に積む。
    @ViewBuilder
    private var header: some View {
        if dynamicTypeSize >= .accessibility1 {
            VStack(alignment: .leading, spacing: SpacingToken.x2) {
                nameAndBadges
                percentBlock(alignment: .leading)
            }
        } else {
            HStack(alignment: .top, spacing: SpacingToken.x2) {
                nameAndBadges
                Spacer(minLength: SpacingToken.x2)
                percentBlock(alignment: .trailing)
            }
        }
    }

    private var nameAndBadges: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            Text(candidate.natureClassLabel)
                .font(TextStyleToken.heading.font)
                .foregroundStyle(ColorToken.textPrimary.color)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("reverseCandidateNature-\(candidate.id)")
            if !candidate.guideNames.isEmpty || !candidate.exact {
                SpeedFlowLayout(spacing: SpacingToken.x1) {
                    ForEach(candidate.guideNames, id: \.self) { ReverseBadge(text: $0, isWarning: false) }
                    // 一致しない候補だけ「ほぼ合う候補」(一致した候補には出さない)。文は読み上げの値として残す。
                    if !candidate.exact {
                        ReverseBadge(text: ReverseLabels.closeCandidate, isWarning: true)
                            .accessibilityValue(candidate.matchLabel)
                            .accessibilityIdentifier("reverseCandidateMatch-\(candidate.id)")
                    }
                }
            }
        }
    }

    private func percentBlock(alignment: HorizontalAlignment) -> some View {
        VStack(alignment: alignment, spacing: 0) {
            Text(candidate.percentRangeText)
                .font(TextStyleToken.heading.font.monospacedDigit())
                .foregroundStyle(ColorToken.textPrimary.color)
                .multilineTextAlignment(alignment == .leading ? .leading : .trailing)
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityIdentifier("reverseCandidatePercent-\(candidate.id)")
            Text(ReverseLabels.predictedPercent)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
        }
    }
}

/// 小さい角丸のバッジ(目安の名前・「ほぼ合う候補」・「参考」)。警告色は枠だけ(色だけで意味を伝えず、文字を併記する)。
private struct ReverseBadge: View {
    let text: String
    let isWarning: Bool

    var body: some View {
        Text(text)
            .font(TextStyleToken.caption.font)
            .foregroundStyle(isWarning ? ColorToken.warning.color : ColorToken.textSecondary.color)
            .fixedSize(horizontal: false, vertical: true)
            .padding(.horizontal, SpacingToken.x2)
            .padding(.vertical, SpacingToken.x1)
            .background(ColorToken.tableZebra.color, in: Capsule())
            .overlay(
                Capsule().stroke(
                    isWarning ? ColorToken.warning.color : ColorToken.borderHairline.color,
                    lineWidth: CalcScreenMetrics.hairlineBorderWidth))
    }
}

/// 予測%の上限のバー(装飾。幅 = 上限 / 100 を 0...1 に収めた値。動かさない)。
private struct ReverseCandidateBar: View {
    let fraction: Double
    private static let height: CGFloat = 8

    var body: some View {
        GeometryReader { proxy in
            ZStack(alignment: .leading) {
                Capsule().fill(ColorToken.tableZebra.color)
                    .overlay(Capsule().stroke(ColorToken.borderHairline.color, lineWidth: CalcScreenMetrics.hairlineBorderWidth))
                Capsule().fill(ColorToken.brandPrimary.color)
                    .frame(width: max(Self.height, CGFloat(fraction) * proxy.size.width))
            }
        }
        .frame(height: Self.height)
    }
}
