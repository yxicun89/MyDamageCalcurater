// G05ReverseText: 逆算画面の見た目の作り直し(G-05 第 3 弾。ADR-0539)で足す文言・純粋な判定の置き場。
//
// 値は ADR-0501「G-05 逆算の作り直し」章の表どおり。語は Web の `web/src/i18n/ja.ts`(`reverseScreenText`・`reverseResultText`)と同じ。
// 宣言の名前・型は変えない。

/// 逆算画面の語(Web の `reverseScreenText`・`reverseResultText` と同じ語)。
public enum ReverseLabels {
    /// カードの見出し(Web の `myRegionLabel`)。
    public static let myRegion = "自分"
    /// カードの見出し(Web の `theirRegionLabel`)。
    public static let opponentRegion = "相手"
    /// 観測欄の単位(`.percent` = 「%」、`.damage` = 「HP」。Web の `percentUnitLabel`・`damageUnitLabel`。旧: 「ダメージ」)。
    public static func unitLabel(_ kind: ObservationKind) -> String { kind == .percent ? "%" : "HP" }
    /// 観測欄の見える名前(Web の `observationLabel`)。例: 「ダメージ1」。
    public static func observationLabel(_ number: Int) -> String { "ダメージ\(number)" }
    /// 観測欄の下の単位の 1 行(Web の `observationHintLabel`)。`.defender` = 「相手の HP が減った割合(%)」、
    /// `.attacker` = 「自分の HP が減った実数値(HP)」。
    public static func observationHint(side: ReverseSide) -> String {
        switch side {
        case .defender: return "相手の HP が減った割合(%)"
        case .attacker: return "自分の HP が減った実数値(HP)"
        }
    }
    /// 候補の予測%に添える短いラベル(Web の `predictedPercentLabel`)。
    public static let predictedPercent = "予測"
    /// 観測を厳密に説明できない候補のバッジ(Web の `closeCandidateLabel`。用語集の「ほぼ合う候補」)。
    public static let closeCandidate = "ほぼ合う候補"
    /// 全候補が不一致のときに SP の範囲へ添えるバッジ(Web の `referenceRangeLabel`)。
    public static let referenceRange = "参考"
    /// 厳密に合う候補が 1 件も無いときの案内(状態の通知なので文。Web の `noExactCandidateNotice`)。
    public static let noExactNotice = "入力したダメージにぴったり合う振り方が見つかりません(技・持ち物・入力した値を確かめてください)"
    /// 逆算の計算中(静止したアイコン + 一言。Web の `loadingNotice`)。
    public static let loading = "計算中"
    /// 防御側の結果に添える前提の 1 行(Web の `assumedHpNote`)。例: 「H32 を仮定した結果です」。
    public static func assumedHP(_ sp: Int) -> String { "H\(sp) を仮定した結果です" }
    /// 一致件数の数値行(旧: 「入力したダメージと一致: 2 件 / 候補 3 件」)。例: 「一致 2 件 / 候補 3 件」。
    public static func exactCount(exact: Int, total: Int) -> String { "一致 \(exact) 件 / 候補 \(total) 件" }
    /// 厳密に合う候補が 1 件も無く、候補は 1 件以上ある(案内と「参考」バッジを出す条件。判定は engine の `exactCount` のまま)。
    public static func hasNoExactCandidate(exactCount: Int, candidateCount: Int) -> Bool { exactCount == 0 && candidateCount > 0 }
}
