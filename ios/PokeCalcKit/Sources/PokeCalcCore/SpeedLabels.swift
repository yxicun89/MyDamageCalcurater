// SpeedLabels: 素早さ画面の共通の文言(P6-24。ADR-0503)。調整名・性格名・エラー文言・入口。
// 画面の見出し・カード・「これ以上」の文言は `SpeedMatchupLabels`(I-speed-4c。ADR-0610)。
// Web の `speedScreenText`・`speedPresetText`(web/src/i18n/speed.ts)と同じ語にそろえる(違いは ADR-0610 追記に書く)。

public enum SpeedLabels {
    // ---- 入口・見出し ----
    /// ルート画面の入口ボタン。
    public static let openButton = "素早さを比べる"
    public static let screenTitle = "素早さ比較"
    public static let selfRegion = "自分のポケモン"
    public static let loading = "読み込み中"

    // ---- 場 ----
    public static let fieldGroup = "場の状態"
    public static let trickRoom = "トリックルーム"
    public static let summarySeparator = "・"

    // ---- 自分のポケモン ----
    public static let pokemon = "ポケモン"
    public static let unselected = "未選択"
    public static let scarf = "こだわりスカーフ"
    public static let selfTailwind = "追い風(自分側)"
    public static let sp = "素早さ SP"
    public static let natureGroup = "性格補正"
    public static let rank = "ランク"
    /// ピッカー(シート)の検索欄の案内と、0件のとき。
    public static let pokemonSearchPrompt = "ポケモンの名前で絞り込み"
    public static let pokemonNoMatch = "一致するポケモンがいません"
    public static let close = "閉じる"
    /// SP のステッパーのアクセシビリティ名(ランクは `CalcConditionLabels.rankIncrement`/`rankDecrement` と同じ流儀)。
    public static let spIncrement = "素早さ SP を増やす"
    public static let spDecrement = "素早さ SP を減らす"

    /// 位置の入力方法の名前(用語集テスト〈GlossaryTests〉が語を見る。画面では使わない)。
    public static func mode(_ mode: SpeedInputMode) -> String {
        switch mode {
        case .preset: return "定番の振り方"
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

    // ---- 結果 ----
    /// 今の振りを入れたときの自分の素早さ(実数値)。
    public static func selfSpeed(_ speed: Int) -> String { "自分の素早さ \(speed)" }

    // ---- エラー(サーバーの英語 message は出さず、code から日本語にする) ----
    public static let unavailable = "素早さのサーバーに接続できません"
    public static let errorFallback = "素早さの計算に失敗しました"

    /// `PokeCalcError.code` → 日本語。契約の `ErrorCode` はすべて持つ。通信失敗・応答が読めないときは
    /// 「接続できません」(Web の `speed_unavailable` と同じ)。未知の code は汎用の文言。
    public static func errorMessage(forCode code: String) -> String {
        switch code {
        case "invalid_request": return "入力の形が正しくありません。値の範囲を確認してください"
        case "missing_header": return "端末の情報が送られていません。アプリを開き直してください"
        case "invalid_header": return "端末の情報が正しくありません。アプリを開き直してください"
        case "unknown_pokemon": return "このポケモンはデータにありません"
        case "request_too_large": return "入力が大きすぎます"
        case "master_unavailable": return "ポケモンのデータを読み込めません"
        // gateway が上流(speed)へ届かないときの 503(契約外の gateway の code)も、通信失敗と同じ文言にする。
        case PokeCalcError.Code.transport, PokeCalcError.Code.decode, "upstream_unavailable": return unavailable
        default: return errorFallback  // internal_error・not_found・未知の code
        }
    }
}
