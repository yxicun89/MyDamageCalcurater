// AttackerStatLabels: 計算画面の攻撃側「攻撃」「特攻」の2ブロックと技の絞り込みの文言(ADR-0518)。
// Web の `web/src/i18n/ja.ts`(`attackerStatText`・`calcScreenText`)と同じ語を使う。View は直書きせずここを参照する。

public enum AttackerStatLabels {
    /// ブロックの見出し(「攻撃」「特攻」)。
    public static func statName(_ stat: AttackStat) -> String {
        switch stat {
        case .atk: return "攻撃"
        case .spa: return "特攻"
        }
    }

    /// 選んだ技が使う側の見出しに足す語(色だけに頼らず、文字でも強調する)。
    public static let usedSuffix = "(この技で使用)"

    /// 見出しの文字列。`isUsed` のとき「攻撃(この技で使用)」。
    public static func heading(for stat: AttackStat, isUsed: Bool) -> String {
        statName(stat) + (isUsed ? usedSuffix : "")
    }

    /// プリセットの名前(「攻撃の調整」)。
    public static func presetGroupLabel(_ stat: AttackStat) -> String { statName(stat) + "の調整" }
    /// SP の数値入力の名前(「攻撃のSP」)。
    public static func spLabel(_ stat: AttackStat) -> String { statName(stat) + "のSP" }
    /// 性格補正の選択の名前(「攻撃の性格補正」)。
    public static func natureGroupLabel(_ stat: AttackStat) -> String { statName(stat) + "の性格補正" }

    /// 性格補正の選択肢の名前(上昇・補正なし・下降)。
    public static func modifierName(_ choice: NatureChoice) -> String {
        switch choice {
        case .up: return "上昇"
        case .neutral: return "補正なし"
        case .down: return "下降"
        }
    }

    /// プリセットのどれとも一致しない値のときの印。
    public static let custom = "カスタム"

    /// SP が 0〜32 の整数でないときの明示エラー(計算しない)。
    public static func spInvalid(_ stat: AttackStat) -> String { statName(stat) + "のSPは0〜32の整数で入力してください" }

    /// 補正の組み合わせに当たる性格がマスタに無いときの明示エラー(計算しない)。
    /// 文言は Web の出荷済み(`attackerStatText.natureUnresolved`。「マスタ」を使わない平易な語)に合わせる。
    public static let natureUnresolved = "この性格補正の組み合わせに当たる性格が、データにありません"

    /// 攻撃と特攻を同じ向きにできない理由(選べない選択肢の説明)。
    public static let sameDirectionReason = "攻撃と特攻の両方を上昇、または両方を下降にすることはできません"

    /// 変化技を選んだ状態(UI からは選べないが、安全網として計算せずに案内する)。
    public static let statusMoveNotice = "変化技はダメージを計算しません"

    /// ダメージを与える技を1つも覚えない種族のときの案内(計算しない)。
    public static let noDamagingMovesNotice = "このポケモンはダメージを与える技を覚えないため、計算できません"

    /// 構築の個体を呼んでいる間の注記(iOS だけ。入力欄の値は使っていない、操作すると構築の指定が外れる)。
    public static let teamSourceNotice = "構築の個体の値で計算しています。ここを変えると構築の指定を外します"

    /// 数値キーボード(SP 欄)のツールバーのボタン。数値キーボードには確定キーが無いので、閉じる手段として置く
    /// (識別子 `calcKeyboardDone`)。
    public static let keyboardDone = "完了"
}
