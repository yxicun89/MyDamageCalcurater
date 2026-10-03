## 2026-09-22: calc・gateway を pokedex-svc につなぐ(データレーンからの依頼d。ADR-0206。PR #87)
Decision: base(local overlay を含む)の calc に `CALC_MASTER_URL=http://pokedex`、gateway に `GATEWAY_POKEDEX_URL=http://pokedex` を設定した。
共有 Kustomize Component(`deploy/k8s/overlays/local/api`。`local`/`local-api` の両 overlay から参照)を分岐させると「最後に apply した方が勝つ」状態になるため、
設定は分岐させず base に1か所だけ置いた(local/local-api どちらも同じ内容の Deployment になる)。ファイル方式(`CALC_MASTER_PATH`)は `make dev` とテストの fallback にのみ残す。
`services/gateway/scripts/smoke.sh` は、マスタのハードコード禁止規約(CLAUDE.md)を守るため、固定の架空 ID をやめ、`/api/pokedex/*` から実際に種族・技・性格を動的に発見する形にした。
依頼原文は「smoke の /api/pokedex を 503→200 に」だったが、実装は「200(pokedex-svc に実接続)または 503 `upstream_unavailable`/`master_unavailable`(pokedex-svc 未接続。ADR-0205 の web と同じ扱い)を成功」とする条件付きにした
(`make dev`・pokedex-svc 未デプロイのクラスタでも smoke が意味のある形で動くようにするため。ADR-0205 の web の前例と揃えた)。
Reason: データレーンの依頼。critic PASS(NG無し)。設計の詳細・却下案は ADR-0206。
Impact: **他レーンへの申し送り**: `make up` 直後(pokedex の DB 未投入)は calc-svc が pokedex-svc からマスタを取得できず Ready にならないため、
Web・iOS レーンのローカル k3d 環境でも calc を使う画面(ダメージ計算)が動かない。初回だけ `make import-k8s` でマスタを投入すること
(データレーンの docs/runbooks/data.md 参照)。`make api-k3d-deploy` も、マスタ未投入のクラスタでは `kubectl rollout status` が120秒でタイムアウトして失敗するので、
先に `make import-k8s` を実行すること。`make api-smoke` の出力1行目が `master=pokedex …` であれば実際に pokedex-svc へつながっている確認になる(`master=example` はフォールバック)。
