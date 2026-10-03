import Foundation

// BalanceLabels: タイプバランスの倍率・見出し・エラーの日本語(ADR-0415、docs/type-balance-design.md §10)。
// Web の web/src/domain/balanceLabels.ts と web/src/i18n/ja.ts(balanceLabelText・balanceScreenText・balanceErrorText)と同じ語。
// 倍率の文字列は balance の応答のまま使い、語は応答の category から選ぶ(値の範囲から判定し直さない)。

public enum BalanceLabels {
    /// 防御相性の倍率表示(「×2 弱点」)。
    public static func defenseLabel(multiplier: String, category: BalanceDefenseCategory) -> String {
        "\(multiplierLabel(multiplier)) \(defenseWord(category))"
    }

    /// 攻撃範囲の倍率表示(「×2 抜群」)。攻撃技が無ければ nil で「攻撃技なし」。
    public static func coverageLabel(_ multiplier: BalanceCoverageMultiplier?) -> String {
        guard let multiplier else { return BalanceScreenText.coverageNoAttackMove }
        return "\(multiplierLabel(multiplier.rawValue)) \(coverageWord(multiplier))"
    }

    /// 語を添えない倍率表示(「×3/2」)。
    public static func multiplierLabel(_ multiplier: String) -> String {
        "×\(multiplier)"
    }

    /// 仮想敵の受ける/与える倍率(`MatchupMultiplier`)。攻撃技が無ければ nil で「攻撃技なし」。
    /// category を持たないので、倍率の範囲から弱点・耐性を判定し直さない(語を添えない)。
    public static func matchupMultiplierLabel(_ multiplier: String?) -> String {
        guard let multiplier else { return BalanceScreenText.coverageNoAttackMove }
        return multiplierLabel(multiplier)
    }

    /// 「安全に受けられるか」(`ThreatMatchup.safe`)。応答の真偽値をそのまま語にする。
    public static func safeLabel(_ safe: Bool) -> String {
        safe ? BalanceScreenText.safe : BalanceScreenText.unsafe
    }

    /// 「抜群が取れるか」(`ThreatMatchup.superEffective`)。応答の真偽値をそのまま語にする。
    public static func superEffectiveLabel(_ superEffective: Bool) -> String {
        superEffective ? BalanceScreenText.superEffective : BalanceScreenText.notSuperEffective
    }

    /// タイプの一覧(「ほのお・みず」。空なら「なし」)。
    public static func typeListText(_ types: [PokeType]) -> String {
        types.isEmpty
            ? BalanceScreenText.noneLabel
            : types.map(PokeTypeLabel.japaneseName(for:)).joined(separator: BalanceScreenText.listSeparator)
    }

    private static func defenseWord(_ category: BalanceDefenseCategory) -> String {
        switch category {
        case .quadWeak, .weak: return "弱点"
        case .neutral: return "等倍"
        case .resist, .quadResist: return "耐性"
        case .immune: return "無効"
        }
    }

    private static func coverageWord(_ multiplier: BalanceCoverageMultiplier) -> String {
        switch multiplier {
        case .zero: return "無効"
        case .half: return "いまひとつ"
        case .neutral: return "等倍"
        case .double: return "抜群"
        }
    }
}

/// 画面の固定文言(Web の `balanceScreenText` と同じ語)。
public enum BalanceScreenText {
    public static let screenTitle = "タイプバランス"
    public static let addMemberLabel = "メンバーを追加"
    public static let loadTeamLabel = "構築から読み込む"
    public static let abilityLabel = "特性"
    public static let noAbilityOption = "なし"
    public static let noneLabel = "なし"
    public static let loadingNotice = "計算中"
    public static let emptyNotice = "メンバーを追加すると、防御相性と攻撃範囲が表示されます"
    public static let defenseTableLabel = "防御相性"
    public static let teamSummaryTableLabel = "チームの集計"
    public static let coverageTableLabel = "攻撃範囲"
    public static let attackTypeColumnLabel = "攻撃タイプ"
    public static let weakColumnLabel = "弱点"
    public static let quadWeakColumnLabel = "うち×4"
    public static let resistColumnLabel = "耐性"
    public static let immuneColumnLabel = "無効"
    public static let neutralColumnLabel = "等倍"
    public static let defenseTypeColumnLabel = "防御タイプ"
    public static let bestMultiplierColumnLabel = "最大倍率"
    public static let effectiveColumnLabel = "有効"
    public static let superEffectiveColumnLabel = "抜群"
    public static let coverageNoAttackMove = "攻撃技なし"
    public static let teamTooManyMembers = "メンバーは\(TeamLimits.maxMembers)体までです"
    public static let abilityEffectNote = "特性"
    public static let emptyTeamLoadNotice = "保存済みの構築がありません"
    public static let memberColumnLabel = "メンバー"
    public static let attackTypesLabel = "攻撃タイプ"
    public static let duplicateMove = "同じ技が重複しています"
    public static let tooManyMoves = "技は\(TeamLimits.maxMovesPerMember)個までです"

    // 第3段: 仮想敵(Web の balanceScreenText と同じ語)
    public static let threatsRegionLabel = "仮想敵"
    public static let addThreatLabel = "仮想敵を追加"
    public static let threatMatchupTableLabel = "相性"
    public static let incomingColumnLabel = "受ける倍率"
    public static let outgoingColumnLabel = "与える倍率"
    public static let safeColumnLabel = "安全"
    public static let safe = "安全"
    public static let unsafe = "注意"
    public static let superEffective = "抜群"
    public static let notSuperEffective = "ふつう"
    public static let threatsLoadingNotice = "仮想敵を計算中"
    public static let tooManyThreats = "仮想敵は\(TeamLimits.maxMembers)体までです"
    public static let emptyThreatsNotice = "仮想敵を追加すると、自分のメンバーとの相性が表示されます"
    // 第3段: おすすめタイプ(Web と同じ語)
    public static let recommendationsRegionLabel = "おすすめタイプ"
    public static let recommendationsLoadingNotice = "おすすめタイプを計算中"
    public static let candidatesTableLabel = "おすすめタイプの候補"
    public static let typesColumnLabel = "タイプ"
    public static let defenseCoveredColumnLabel = "ふさぐ防御の穴"
    public static let offenseCoveredColumnLabel = "ふさぐ攻撃範囲の穴"
    public static let pokemonColumnLabel = "ポケモン"
    public static let abilityOptionsTableLabel = "特性で補えるポケモン"
    public static let listSeparator = "・"
    // 第3段: 技範囲チェッカー(Web に画面が無いので iOS で決めた文言)
    public static let moveRangeRegionLabel = "技範囲チェッカー"
    public static let moveRangeAddMoveLabel = "技を追加"
    public static let moveRangeEmptyNotice = "技を1つ以上選ぶと、攻撃範囲と受けられるポケモンが表示されます"
    public static let moveRangeLoadingNotice = "技範囲を計算中"
    public static let moveRangeTypeChartLabel = "技構成の攻撃範囲"
    public static let walledByLabel = "半減以下で受けられるポケモン"
    public static let walledByAbilityLabel = "特性で半減以下にできるポケモン"
    public static let noWalledNotice = "受けられるポケモンはいません"
    public static let retryLabel = "再計算"

    public static func threatGroupLabel(_ number: Int) -> String { "仮想敵\(number)" }
    public static func removeThreatLabel(_ number: Int) -> String { "仮想敵\(number)を削除" }
    public static func threatRegionLabel(_ number: Int, name: String) -> String { "仮想敵\(number)(\(name))" }
    public static func safeMembersLabel(_ count: Int) -> String { "安全に受けられる \(count)人" }
    public static func superEffectiveMembersLabel(_ count: Int) -> String { "抜群を取れる \(count)人" }
    public static func defenseHolesLabel(_ list: String) -> String { "防御の穴: \(list)" }
    public static func offenseHolesLabel(_ list: String) -> String { "攻撃範囲の穴: \(list)" }
    public static func abilityOptionEntryLabel(name: String, ability: String, multiplier: String) -> String {
        "\(name)(\(ability) \(multiplier))"
    }
    public static func moreCountLabel(_ count: Int) -> String { "ほか \(count)件" }
    public static func moveRangeMoveLabel(_ slot: Int) -> String { "技\(slot)" }

    public static func weakSummary(weak: Int, quadWeak: Int) -> String { "弱点 \(weak)(うち×4 \(quadWeak))" }
    public static func resistSummary(_ count: Int) -> String { "耐性 \(count)" }
    public static func immuneSummary(_ count: Int) -> String { "無効 \(count)" }
    public static func neutralSummary(_ count: Int) -> String { "等倍 \(count)" }
    public static func effectiveSummary(_ count: Int) -> String { "有効 \(count)人" }
    public static func superEffectiveSummary(_ count: Int) -> String { "抜群 \(count)人" }
    public static func memberGroupLabel(_ number: Int) -> String { "メンバー\(number)" }
    public static func removeMemberLabel(_ number: Int) -> String { "メンバー\(number)を削除" }
    public static func moveLabel(_ slot: Int) -> String { "技\(slot)" }
}

/// balance の `ErrorCode` → 日本語(Web の `balanceErrorText` と同じ。サーバーの英語の `message` は画面に出さない)。
/// Web との差は、端末情報の不備の「ページを開き直してください」を「アプリを開き直してください」にした1点だけ。
public enum BalanceErrorText {
    /// 通信失敗・デコード失敗・接続先なしの写像先のコード。
    public static let unavailableCode = UnavailableBalanceService.unavailableCode

    static let fallback = "タイプバランスを計算できませんでした。しばらくしてからもう一度お試しください"
    private static let headerProblem = "端末の情報を送れませんでした。アプリを開き直してください"

    public static func message(forCode code: String) -> String {
        switch code {
        // 0.8.0 の missing_header / invalid_header と、0.7.0 の missing_request_context(旧)は同じ文言。
        case "missing_header", "invalid_header", "missing_request_context":
            return headerProblem
        case "invalid_request":
            return "リクエストが正しくありません。入力を見直してください"
        case "request_too_large":
            return "入力が大きすぎます。メンバーや技を減らしてください"
        case "unknown_pokemon":
            return "選んだポケモンがサーバーのマスタにありません。選び直してください"
        case "unknown_move":
            return "選んだ技がサーバーのマスタにありません。選び直してください"
        case "unknown_ability":
            return "選んだ特性がサーバーのマスタにありません。選び直してください"
        case "master_unavailable":
            return "サーバーのマスタを読み込めません。しばらくしてからもう一度お試しください"
        case "overloaded":
            return "サーバーが混み合っています。しばらくしてからもう一度お試しください"
        case "internal_error":
            return "サーバーでエラーが起きました。しばらくしてからもう一度お試しください"
        case unavailableCode:
            return "タイプバランスの API に接続できません"
        default:
            return fallback
        }
    }
}

/// タイプバランス画面が出すエラー。`message` は常に `BalanceErrorText` から引く。
public struct BalanceScreenError: Equatable, Sendable {
    public let code: String
    public var message: String { BalanceErrorText.message(forCode: code) }

    /// `PokeCalcError` の通信失敗・デコード失敗は `balance_unavailable`、サービスのコードはそのコード。
    /// それ以外のエラーは未知のコード(汎用文言)。
    public init(_ error: any Error) {
        guard let pokeCalcError = error as? PokeCalcError else {
            code = "client_unexpected_error"
            return
        }
        switch pokeCalcError.code {
        case PokeCalcError.Code.transport, PokeCalcError.Code.decode:
            code = BalanceErrorText.unavailableCode
        default:
            code = pokeCalcError.code
        }
    }
}

/// 長い一覧・名前の解決の上限(端末で出し切らない・マスタを引き過ぎない。ADR-0415 §8)。
public enum BalanceDisplayLimits {
    /// 1つの一覧(候補のポケモン・受けられるポケモン)に出す件数。超えた分は「ほか N件」。
    public static let pokemonPerList = 20
    /// 応答に出たポケモンから特性名を引く(`species(key:)`)最大の件数。引けない分は特性 ID のまま出す。
    public static let abilityNameResolveLimit = 40
}
