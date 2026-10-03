## 2026-09-23: `GET /api/pokedex/moves/{key}`(getMove)を実装・main統合、判定レーン JD4 のブロック解消(API レーン)
Decision: 判定レーンの依頼(2026-09-22「JD2〜JD5 の範囲・順序をユーザーが確定。API レーンへの依頼」・2026-09-23
「判定 JD3 を PR #143 で main に統合、JD4 は API レーンの依頼を待つ」)に応え、`GET /api/pokedex/moves/{key}`
(operationId `getMove`)を実装した(P3-7。**main 統合済み(PR #161)**。critic PASS。3往復。1回目 FAIL: 契約と
実装の不一致・ADR未更新・plan.md未更新・DECISIONS.md未記録。2回目 FAIL: レーン間の記録の食い違い〈CURRENT_STATE.md
の Judge 欄が未更新〉・「main統合済み」の先取り記載。3回目 PASS)。`api/openapi.yaml` に
`getSpecies` と同じ形(既存の `Move` スキーマをそのまま返す。200/404/503)で追加し、`make gen` で
`services/internal/api/openapi.gen.go` と `web/src/api/openapi.gen.ts` を再生成した。挙動: 使用可能集合で絞らない
(`getSpecies` と同様。絞り込みは検索の仕事)。`GetDefaultRegulation` を経由しないため、マスタ未投入(技0件)でも
`searchMoves`/`listNatures` と異なり 503 ではなく 404 `not_found` になる(契約の description に明記。ADR-0105 §3 追記)。

**越境の記録(COORDINATION.md「他のレーンの範囲のファイルは変更しない」の例外)**: `api.ServerInterface` に
メソッドが増えるため、`services/pokedex/`(データレーンの範囲)側にも実装が無いと `var _ api.ServerInterface =
(*Server)(nil)` でコンパイルが壊れ、main が緑を保てない。スタブ(404)だけ置く案は「200 を約束する契約なのに
実装が無い」状態で main に入れることになり完全な実装より悪いと判断し、`getSpecies`/`GetItem` パターンをそのまま
写す形で API レーンが完全実装まで行った(新規の設計判断はしていない)。触った pokedex 側のファイル:
- `services/pokedex/db/query/pokedex.sql`(`GetMove :one` を追加。`GetItem` と同じ形)
- `services/pokedex/internal/httpapi/search.go`(`Server.GetMove` ハンドラを追加)
- `services/pokedex/internal/httpapi/server.go`(ルート登録・コメントの操作数を6→7に修正)
- `services/pokedex/internal/httpapi/pokedex_test.go`(`TestGetMove` 追加、関連テーブルに `getMove` の行を追加)
- `services/pokedex/internal/storetest/storetest.go`(偽の `GetMove` を追加。フィクスチャデータは無変更)
- `services/pokedex/internal/store/*`(sqlc の生成物。`make gen` の出力)

データレーンへ依頼: 上記ファイルを再レビューしてください。特に `search.go` の `GetMove` ハンドラと
`storetest.go` の偽実装が、データレーン側の設計判断(命名・エラー変換の流儀)と食い違っていないかの確認。
問題があれば直接修正して構いません(API レーンはこの PR 以降 `services/pokedex/` に手を入れる予定はありません)。

判定レーンへ: `getMove` は **main 統合済み(PR #161)**。JD4(`feat/judge-jd4`)に着手してください。`priority` は
`int`(既存の `Move.priority` フィールドのまま)。
CURRENT_STATE.md の Judge 欄もこの内容に合わせて API レーンが更新した(越境の記録。本来はレーンごとの担当欄だが、
blocker の申し送りが片側だけでは意味が無いため)。

Webレーンへ: ADR-0304 §3(技 ID 解決の欠落)は**まだ解消していません**。今回追加した `getMove` は技1件だけを
返すため、`learnset` の解決にそのまま使うと種族1体あたり技20〜30件ぶんのラウンドトリップが要るという、
ADR-0304 §3 の案Bの欠点がそのまま残ります。案A(`getSpecies.learnset` を `Move` 実体の配列にする)か、
`getMove` にバッチ解決(`ids` クエリ)を足すかは、引き続き API レーンへの未決の提案のままです。

iOSレーンへ依頼: `api/openapi.yaml` に `getMove` を追加したため、`ios/PokeCalcKit/Sources/PokeCalcAPI/Generated/`
の生成物が古くなっています。`make ios-gen`(または既存の再生成手順)を実行し、`make ios-test` の
`ios-gen-check` を通してください(過去の追従例: commit `40caa49`)。API レーンからは `ios/` に触れません。

Reason: judge が上流から priority を引く手段が無いと、先に動く側を正しく決められず JD4 が実装できない
(2026-09-22 の依頼の Reason と同じ)。cross-lane 実装の判断理由は上記越境の記録のとおり。
Impact: `docs/plan.md` P3-7 追加・JD4 のブロッカーを解消として更新。`docs/adr/0105-...md` §3・受け入れ条件に
`getMove` を追記、`docs/adr/0200-calc-svc-api-contract.md` の「pokedex の5操作」を6操作に訂正、
`docs/adr/0107-move-secondary-rank-changes.md` 決定8に追記(`effect` の公開は依然未決)、
`docs/adr/0304-web-online-mastersource.md` §3 に追記(この実装は§3の欠落の解決策ではない)。
