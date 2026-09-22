// TeamStore: 構築の永続化境界(P6-2c・ADR-0500 §4「構築(team)は API の契約が無い(P5-4)。
// `TeamStore` プロトコルと端末内の実装(UserDefaults に JSON)で作り、team-svc の契約ができたら
// API 実装を足す」)。
//
// `PokeCalcService`(ADR-0500 §3)と同じ発想: 画面(ViewModel)はこのプロトコルだけに依存し、
// 実装が端末内(`LocalTeamStore`)か将来の API かを意識しない。

/// 構築(`Team`)の一覧・取得・保存・削除。並べ替え(reorder/move)は持たない
/// (requirements.md に並べ替えの要求が無く、UI 無しで作ると検証できないため。ADR-0501「P6-2c」2章)。
public protocol TeamStore: Sendable {
    /// 保存済みの構築をすべて返す(追加順)。
    func list() async throws -> [Team]
    /// `id` の構築を1つ返す(無ければ nil)。
    func get(id: String) async throws -> Team?
    /// `id` が既存なら更新、無ければ追加する。`TeamValidator.firstViolation(in:)` に引っかかる値は
    /// その `PokeCalcError` を投げて保存しない(値を丸めて保存し直さない)。
    func save(_ team: Team) async throws
    /// `id` の構築を削除する。無い `id` を渡しても失敗しない(冪等)。
    func delete(id: String) async throws
}
