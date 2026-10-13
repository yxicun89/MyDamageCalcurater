// G05TeamText: 構築画面の見た目の作り直し(G-05 第 2 弾。ADR-0537)で足す文言・純粋な判定の置き場。
//
// 値は ADR-0501「G-05 構築の作り直し」章の表どおり。Web の `web/src/i18n/team.ts` と同じ語にそろえる。

extension TeamLabels {
    /// 数字キーボードを閉じるボタン(SP の数値欄)。
    public static let keyboardDone = "完了"
    /// 一覧の読み込み中・マスタ読み込み中(静止したアイコン + 一言)。
    public static let loading = "読み込み中"
    /// 6 枠のタイルの格子の名前(読み上げ)。
    public static let slotsGroup = "メンバー"
    /// 空きのタイル(「+」)の見える名前・読み上げ名。例: 「2体目を追加」。
    public static func slotAdd(_ number: Int) -> String { "\(number)体目を追加" }
    /// 技の欄の見出し(旧「わざ」)。
    public static let movesHeading = "技"
    /// 技の枠が空のときの表示と、技ピッカー先頭の行(旧「技を選択」「外す」)。
    public static let moveNone = "(なし)"
    /// ニックネーム欄の名前(旧「ニックネーム(任意)」)。
    public static let nicknameField = "ニックネーム"
    /// SP の合計と残り(数値)。例: 「合計 66/66(残り 0)」。
    public static func spSummary(total: Int) -> String {
        "合計 \(total)/\(SPLimits.maxTotal)(残り \(SPLimits.maxTotal - total))"
    }
    /// SP の − / + ボタンの読み上げ名(docs/glossary.md「増やす/減らす」)。例: 「こうげきの能力ポイントを増やす」。
    public static func spIncrease(statName: String) -> String { "\(statName)の能力ポイントを増やす" }
    public static func spDecrease(statName: String) -> String { "\(statName)の能力ポイントを減らす" }
    /// SP の数値欄の読み上げ名。例: 「こうげきの能力ポイント」。
    public static func spField(statName: String) -> String { "\(statName)の能力ポイント" }
}

/// SP の − / + ボタンが押せるか、合計のバーの満ち具合(ViewModel の検査と同じ上限: 1 ステータス `SPLimits.maxPerStat`・合計 `SPLimits.maxTotal`)。
public enum TeamSPControl {
    /// + が押せる(その欄が上限未満、かつ合計が上限未満)。
    public static func canIncrease(value: Int, total: Int) -> Bool {
        value < SPLimits.maxPerStat && total < SPLimits.maxTotal
    }
    /// − が押せる(その欄が 0 より大きい)。
    public static func canDecrease(value: Int) -> Bool { value > 0 }
    /// バーの満ち具合(0...1。合計 ÷ 上限。範囲外は丸める)。
    public static func fillFraction(total: Int) -> Double {
        min(1, max(0, Double(total) / Double(SPLimits.maxTotal)))
    }
}
