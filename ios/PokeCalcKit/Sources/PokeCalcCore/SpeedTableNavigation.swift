// SpeedTableNavigation: 素早さ表の「自分の位置」の純粋な計算(I-speed-2 / F-06。ADR-0517)。
// 表は遅延描画なので、自分の行が画面外にあることがある。行の並びと「いま描かれている行」だけから、
// 自分の行・ジャンプの向き・読み上げの文を決める(View に判定を持たせない)。

/// 自分の行が画面のどちら側にあるか。
public enum SpeedSelfRowDirection: Equatable, Sendable {
    case visible
    /// 自分の行は画面より上(上へ戻る)。
    case above
    /// 自分の行は画面より下。
    case below
}

public enum SpeedTableNavigation {
    /// 自分の行(自分と同じ段、または境界の印)の添字。位置が無ければ nil。
    /// トリックルームの昇順でも、表の並びのままの添字(並べ替えは `tableRows` が済ませている)。
    public static func selfRowIndex(in rows: [SpeedTableRow]) -> Int? {
        rows.firstIndex { row in
            switch row {
            case .selfBoundary: return true
            case .tier(let tier): return tier.isSelf
            }
        }
    }

    /// `visible` は、いま描かれている行の添字。まだ何も描かれていない間は `.visible`(ボタンを出さない)。
    public static func direction(selfIndex: Int?, visible: Set<Int>) -> SpeedSelfRowDirection? {
        guard let selfIndex else { return nil }
        guard let first = visible.min(), let last = visible.max() else { return .visible }
        if selfIndex < first { return .above }
        if selfIndex > last { return .below }
        return .visible
    }

    /// 「全3段・自分は2段目」。空の表は nil。境界のときは前後の段で言う。
    public static func summary(rows: [SpeedTableRow]) -> String? {
        let tierCount = rows.filter { if case .tier = $0 { return true } else { return false } }.count
        guard !rows.isEmpty else { return nil }
        let total = SpeedLabels.tableTotal(tierCount)
        guard let index = selfRowIndex(in: rows) else { return total }
        let tiersBefore = rows[..<index].filter { if case .tier = $0 { return true } else { return false } }.count
        switch rows[index] {
        case .tier:
            return total + SpeedLabels.summarySeparator + SpeedLabels.selfAtTier(tiersBefore + 1)
        case .selfBoundary:
            let position: String
            if tiersBefore == 0 {
                position = SpeedLabels.selfBeforeTier(1)
            } else if tiersBefore == tierCount {
                position = SpeedLabels.selfAfterTier(tierCount)
            } else {
                position = SpeedLabels.selfBetweenTiers(tiersBefore, tiersBefore + 1)
            }
            return total + SpeedLabels.summarySeparator + position
        }
    }

    public static func jumpLabel(_ direction: SpeedSelfRowDirection) -> String {
        switch direction {
        case .visible: return SpeedLabels.jumpToSelf
        case .above: return SpeedLabels.jumpToSelfAbove
        case .below: return SpeedLabels.jumpToSelfBelow
        }
    }
}
