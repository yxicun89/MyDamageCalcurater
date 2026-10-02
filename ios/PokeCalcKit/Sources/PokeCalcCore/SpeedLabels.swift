// SpeedLabels: 素早さ比較の画面の固定文言(P6-24。ADR-0503)。
// Web の `speedScreenText`・`speedPresetText`(web/src/i18n/ja.ts)と同じ文言にそろえる(違いは ADR に書く)。
// 範囲の数値(SP の最大・ランクの範囲)は `SPLimits` / `RankLimits` から埋め込む(直書きしない。coding-rules §2)。

public enum SpeedLabels {
    // ---- 入口・見出し ----
    /// ルート画面の入口ボタン。
    public static let openButton = "素早さを比べる"
    public static let screenTitle = "素早さ比較"
    public static let tableRegion = "素早さの表"
    public static let selfRegion = "自分のポケモン"
    public static let loading = "読み込み中"
    public static let positionLoading = "位置を計算中"

    // ---- 表 ----
    public static let filterGroup = "表の絞り込み"
    public static let filterMinimumNotice = "少なくとも1つは選ぶ必要があります"
    public static let fieldGroup = "場の状態"
    public static let tableTailwind = "追い風(相手側)"
    public static let trickRoom = "トリックルーム"
    public static let tie = "同速"
    public static let noTie = "同速なし"
    public static let selfTier = "自分と同速"
    public static let selfBoundary = "ここに自分が入る"
    public static let entrySeparator = "・"

    public static func tierSpeed(_ speed: Int) -> String { "素早さ \(speed)" }

    // ---- 自分のポケモン ----
    public static let modeGroup = "入力の方法"
    public static let pokemon = "ポケモン"
    public static let unselected = "未選択"
    public static let presetGroup = "調整"
    public static let scarf = "こだわりスカーフ"
    public static let selfTailwind = "追い風(自分側)"
    public static let paralysis = "まひ"
    public static let sp = "素早さ SP"
    public static let natureGroup = "性格補正"
    public static let rank = "ランク"
    public static let rawValue = "実数値"
    /// ピッカー(シート)の検索欄の案内と、0件のとき。
    public static let pokemonSearchPrompt = "ポケモンの名前で絞り込み"
    public static let pokemonNoMatch = "一致するポケモンがいません"
    public static let close = "閉じる"
    /// SP のステッパーのアクセシビリティ名(ランクは `CalcConditionLabels.rankIncrement`/`rankDecrement` と同じ流儀)。
    public static let spIncrement = "素早さ SP を増やす"
    public static let spDecrement = "素早さ SP を減らす"

    public static func mode(_ mode: SpeedInputMode) -> String {
        switch mode {
        case .preset: return "プリセット"
        case .custom: return "カスタム"
        case .raw: return "実数値"
        }
    }

    /// 表の行の調整名(`PresetId`)。契約には含まれず、クライアントの文言資源が持つ。
    public static func preset(_ id: SpeedPresetID) -> String {
        switch id {
        case .uninvested: return "無振り"
        case .neutralMax: return "準速"
        case .max: return "最速"
        case .maxScarf: return "最速スカーフ"
        case .maxPlus1: return "最速+1"
        case .maxPlus2: return "最速+2"
        }
    }

    /// 自分のポケモンで選べる3つの調整名(表の行の同じ調整と同じ語)。
    public static func preset(_ id: SpeedMinimalPreset) -> String {
        switch id {
        case .uninvested: return preset(SpeedPresetID.uninvested)
        case .neutralMax: return preset(SpeedPresetID.neutralMax)
        case .max: return preset(SpeedPresetID.max)
        }
    }

    public static func nature(_ nature: SpeedNature) -> String {
        switch nature {
        case .minus: return "下降"
        case .neutral: return "補正なし"
        case .plus: return "上昇"
        }
    }

    // ---- 入力の範囲外(画面で送信前に止める) ----
    public static func spRange(max: Int) -> String { "能力ポイントは0〜\(max)の整数で入力してください" }
    public static func rankRange(min: Int, max: Int) -> String { "ランクは\(min)〜+\(max)の整数で入力してください" }
    /// 実数値の下限(契約の minimum: 1)。上限は speed サービスだけが式から導くので画面では判定しない。
    public static let rawValueRange = "実数値は1以上の整数で入力してください"

    // ---- 結果 ----
    public static func selfSpeed(_ speed: Int) -> String { "実数値 \(speed)" }
    public static func faster(_ rows: Int) -> String { "自分より速い \(rows)行" }
    public static func slower(_ rows: Int) -> String { "自分より遅い \(rows)行" }
    /// トリックルーム中の行動順の読み替え(速い = 後に動く、遅い = 先に動く。ADR-0607 §4)。
    public static func movesBefore(_ rows: Int) -> String { "自分より先に動く \(rows)行" }
    public static func movesAfter(_ rows: Int) -> String { "自分より後に動く \(rows)行" }

    // ---- エラー(サーバーの英語 message は出さず、code から日本語にする) ----
    public static let unavailable = "素早さの API に接続できません"
    public static let errorFallback = "素早さの計算に失敗しました"

    /// `PokeCalcError.code` → 日本語。契約の `ErrorCode` はすべて持つ。通信失敗・応答が読めないときは
    /// 「接続できません」(Web の `speed_unavailable` と同じ)。未知の code は汎用の文言。
    public static func errorMessage(forCode code: String) -> String {
        switch code {
        case "invalid_request": return "入力の形が正しくありません。値の範囲を確認してください"
        case "missing_header": return "端末の識別情報が送られていません"
        case "invalid_header": return "端末の識別情報の形が正しくありません"
        case "unknown_pokemon": return "このポケモンはマスタにありません"
        case "request_too_large": return "入力が大きすぎます"
        case "master_unavailable": return "ポケモンのマスタを読み込めません"
        case PokeCalcError.Code.transport, PokeCalcError.Code.decode: return unavailable
        default: return errorFallback  // internal_error・not_found・未知の code
        }
    }
}
