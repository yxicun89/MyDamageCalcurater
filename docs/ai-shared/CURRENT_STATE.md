# Current State(索引)

レーンごとの状態(Lane / Branch / Active / Status / Next)は **`docs/ai-shared/state/` のレーン別ファイル**に分けた
(2026-10-03。全レーンが同じファイルを書き換えて PR のたびに競合するのを避けるため)。

- **自分のレーンのファイルだけを書き換える**。他レーンのファイルは読むだけにする。
- 新しい節は作らず、レーンのファイルの `Status` / `Next` を更新する。レーンが増えたらファイルを足してこの表に 1 行足す。
- 共有の決まりごと(ID・型・サービス境界)は `state/shared-interfaces.md`。

| レーン | ファイル | 概要 |
|---|---|---|
| Damage Calculator | [state/data.md](state/data.md) | データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md) |
| API | [state/api.md](state/api.md) | API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進め |
| Web | [state/web.md](state/web.md) | Web(`web/`・Playwright。どの AI が進めてもよい) |
| iOS | [state/ios.md](state/ios.md) | iOS(`ios/`。M3 の Phase 6。どの AI が進めてもよい) |
| Type Balance Checker | [state/type-balance.md](state/type-balance.md) | タイプバランス(どの AI が進めてもよい。COORDINATION.md) |
| Speed | [state/speed.md](state/speed.md) | 素早さ(素早さ比較サービス。`services/speed/`・`web/src/speed/`。どの AI が進めても |
| Judge | [state/judge.md](state/judge.md) | 判定(素早さ×ダメージ連動。`services/judge/`・`web/src/judge/`。どの AI が進めても |
| Ops | [state/ops.md](state/ops.md) | 運用(deploy・scripts・AIエージェントの権限設定。専任セッションなし。空席時は手が空いたレーンが調整役の |
| Shared Interfaces | [state/shared-interfaces.md](state/shared-interfaces.md) |  |
