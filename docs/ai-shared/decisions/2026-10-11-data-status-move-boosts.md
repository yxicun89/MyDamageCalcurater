## 2026-10-11: 変化技のランク変化を move_effects に取り込む(CB-1)
Decision: Showdown のトップレベル `boosts` を、確率 100 の move_effects として取り込む。`target` が自分(self・allies)なら自分、相手(normal・allAdjacentFoes 等)なら相手のランク変化。味方だけ(adjacentAlly)は取り込まず警告。取得物には `boosts` を必須で出し、古い取得物は拒否する(`make import-fetch` で取り直す)。詳細は ADR-0107「CB-1」。
Reason: 連続技の計算(docs/plan/combo-calc.md)が、りゅうのまい・からをやぶる・なきごえ等の変化を手の間に反映するため。確率 100% の効果だけを扱う(ユーザー決定)。
Impact:
- engine: コード変更なし。ダメージ計算・ゴールデンは不変。
- API: 内部 `/internal/pokedex/master` の `MasterMove.effect` に値が増えるだけ(契約不変)。公開 `Move.effect` は CB-4(API レーン)。
- calc-svc: 調整(outspeed 目標の先に使う技。ADR-0177 §4)で変化技を選ぶと素早さの段数が入る(りゅうのまい +1 等。これまでは 0)。engine と ADR-0177 §4 の意図どおり。
- Web・iOS: UI 変更なし。ただし調整の結果は変わる。連続技の画面は CB-4 以降。
- 既知の制限: 条件付きの変化技(growth の天候・clangoroussoul の HP・noretreat の2回目 等)は表の値を入れ、警告 `move-effect-conditional` を出す。CB-3 で再検討。
- DB: migration なし。デプロイ後に `make import-fetch` で取り直し、取り込む(古い取得物は拒否される)。
