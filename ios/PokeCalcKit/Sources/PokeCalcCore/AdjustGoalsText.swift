// AdjustGoalsText: 調整の「目標」方式(F-11。ADR-0331 §5〜§7・ADR-0525)の文言。
//
// Web の `web/src/i18n/adjust.ts`(goals の節)と同じ語にそろえる。View はここが作った文字列をそのまま描く。
// サーバーの message は使わない(`AdjustText.errorMessage(for:)` と同じ)。

extension AdjustText {
    // MARK: - モード・領域

    /// モードの選択肢(「調整の内容」の先頭)。
    public static let goalsModeLabel = "目標から振り方を決める"
    public static let addGoalButton = "目標を追加"
    public static let removeGoalVisible = "外す"
    public static let noGoalsNotice = "目標を追加してください"
    public static let goalKindField = "種類"
    public static let goalOpponentSpeciesField = "相手のポケモン"
    public static let goalPresetField = "相手の振り方"
    public static let goalOpponentMoveField = "相手の技"
    public static let goalOwnMoveField = "自分の技"
    public static let goalBoostMoveField = "先に使う技"
    public static let goalBoostMoveNone = "使わない"
    public static let goalBoostMoveHint = "ニトロチャージのように自分の素早さが上がる技を選ぶと、上がったあとの素早さで比べます"
    public static let goalHitsAndChanceField = "発数・確率"
    public static let goalMoveSheetClose = "閉じる"
    public static let hitsDecrease = "発数を減らす"
    public static let hitsIncrease = "発数を増やす"

    /// 機能を使えないときの案内(iOS 独自。Web は常に有効にしているため文言が無い)。
    public static let goalsUnavailable = "目標から振り方を決める機能は今は使えません。ほかの調整の内容をお使いください"

    public static func goalKindOption(_ kind: AdjustGoalKind) -> String {
        switch kind {
        case .outspeed: return "素早さを上回る"
        case .survive: return "この技を耐える"
        case .ko: return "この技で倒す"
        }
    }

    /// 目標のカードの名前(group の名前。n は 1 からの位置)。
    public static func goalCardLegend(_ n: Int) -> String { "目標 \(n)" }
    /// 目標のカードの欄の accessibility label(「目標 n の<見えるラベル>」。見える語を含む)。
    public static func goalFieldName(_ n: Int, _ visibleLabel: String) -> String { "目標 \(n) の\(visibleLabel)" }
    public static func removeGoalName(_ n: Int) -> String { "目標 \(n) を外す" }
    public static func goalLimitHint(_ max: Int) -> String { "目標は \(max) つまでです" }

    // MARK: - 送信前の検査

    public static let goalsRequiredMessage = "目標を追加してください"
    public static func goalOpponentRequiredMessage(_ n: Int) -> String { "目標 \(n): 相手のポケモンを選んでください" }
    public static func goalOpponentMoveRequiredMessage(_ n: Int) -> String { "目標 \(n): 相手の技を選んでください" }
    public static func goalOwnMoveRequiredMessage(_ n: Int) -> String { "目標 \(n): 自分の技を選んでください" }
    public static func goalNatureNotFoundMessage(_ n: Int) -> String { "目標 \(n): 相手の振り方に合う性格がマスタにありません" }

    // MARK: - 結果

    public static let goalsPlanHeading = "目標をすべて満たす振り方"
    public static let goalsNearestHeading = "目標に一番近い振り方"
    public static let goalsInfeasibleNotice = "すべての目標は満たせませんでした"
    public static let goalOutcomesLabel = "目標ごとの結果"

    /// 「相手(振り方)」(例「テストA(最速)」)。
    public static func goalOpponentName(_ speciesName: String, _ presetLabel: String) -> String {
        "\(speciesName)(\(presetLabel))"
    }

    /// 目標1件の結果の文(Web の `outspeedOutcome` / `surviveOutcome` / `koOutcome` と同じ)。
    /// n は 1 からの位置。確率は 0.1% 単位の切り捨て。
    public static func goalOutcomeLine(_ n: Int, snapshot: AdjustGoalSnapshot, outcome: AdjustGoalOutcome) -> String {
        let opponent = snapshot.opponentName
        let move = snapshot.moveName ?? ""
        let chance = chancePercentText(outcome.chancePercent ?? 0)
        switch snapshot.kind {
        case .outspeed:
            let prefix = speedBoostPrefix(moveName: snapshot.boostMoveName, rank: outcome.selfSpeedRank)
            let verdict = outcome.met ? "動けます" : "は動けません"
            let speeds = "(自分 \(outcome.selfSpeed ?? 0) / 相手 \(outcome.opponentSpeed ?? 0))"
            return "目標 \(n): \(prefix)\(opponent)より先に\(verdict)\(speeds)"
        case .survive:
            return outcome.met
                ? "目標 \(n): \(opponent)の\(move)を\(snapshot.hits)発耐えます(耐える確率 \(chance))"
                : "目標 \(n): \(opponent)の\(move)を\(snapshot.hits)発は耐えられません(耐える確率 \(chance))"
        case .ko:
            return outcome.met
                ? "目標 \(n): \(move)で\(opponent)を\(snapshot.hits)発で倒せます(倒す確率 \(chance))"
                : "目標 \(n): \(move)で\(opponent)を\(snapshot.hits)発では倒せません(倒す確率 \(chance))"
        }
    }

    /// 「先に使う技」で自分の素早さのランクが変わったときの前置き(技を選んでいないときは空)。
    static func speedBoostPrefix(moveName: String?, rank: Int?) -> String {
        guard let moveName else { return "" }
        let rank = rank ?? 0
        if rank > 0 { return "\(moveName)で素早さが\(rank)段階上がったあと、" }
        if rank < 0 { return "\(moveName)で素早さが\(-rank)段階下がったあと、" }
        return "\(moveName)では素早さは上がりません。"
    }
}
