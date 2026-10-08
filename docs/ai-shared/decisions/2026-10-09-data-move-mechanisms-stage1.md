## 2026-10-09: 技の機構の段階1(I-data-4。データレーンから Web・iOS・API へ)
Decision: ADR-0142 を採用した。多段・固定ダメージ・一撃必殺・必ず急所・防御ランク無視・攻撃/防御に使う能力値の機構を engine が計算し、中身(move_mechanism_params)があるときだけ「未対応」の印を外す。
Reason: 対戦でよく使う多段技・ちきゅうなげ系・ボディプレス/イカサマ/サイコショックの数値が誤ったまま印だけ付いていたため(issue #271 / #270)。
Impact:
- 内部 API(`GET /internal/pokedex/master`): `MasterMove.mechanismParams` が増えた(必須キー・null は中身なし)。公開 API(`CalcResult` 等)は変えていない。受け取る calc-svc はキーが無い本文を null と同じに扱う。
- WASM(Web 内部): 入力 `move.mechanismParams`・特性の効果 `maxMultiHit`・`preventsOHKO`、結果 `hitRolls`(常に配列。単発・ダメージなしは `[]`)が増えた。`rolls` は多段のとき1回の使用の同じ段の合計。確定数は「1回の使用 = 回数ぶんの独立な乱数」で数える。Web の `exportSnapshot` は `mechanismParams: null` を運ぶ。
- 多段の回数は既定で oracle と同じ(範囲は最小+1、スキルリンクは最大)。利用者が回数を選ぶ入力・1発ごとのダメージの公開 API への出力・一撃必殺の命中率の表示は段階2(API・Web・iOS レーンで別 ADR)。
- 調整の目標探索・配分は、参照する能力が技と合わない技(alt_offense_stat / alt_defense_stat の中身)を結果の Unsupported に印で残す(段階2)。
- デプロイ順はアプリ(calc-svc・pokedex-svc・Web)→ マスタ取り込み → master-release。先に取り込むと新しい効果定義のキーを古い calc-svc が拒否する。
