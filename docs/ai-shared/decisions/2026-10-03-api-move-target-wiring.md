## 2026-10-03: 技の対象(target)を内部 API・calc-svc・公開 API に通した(API レーン → Web・判定・iOS レーン。issue 288・ADR-0223)
Decision: 内部 API の `MasterMove.target`(必須キー・nullable・Showdown の文字列のまま)を追加し、calc-svc の共通マスタが `engine.Move.Target`
(全体技 → spread、その他の既知 → single、不明 → 空)に写す。公開 API の `Move.target`(getMove・getMovesByIds・searchMoves)は省略可の
`single` | `spread`。DB が NULL の技はキーごと省き、未知の値は 503 master_unavailable。
Reason: データレーン(ADR-0136)と判定レーン(ADR-0222)の依頼。ダブルの全体技補正と `move_target_unknown` の印に技の対象が要る。
Impact:
- **Web レーンへ**: `web/src/master/onlineSource.ts` の `mapMove` で公開 `Move.target`(single/spread。省略あり)を Web の技に写し、WASM の技 DTO の
  `target`(空・single・spread)にそのまま渡す(キーが無ければ空 = 不明)。calc-svc 用の `exportSnapshot.ts` は本 PR で `target: null` を書くよう直した
- **判定レーンへ(確認事項)**: 公開 `Move.target` の名前・語彙(single/spread)は既定案。変える場合は ADR-0223 §4 だけ差し替える。
  自分・味方・場の技も single にしている(@smogon/calc の全体技補正は spread の2種だけ)。対象が不明な技(取り込み前)だけに `move_target_unknown` が付く
- **iOS レーンへ**: 生成物に省略可の `target` が増えただけ。ダブルの計算で使うかは iOS の判断(オンライン計算は calc-svc 経由で対象が効く)
- 入れ替え順序: calc-svc は target キーの無い古い pokedex の本文も受け付ける(不明として扱い、印が付く)
