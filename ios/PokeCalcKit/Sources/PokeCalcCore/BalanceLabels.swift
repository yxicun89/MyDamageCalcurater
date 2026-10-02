// BalanceLabels: タイプバランス画面の固定文言(P6-26。ADR-0505)。
// Web の `balanceScreenText`・`balanceLabelText`・`balanceErrorText`(web/src/i18n/ja.ts)と同じ語にそろえる。違いは次の 4 点だけ(ADR-0505 §9):
//  1. 入力は構築から選ぶ(メンバー・特性・技の手入力は無い)ので、入口・案内・エラー文の「選び直してください」は「構築を見直してください」。
//  2. 「ページを開き直してください」は「アプリを開き直してください」。
//  3. 特性が倍率を変えた旨の添え書き(Web にはまだ無い。iOS は色だけにしない)。
//  4. 技が1つも無い構築の案内・構築が0体の案内・構築を読み込めない案内(Web には無い iOS 固有の入口)。
// 倍率の数値は応答の文字列のまま(`×2`・`×3/4`)。語は応答の分類(category)から選び、値の範囲を iOS で判定し直さない(ADR-0303 §1 と同じ立場)。

public enum BalanceLabels {
    // ---- 入口・見出し ----
    /// ルート画面の入口ボタン。
    public static let openButton = "タイプバランス"
    public static let screenTitle = "タイプバランス"
    public static let teamRegion = "構築を選ぶ"
    public static let selectPrompt = "構築を選ぶと、防御相性と攻撃範囲を解析します"
    public static let noTeams = "まだ構築がありません。構築ビルダーで作ると解析できます"
    /// 構築の読み込みに失敗したとき(balance の失敗とは別。計算・balance には影響しない)。
    public static let teamLoadFailed = "構築を読み込めません"
    public static func memberCount(_ count: Int) -> String { "\(count)体" }
    /// メンバーが0体の構築を選んだとき(API は呼ばない)。
    public static let noMembers = "この構築にはポケモンがいません。構築ビルダーで追加してください"
    public static let loading = "解析中"
    public static let retry = "もう一度解析する"

    // ---- 防御相性(analyze) ----
    public static let defenseRegion = "防御相性"
    public static let teamSummaryRegion = "チームの集計"
    public static let attackTypeLabel = "攻撃タイプ"
    public static let weakLabel = "弱点"
    public static let quadWeakLabel = "うち×4"
    public static let resistLabel = "耐性"
    public static let immuneLabel = "無効"
    public static let neutralLabel = "等倍"

    /// `DefenseCategory` → 語(`quad_weak` も `weak` も「弱点」。×4 かどうかは倍率に出る)。
    public static func defenseCategoryWord(_ category: BalanceDefenseCategory) -> String {
        switch category {
        case .quadWeak, .weak: return "弱点"
        case .neutral: return "等倍"
        case .resist, .quadResist: return "耐性"
        case .immune: return "無効"
        }
    }

    /// 防御相性の倍率表示(「×2 弱点」。倍率は応答の文字列のまま)。
    public static func defenseText(multiplier: String, category: BalanceDefenseCategory) -> String {
        "×\(multiplier) \(defenseCategoryWord(category))"
    }

    /// 特性が倍率を変えたときの添え書き(`source == .ability` の行だけ。色だけにしない)。
    public static func abilityNote(_ effect: BalanceDefenseEffect) -> String {
        switch effect {
        case .immune: return "特性で無効"
        case .absorb: return "特性で吸収"
        case .multiplier: return "特性で倍率が変わる"
        case .none: return "特性の影響"
        }
    }

    /// チームの集計 1 行(例: `弱点 2(うち×4 1)・耐性 1・無効 0・等倍 3`)。数は応答のまま。
    public static func summaryText(weak: Int, quadWeak: Int, resist: Int, immune: Int, neutral: Int) -> String {
        "\(weakLabel) \(weak)(\(quadWeakLabel) \(quadWeak))・\(resistLabel) \(resist)・\(immuneLabel) \(immune)・\(neutralLabel) \(neutral)"
    }

    // ---- 攻撃範囲(coverage) ----
    public static let coverageRegion = "攻撃範囲"
    public static let teamCoverageRegion = "チームの攻撃範囲"
    public static let defenseTypeLabel = "防御タイプ"
    /// 技が1つも登録されていない構築では coverage を呼ばない。そのときの案内。
    public static let coverageNoMoves = "どのポケモンにも技が登録されていないため、攻撃範囲は出せません"
    /// 攻撃技が無い(`bestMultiplier == nil`)ときの表示。
    public static let coverageNoAttackMove = "攻撃技なし"

    /// `CoverageMultiplier` → 語。
    public static func coverageWord(_ multiplier: BalanceCoverageMultiplier) -> String {
        switch multiplier {
        case .zero: return "無効"
        case .half: return "いまひとつ"
        case .neutral: return "等倍"
        case .double: return "抜群"
        }
    }

    /// 攻撃範囲の倍率表示(「×2 抜群」。nil は「攻撃技なし」)。
    public static func coverageText(_ multiplier: BalanceCoverageMultiplier?) -> String {
        guard let multiplier else { return coverageNoAttackMove }
        return "×\(multiplier.rawValue) \(coverageWord(multiplier))"
    }

    /// チームの攻撃範囲の人数(例: `有効 4体・抜群 2体`)。数は応答のまま。
    public static func teamCoverageText(effectiveMembers: Int, superEffectiveMembers: Int) -> String {
        "有効 \(effectiveMembers)体・抜群 \(superEffectiveMembers)体"
    }

    // ---- エラー(サーバーの英語 message は出さず、code から日本語にする。ADR-0802) ----
    public static let unavailable = "タイプバランスの API に接続できません"
    public static let errorFallback = "タイプバランスを計算できませんでした。しばらくしてからもう一度お試しください"

    /// `PokeCalcError.code` → 日本語。契約の `ErrorCode` 10 値はすべて持つ(`BalanceContractSyncTests` が漏れを検出する)。
    /// 通信できない・応答が読めない・契約外のステータスは「接続できません」(Web の `balance_unavailable` と同じ)。未知の code・`not_found` は汎用。
    public static func errorMessage(forCode code: String) -> String {
        switch code {
        case "missing_request_context": return "端末の情報を送れませんでした。アプリを開き直してください"
        case "invalid_request": return "リクエストが正しくありません。構築を見直してください"
        case "request_too_large": return "入力が大きすぎます。構築のメンバーや技を減らしてください"
        case "unknown_pokemon": return "構築のポケモンがサーバーのマスタにありません。構築を見直してください"
        case "unknown_move": return "構築の技がサーバーのマスタにありません。構築を見直してください"
        case "unknown_ability": return "構築の特性がサーバーのマスタにありません。構築を見直してください"
        case "master_unavailable": return "サーバーのマスタを読み込めません。しばらくしてからもう一度お試しください"
        case "overloaded": return "サーバーが混み合っています。しばらくしてからもう一度お試しください"
        case "internal_error": return "サーバーでエラーが起きました。しばらくしてからもう一度お試しください"
        case PokeCalcError.Code.transport, PokeCalcError.Code.decode, PokeCalcError.Code.unexpectedStatus: return unavailable
        default: return errorFallback
        }
    }
}
