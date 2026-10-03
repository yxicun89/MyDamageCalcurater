## 2026-10-03: ダブルを計算に反映した(API レーン → Web・iOS・判定・データレーンへ。issue #232・#288・ADR-0222)
- `format=double` で、防御側の壁が 2732/4096(シングルは 1/2)、全体技(`move.target=spread`)が基礎ダメージ ×0.75 になった。ゴールデン(doubles)は @smogon/calc 0.12.0 の Champions 世代と全件一致
- 技の対象がマスタに無い間(#288 がデータレーンでマスタ化するまで)、calc-svc のダブルは全攻撃技に印 `{target: move, reason: move_target_unknown}` が付く。数値は単体技として計算(全体技なら過大)。クライアントは未知の reason を汎用の文言で出す(ADR-0215)。判定画面の「ダブル」を戻すのは #288 の後
- `move.target` は WASM 境界・engine で `""`・`single`・`spread` 以外を拒否(invalid_enum)
- Web レーン: `web/src/engine/types.ts` の印の reason の型・説明に `move_target_unknown` を足す(zero_power の後に付く)
- PR #497 マージ後の作業(format=double の印を外す・既存テストの期待の修正)は ADR-0222 §5
- 未決(人間の確認): teraType の扱い(ポケモンチャンピオンズにテラスタルは無い。#497 の印のまま・無視・400・印を消す)と、相手1体のときの全体技の見せ方(ADR-0222 §4)
