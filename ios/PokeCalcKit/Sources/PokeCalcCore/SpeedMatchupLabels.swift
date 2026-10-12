// SpeedMatchupLabels: 素早さ画面(相手を選んで上を取る最低ライン。I-speed-4c。ADR-0610)の固定文言。
// Web(I-speed-4b)と同じ語・同じ規則にそろえる(違いは ADR-0610 追記に書く)。見出しの語は ADR-0610 §1 の「B の要約語」。
//
// spec-writer の足場: 定数(見出し・ラベル)は本物の文言。関数(値を埋め込む文言)は未実装で空文字を返す。
// 文言そのものは SpeedMatchupLabelsTests が固定する。implementer は関数を埋める(定数は変えない)。
// 既存の `SpeedLabels` の語(`preset`・`nature`・`scarf`・`rank`・`sp`・`trickRoom`・`errorMessage(forCode:)` など)は再利用する。

public enum SpeedMatchupLabels {
    // ---- 領域の見出し ----
    public static let opponentRegion = "相手のポケモン"
    public static let assumptionGroup = "相手の振りの想定"
    public static let currentGroup = "自分の今の振り"
    public static let currentToggle = "今の振りを入れる"
    public static let currentPresetGroup = "かんたん設定"
    public static let matchupRegion = "上を取る最低ライン"
    public static let reachRegion = "これ以上"

    // ---- 入力 ----
    public static let selfParalysis = "まひ(自分)"
    public static let opponentTailwind = "追い風(相手側)"
    public static let opponentParalysis = "まひ(相手)"
    public static let opponentUnselected = "相手を選ぶ"
    public static let selfUnselected = "自分を選ぶ"
    public static let assumptionMinimumNotice = "想定は少なくとも1つ必要です"

    // ---- 状態 ----
    public static let matchupHint = "自分と相手のポケモンを選ぶと、上を取る最低ラインを表示します"
    public static let matchupLoading = "最低ラインを計算中"
    public static let reachHint = "「今の振り」を入れると、どの族まで抜けるかを表示します"
    public static let reachLoading = "族の区切りを計算中"

    // ---- 連続技へのリンク(連続技が計算画面にあるときだけ出す) ----
    public static let comboLink = "この相手で連続技を計算"

    // ---- 行動順のバッジ(今の振りがあるとき。色だけに頼らず文字で出す) ----
    public static func orderBadge(_ order: SpeedTurnOrder) -> String {
        switch order {
        case .selfFirst: return "先に動く"
        case .opponentFirst: return "後に動く"
        case .speedTie: return "同速"
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

    /// 性格ごとの行の本文(トリックルームなし)。最小 SP が 0 なら「無振りでも抜ける」、それ以外は「S31以上で抜ける」、
    /// 区間なしは「届かない」。
    public static func firstLine(_ range: SpeedSPRange?) -> String {
        guard let range else { return "届かない" }
        return range.min == 0 ? "無振りでも抜ける" : "S\(range.min)以上で抜ける"
    }

    /// 性格ごとの行の本文(トリックルーム中)。「S12まで下を取れる」、区間なしは「下を取れない」。
    public static func trickRoomLine(_ range: SpeedSPRange?) -> String {
        guard let range else { return "下を取れない" }
        return "S\(range.max)まで下を取れる"
    }

    /// 同速の SP。1 点なら「S17で同速」、区間なら「S10〜S12で同速」。
    public static func tieLine(_ range: SpeedSPRange) -> String {
        range.min == range.max ? "S\(range.min)で同速" : "S\(range.min)〜S\(range.max)で同速"
    }

    private static func movesFirst(_ natures: [SpeedNatureLine], _ nature: SpeedNature) -> SpeedSPRange? {
        natures.first { $0.nature == nature }?.movesFirst
    }

    /// 見出し(トリックルームなし)。補正なしの最小 SP が 0 →「無振りで抜ける」/ 補正なしで足りる →「準速で抜ける」/
    /// 補正なしは届かず上昇で足りる →「最速なら S18で抜ける」/ どれも届かない →「最速でも届かない」。
    public static func headline(natures: [SpeedNatureLine]) -> String {
        if let neutral = movesFirst(natures, .neutral) {
            return neutral.min == 0 ? "無振りで抜ける" : "準速で抜ける"
        }
        if let plus = movesFirst(natures, .plus) { return "最速なら S\(plus.min)で抜ける" }
        return "最速でも届かない"
    }

    /// 見出し(トリックルーム中)。上昇でも 32 まで →「最速でも下を取れる」/ 補正なしで 32 まで →「準速でも下を取れる」/
    /// 補正なしの上限 k →「補正なしなら S12まで下を取れる」/ 下降だけ →「下降なら S5まで下を取れる」/ どれも無い →「どう振っても下を取れない」。
    public static func trickRoomHeadline(natures: [SpeedNatureLine]) -> String {
        let top = SPLimits.maxPerStat
        if movesFirst(natures, .plus)?.max == top { return "最速でも下を取れる" }
        if let neutral = movesFirst(natures, .neutral) {
            return neutral.max == top ? "準速でも下を取れる" : "補正なしなら S\(neutral.max)まで下を取れる"
        }
        if let minus = movesFirst(natures, .minus) { return "下降なら S\(minus.max)まで下を取れる" }
        return "どう振っても下を取れない"
    }

    // ---- 「これ以上」(族の区切り。P は調整名〈`SpeedLabels.preset`〉) ----

    /// 「最速 102族まで抜ける」。トリックルーム中は「最速 102族までは後に動く」。区間なしは「最速 はどの族も抜けない」
    /// (トリックルーム中は「最速 より後に動く族なし」)。
    public static func reachSlower(preset: String, range: SpeedBaseSpeedRange?, trickRoom: Bool) -> String {
        guard let range else { return trickRoom ? "\(preset) より後に動く族なし" : "\(preset) はどの族も抜けない" }
        return trickRoom ? "\(preset) \(range.max)族までは後に動く" : "\(preset) \(range.max)族まで抜ける"
    }

    /// 「最速 103族と同速」「最速 10〜12族と同速」。
    public static func reachTie(preset: String, range: SpeedBaseSpeedRange) -> String {
        range.min == range.max ? "\(preset) \(range.min)族と同速" : "\(preset) \(range.min)〜\(range.max)族と同速"
    }

    /// 「最速 104族以上に抜かれる」。トリックルーム中は「最速 104族以上には先に動ける」。区間なしは「最速 に抜かれる族なし」
    /// (トリックルーム中は「最速 には先に動けない」)。
    public static func reachFaster(preset: String, range: SpeedBaseSpeedRange?, trickRoom: Bool) -> String {
        guard let range else { return trickRoom ? "\(preset) には先に動けない" : "\(preset) に抜かれる族なし" }
        return trickRoom ? "\(preset) \(range.min)族以上には先に動ける" : "\(preset) \(range.min)族以上に抜かれる"
    }

    /// 「4体」
    public static func count(_ count: Int) -> String { "\(count)体" }

    /// 「例: テストデンキネズミ」
    public static func example(_ name: String) -> String { "例: \(name)" }
}
