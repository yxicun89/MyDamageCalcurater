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
Status: Phase 3 完了(PR #14・#23・#30)、P3-4 マスタを pokedex-svc の内部 API から(ADR-0204。PR #42)、P3-5 gateway の GATEWAY_WEB_URL(ADR-0205。PR #54)、pokedex-svc の契約 description(PR #59)、copyAbilityEffect のディープコピー修正(P2-3b critic 指摘。PR #81)、P3-6 calc・gateway を pokedex-svc につなぐ(ADR-0206。critic PASS。PR #87)は main に統合済み
Next: DOC-api(calc・gateway の README を coding-rules §8 に、docs/runbooks/api.md)

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
Status: P6-1(ADR-0500)・P6-2a 計算画面・契約追従・P6-2b 逆算画面・生成の internal タグ除外・DOC-ios は main に統合済み(PR #31・#53)。
P6-2c 構築ビルダー(一覧・編集画面・ニックネーム・XCUITest)は完了(critic 2回目 PASS)。`make ios-test`(XCTest 244件・
XCUITest 10件)が緑。api/openapi.yaml 側の ADR-0105(pokedex)更新に追従して `make ios-gen` 済み。ブランチにあり未 PR。
Next: PR(P6-2b・internal タグ除外・P6-2c をまとめて main へ)→ P6-2d(構築から個体を呼び出す配線。plan.md 参照)→
P6-3(`make ios-test` の総仕上げ)→ P6-4(手順書は AGENTS.md「手順書の書き方」)。

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
Branch: feat/speed-sp3(SP4 は feat/speed-sp4 → PR #86 で main に統合。作業ディレクトリ ~/MyDamageCalcurater-speed)
Status: SP0〜SP2・SP4(pokedex export の read model を k3d の speed に読ませる配線。ADR-0603。critic PASS。fixture データで k3d への
実配線・非回帰を確認済み)は完了・main に統合(PR #32・#36・#52・#83・#86)。SP3(Web の素早さ画面。ADR-0604。左=速い順の表
〈道具・ランクの絞り込み付き〉・右=自分のポケモン〈preset/custom/raw〉。web/src/speed/ の中に SpeedScreen・speedClient・生成型を置き、
web/src/app/{routes.ts,screens.tsx}・i18n/ja.ts・App.tsx に最小限の追記〈Web レーンと合意〉)は critic PASS(3回目。1回目 NG 重要2件
〈絞り込みUIの見送りが未記録・ja.ts追記範囲がADR合意を超過〉・2回目 NG 重要1件〈絞り込み UI 追加で境界線が誤った位置に出る回帰〉を
修正)。PR 作成待ち。**SP4 の実データ(pokedex-svc の DB)での最終確認は未実施**(DSN の取り扱いがこのセッションの権限で扱えないため。
`make pokedex-export`(データレーンの docs/runbooks/data.md の手順で DB を用意した状態で、POKEDEX_DATABASE_DSN を設定して実行)→
`make speed-k3d-deploy-readmodel && make speed-smoke-readmodel` を、DSN を扱えるセッションか人間が実行して確認する)。空の roster の
扱いは pokedex export が1件以上を返す前提のまま(ADR-0603 影響。実データで0件になる状況が起きたら別途決める)
Next: SP3 の PR を作って main に統合 → SP5(GitOps。ADR-0603 で SP4 から分離。イメージの digest が決まる段階で着手)

## Judge
Lane: 判定(素早さ×ダメージ連動。`services/judge/`。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/judge-jd1(作業ディレクトリ ~/MyDamageCalcurater-judge。PR 作成待ち。JD0 の feat/judge-jd0 は PR #92 で main に統合済み・削除)
Status: JD0(基盤。PR #92)に続き JD1(判定 API 本体)完了。ADR-0701: `POST /api/judge/v1/outspeed-and-ko` を実装。
素早さは `engine.EffectiveStat`(実数値→ランク)→ こだわりスカーフ(×6144/4096 五捨五超入。既定 item ID `choicescarf`、
`JUDGE_CHOICE_SCARF_ITEM_ID` で上書き可)。性格は新設 `Pokedex.Natures`(`GET /api/pokedex/natures`。1リクエスト1回で
attacker・defender 両方を解決)。上流呼び出しは逐次(natures→species×2→calc。並列化しない。検査順を契約に書き
エラーの勝ち負けを固定するため)。response は `outspeeds`・`speedTie`・`attackerSpeed`・`defenderSpeed`・`ko`。
critic 2回目で PASS(1回目 NG 重要3件: 上流エラーのログ未記録・pokedex 400 の扱いが ADR 未記載・defender 側スカーフ/
種族差のテスト欠如。すべて修正・テスト追加済み)。`make test`/`make lint`/`make build`(ルート)が緑
Next: PR を作って main へ統合(このセッションの残タスク)。JD2 以降(複数の相手候補・場の効果・画面)は
plan.md の方針どおり、着手前にユーザーへ確認する

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- balance の type chart: P1-13 の `testdata/golden/typechart.json` をバイト複製して同梱(ADR-0015)。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
