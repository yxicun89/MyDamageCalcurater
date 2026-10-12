// SpeedMatchupLabels: 素早さ画面(相手を選んで先に動く最低ライン。I-speed-4c。ADR-0610)の固定文言。
// Web(`web/src/i18n/speed.ts` の `speedScreenText`)と同じ語・同じ空白・同じ規則にそろえる。
// iOS 固有で残す差(「アプリ」と「ページ」など)は ADR-0610 §8 に列挙する。見出しの語は ADR-0610 §1 の「B の要約語」。
// 既存の `SpeedLabels` の語(`preset`・`nature`・`scarf`・`rank`・`sp`・`trickRoom`・`errorMessage(forCode:)` など)は再利用する。

public enum SpeedMatchupLabels {
    // ---- 領域の見出し ----
    public static let opponentRegion = "相手のポケモン"
    public static let assumptionGroup = "相手の振り方の想定"
    public static let currentGroup = "自分の今の振り"
    public static let currentToggle = "今の振りも入れる"
    public static let currentPresetGroup = "かんたん設定"
    public static let matchupRegion = "先に動くための最低ライン"
    public static let matchupRegionTrickRoom = "先に動ける上限(トリックルーム中)"
    public static let reachRegion = "これ以上"

    // ---- 入力 ----
    public static let selfParalysis = "まひ(自分)"
    public static let opponentTailwind = "追い風(相手側)"
    public static let opponentParalysis = "まひ(相手)"
    public static let opponentUnselected = "相手を選ぶ"
    public static let selfUnselected = "自分を選ぶ"
    public static let assumptionMinimumNotice = "想定は少なくとも1つ選びます"

    // ---- 説明の一言(Web の speedScreenText と同じ文) ----
    public static let currentHint = "入れると、今の素早さで先に動けるかと、「これ以上」の一覧が出ます"
    public static let assumptionHint = "相手がどこまで素早さを振っているかは分からないので、いくつか並べて見ます"
    public static let trickRoomHint = "トリックルーム中は、素早さが低いほうが先に動きます"
    public static let reachIntro =
        "今の素早さで、素早さの基本値がどこまでのポケモンを抜けるかの一覧です(「100 族」は、素早さの基本値が 100 のポケモンのことです)"

    // ---- 状態 ----
    public static let matchupHint = "自分と相手のポケモンを選ぶと、相手より先に動くために必要な素早さの振り方を表示します"
    public static let matchupLoading = "最低ラインを計算中"
    public static let reachHint = "「今の振りも入れる」をオンにすると、ここに「これ以上」の一覧が出ます"
    public static let reachLoading = "「これ以上」を計算中"

    // ---- 連続技へのリンク(連続技が計算画面にあるときだけ出す) ----
    public static let comboLink = "この相手で連続技を計算"

    // ---- 行動順のバッジ(今の振りがあるとき。色だけに頼らず文字で出す) ----
    public static func orderBadge(_ order: SpeedTurnOrder) -> String {
        switch order {
        case .selfFirst: return "今の振りなら先に動く"
        case .opponentFirst: return "今の振りなら後に動く"
        case .speedTie: return "今の振りなら同速"
        }
    }

    // ---- 値を埋め込む文言 ----

    /// 「相手の素早さ 200」
    public static func opponentSpeed(_ speed: Int) -> String { "相手の素早さ \(speed)" }

    /// カスタム想定の名前。「カスタム(S20・上昇)」「カスタム(S32・補正なし・+1・スカーフ)」(ランク 0 とスカーフなしは省く)。
    public static func customAssumption(_ custom: SpeedCustomAssumption) -> String {
        var parts = ["S\(custom.sp)", SpeedLabels.nature(custom.nature)]
        if custom.rank != 0 { parts.append(custom.rank > 0 ? "+\(custom.rank)" : "\(custom.rank)") }
        if custom.scarf { parts.append("スカーフ") }
        return "カスタム(" + parts.joined(separator: "・") + ")"
    }

    /// 性格ごとの行の本文(トリックルームなし)。最小 SP が 0 なら「振らなくても抜ける」、それ以外は「S31 以上で抜ける」、
    /// 区間なしは「上を取れません」。
    public static func firstLine(_ range: SpeedSPRange?) -> String {
        guard let range else { return "上を取れません" }
        return range.min == 0 ? "振らなくても抜ける" : "S\(range.min) 以上で抜ける"
    }

    /// 性格ごとの行の本文(トリックルーム中)。「S12 まで振っても先に動ける」、区間なしは「先に動けません」。
    public static func trickRoomLine(_ range: SpeedSPRange?) -> String {
        guard let range else { return "先に動けません" }
        return "S\(range.max) まで振っても先に動ける"
    }

    /// 同速の SP。1 点なら「S17 で同速」、区間なら「S10〜12 で同速」。
    public static func tieLine(_ range: SpeedSPRange) -> String {
        range.min == range.max ? "S\(range.min) で同速" : "S\(range.min)〜\(range.max) で同速"
    }

    private static func movesFirst(_ natures: [SpeedNatureLine], _ nature: SpeedNature) -> SpeedSPRange? {
        natures.first { $0.nature == nature }?.movesFirst
    }

    /// 見出し(トリックルームなし)。補正なしの最小 SP が 0 →「無振りで抜ける」/ 補正なしで足りる →「準速で抜ける」/
    /// 補正なしは届かず上昇で足りる →「最速なら S18 以上で抜ける」/ どれも届かない →「どう振っても上を取れません」。
    public static func headline(natures: [SpeedNatureLine]) -> String {
        if let neutral = movesFirst(natures, .neutral) {
            return neutral.min == 0 ? "無振りで抜ける" : "準速で抜ける"
        }
        if let plus = movesFirst(natures, .plus) { return "最速なら S\(plus.min) 以上で抜ける" }
        return "どう振っても上を取れません"
    }

    /// 見出し(トリックルーム中)。上昇でも 32 まで →「最速でも先に動ける」/ 補正なしで 32 まで →「準速でも先に動ける」/
    /// 補正なしの上限 k →「S12 まで振っても先に動ける」/ 下降だけ →「下降なら S5 まで振っても先に動ける」/
    /// どれも無い →「どう振っても先に動けません」。
    public static func trickRoomHeadline(natures: [SpeedNatureLine]) -> String {
        let top = SPLimits.maxPerStat
        if movesFirst(natures, .plus)?.max == top { return "最速でも先に動ける" }
        if let neutral = movesFirst(natures, .neutral) {
            return neutral.max == top ? "準速でも先に動ける" : "S\(neutral.max) まで振っても先に動ける"
        }
        if let minus = movesFirst(natures, .minus) { return "下降なら S\(minus.max) まで振っても先に動ける" }
        return "どう振っても先に動けません"
    }

    // ---- 「これ以上」(族の区切り。P は調整名〈`SpeedLabels.preset`〉) ----

    /// 「最速 102 族まで抜ける」。トリックルーム中は「最速 102 族までは後に動く」。区間なしは「抜ける族はありません」
    /// (トリックルーム中は「後に動く族はありません」)。調整名は区間があるときだけ付ける。
    public static func reachSlower(preset: String, range: SpeedBaseSpeedRange?, trickRoom: Bool) -> String {
        guard let range else { return trickRoom ? "後に動く族はありません" : "抜ける族はありません" }
        return trickRoom ? "\(preset) \(range.max) 族までは後に動く" : "\(preset) \(range.max) 族まで抜ける"
    }

    /// 「最速 103 族と同速」「最速 10〜12 族と同速」。
    public static func reachTie(preset: String, range: SpeedBaseSpeedRange) -> String {
        range.min == range.max ? "\(preset) \(range.min) 族と同速" : "\(preset) \(range.min)〜\(range.max) 族と同速"
    }

    /// 「最速 104 族以上に抜かれる」。トリックルーム中は「最速 104 族以上には先に動ける」。区間なしは「抜かれる族はありません」
    /// (トリックルーム中は「先に動ける族はありません」)。
    public static func reachFaster(preset: String, range: SpeedBaseSpeedRange?, trickRoom: Bool) -> String {
        guard let range else { return trickRoom ? "先に動ける族はありません" : "抜かれる族はありません" }
        return trickRoom ? "\(preset) \(range.min) 族以上には先に動ける" : "\(preset) \(range.min) 族以上に抜かれる"
    }

    /// 「4体」
    public static func count(_ count: Int) -> String { "\(count)体" }

    /// 「例: テストデンキネズミ」
    public static func example(_ name: String) -> String { "例: \(name)" }
}
