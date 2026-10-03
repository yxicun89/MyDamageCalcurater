## 2026-09-22: Web P4-5 の方針(ADR-0301)と、API レーンへの連絡(Web レーン、Claude Code。既定案で進行・ユーザー未確認。深夜)
Decision: (1) 画面は解決済みの実体のまま、API 実装が実体 → ID に写す(解決層は MasterData の1か所)。写像の表は ADR-0301 §2。
(2) 計算モードの既定はオフライン(WASM)。オンラインは pokedex-svc(P2-3)と gateway(P3-2)が揃ってマスタを API から読めるようになったら既定を見直す。
API に届かないとき自動で WASM に切り替えない(どちらの結果か分からなくなるため)。
(3) Web の例データの種族キーを `SpeciesKey`(`9001-000` の形)に合わせる(ADR-0300 §3 を改める)。
(4) **API レーンへ**: ルート Makefile の `gen-ts` を実装した(`web/src/api/openapi.gen.ts` を生成してコミット)。`make gen` に含まれるので、
`api/openapi.yaml` を変えたら一度 `make web-install` してから `make gen` する(web の依存が無いと gen-ts は失敗する。スキップしない)。
Reason: ADR-0011 §10 の持ち越し(P4-5)。API の契約(ADR-0200)が main に入ったため。
Impact: 他レーンのファイルは変更しない(gen-ts は Web の持ち物のターゲット)。
