// ReverseCandidateDisplay: 逆算候補(`ReverseCandidate` / `ReverseResult`)を画面に出す形へ
// 整形する(P6-2b・ADR-0010 §R3・R4、ADR-0501「P6-2b の受け入れ条件」3)。
//
// サーバーの値を丸め直さない・並べ替えない(候補の順序はサーバーの一致度順=ADR-0010 §R4 のまま)。
// 純粋な整形だけを持ち、View はここが作った文字列・値をそのまま描く(`BulkRowDisplay` と同じ方針)。

/// 逆算候補1件(`ReverseCandidate`)を画面向けに整形した値。
public struct ReverseCandidateDisplay: Identifiable, Equatable, Sendable {
    /// XCUITest の `reverseCandidateRow-<id>` に使う安定な ID(`<natureClass の rawValue>@<itemId。nil は "-">`)。
    public let id: String
    public let natureClass: NatureClass
    /// 「補正なし」「B上昇」のような性格クラスの表示名。
    public let natureClassLabel: String
    public let itemId: String?
    public let itemLabel: String
    /// 「B 20〜23」「B 17, 19〜32」のような SP の範囲(全区間を出す。1区間に畳まない。ADR-0010 §R3)。
    public let spRangeText: String
    /// 範囲に SP 0 / 32 が入るときだけ付く目安の名前(0 側が先。無ければ空)。
    public let guideNames: [String]
    public let exact: Bool
    /// 「観測と一致」/「一致なし(最も近い SP)」。
    public let matchLabel: String
    /// 「12.3〜15.6%」のような想定ダメージ幅。
    public let percentRangeText: String

    public init(candidate: ReverseCandidate, stat: StatKey, items: [Item]) {
        natureClass = candidate.natureClass
        natureClassLabel = Self.natureClassLabel(candidate.natureClass, stat: stat)
        itemId = candidate.itemId
        itemLabel = BulkRowDisplay.itemLabel(itemId: candidate.itemId, items: items)
        spRangeText = Self.spRangeText(candidate.ranges, stat: stat)
        guideNames = Self.guideNames(ranges: candidate.ranges, natureClass: candidate.natureClass, stat: stat)
        exact = candidate.exact
        matchLabel = Self.matchLabel(exact: candidate.exact)
        percentRangeText = BulkRowDisplay.percentRangeText(minPercent: candidate.minPercent, maxPercent: candidate.maxPercent)
        id = "\(candidate.natureClass.rawValue)@\(candidate.itemId ?? Self.noItemIDPlaceholder)"
    }

    /// `id` で「持ち物なし」を表す記号(`BulkRowDisplay` と同じ規則)。
    private static let noItemIDPlaceholder = "-"
    /// design.md の%幅・SP 範囲の区切り文字(U+301C 波ダッシュ。`BulkRowDisplay.percentRangeText` と同じ)。
    private static let rangeSeparator = "\u{301C}"

    // MARK: - 部品(純粋関数。ReverseCandidateDisplayTests が個別に固定する)

    /// ステータスキーの1文字表記(H/A/B/C/D/S)。
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

    /// 性格クラスの表示名。無補正はステータスによらず「補正なし」、上昇は「<統計>上昇」。
    public static func natureClassLabel(_ natureClass: NatureClass, stat: StatKey) -> String {
        switch natureClass {
        case .neutral: return "補正なし"
        case .plus: return "\(statLetter(stat))上昇"
        }
    }

    /// 「B 20〜23」「B 17, 19〜32」のような SP 範囲の文字列(区間は ", " で区切り、1点は端点だけを出す)。
    public static func spRangeText(_ ranges: [SPRange], stat: StatKey) -> String {
        let parts = ranges.map { range in
            range.min == range.max ? "\(range.min)" : "\(range.min)\(rangeSeparator)\(range.max)"
        }
        return "\(statLetter(stat)) \(parts.joined(separator: ", "))"
    }

    /// 「観測と一致」/「一致なし(最も近い SP)」。
    public static func matchLabel(exact: Bool) -> String {
        exact ? "観測と一致" : "一致なし(最も近い SP)"
    }

    /// 目安の名前(Web の ADR-0300 §7 と同じ規則。ADR-0010 §R3「表示層が付けてよい」)。
    /// `ranges` の最初の区間の下端が 0、最後の区間の上端が `SPLimits.maxPerStat` のときだけ、それぞれに1つ名前を付ける
    /// (0 側が先)。hp / spe は逆算の関連ステータスにならないので常に空。
    public static func guideNames(ranges: [SPRange], natureClass: NatureClass, stat: StatKey) -> [String] {
        let category: GuideStatCategory
        switch stat {
        case .def, .spd: category = .defender
        case .atk, .spa: category = .attacker
        case .hp, .spe: return []
        }
        var names: [String] = []
        if let first = ranges.first, first.min == 0 {
            names.append(guideName(category: category, atMax: false, natureClass: natureClass, stat: stat))
        }
        if let last = ranges.last, last.max == SPLimits.maxPerStat {
            names.append(guideName(category: category, atMax: true, natureClass: natureClass, stat: stat))
        }
        return names
    }

    /// `guideNames` が名付けるステータスの側(与えたダメージ = attacker 側の A/C、受けたダメージ = defender 側の H/B/D)。
    private enum GuideStatCategory {
        case defender
        case attacker
    }

    private static func guideName(category: GuideStatCategory, atMax: Bool, natureClass: NatureClass, stat: StatKey) -> String {
        let letter = statLetter(stat)
        switch (category, atMax, natureClass) {
        case (.defender, false, .neutral): return "H振り"
        case (.defender, false, .plus): return "H振り+\(letter)補正"
        case (.defender, true, .neutral): return "H\(letter)振り"
        case (.defender, true, .plus): return "H\(letter)特化"
        case (.attacker, false, .neutral): return "無振り"
        case (.attacker, false, .plus): return "\(letter)補正のみ"
        case (.attacker, true, .neutral): return "\(letter)振り"
        case (.attacker, true, .plus): return "\(letter)特化"
        }
    }
}

/// 逆算の結果全体(`ReverseResult`)を画面向けに整形した値。
public struct ReverseResultDisplay: Sendable {
    public let side: ReverseSide
    public let stat: StatKey
    public let assumedHPSP: Int
    /// サーバーの順のまま(並べ替えない・グループ化しない。ADR-0010 §R4)。
    public let candidates: [ReverseCandidateDisplay]
    public let exactCount: Int
    /// 「観測と一致: 2 件 / 候補 3 件」。
    public let exactCountText: String
    /// 「相手の HP の SP を 32(H32)と仮定した結果です」。defender だけに出す(attacker は相手の H を
    /// 計算に使わないので前提を出さない。ADR-0010 §R1)。
    public let premiseText: String?

    public init(result: ReverseResult, items: [Item]) {
        side = result.side
        stat = result.stat
        assumedHPSP = result.assumedHPSP
        candidates = result.candidates.map { ReverseCandidateDisplay(candidate: $0, stat: result.stat, items: items) }
        exactCount = result.exactCount
        exactCountText = "観測と一致: \(result.exactCount) 件 / 候補 \(result.candidates.count) 件"
        premiseText = result.side == .defender
            ? "相手の HP の SP を \(result.assumedHPSP)(H\(result.assumedHPSP))と仮定した結果です"
            : nil
    }
}
