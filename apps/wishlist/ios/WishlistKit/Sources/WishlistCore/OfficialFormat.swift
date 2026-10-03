import Foundation

/// 公式サイトの販売状況の表示用の純関数(フェーズ4-3。docs/phase4-spec.md AC-IOS-OFF-01〜04)。
/// 文言は PWA(web/src/lib/official.ts)と同じ。日付は `PriceFormat.jstDate`(JST の M/D)。推測の文言を足さない。
public enum OfficialFormat {
    /// 変化を添える期間(changedAt から 7 日 = 7×24 時間以内)
    public static let changeDays = 7

    /// `販売中` / `予約受付中` / `在庫切れ` / `販売終了` / `判定できません` / `判定できません(複数の表示)` /
    /// `取得しません(robots.txt)` / `取得できませんでした`
    public static func label(_ state: OfficialState) -> String {
        switch state {
        case .available: return "販売中"
        case .preorder: return "予約受付中"
        case .soldout: return "在庫切れ"
        case .ended: return "販売終了"
        case .unknown: return "判定できません"
        case .ambiguous: return "判定できません(複数の表示)"
        case .blocked: return "取得しません(robots.txt)"
        case .failed: return "取得できませんでした"
        }
    }

    private static func isJudged(_ state: OfficialState) -> Bool { state != .failed && state != .blocked }

    /// `公式: 予約受付中(10/4 確認)`(日付は checkedAt)。status が failed・blocked なら `公式: 取得できませんでした(10/4)`(「確認」を付けない)。
    /// nil なら `公式: まだ確認していません`。
    public static func summary(_ status: OfficialStatus?) -> String {
        guard let status else { return "公式: まだ確認していません" }
        let date = PriceFormat.jstDate(status.checkedAt)
        let confirmed = isJudged(status.status) ? " 確認" : ""
        return "公式: \(label(status.status))(\(date)\(confirmed))"
    }

    /// `根拠: 予約受付中・予約する`。根拠が無ければ nil
    public static func evidence(_ status: OfficialStatus?) -> String? {
        guard let status, !status.evidence.isEmpty else { return nil }
        return "根拠: " + status.evidence.joined(separator: "・")
    }

    /// changedAt と previousStatus があり、now − changedAt が 7 日以内(未来も含む)なら `10/3 に 販売中 → 販売終了`。それ以外は nil
    public static func change(_ status: OfficialStatus?, now: Date) -> String? {
        guard let status, let changedAt = status.changedAt, let previous = status.previousStatus else { return nil }
        guard now.timeIntervalSince(changedAt) <= Double(changeDays) * 24 * 60 * 60 else { return nil }
        return "\(PriceFormat.jstDate(changedAt)) に \(label(previous)) → \(label(status.status))"
    }

    /// lastResult が status と違う(判定済みの status を残したまま、最後の試行が failed・blocked)なら
    /// `最新の確認(10/5): 取得できませんでした`(日付は lastAttemptAt)。それ以外は nil
    public static func lastAttempt(_ status: OfficialStatus?) -> String? {
        guard let status, isJudged(status.status), !isJudged(status.lastResult) else { return nil }
        return "最新の確認(\(PriceFormat.jstDate(status.lastAttemptAt))): \(label(status.lastResult))"
    }
}

/// 詳細シートの「公式」の行(ItemDetailViewModel.official)。nil の行は出さない。
public struct OfficialLines: Sendable, Equatable {
    public var summary: String
    public var evidence: String?
    public var change: String?
    public var lastAttempt: String?

    public init(summary: String, evidence: String? = nil, change: String? = nil, lastAttempt: String? = nil) {
        self.summary = summary
        self.evidence = evidence
        self.change = change
        self.lastAttempt = lastAttempt
    }
}
