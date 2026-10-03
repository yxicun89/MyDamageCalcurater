## 2026-09-21: Web レーンの構成(ADR-0300)と、他レーンへの提案2件(Web レーン、Claude Code。既定案で進行・ユーザー未確認)
Decision: (1) Web は計算を `CalcEngine` の後ろに置き、WASM(ADR-0011 の JSON 契約)で先に作る。マスタは `MasterData` の後ろに置き、
pokedex-svc ができるまで架空の例データ(名前は `テスト`、ID は `example-`、図鑑番号 9001 以降)。タイプ相性表だけは
`testdata/golden/typechart.json` を Vite の別名で複製せずに読む(Web は読むだけで変更しない)。
(2) **提案(データレーン宛て)**: 攻撃側プリセット(無振り / A(C)特化 / A(C)振り。ADR-0300 §5)は、いまは Web が持つ。
防御プリセット(ADR-0009)と同じく engine が持つ方が一貫し、iOS(M3)も同じ定義を使うので、既定案は
「データレーンが `AttackerPresetCatalog()` を engine と WASM 境界に足し、Web はそれに切り替える」。急がない(M3 より前ならよい)。
(3) **提案(全レーン宛て)**: Web のテスト(`make web-test` / `web-lint`)は、まだ `make test` / `make lint` に含めない
(含めると `web/node_modules` の無い他のレーンの作業ディレクトリでルートの `make test` が失敗する)。
既定案は「P4-6 で、`node_modules` が無ければ `npm ci` してから実行する形で `make test` / `make lint` に加える」。
Reason: 4レーン制で API・データの成果を待たずに Web を進めるため(COORDINATION.md「レーン間の依存と共有ファイル」)。
Impact: ルートの Makefile には `include web/Makefile` の1行だけを足した(ターゲットは `web-` 接頭辞)。engine・openapi.yaml は変更しない。
docs/design.md に bg.glass のぼかし量(Web は 20px。iOS はシステムのマテリアル)を1行追記した(P4-1)。
異議があれば追記すること(既定案で進む原則)。
