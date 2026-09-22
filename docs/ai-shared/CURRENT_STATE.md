# Current State

## Damage Calculator
Lane: データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R・P1-13・P1-11・P1-12・P2-1b・P2-1c・P2-2a・P2-2b・P2-2c・P2-2d・P2-3(pokedex-svc。内部 API・公開 API・natures・balance/speed 向け export。ADR-0105)は完了(critic レビュー済み)
Status(追記): P2-3b(無効・吸収の特性)も完了・main 統合済み(ADR-0106)。calc・gateway の pokedex-svc 接続(API レーンの依頼)も PR #87 で解決済み(api-smoke で master=pokedex 確認済み)。
Next: P5-6(技の追加効果によるランク変化。判定レーンからの提案。DECISIONS.md 2026-09-22)に着手中。人間の確認待ち(plan.md ブロッカー): 観測%の丸め方(整数%表示は確認済み)、公開のタイミング(LICENSE・クリーンコピー)

## API
Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Active: なし
Branch: (次は main から feat/api-<名前> を切る。作業ディレクトリ ~/MyDamageCalcurater-api)
Status: Phase 3 完了(PR #14・#23・#30)、P3-4〜P3-6(ADR-0204/0205/0206。PR #42/#54/#87)、DOC-api(README・手順書。PR #117)は main に統合済み
Next: 特に無し。他レーン(データ・Web・iOS)からの依頼待ち

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
Status: **M3(iPhone で使える)は完了**。P6-1(ADR-0500)・P6-2a 計算画面・契約追従・P6-2b 逆算画面・P6-2c 構築ビルダー
(一覧・編集・ニックネーム)・P6-2d(構築から個体を呼び出す配線)・生成の internal タグ除外・DOC-ios は main に統合済み
(PR #31・#53・#91・#119)。`make ios-test`(gen-check・XCTest 284件・XCUITest 12件・Info.plist 検査)が緑(P6-3)。
P6-4 の手順書 `docs/runbooks/ios-device-install.md` を作成しコミット済み(ブランチにあり PR 作成中)。
Next: PR を main へ。その後は M3 完了なので、ユーザーからの新規要望待ち(実機インストール・署名は手順書どおり
人間が行う)。将来の候補: engine の Champions マスタが pokedex-svc 経由になったら iOS のモック/実マスタの
差し替え動作を再確認、Web の record/team-svc(M2)が進んだら iOS の構築を端末内保存から API 保存へ移行するかを検討。

## Type Balance Checker
Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Active: なし(TB6 完了・main 統合済み。次はユーザー指示待ち)
Branch: 次は main から feat/tb-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status: 設計書(docs/type-balance-design.md)の TB0〜TB6 はすべて main に統合済み(TB6: 技範囲チェッカー、PR #65)。P2-3b(特性の無効・吸収)の実データ確認を完了(2026-09-23): データレーンが再生成した export(348 pokemon・moves・216 abilities)で `make balance-k3d-deploy-readmodel && make balance-smoke-readmodel` を実行し、`POST .../team-balance/analyze` でチリーン(levitate)への ground 攻撃が `{"category":"immune","effect":"immune","multiplier":"0","source":"ability"}` になること、`POST .../move-range/analyze`(thunderbolt)の `walledByAbility` にエモンガ(motordrive)が正しく含まれることを実データで確認済み。メガフォームの nameJa が英語表記のままの件はデータレーンへ確認候補として残る(ブロッカーではない)
Next: (Web レーンは `make gen-ts` 実行済み。`web/src/api/balance.gen.ts` に move-range の型が反映済みであることを確認した)設計書の TB0〜TB6 はすべて完了・実データ確認済み、以後はユーザーからの新規要望待ち
メモ: `make balance-k3d-deploy`(local overlay)で上書きすると Application は OutOfSync になる(manual sync なので戻らない)。GitOps に戻すときは Argo CD で Sync

## Speed
Lane: 素早さ(素早さ比較サービス。`services/speed/`・`web/src/speed/`。どの AI が進めてもよい)
Active: なし(SP0〜SP5 すべて完了。次の要望待ち)
Branch: 次は main から feat/speed-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-speed。SP5 は feat/speed-sp5 → PR #97 で main に統合)
Status: SP0〜SP5 すべて完了・main に統合(PR #32・#36・#52・#83・#86・#93・#97)。SP4 の実データ確認はユーザーが2026-09-24 に実施:
`mysql` Service はクラスタ内部の DNS 名で Mac からは解決できないため、`kubectl -n pokecalc port-forward svc/mysql 3306:3306` を張り、
DSN のホストを `127.0.0.1` に付け替えて `make pokedex-export`(348 pokemon)→ `make speed-k3d-deploy-readmodel` →
`make speed-smoke-readmodel` を実行(初回はロールアウト直後で 504、再実行で `speed readmodel smoke: pokemon=0003-000 list=200 table=200`)。
SP5 の実際の Argo CD への適用(`speed-argocd-app`・`speed-registry-push`・sync)は未実施のまま(ADR-0605 §4。共有クラスタへの変更のため
人間の確認のもとで、必要になったときに)
Next: 特に無し。他レーンからの依頼(Codexレビュー issue #105・#108。上記)かユーザーからの新規要望待ち。balance-registry →
pokecalc-registry への改名提案はタイプバランスレーンへ既定案で提示済み(DECISIONS.md 2026-09-23)。
Codexレビューissue(2026-09-23、タイプバランスレーンから連絡): #105(Argo CD導入・digest固定の共有スクリプト化)はタイプバランスレーンが
主担当で進め、できたら docs/runbooks/speed.md の該当節をその呼び出しに差し替えるだけになる見込み(今は着手不要)。#108(read model の
dataVersion・rollout一本化)はデータレーンが主担当で、連絡が来たら合わせる(今は着手不要)

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
