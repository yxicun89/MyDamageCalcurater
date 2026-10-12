// G05Text: 見た目の作り直し(G-05。ADR-0536)で足す文言の置き場。
//
// spec-writer の骨組み: 値は実装者が ADR-0501「G-05 このアプリについて・お気に入り・履歴」章の表どおりに入れる
// (今は空。`G05AboutFavoritesTextTests` が失敗する)。実装者は値を入れたあと、必要なら各 Labels の本体ファイルへ移してよい
// (宣言の名前・型は変えない)。

/// 共通の部品の語(docs/glossary.md「共通の部品の語」。Web の `web/src/i18n/ui.ts` と同じ語)。
public enum CommonPartsText {
    /// 説明ボタンの見える名前。
    public static let help = "説明"
    /// シートの閉じるボタン(アイコンだけ)の読み上げ名。
    public static let close = "閉じる"
}

extension AboutText {
    /// 「非公式表示」節の見出し(Web の `aboutText.unofficialHeading`)。
    public static let unofficialHeading = "非公式表示"
    /// 「データの出典」節の見出し。
    public static let dataSourcesHeading = "データの出典"
}

extension FavoritesLabels {
    /// 行の見える文字(読み上げ名は `useAccessibilityLabel`・`removeAccessibilityLabel`)。
    public static let useButton = "使う"
    public static let removeButton = "削除"
    /// calc の無い旧お気に入りの行に添える短い印。
    public static let legacyFavoriteHint = "個体だけ(計算の条件なし)"
    /// 一覧の件数(例: 「3/100件」。分母は `RequestLimits.maxFavorites`)。
    public static func countText(_ count: Int) -> String { "\(count)/\(RequestLimits.maxFavorites)件" }
    /// 削除の 1 段目で出す確認(取り消せない操作の確認なので文を置いてよい)。
    public static func removeConfirmMessage(title: String) -> String { "「\(title)」をお気に入りから外します。よろしいですか?" }
    /// 確認の「削除する」(危険ボタン)。
    public static let removeConfirmAction = "削除する"
    /// 確認の取りやめ。
    public static let removeCancelAction = "やめる"
    /// 削除ボタンの読み上げ名(見える文字「削除」を含む)。
    public static func removeAccessibilityLabel(title: String) -> String { "「\(title)」を削除" }
}

extension DeviceDataText {
    /// `explanation` を 1 文ずつ(句点で区切り、句点は文に残す)に分けた箇条。文面・順序は変えない。
    public static var explanationBullets: [String] {
        explanation.flatMap { paragraph in
            paragraph.split(separator: "。", omittingEmptySubsequences: true).map { "\($0)。" }
        }
    }
}

extension FavoritesLabels {
    /// 空の状態の行動のボタン(計算画面へ移る)。
    public static let goToCalcButton = "計算へ"
}

extension CalcHistoryLabels {
    /// 計算履歴の最初の読み込み中(静止したアイコン + 一言)。
    public static let loading = "計算履歴を読み込んでいます…"
}
