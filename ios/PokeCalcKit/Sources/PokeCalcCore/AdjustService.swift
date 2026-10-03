// AdjustService: 調整画面(AJ7。ADR-0502 §3)が依存する境界。
//
// `PokeCalcService` には混ぜない(`DeviceDataService` と同じ形。既存の準拠型〈テストの StubPokeCalcService など〉を
// 変えずに足すため)。実装は `APIPokeCalcService`(拡張。HTTP)と `MockAdjustService`(架空データ。計算しない)。
// 失敗は `PokeCalcError`、取り消しは `CancellationError`(包まない。`APIPokeCalcService.send` と同じ)。

/// 調整 API(`/api/calc/adjust/*`。ADR-0250)と技の逆引き(`listMoveLearners`。ADR-0251)。
public protocol AdjustService: Sendable {
    /// 指数と HP の 16n / 16n-1 ライン(openapi `adjustIndices`)。
    func adjustIndices(_ request: AdjustIndicesRequest) async throws -> AdjustIndicesResult
    /// 相手を n 発で倒せる最小の A / C の SP(openapi `adjustMinSpToKo`)。
    func adjustMinSpToKo(_ request: AdjustSearchRequest) async throws -> AdjustKOResult
    /// 相手の技を n 発耐える最小の H と B / D の SP(openapi `adjustMinSpToSurvive`)。
    func adjustMinSpToSurvive(_ request: AdjustSearchRequest) async throws -> AdjustSurviveResult
    /// SP 配分の提案(openapi `adjustAllocation`)。
    func adjustAllocation(_ request: AdjustAllocationRequest) async throws -> AdjustAllocationResult
    /// 技を覚えるポケモンの1ページ(openapi `listMoveLearners`。`limit` / `offset` をそのまま送る)。
    /// 総数は返らない。返った件数が `limit` 未満なら最後のページ(ADR-0251 §1)。
    func moveLearners(moveId: String, limit: Int, offset: Int) async throws -> [SpeciesSummary]
}
