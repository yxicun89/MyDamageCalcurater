// JudgeLabels: 判定画面の固定文言(P6-25。ADR-0504)。
// Web の `judgeScreenText`・`judgeErrorText`(web/src/i18n/ja.ts)と同じ文言にそろえる。違いは次の4点だけ(ADR-0504 §9):
//  1. 技は ID の自由入力ではなくマスタから選ぶので、ラベルは「技」(Web は「技の ID」)。
//  2. 能力ポイント・ランクのラベルのステータス名は日本語名(Web は英字1文字)。
//  3. 確定数の確率は小数第1位固定(`BulkRowDisplay.koText` と同じ。Web は数値そのまま)。
//  4. 未対応の印の添え書き(ADR-0708 の画面側。Web にはまだ無い)と、構築から呼び出す入口の文言。
// 範囲の数値(能力ポイント・ランク・候補数)は `SPLimits`・`RankLimits`・`RequestLimits` から埋め込む(直書きしない。coding-rules §2)。

public enum JudgeLabels {
    // ---- 入口・見出し ----
    /// ルート画面の入口ボタン。
    public static let openButton = "抜いて倒せるか判定"
    public static let screenTitle = "判定"
    public static let attackerRegion = "自分のポケモン"
    public static let defendersRegion = "相手の候補"
    public static let resultRegion = "判定結果"

    // ---- 個体の入力(自分と候補で同じ語を使う) ----
    public static let species = "ポケモン"
    public static let nature = "性格"
    public static let ability = "特性"
    public static let item = "持ち物"
    public static let move = "技"
    public static let unselected = "未選択"
    public static let loadingMaster = "読み込み中"
    public static func spLabel(_ stat: StatKey) -> String { "\(StatKeyLabel.japaneseName(for: stat))の能力ポイント" }
    public static func rankLabel(_ stat: StatKey) -> String { "\(StatKeyLabel.japaneseName(for: stat))のランク" }
    public static let spTotalCaption = "能力ポイントの合計"
    public static func spIncrement(_ stat: StatKey) -> String { "\(StatKeyLabel.japaneseName(for: stat))の能力ポイントを増やす" }
    public static func spDecrement(_ stat: StatKey) -> String { "\(StatKeyLabel.japaneseName(for: stat))の能力ポイントを減らす" }
    public static func rankIncrement(_ stat: StatKey) -> String { "\(StatKeyLabel.japaneseName(for: stat))のランクを上げる" }
    public static func rankDecrement(_ stat: StatKey) -> String { "\(StatKeyLabel.japaneseName(for: stat))のランクを下げる" }

    // ---- 状態異常(ADR-0512。Web の `statusLabel`・`statusOptionLabel` と同じ語) ----
    /// 見出し・選択ボタンの名前・シートの題。
    public static let status = "状態異常"
    public static func statusName(_ status: JudgeStatus) -> String {
        switch status {
        case .none: return "なし"
        case .burn: return "やけど"
        case .paralysis: return "まひ"
        case .poison: return "どく"
        case .badlyPoison: return "もうどく"
        case .sleep: return "ねむり"
        case .freeze: return "こおり"
        }
    }

    // ---- 選択シート(性格・特性・持ち物) ----
    public static let optionSheetClose = "閉じる"
    public static let optionSheetEmpty = "選べる候補がありません"
    /// マスタ(性格・持ち物・技・種族)を読み込めないとき。判定の失敗とは別の文言(判定・計算・構築に影響しない)。
    public static let masterLoadFailed = "性格・持ち物などの一覧を読み込めません"

    // ---- 場の効果(speedField) ----
    public static let speedFieldGroup = "場の効果"
    public static let trickRoom = "トリックルーム"
    public static let attackerTailwind = "自分の側の追い風"
    public static let defenderTailwind = "相手の側の追い風"
    /// 相手側の追い風が候補ごとではない理由(ADR-0703 §5)。
    public static let defenderTailwindNotice = "相手の側の追い風は、すべての相手候補に同じように適用されます"

    // ---- 相手候補の増減(1始まりの番号) ----
    public static func candidate(_ number: Int) -> String { "相手候補\(number)" }
    public static let addCandidate = "相手候補を追加"
    public static func removeCandidate(_ number: Int) -> String { "相手候補\(number)を削除" }
    public static func maxCandidatesNotice(_ max: Int) -> String { "相手候補は\(max)件までです" }
    public static let minimumCandidateNotice = "相手候補は少なくとも1件必要です"

    // ---- 送信 ----
    public static let submit = "判定する"
    public static let loading = "判定中"
    public static let emptyResult = "「判定する」を押すと結果が出ます"

    // ---- 送信前の検査(違反していれば judge を呼ばずに理由を出す。誰の入力かを先頭に付ける) ----
    public static func validationMessage(_ error: JudgeValidationError) -> String {
        switch error {
        case .requiredMissing(let target):
            return "\(subject(target)): ポケモン・性格・技をすべて選んでください"
        case .moveIdInvalid(let target):
            return "\(subject(target)): 技の ID が長すぎます(最大\(RequestLimits.maxJudgeMoveIdLength)文字)"
        case .spTotalExceeded(let target):
            return "\(subject(target)): 能力ポイントの合計は\(SPLimits.maxTotal)までです"
        }
    }

    /// 検査の対象の名前(自分のポケモン / 相手候補N。N は 1 始まり)。
    public static func subject(_ target: JudgeTarget) -> String {
        switch target {
        case .attacker: return attackerRegion
        case .candidate(let index): return candidate(index + 1)
        }
    }

    // ---- 結果(judge の値をそのまま出す。「勝ち」「負け」に丸めた語は持たない。ADR-0700 §6-1) ----
    public static func speed(attacker: Int, defender: Int) -> String { "素早さ \(attacker) 対 \(defender)" }
    public static func priority(attacker: Int, defender: Int) -> String { "優先度 \(attacker) 対 \(defender)" }
    public static let outspeedsTrue = "素早さで上回る"
    public static let outspeedsFalse = "素早さで下回る"
    /// 同速(`outspeeds` と同時に true にならない。真偽値1つに丸めない)。
    public static let speedTie = "同速"
    public static let attackerMovesFirst = "自分が先に動く"
    public static let defenderMovesFirst = "相手が先に動く"
    /// 優先度も素早さも同じで行動順が決まらないとき(ADR-0704 §2)。
    public static let turnOrderTie = "どちらが先に動くか決まらない"
    public static let attackerKoPrefix = "自分の技で相手を"
    public static let defenderKoPrefix = "相手の技で自分が"
    public static let koNone = "倒せない"
    public static func koGuaranteed(hits: Int) -> String { "確定\(hits)発" }
    /// `percent` は `displayChancePercent`(小数第1位固定で整形する)。
    public static func koRandom(hits: Int, percent: String) -> String { "乱数\(hits)発(\(percent)%)" }

    // ---- 素早さに反映した補正・反映していない入力(ADR-0710・ADR-0714・ADR-0512。Web の `speedAppliedNote` 等と同じ語) ----
    /// 誰の素早さか(`attackerSpeed*` = 自分、`defenderSpeed*` = その行の相手候補)。
    public static let speedSideSelf = "自分"
    public static let speedSideOpponent = "相手"
    /// 契約 `SpeedFactor` の値 → 文言。**未知の値は `speedFactorUnknown`**(落とさない。契約に値が増えた古いアプリの挙動)。
    public static func speedFactorName(_ value: String) -> String {
        switch value {
        case "rank": return "ランク補正"
        case "tailwind": return "追い風"
        case "ability": return "特性"
        case "choiceScarf": return "こだわりスカーフ"
        case "item": return "持ち物"
        case "paralysis": return "まひ"
        default: return speedFactorUnknown
        }
    }
    /// 契約 `SpeedIgnoredInput` の値 → 文言。未知の値は `speedIgnoredUnknown`。
    public static func speedIgnoredName(_ value: String) -> String {
        switch value {
        case "abilityId": return "特性"
        case "itemId": return "持ち物"
        case "fieldWeather": return "天候"
        default: return speedIgnoredUnknown
        }
    }
    /// 未知の値のフォールバック(契約の英語の値を画面に出さない)。
    public static let speedFactorUnknown = "その他の補正"
    public static let speedIgnoredUnknown = "その他の入力"
    /// 「自分の素早さに反映: ランク補正・追い風」(`names` は `speedFactorName` 済み。区切りは「・」)。
    public static func speedAppliedNote(side: String, names: [String]) -> String {
        "\(side)の素早さに反映: \(names.joined(separator: "・"))"
    }
    /// 「相手の素早さに特性・持ち物・天候は反映していません」(`names` は `speedIgnoredName` 済み)。
    public static func speedIgnoredNote(side: String, names: [String]) -> String {
        "\(side)の素早さに\(names.joined(separator: "・"))は反映していません"
    }

    // ---- 未対応の印(ADR-0708 §4・§5。方向ごとに分けて出す。混ぜない) ----
    /// その方向の確定数を「確定した数」として見せない旨(印が1つでもある行の、その方向の確定数の直下に付ける)。
    public static let attackerKoUnreliable = "自分の技の確定数は当てにならない可能性があります"
    public static let defenderKoUnreliable = "相手の技の確定数は当てにならない可能性があります"
    /// 印の一覧の見出し付き文言。`detail` は `UnsupportedNoticeText.rowNote` の結果(`未対応: 技「…」(多段技)`)。
    public static func attackerKoUnsupportedNotice(detail: String) -> String { "自分の技の確定数は正確でない可能性があります(\(detail))" }
    public static func defenderKoUnsupportedNotice(detail: String) -> String { "相手の技の確定数は正確でない可能性があります(\(detail))" }

    // ---- エラー(サーバーの英語 message は出さず、code から日本語にする) ----
    public static let unavailable = "判定の API に接続できません"
    public static let errorFallback = "判定に失敗しました"

    /// `PokeCalcError.code` → 日本語。契約の `ErrorCode` はすべて持つ(`JudgeContractSyncTests` が漏れを検出する)。
    /// 通信できない・応答が読めない・契約外のステータスは「接続できません」(Web の `judge_unavailable` と同じ)。未知の code は汎用の文言。
    public static func errorMessage(forCode code: String) -> String {
        switch code {
        case "invalid_request": return "入力の形が正しくありません"
        case "unknown_species": return "このポケモンはマスタにありません"
        case "unknown_move": return "この技の ID はマスタにありません"
        case "unknown_nature": return "この性格はマスタにありません"
        case "request_too_large": return "入力が大きすぎます"
        case "upstream_unavailable": return "判定に必要なサービスに接続できません"
        case "missing_header": return "端末の識別情報がありません"
        case "invalid_header": return "端末の識別情報が正しくありません"
        case PokeCalcError.Code.transport, PokeCalcError.Code.decode, PokeCalcError.Code.unexpectedStatus: return unavailable
        default: return errorFallback  // internal_error・not_found・未知の code
        }
    }

    /// どの候補で失敗したかの補助の行(サーバーの message の `defenders[<index>]` から取った 1 始まりの番号)。
    public static func failedCandidate(_ number: Int) -> String { "\(candidate(number))の入力で失敗しました" }
}
