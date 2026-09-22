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

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x2) {
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
            VStack(alignment: .leading, spacing: SpacingToken.x2) {
                ForEach(result?.candidates ?? []) { candidate in
                    ReverseCandidateCardView(candidate: candidate)
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

/// 候補1件のカード: 性格クラス・持ち物、SP の範囲、目安の名前、一致の文言、想定ダメージ幅。
private struct ReverseCandidateCardView: View {
    let candidate: ReverseCandidateDisplay

    var body: some View {
        VStack(alignment: .leading, spacing: SpacingToken.x1) {
            HStack(alignment: .firstTextBaseline) {
                Text("\(candidate.natureClassLabel) / \(candidate.itemLabel)")
                    .font(TextStyleToken.heading.font)
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .lineLimit(2)
                    .minimumScaleFactor(CalcScreenMetrics.compactMinimumScaleFactor)
                Spacer(minLength: SpacingToken.x2)
                Text(candidate.percentRangeText)
                    .font(TextStyleToken.body.font.monospacedDigit())
                    .foregroundStyle(ColorToken.textPrimary.color)
                    .lineLimit(1)
                    .fixedSize()
            }
            Text(candidate.spRangeText)
                .font(TextStyleToken.body.font.monospacedDigit())
                .foregroundStyle(ColorToken.textPrimary.color)
                .accessibilityIdentifier("reverseCandidateRange-\(candidate.id)")
            if !candidate.guideNames.isEmpty {
                Text(candidate.guideNames.joined(separator: " / "))
                    .font(TextStyleToken.caption.font)
                    .foregroundStyle(ColorToken.textSecondary.color)
            }
            // danger はエラー表示用の色(design.md)。「一致なし」は入力エラーではないので、
            // exact の有無にかかわらず textSecondary のまま出す。
            Text(candidate.matchLabel)
                .font(TextStyleToken.caption.font)
                .foregroundStyle(ColorToken.textSecondary.color)
                .accessibilityIdentifier("reverseCandidateMatch-\(candidate.id)")
        }
        .padding(SpacingToken.x3)
        .frame(maxWidth: .infinity, alignment: .leading)
        .glassCard(cornerRadius: RadiusToken.input)
        .accessibilityIdentifier("reverseCandidateRow-\(candidate.id)")
        .transition(.opacity.combined(with: .scale(scale: 0.96)))
    }
}
