// PokeCalcService: 画面が依存する唯一の境界(ADR-0500 §3)。
// 実装は2つ: `APIPokeCalcService`(サーバー)と `MockPokeCalcService`(架空データ)。
// どちらも同じプロトコルを満たすので、画面(ViewModel)は接続先を意識しない。

/// マスタ参照・ダメージ計算・逆算をまとめた境界。
public protocol PokeCalcService: Sendable {
    /// ポケモンを日本語名で前方一致検索(空なら全件、`limit` まで)。
    func searchSpecies(query: String, limit: Int) async throws -> [SpeciesSummary]
    /// 種族の詳細(タイプ・種族値・特性・覚える技)。
    func species(key: String) async throws -> SpeciesDetail
    /// 技を日本語名で前方一致検索。
    func searchMoves(query: String, limit: Int) async throws -> [Move]
    /// 技を ID で1件引く(openapi `getMove`。マスタに無ければ `PokeCalcError.Code.notFound`)。
    /// 検索結果に一度も現れていない「選択中の技」の名前を解決するためだけに使う
    /// (learnset 全件の実体化には使わない。ADR-0501「issue #68 の残り」)。
    func move(id: String) async throws -> Move
    /// 技を ID の集合でまとめて解決する(openapi `getMovesByIds`。`move(id:)` の複数版)。
    /// マスタに無い ID は結果から黙って省く(エラーにしない。404 は無い)。重複した ID は入力からも
    /// 応答からも1つにまとめる。`ids` が `RequestLimits.maxMoveBatchIds` を超えるときは、呼び出し側で
    /// 待たずにこの実装がその件数ずつに分割して複数回呼ぶ(呼び出し元は分割を意識しない)。
    /// `ids` が空なら通信せず空配列を返す。応答の順序は渡した `ids`(重複除去後)の順
    /// (ADR-0501「getMovesByIds による構築編集の技の一括解決」)。
    func moves(ids: [String]) async throws -> [Move]
    /// 持ち物を日本語名で前方一致検索。
    func searchItems(query: String, limit: Int) async throws -> [Item]
    /// 性格の一覧(補正する能力)。
    func natures() async throws -> [Nature]
    /// 1 vs 1 のダメージ計算。
    func calcDamage(_ request: CalcRequest) async throws -> CalcResult
    /// 防御側の代表調整すべてに対する一括計算。
    func calcBulk(_ request: BulkCalcRequest) async throws -> BulkCalcResult
    /// 観測ダメージから相手の調整候補を逆算(ADR-0010 §R)。
    func reverse(_ request: ReverseRequest) async throws -> ReverseResult
}
