import Foundation

// AdjustText: 調整画面(AJ7。ADR-0502 §6)の文言と表示の書式。
//
// Web の `web/src/i18n/ja.ts` の `adjustScreenText`・`adjustErrorText`・`adjustClientText` と同じ語にそろえる
// (iOS は i18n のファイルを持たないので、`DeviceDataText`・`MasterSearchLabels` と同じくコードの1か所に置く)。
// View はここが作った文字列をそのまま描き、画面のコードに日本語を直書きしない。
// サーバーの `PokeCalcError.message`(英語の内部メッセージを含みうる)は使わない(ADR-0411 §3・ADR-0319 §6)。

public enum AdjustText {
    // MARK: - 画面・入口

    /// ルート画面のボタンと画面の見出し(Web のタブ名 `appText.adjustTabLabel` と同じ語)。
    public static let screenTitle = "調整"

    // MARK: - 領域(見出しの語。VoiceOver の見出しにもなる)

    public static let ownRegion = "自分"
    public static let modeRegion = "調整の内容"
    public static let opponentRegion = "相手"
    public static let goalRegion = "目標"
    public static let resultRegion = "調整の結果"
    public static let learnersRegion = "この技を覚えるポケモン"

    // MARK: - 欄(見えるラベルは短い語、accessibilityLabel は「<領域>の<ラベル>」。見える語を含む)

    public static let speciesField = "ポケモン"
    public static let natureField = "性格"
    public static let abilityField = "特性"
    public static let itemField = "持ち物"
    public static let moveField = "技"
    public static let presetField = "調整"
    public static let ownSpeciesLabel = "自分のポケモン"
    public static let ownNatureLabel = "自分の性格"
    public static let ownAbilityLabel = "自分の特性"
    public static let ownItemLabel = "自分の持ち物"
    public static let ownMoveLabel = "自分の技"
    public static let opponentSpeciesLabel = "相手のポケモン"
    public static let opponentPresetLabel = "相手の調整"
    public static let opponentMoveLabel = "相手の技"
    /// 未選択のときに出す文言(空の表示にしない)。
    public static let speciesPlaceholder = "ポケモンを選ぶ"
    public static let naturePlaceholder = "性格を選ぶ"
    public static let movePlaceholder = "技を選ぶ"
    public static let unselectedOption = "未選択"

    // MARK: - 固定する能力ポイント(下限。ADR-0150 §8)

    public static let fixedSPGroup = "固定する能力ポイント"
    public static let fixedSPHint = "ここで決めた値より下には振りません。残りを調整に回します"

    // MARK: - モード

    public static let focusField = "耐久の基準"
    public static let ceilingGroup = "振ってよい上限"
    public static let offenseCategoryField = "攻撃の分類"
    public static let minSpeedField = "素早さの目標(実数値)"
    public static let minSpeedHint = "この実数値以上になるように S に振ります。空なら目標なし"
    public static let useGoalField = "目標を指定する"

    // MARK: - 目標

    public static let hitsField = "発数"
    public static let thresholdField = "確率"

    // MARK: - 送信

    public static let submitButton = "調整する"
    public static let loadingNotice = "計算中"
    public static let emptyResultNotice = "「調整する」を押すと結果が出ます"

    // MARK: - 送信前の検査(違反なら API を呼ばずにこの文を出す)

    public static let ownRequired = "自分のポケモンと性格を選んでください"
    public static let ownMoveRequired = "自分の技を選んでください"
    public static let opponentRequired = "相手のポケモンを選んでください"
    public static let opponentMoveRequired = "相手の技を選んでください"
    public static let ceilingBelowFixed = "上限は固定する能力ポイント以上にしてください"
    public static let categoryMismatch = "攻撃の分類と自分の技の分類をそろえてください"
    public static let minSpeedInvalid = "素早さの目標は0以上の整数で入力してください"
    public static let natureNotFound = "相手の調整に合う性格がマスタにありません"

    // MARK: - 結果の見出し・固定文

    public static let indicesHeading = "今の振り方の指数"
    public static let firepowerIndexLabel = "火力指数"
    public static let firepowerIndexNone = "技を選ぶと出します"
    public static let physicalBulkLabel = "物理耐久指数"
    public static let specialBulkLabel = "特殊耐久指数"
    public static let indexNote = "火力指数の補正はタイプ一致だけを含めます(持ち物・特性・テラスタルは含めません)"
    public static let hpLineHeading = "HP の 16n"
    public static let next16nLabel = "次の 16n"
    public static let prev16nLabel = "前の 16n"
    public static let next16nMinus1Label = "次の 16n-1"
    public static let prev16nMinus1Label = "前の 16n-1"
    public static let maxIndexHeading = "指数が最大になる振り方"
    public static let minSpHeading = "目標を満たす最小の振り方"
    public static let minSpNotRequested = "目標を指定すると、目標を満たす最小の振り方も出します"
    public static let speedMet = "素早さの目標を満たします"
    public static let speedNotMet = "素早さの目標に届きません"

    // MARK: - 技を覚えるポケモン

    public static let learnersButton = "覚えるポケモン"
    public static let ownLearnersButtonLabel = "自分の技を覚えるポケモン"
    public static let opponentLearnersButtonLabel = "相手の技を覚えるポケモン"
    public static let learnersEmpty = "この技を覚えるポケモンはいません"
    public static let learnersMore = "続きを読み込む"
    public static let learnersLoading = "読み込み中"
    public static let learnersClose = "閉じる"
    /// 数字キーボードの上の「完了」(テンキーには Return が無い)。
    public static let keyboardDone = "完了"

    // MARK: - エラー(code → 日本語。Web の adjustErrorText と同じ語)

    /// 通信できない・応答が読めない(Web の `adjust_unavailable` と同じ文)。
    public static let unavailable = "調整の API に接続できません"
    /// 下のどれにも当たらないコード。
    public static let errorFallback = "調整に失敗しました"
    /// openapi `ErrorCode` → 文言。
    public static let errorMessages: [String: String] = [
        "invalid_json": "入力の形が正しくありません",
        "unknown_field": "入力の形が正しくありません",
        "invalid_enum": "選んだ項目の値が正しくありません",
        "invalid_input": "入力の値が範囲の外です。能力ポイント・上限・発数・確率を確かめてください",
        "unknown_species": "このポケモンはマスタにありません",
        "unknown_move": "この技はマスタにありません",
        "unknown_nature": "この性格はマスタにありません",
        "unknown_item": "この持ち物はマスタにありません",
        "unknown_ability": "この特性はマスタにありません",
        "not_found": "見つかりませんでした。入力を確かめてください",
        "missing_header": "端末の識別子を送れませんでした。アプリを開き直してください",
        "invalid_header": "端末の識別子を送れませんでした。アプリを開き直してください",
        "type_chart_missing": "タイプ相性表を読み込めていません。しばらくしてからお試しください",
        "master_unavailable": "マスタの準備ができていません。しばらくしてからお試しください",
        "upstream_unavailable": "調整に必要なサービスに接続できません",
    ]

    // MARK: - 選択肢の表示名(語彙の表。Web の modeLabel・focusOption・offenseCategoryOption と同じ語)

    public static func modeLabel(_ mode: AdjustMode) -> String {
        switch mode {
        case .indices: return "指数と 16n を見る"
        case .bulk: return "耐久に振る"
        case .offense: return "攻撃と素早さに振る"
        case .minKo: return "倒せる最小の振り方"
        case .minSurvive: return "耐えられる最小の振り方"
        }
    }

    public static func focusLabel(_ focus: BulkFocus) -> String {
        switch focus {
        case .physical: return "物理(H×B)"
        case .special: return "特殊(H×D)"
        case .both: return "物理と特殊の両方"
        }
    }

    /// 攻撃の分類の選択肢(変化は選べないので「物理(A)」と同じ語を返す防御的な既定)。
    public static func offenseCategoryLabel(_ category: MoveCategory) -> String {
        switch category {
        case .physical, .status: return "物理(A)"
        case .special: return "特殊(C)"
        }
    }

    /// 能力の1文字(H/A/B/C/D/S。Web の `statLetterJa` と同じ)。
    public static func statLetter(_ stat: StatKey) -> String {
        switch stat {
        case .hp: return "H"
        case .atk: return "A"
        case .def: return "B"
        case .spa: return "C"
        case .spd: return "D"
        case .spe: return "S"
        }
    }

    public static func hpLineKindLabel(_ kind: HPLineKind) -> String {
        switch kind {
        case .none: return "16n でも 16n-1 でもない"
        case .line16n: return "16n"
        case .line16nMinus1: return "16n-1"
        }
    }

    // MARK: - 書式

    /// 固定 SP の欄の見えるラベル(例「H の固定ポイント」)。
    public static func fixedSPLabel(_ stat: StatKey) -> String { "\(statLetter(stat)) の固定ポイント" }
    /// 固定 SP の合計(例「合計 36 / 66」)。
    public static func fixedSPTotal(_ total: Int) -> String { "合計 \(total) / \(SPLimits.maxTotal)" }
    /// 上限の欄のラベル(例「B の上限」)。
    public static func ceilingLabel(_ stat: StatKey) -> String { "\(statLetter(stat)) の上限" }
    /// 発数の選択肢(例「2発」)。
    public static func hitsOption(_ hits: Int) -> String { "\(hits)発" }
    /// 確率の選択肢(100 は「確定(100%)」、ほかは「90% 以上」)。
    public static func thresholdOption(_ percent: Double) -> String {
        percent >= certainPercent ? "確定(\(Int(certainPercent))%)" : "\(Int(percent))% 以上"
    }
    /// 固定 SP の範囲違反(例「能力ポイントは0〜32の整数で入力してください」)。
    public static var spRangeInvalid: String { "能力ポイントは0〜\(SPLimits.maxPerStat)の整数で入力してください" }
    /// 固定 SP の合計違反(例「能力ポイントの合計は66までです」)。
    public static var spTotalExceeded: String { "能力ポイントの合計は\(SPLimits.maxTotal)までです" }

    /// 確率の「確定」(契約の `thresholdPercent` の既定)。
    private static let certainPercent = 100.0
    /// 浮動小数の誤差で 0.1% 単位の切り捨てが1つ下がらないための余り(例 `4.35 * 10` が `43.49999…` になる)。
    private static let floorEpsilon = 1e-9

    /// engine の生の確率(%)を 0.1% 単位で**切り捨て**た表示(100 → "100%"、99.99 → "99.9%"、37.5 → "37.5%")。
    public static func chancePercentText(_ percent: Double) -> String {
        let tenths = Int((percent * 10 + floorEpsilon).rounded(.down))
        let whole = tenths / 10
        let fraction = tenths % 10
        return fraction == 0 ? "\(whole)%" : "\(whole).\(fraction)%"
    }

    /// エラーを画面の文に写す。`PokeCalcError` は `errorMessages[code]`(無ければ `errorFallback`)、
    /// `client_transport_error` / `client_decode_error` は `unavailable`、それ以外の Error は `errorFallback`。
    /// サーバーの message は使わない。
    public static func errorMessage(for error: any Error) -> String {
        guard let error = error as? PokeCalcError else { return errorFallback }
        if error.code == PokeCalcError.Code.transport || error.code == PokeCalcError.Code.decode { return unavailable }
        return errorMessages[error.code] ?? errorFallback
    }

    /// 「実数値 H 155 / A 120 / …」。
    public static func statsLine(_ stats: StatBlock) -> String { "実数値 " + statBlockText(stats) }
    /// 「能力ポイント H 4 / A 0 / …」。
    public static func planSPLine(_ sp: StatBlock) -> String { "能力ポイント " + statBlockText(sp) }
    /// 「<ラベル> <値>」(例「火力指数 12000」)。
    public static func indexLine(_ label: String, _ value: Int64) -> String { "\(label) \(value)" }
    /// 「HP 160(16n)」。
    public static func hpCurrent(_ report: HPLineReport) -> String { "HP \(report.hp)(\(hpLineKindLabel(report.current)))" }
    /// 次・前の 16n / 16n-1 の4行(次の16n・前の16n・次の16n-1・前の16n-1 の順)。
    /// 「<ラベル>: HP n(H sp、±差)」、無ければ「<ラベル>: なし」。
    public static func hpLinePoints(_ report: HPLineReport) -> [String] {
        [
            hpLinePointText(next16nLabel, report.next16n),
            hpLinePointText(prev16nLabel, report.prev16n),
            hpLinePointText(next16nMinus1Label, report.next16nMinus1),
            hpLinePointText(prev16nMinus1Label, report.prev16nMinus1),
        ]
    }
    /// 「A に 12 振れば 2発で倒せます(確率 100%)」/「A に 32 振っても 2発では倒せません(確率 37.5%)」。
    public static func koLine(_ result: AdjustKOResult, hits: Int) -> String {
        let spText = "\(statLetter(result.stat)) に \(result.sp) 振"
        let chance = "(確率 \(chancePercentText(result.chancePercent)))"
        return result.feasible
            ? "\(spText)れば \(hitsOption(hits))で倒せます\(chance)"
            : "\(spText)っても \(hitsOption(hits))では倒せません\(chance)"
    }
    /// 「H に 20・B に 8 振れば 1発耐えます(確率 100%)」/「… 振っても 1発は耐えられません(確率 …)」。
    public static func surviveLine(_ result: AdjustSurviveResult, hits: Int) -> String {
        let spText = "H に \(result.hpSp)・\(statLetter(result.stat)) に \(result.statSp) 振"
        let chance = "(確率 \(chancePercentText(result.chancePercent)))"
        return result.feasible
            ? "\(spText)れば \(hitsOption(hits))耐えます\(chance)"
            : "\(spText)っても \(hitsOption(hits))は耐えられません\(chance)"
    }
    /// 「残りの能力ポイント 30」。
    public static func remainingLine(_ remaining: Int) -> String { "残りの能力ポイント \(remaining)" }
    /// 「合計 66」。
    public static func planTotal(_ total: Int) -> String { "合計 \(total)" }
    /// 「目標を満たします(確率 …)」/「目標に届きません(確率 …)」。
    public static func goalLine(met: Bool, chancePercent: Double) -> String {
        "\(met ? "目標を満たします" : "目標に届きません")(確率 \(chancePercentText(chancePercent)))"
    }
    /// 「<技名>を覚えるポケモン」。
    public static func learnersHeading(_ moveName: String) -> String { "\(moveName)を覚えるポケモン" }

    // MARK: - 内部

    /// 「H 155 / A 120 / B 90 / C 80 / D 85 / S 120」(能力の並びは H A B C D S)。
    private static func statBlockText(_ block: StatBlock) -> String {
        [
            (StatKey.hp, block.hp), (.atk, block.atk), (.def, block.def),
            (.spa, block.spa), (.spd, block.spd), (.spe, block.spe),
        ].map { "\(statLetter($0.0)) \($0.1)" }.joined(separator: " / ")
    }

    private static func hpLinePointText(_ label: String, _ point: HPLinePoint?) -> String {
        guard let point else { return "\(label): なし" }
        let delta = point.spDelta > 0 ? "+\(point.spDelta)" : "\(point.spDelta)"
        return "\(label): HP \(point.hp)(H \(point.sp)、\(delta))"
    }
}
