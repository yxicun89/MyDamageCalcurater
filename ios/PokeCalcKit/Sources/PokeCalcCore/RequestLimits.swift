/// 計算・逆算の要求に含められる件数の上限(issue #110。ADR-0501「issue #110 の受け入れ条件(iOS 側)」
/// 2章・8章)。
///
/// 正は `api/openapi.yaml` の `maxItems`(`ReverseRequest.observations` / `ReverseRequest.itemCandidates` /
/// `BulkCalcRequest.itemVariants`)。この enum はその写しで、ViewModel・View はここだけを参照し、
/// 件数を直書きしない(coding-rules §2)。契約と写しのずれは `ios/scripts/check-request-limits.sh`
/// (`make ios-test` の一部)が検出する。
public enum RequestLimits {
    /// `ReverseRequest.observations.maxItems`。
    public static let maxObservations = 16
    /// `ReverseRequest.itemCandidates.maxItems`。
    public static let maxItemCandidates = 64
    /// `BulkCalcRequest.itemVariants.maxItems`。
    public static let maxItemVariants = 64

    /// 送る配列の先頭に入る null(持ち物なし)の1件を除いた、トグルで選べる ID の数
    /// (ADR「issue #110」3章)。
    public static let maxSelectableItemCandidates = maxItemCandidates - 1
    /// 同上(計算画面の比較トグル)。
    public static let maxSelectableItemVariants = maxItemVariants - 1

    /// `getMovesByIds` の `ids`(クエリパラメータ)の `maxItems`。1回の呼び出しに渡せる技 ID の上限
    /// (ADR-0501「getMovesByIds による構築編集の技の一括解決」1章)。`PokeCalcService.moves(ids:)` は
    /// これを超える集合をこの件数ずつに分割して複数回呼ぶ。`ReverseRequest.observations` 等と違い、
    /// `components.schemas` のプロパティではなく `paths./api/pokedex/moves/batch.get` のクエリパラメータ
    /// なので、`check-request-limits.sh` の照合はスキーマの `maxItems` とは別の経路で行う。
    public static let maxMoveBatchIds = 64

    /// `listMoveLearners`(`GET /api/pokedex/moves/{key}/learners`)の `limit` の `default`。調整画面の
    /// 「この技を覚えるポケモン」の1ページの件数(Web の `LEARNERS_PAGE_SIZE` と同じ。ADR-0502 §7)。
    /// 返った件数がこれちょうどなら「続きを読み込む」を出す(総数は返らない。ADR-0251 §1)。
    public static let moveLearnersPageSize = 50
    /// `AdjustHits` の `maximum`(engine の `MaxAdjustHits`)。調整画面の発数の選択肢の上限(ADR-0502 §2)。
    public static let maxAdjustHits = 10
}

/// 件数の上限に達したことを画面に出す文言(`MasterSearchLabels` と同じ理由でコードに1か所持つ)。
/// 件数は `RequestLimits` から埋め込む(数字を文字列に直書きしない)。
public enum RequestLimitLabels {
    public static let observationsReachedLimit =
        "観測は最大\(RequestLimits.maxObservations)件までです"
    public static let itemCandidatesReachedLimit =
        "持ち物の候補は「持ち物なし」を含めて最大\(RequestLimits.maxItemCandidates)件までです"
    public static let itemVariantsReachedLimit =
        "比較する持ち物は「持ち物なし」を含めて最大\(RequestLimits.maxItemVariants)件までです"
}
