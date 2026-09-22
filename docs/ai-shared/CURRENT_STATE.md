# Current State

## Damage Calculator
Lane: データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R・P1-13・P1-11・P1-12・P2-1b・P2-1c・P2-2a・P2-2b・P2-2c・P2-2d・P2-3(pokedex-svc。内部 API・公開 API・natures・balance/speed 向け export。ADR-0105)は完了(critic レビュー済み)
Next: P2-3b(無効・吸収の特性を engine・DB・export に足す)→ P3-1〜3(API レーンが実装中。gateway を pokedex-svc に向ける依頼は main 統合後に送る)→ P4-1〜7。人間の確認待ち(plan.md ブロッカー): 観測%の丸め方(整数%表示は確認済み)、公開のタイミング(LICENSE・クリーンコピー)、P2-1c の裁定

## API
Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Active: なし
Branch: (次は main から feat/api-<名前> を切る。作業ディレクトリ ~/MyDamageCalcurater-api)
Status: Phase 3 完了(PR #14・#23・#30)、P3-4 マスタを pokedex-svc の内部 API から(ADR-0204。PR #42)、P3-5 gateway の GATEWAY_WEB_URL(ADR-0205。PR #54)、pokedex-svc の契約 description を ADR-0105 に合わせる(PR #59)は main に統合済み。ユーザー指示(2026-09-22)により利用枠をデータレーンに集中させるため一旦停止
Next: データレーン依頼 d(gateway の /api/pokedex/* を pokedex-svc(Service 名 pokedex、ポート80)へ、calc の local overlay を CALC_MASTER_URL=http://pokedex の URL 方式へ切り替え)。疎通確認には pokedex-svc の実データ投入が要る(データレーンの docs/runbooks/data.md の手順: make up → make import-fetch/import-dry-run → make import または make import-k8s → kubectl -n pokecalc get deploy pokedex で Ready 確認)。その後は DOC-api(calc・gateway の README を coding-rules §8 に、docs/runbooks/api.md)

## Web
Lane: Web(`web/`・Playwright。どの AI が進めてもよい)
Active: Claude Code(ユーザー指示で再開)
Branch: feat/web-p4(作業ディレクトリ ~/MyDamageCalcurater-web)
Status: P4-1〜P4-6・P4-8〜P4-12(仮想敵 threats・おすすめタイプ recommendations を含む。ADR-0303)・P4-14・P4-15・DOC-web・P4-7(M1 完了報告)完了(critic PASS。main 統合済み。PR #82・#84)。
データレーンの依頼(P2-3b・ADR-0106)にも追従済み(PR #84): AbilityEffect に defImmuneTypes・defAbsorbTypes、
exportBalanceReadModel に absorb と ADR-0106 §決定7の出力順・無効優先。本物の engine.wasm で無効・吸収を結合テスト確認済み。
verify-m1.md を完成版にした: P2-2c/d・P2-3・P3-3 が main に入り、k3d(gateway 経由 http://localhost:8080)で
計算・逆算・タイプバランス(仮想敵・おすすめタイプ含む)を実地確認(pokedex-svc は実データ投入済みだが、
gateway/calc-svc のマスタ参照先はまだ pokedex-svc に向いていない。API レーンの依頼 d が一時停止中)。
P4-5 は Chrome で確認済み(Safari は未確認。人間の作業)
Next: (1) pokedex-svc の公開 API から Web のオンライン MasterSource を作る(ADR-0301 §4。gateway/calc の
pokedex 配線待ちなので、API レーンの依頼 d が進んでから本格着手するのが自然)。
(2) 続いて P5-5(構築ビルダー等)は record/team の API 待ち。
(3) 人間へのお願い: docs/verify-m1.md §4 を Safari で確認(P4-5)

## iOS
Lane: iOS(`ios/`。M3 の Phase 6。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/ios-p6(作業ディレクトリ ~/MyDamageCalcurater-ios)
Status: P6-1(ADR-0500)・P6-2a 計算画面・契約追従は main に統合済み(PR #31)。P6-2b 逆算画面(critic PASS)・生成の internal タグ除外・DOC-ios(ios/README.md を coding-rules §8 の形に、ADR-0501・docs/runbooks/ios.md。critic PASS)はブランチにあり未 PR。`make ios-test`(XCTest 174 件・XCUITest 9 件)が緑
Next: PR(P6-2b・internal タグ除外・DOC-ios をまとめて main へ)→ P6-2c 構築(端末内保存の TeamStore、Showdown 形式は後回し)→ P6-3 → P6-4(手順書は AGENTS.md「手順書の書き方」)

## Type Balance Checker
Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Active: なし(TB6 完了・main 統合済み。次はユーザー指示待ち)
Branch: 次は main から feat/tb-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status: 設計書(docs/type-balance-design.md)の TB0〜TB6 はすべて main に統合済み(TB6: 技範囲チェッカー、PR #65)。P2-3b(特性の無効・吸収)は main に engine 側の実装が入った(ADR-0106。データレーンの別ライン)ので、balance の read model 再生成待ちは解消に近づいている見込み。メガフォームの nameJa が英語表記のままの件はデータレーンへ確認候補として残る(ブロッカーではない)
Next: (1) データレーンが export(data/generated/readmodel)を再生成したら `make balance-k3d-deploy-readmodel && make balance-smoke-readmodel` で実データ確認(特性の無効・吸収を含む)する。(2) Web レーンが `make gen-ts` を実行して `web/src/api/balance.gen.ts` に move-range の型を反映する。設計書の TB0〜TB6 はすべて完了、以後はユーザーからの新規要望待ち
メモ: `make balance-k3d-deploy`(local overlay)で上書きすると Application は OutOfSync になる(manual sync なので戻らない)。GitOps に戻すときは Argo CD で Sync

## Speed
Lane: 素早さ(素早さ比較サービス。`services/speed/`・`web/src/speed/`。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/speed-sp4(SP2 は feat/speed-sp2 → PR #83 で main に統合。作業ディレクトリ ~/MyDamageCalcurater-speed)
Status: SP0〜SP2 は完了・main に統合(PR #32・#36・#52・#83)。SP4(pokedex export の read model を k3d の speed に読ませる配線。ADR-0603。
balance の ADR-0403 と同じ形: `cmd/checkreadmodel`・`scripts/k3d-deploy-readmodel.sh`・`scripts/smoke-readmodel.sh`・`deploy/k8s/overlays/local-readmodel`)は
critic PASS(2回目。1回目 NG 重要1件〈ADR-0600 §2 と ADR-0603 の GitOps 記述の矛盾。ADR-0600 に変更履歴を追記・docs/speed-design.md の段階表から
GitOps を SP5 として分離・plan.md に SP5 を追加して修正〉)。fixture データ(testdata/pokemon.example.json)で k3d への実配線・非回帰(架空データの
local overlay)を確認済み。**実データ(pokedex-svc の DB)での最終確認は未実施**(DSN の取り扱いがこのセッションの権限で扱えないため。
`make pokedex-export`(データレーンの docs/runbooks/data.md の手順で DB を用意した状態で、POKEDEX_DATABASE_DSN を設定して実行)→
`make speed-k3d-deploy-readmodel && make speed-smoke-readmodel` を人間または権限のあるセッションで実行して確認する)。PR 作成待ち
Next: SP4 の PR を作って main に統合(実データでの最終確認は、DSN を扱えるセッションで PR 前後どちらでもよい)→ SP3(`web/src/speed/` の画面は
素早さレーンのまま。タブ登録は3か所に1件ずつ: web/src/app/routes.ts の SCREEN_ROUTES・web/src/i18n/ja.ts の appText.speedTabLabel・
web/src/app/screens.tsx の SCREEN_COMPONENTS。P4-10 は PR #44 で main に統合済み。テストの例は web/src/App.routing.test.tsx と
web/e2e/routing.spec.ts)→ SP5(GitOps。ADR-0603 で SP4 から分離。イメージの digest が決まる段階で着手)。SP4 までに決めた: 空の roster の
扱いは pokedex export が1件以上を返す前提のまま(ADR-0603 影響。実データで0件になる状況が起きたら別途決める)

## Judge
Lane: 判定(素早さ×ダメージ連動。`services/judge/`。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/judge-jd0(作業ディレクトリ ~/MyDamageCalcurater-judge)
Status: JD0 の spec-writer まで完了。ADR-0700(基盤・上流の呼び方・エラーの正規化・受け入れ条件8件)を採用し、docs/judge-design.md §4 の
未決事項5件をすべて決定に変えた(同速は `outspeeds` と `speedTie` を別に返す / JD1 は自分が殴る側だけ / 独自 Ingress `/api/judge`(gateway は変更しない) /
ADR 帯 0700 / 技の追加効果は request の `ranks` で受ける)。`services/judge/` に契約(api/openapi.yaml。JD0 は healthz のみ)・go.mod・Makefile・README と、
**失敗する状態のテスト**(internal/httpapi/server_test.go・internal/client/{client,pokedex,calc}_test.go・cmd/api/config_test.go)を置いた。
ルート Makefile に `include services/judge/Makefile`、go.work に `./services/judge` を追記済み。実装は未了なので `make judge-test` はコンパイルエラーで落ちる(想定どおり)
Next: implementer が JD0 のテストを通す最小実装を書く(`make judge-gen` → `internal/api` の生成 → `internal/client`(Pokedex・Calc・4つの番兵エラー)→
`internal/httpapi`(healthz)→ `cmd/api`(環境変数・graceful shutdown))。そのあと critic → deploy/k8s(base の Deployment・Service・Ingress `/api/judge`)と
Dockerfile を足して `judge-kustomize` / `judge-docker-build` を Makefile に追加 → PR。JD1 は ADR-0700 §5 のとおり endpoint を契約に足すところから

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- balance の type chart: P1-13 の `testdata/golden/typechart.json` をバイト複製して同梱(ADR-0015)。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
