// G05CalcText: 計算画面の見た目の作り直し(G-05 第 5 弾。ADR-0541)で足す文言・判定の置き場。
//
// spec-writer の骨組み: 値は空、判定は常に「押せない」(または nil)。実装者が ADR-0501「G-05 計算の作り直し」章の表どおりに入れる
// (`G05CalcTextTests`・`G05CalcLogicTests` が失敗する)。宣言の名前・型は変えない。値を入れたあと、必要なら各 Labels の本体ファイルへ移してよい。
// 語は Web の `web/src/i18n/ja.ts`(`calcScreenText`・`attackerStatText`)と同じ。

/// 計算画面の見出し・状態の一言。
public enum CalcScreenText {
    /// 攻撃側のポケモンカードの見出し(Web の `attackerRegionLabel`)。
    public static let attackerHeading = ""
    /// 防御側のポケモンカードの見出し(Web の `defenderRegionLabel`)。
    public static let defenderHeading = ""
    /// 計算中の一言(Web の `loadingNotice`。静止したアイコンと並べる。「…」は付けない)。
    public static let loading = ""
    /// 結果の一覧の読み上げ名(Web の `resultsListLabel`)。
    public static let resultsListLabel = ""
    /// 折りたたみ「詳細」を開いているときの読み上げの値(素早さ画面の折りたたみと同じ語)。
    public static let disclosureOpened = ""
    /// 折りたたみ「詳細」を閉じているときの読み上げの値。
    public static let disclosureClosed = ""
}

extension AttackerStatLabels {
    /// SP の − ボタンの読み上げ名(「攻撃のSPを減らす」。用語集「共通の部品の語」の「減らす」)。
    public static func spDecrease(_ stat: AttackStat) -> String { "" }
    /// SP の + ボタンの読み上げ名(「攻撃のSPを増やす」)。
    public static func spIncrease(_ stat: AttackStat) -> String { "" }
}

/// 攻撃側の SP 欄の − / + ボタンが押せるかと、押したあとの文字(Web の `AttackerSPField` の `step`・`atMin`・`atMax` と同じ規則)。
/// 欄の不正な値(33・abc)を誤りとして出す仕様(ADR-0518)は変えない。ここは「ボタンを押したとき」だけを決める。
public enum AttackerSPStep {
    /// 1 ステータスの SP の範囲(`SPLimits.maxPerStat`)。
    public static let minimum = 0
    public static let maximum = 32

    /// − が押せる(欄が 0 以下の整数でない。空欄は 0 として扱うので押せない。整数でない文字は押せる)。
    public static func canDecrease(text: String) -> Bool { false }
    /// + が押せる(欄が 32 以上の整数でない)。
    public static func canIncrease(text: String) -> Bool { false }
    /// 押したあとの欄の文字(0〜32 に丸める。整数でない文字は 0 から数える)。
    public static func stepped(text: String, by delta: Int) -> String { text }
}

extension CalcViewModel {
    /// 攻撃側のカードの持ち物の印(選んだ持ち物の名前。メガ種族は固定のストーン名。持ち物なしは nil)。
    public var attackerItemMark: String? { nil }
    /// 防御側のカードの持ち物の印(メガ種族だけ固定のストーン名。防御側の持ち物の比較は結果の下のチップで行うので、それ以外は nil)。
    public var defenderItemMark: String? { nil }
}
