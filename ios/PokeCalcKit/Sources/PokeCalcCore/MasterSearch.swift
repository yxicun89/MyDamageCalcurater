import Foundation

// MasterSearch: 種族・技(・持ち物)の検索の共通の語彙(issue #68。ADR-0501「issue #68 の受け入れ条件」
// 1〜3章・10章)。`CalcViewModel` / `ReverseViewModel` / `TeamEditViewModel` はどれも
// `searchSpecies` / `searchMoves` の `limit`(openapi の契約上の上限200)を「マスタ全件」ではなく
// 「検索結果の先頭ページ」として扱う。ここに1か所だけ定数を持つ(coding-rules §2)。

/// `searchSpecies` / `searchMoves` / `searchItems` の1ページの上限と、検索のデバウンス。
public enum MasterSearch {
    /// `api/openapi.yaml` の `searchSpecies`/`searchMoves`/`searchItems` の `limit` の `maximum`と
    /// 同じ値。**全件の意味は持たない**(2章): 件数がこれに達しても「まだ他にあるかもしれない」
    /// ことしか意味しない。
    public static let pageLimit = 200
    /// 1キーストロークごとの検索をまとめる待ち時間(3章)。issue #113 が共有のデバウンス/キャンセル
    /// 基盤を入れるまでの、この修正専用の素朴な実装。
    public static let debounceInterval: Duration = .milliseconds(250)
    /// 1回の入力操作で「選択中の技」を決めるために `move(id:)` を呼んでよい上限
    /// (ADR-0501「issue #68 の残り」4章)。learnset 全件を ID で実体化しないための歯止め。
    /// 値は構築の1体の技スロット数(`TeamLimits.maxMovesPerMember`)と同じにし、どの画面でも
    /// 1操作あたりの `getMove` の往復がこれを超えないようにする。
    public static let maxMoveLookupsPerSelection = TeamLimits.maxMovesPerMember
}

/// 種族・技の検索シートで使う案内文言(10章)。マスタに無い表示専用の文言なので
/// `DisplayLabels.swift` と同じ理由でコードに1か所持つ。
public enum MasterSearchLabels {
    /// 検索欄が空のときの案内。
    public static let prompt = "名前の先頭で検索"
    /// 検索結果が `MasterSearch.pageLimit` に達したときの案内(黙って切り捨てない。2章)。
    public static let truncated = "候補が多いので、名前を入力して絞り込んでください"
    /// 検索結果が0件のときの案内。
    public static let noMatch = "一致する候補がありません"
    /// 持ち物の一覧(検索欄の無い `Menu`)が `MasterSearch.pageLimit` に達したときの案内
    /// (ADR-0501「issue #68 の残り」6章。持ち物は名前で絞れないので `truncated` とは別の文言)。
    public static let itemsTruncated = "持ち物が多すぎて、一覧に出ていない持ち物があります"
}

// MARK: - View 層が3画面を同じ形で描けるようにするための名目上の抽象(10章の表)

/// 種族の検索欄(`.searchable()` の `List` を `.sheet` で出す)を持つ画面が満たす最小の API。
/// `CalcViewModel` / `ReverseViewModel` / `TeamEditViewModel` はどれもこの形を持つ(「たまたま似ている」
/// のではなく、ADR-0501「issue #68」10章が3画面共通の契約として定めているため。coding-rules §3)。
/// View 層(`ios/PokeCalc`)はこのプロトコルだけを介して1つの検索シート部品を3画面で共用する。
@MainActor
public protocol MasterSpeciesSearchProviding: AnyObject {
    /// 検索欄の文字(前後空白を落とした値)。
    var speciesQuery: String { get }
    /// いま画面に出す候補(空クエリなら起動時の先頭ページ、それ以外は直近の検索結果)。
    var speciesOptions: [SpeciesSummary] { get }
    var isSearchingSpecies: Bool { get }
    /// 直近の候補が `MasterSearch.pageLimit` に達した。
    var speciesSearchReachedLimit: Bool { get }
    /// 検索欄の文字を同期的に反映する。検索が要る(語が変わった)なら true。
    @discardableResult func setSpeciesQuery(_ text: String) -> Bool
    /// デバウンス → 検索 → 最新の世代のときだけ結果を反映する。
    func runSpeciesSearch() async
}

/// 技の検索欄を持つ画面が満たす最小の API(`MasterSpeciesSearchProviding` と同じ理由)。
/// 候補一覧(`moveOptions` / `moveOptionsByMember`)は画面ごとに形が違う(Team はメンバー単位)ので
/// ここには含めない。View が呼び出し元で渡す。
@MainActor
public protocol MasterMoveSearchProviding: AnyObject {
    var moveQuery: String { get }
    var isSearchingMoves: Bool { get }
    var moveSearchReachedLimit: Bool { get }
    @discardableResult func setMoveQuery(_ text: String) -> Bool
    func runMoveSearch() async
}
