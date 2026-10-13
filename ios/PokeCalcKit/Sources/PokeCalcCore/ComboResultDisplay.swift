// ComboResultDisplay: 連続技の結果の整形(AC-i7・AC-i9。ADR-0535 §8・§10)。API の値を書式にするだけで、計算し直さない。

/// 結果の1手。
public struct ComboStepDisplay: Equatable, Identifiable, Sendable {
    /// 0 始まりの位置(`comboResultStep-<位置>`)。
    public let id: Int
    public let ordinalText: String
    public let moveName: String
    public let chips: [ComboRankChip]
    /// 「40.2〜47.8%」。変化技は「ダメージなし」。
    public let damageText: String
    /// 「先に動く / 相手が先 / 同速」。決まらないときは nil。
    public let orderText: String?

    /// 読み上げ・UI テスト用の1文(技名・チップ・%・行動順を含める)。
    public var summaryText: String {
        ([ordinalText, moveName] + chips.map(\.text) + [damageText] + (orderText.map { [$0] } ?? [])).joined(separator: " ")
    }
}

/// 連続技の結果を画面向けに整形した値。
public struct ComboResultDisplay: Equatable, Sendable {
    public let steps: [ComboStepDisplay]
    public let totalText: String
    /// 合計の最大%のバー(0...1。100% 超はいっぱい)。
    public let totalBarFraction: Double
    public let koText: String
    public let speedSummaryText: String
    /// 相手の素早さが分からないときの最後の手の線。分かるときは nil。
    public let speedLineText: String?
    /// 相手の素早さが分かるときだけ。
    public let opponentSpeedText: String?
    /// 素早さの基本値の例を出すか(相手の素早さが分からず、トリックルームでないとき)。
    public let showsSpeedExample: Bool
    /// 最後の手の開始時の自分の素早さ(例の問い合わせに使う)。
    public let lastSelfSpeed: Int
    public let surviveText: String?
    public let actionsText: String?
    public let perHitText: String?
    public let unsupportedNotice: String?

    /// `stepMoves[i]` は `result.steps[i]` の技(引けなければ nil で、名前は ID のまま出す)。
    public init(result: ComboResult, stepMoves: [Move?], names: UnsupportedMarkNames, trickRoom: Bool) {
        steps = result.steps.enumerated().map { index, step in
            let move = stepMoves[safe: index] ?? nil
            return ComboStepDisplay(
                id: index, ordinalText: ComboText.stepOrdinal(index), moveName: move?.nameJa ?? step.moveId,
                chips: move.map(ComboRankChips.chips(for:)) ?? [],
                damageText: step.damage.map {
                    BulkRowDisplay.percentRangeText(minPercent: $0.minPercent, maxPercent: $0.maxPercent)
                } ?? ComboText.noDamage,
                orderText: ComboText.orderLabel(step.order))
        }
        totalText = ComboText.total(minPercent: result.total.minPercent, maxPercent: result.total.maxPercent)
        totalBarFraction = BulkRowDisplay.barFraction(percent: result.total.maxPercent)
        koText = ComboText.koText(steps: result.steps.count, ko: result.total.ko)
        speedSummaryText = ComboText.speedSummary(first: result.speed.firstSelfSpeed, last: result.speed.lastSelfSpeed)
        let known = result.speed.opponentSpeedKnown
        speedLineText = known ? nil : result.steps.last.map { ComboText.speedLine($0.line) }
        opponentSpeedText = known ? ComboText.opponentSpeed(result.speed.opponentSpeed) : nil
        showsSpeedExample = !known && !trickRoom
        lastSelfSpeed = result.speed.lastSelfSpeed
        if let attack = result.opponentAttack {
            surviveText = ComboText.survive(attack.fast.surviveChancePercent)
            actionsText = ComboText.actions(min: attack.minActions, max: attack.maxActions)
            perHitText = (attack.fast.actions.first ?? attack.slow.actions.first).map {
                ComboText.perHit(minPercent: $0.damage.minPercent, maxPercent: $0.damage.maxPercent)
            }
        } else {
            surviveText = nil
            actionsText = nil
            perHitText = nil
        }
        unsupportedNotice = UnsupportedNoticeText.summary(
            texts: result.unsupported.map { Self.markText($0, names: names) })
    }

    private static func markText(_ mark: ComboMark, names: UnsupportedMarkNames) -> String {
        switch mark {
        case .calc(let mark):
            return UnsupportedMarkLabel.text(for: mark, name: names.name(for: mark))
        case .combo(let target, let reason, let id):
            // 名前の解決は対象だけで決まる(理由は使わない)。
            let name = names.name(for: UnsupportedMark(target: target, reason: .unknown, id: id))
            return "\(UnsupportedMarkLabel.targetName(for: target))「\(name)」(\(ComboText.markReason(reason)))"
        }
    }
}

extension Array {
    /// 範囲外なら nil。
    subscript(safe index: Int) -> Element? {
        indices.contains(index) ? self[index] : nil
    }
}
