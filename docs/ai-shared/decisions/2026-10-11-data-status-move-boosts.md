## 2026-10-11: 変化技のランク変化を move_effects に取り込む(CB-1)
Decision: Showdown のトップレベル `boosts` を、確率 100 の move_effects として取り込む。`target` が自分(self・allies)なら自分、相手(normal・allAdjacentFoes 等)なら相手のランク変化。味方だけ(adjacentAlly)は取り込まず警告。取得物には `boosts` を必須で出し、古い取得物は拒否する(`make import-fetch` で取り直す)。詳細は ADR-0107「CB-1」。
Reason: 連続技の計算(docs/plan/combo-calc.md)が、りゅうのまい・からをやぶる・なきごえ等の変化を手の間に反映するため。確率 100% の効果だけを扱う(ユーザー決定)。
Impact:
- engine: 変更なし。ダメージ計算・ゴールデンは不変。
- API: 内部 `/internal/pokedex/master` の `MasterMove.effect` に値が増えるだけ(契約不変)。公開 `Move.effect` は CB-4(API レーン)。
- Web・iOS: 変更なし(CB-4 以降で連続技の画面が使う)。
- DB: migration なし。デプロイ後に `make import-fetch` で取り直し、取り込む(古い取得物は拒否される)。
