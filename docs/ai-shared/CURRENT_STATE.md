# Current State

## Damage Calculator
Lane: データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R・P1-13・P1-11・P1-12・P2-1b・P2-1c・P2-2a・P2-2b・P2-2c・P2-2d・P2-3(pokedex-svc。内部 API・公開 API・natures・balance/speed 向け export。ADR-0105)は完了(critic レビュー済み)
Status(追記): P2-3b(無効・吸収の特性)も完了・main 統合済み(ADR-0106)。calc・gateway の pokedex-svc 接続(API レーンの依頼)も PR #87 で解決済み(api-smoke で master=pokedex 確認済み)。
Status(追記): P5-6(技の追加効果によるランク変化。ADR-0107)完了・critic PASS(1往復)・**main 統合済み(PR #132)**。engine は乱数を持たず「発動した場合の値」だけを返す。ゴールデン不変。`move_effects` 別表・`MasterMove.effect`(内部API)まで。公開APIへの露出(判定レーンが技IDからランク変化を引く経路)は判定レーンの要件確定後に別途対応。
Status(追記): issue #110 のデータレーン担当分(ADR-0108)完了・critic PASS(1往復)・**main 統合済み(PR #138)**。`engine.CalcBulk`/`CalcReverse` と `engine/wasmapi` に ADR-0208 §1 と同じ件数・範囲の上限を追加し、wasmapi は DTO 変換より前に検査して HTTP との parity を確保。issue #110 は Web・iOS レーンの追従が残っている限りクローズしない。
Status(追記): issue #106(排他制御)完了・critic PASS・**main 統合済み(PR #155)**。`tools/importer/cronjob.sh` に `flock`(非ブロッキング)を追加し、手動Job(`make import-k8s`)と定期CronJobの同時実行を防ぐ(ADR-0109)。Docker(Linux)と実クラスタ(k3d)の両方で実際の排他動作を確認済み(2026-09-23。2つの手動Job同時作成→片方がロック競合で即exit 1→backoffLimitで再試行して成功)。
Status(追記): 2026-09-23、全レーンの main 統合済みの変更をまとめてk3dに再デプロイし、実データ(種族349・技515・move_effects 59件)で計算・一括計算・逆算・タイプバランス・素早さ・判定(JD3複数候補)まで実HTTPで動作確認済み。すべてgreen。既知の制約: 技を個別IDで引く公開APIが無く(`GET /api/pokedex/moves/{id}` は404)、Webのオンライン技選択・持ち物候補比較・判定JD4(相手の技を含めた返り討ち判定)がブロックされたまま。
Status(追記): 2026-09-24、getMove(P3-7。APIレーンが`services/pokedex/`へ越境実装)をレビュー。既存の設計判断(命名・エラー変換・テストの流儀)と食い違いなく、修正不要と判断(DECISIONS.md参照)。判定レーンはJD4に着手可能。上記の「技を個別IDで引く公開APIが無い」制約はこれで解消(バッチ解決はまだ無いのでWebのオンライン技選択は引き続きブロック)。
Status(追記): issue #104(DB資格情報の最小権限分離)完了・critic PASS(1往復)・**main 統合済み(PR #176)**。`pokedex_reader`/`pokedex_importer`/`pokedex_migrator`の3ロールに分離(ADR-0110)。実クラスタで`SHOW GRANTS`により権限が過不足なく一致することを確認済み。既存クラスタからの無停止移行も実地確認済み。
Status(追記): issue #109(HTTPタイムアウト・graceful shutdown)完了・critic PASS(1往復。指摘なし)・**main 統合済み(PR #178)**。`newHTTPServer`/`serve`/`runServe`の3層分離(ADR-0111。services/balanceと同じ値)。`terminationGracePeriodSeconds: 30`を追加。実クラスタで再デプロイ・確認済み。
Status(追記): issue #112(DB接続プール上限)完了・critic PASS(1往復。軽微指摘1件反映)。4環境変数を`services/pokedex/db.OpenPool`経由で適用(ADR-0112)。実クラスタで再デプロイ・確認済み。**main 統合済み(PR #180)**。**これでデータレーン主担当のCodexレビューissue(#104・#106・#109・#112)はすべて完了・main統合済み**。
Next: 他レーンからの依頼待ち。人間の確認待ち(plan.md ブロッカー): 観測%の丸め方(整数%表示は確認済み)、公開のタイミング(LICENSE・クリーンコピー)

## API
Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Active: なし
Branch: (次は main から feat/api-<名前> か fix/api-<名前> を切る。作業ディレクトリ ~/MyDamageCalcurater-api)
Status: Phase 3・issue #110(ADR-0208。PR #130)・issue #103の設計(M2保存データの保持・削除・端末ID境界。ADR-0209。critic PASS。PR #150)は main に統合済み
Status(追記): issue #148のAPIレーン担当分(ADR-0210。私設サービスの境界)完了・critic PASS・**main 統合済み(PR #157)**。`deploy/k8s/overlays/cloud` から gateway の Ingress を削除 patch で除去し、public Ingress/LoadBalancer/NodePort/externalIPs/hostNetwork/hostPort が無いことを構造検査+`kubectl kustomize`実描画検査の2層で固定。端末ID/CORSを認証・到達制御として扱わない回帰テストも追加。
Status(追記): P3-7 `GET /api/pokedex/moves/{key}`(getMove)を実装(判定レーン JD4 の依頼。ADR-0105 §3 追記)。契約・`services/pokedex/`(データレーンの範囲。越境理由と触ったファイル一覧は DECISIONS.md)まで一括実装。critic PASS(3往復)・**main 統合済み(PR #161)**。判定レーンは JD4 に着手し main 統合済み(PR #169)。
Status(追記): issue #69(検索の並びがOpenAPI契約と一致しない)・issue #73(OpenAPIとengineの防御プリセット集合の同期検査)を修正・**main 統合済み(PR #167・#172)**。いずれも契約・テストの整合修正で、SQL・engine・ADR は無変更(既存の設計は元々正しかった)。両issueともclose済み。
Status(追記): issue #113(入力変更時の古い計算要求を抑止・キャンセル)のAPIレーン連携分(「クライアントのcancel伝播」)を修正。gatewayの`ReverseProxy.ErrorHandler`がクライアントの要求中断(`context.Canceled`)を上流障害と区別せず「上流に到達できない」WARN・503 `upstream_unavailable`を返していたのを、クライアント起因のときは何もしない(応答を書かない)よう修正(ADR-0202 §5 追記・AC-G10)。**限界**: gateway→calc-svcへのcontextキャンセル伝播自体は効くが、calc-svc・engineはcontextを見ないため(engineを純粋に保つ絶対ルール2)、issue本文の「calc-svc CPU消費も止める」は未達成のまま(中断された逆算は完走する。ADR-0208の上限で最悪計算量は有界)。Web欄の「APIレーンの連携分が残っているか未確認」はこれで解消(Web欄の更新はWebレーンに委ねる)。issue #113はWeb・iOS・APIすべてのレーン分が完了としてクローズ可能と判断(詳細はDECISIONS.md)
Next: 他レーンからの依頼待ち。issue #103・#148の依頼(データ・Web・iOS・運用レーンへ)、getMove 実装の再レビュー依頼(データレーンへ。60fbe25で対応済み)・iOS再生成依頼(a1f5d5eで対応済み)はDECISIONS.mdに記録済み

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
P4-5 は Chrome で確認済み(Safari は未確認。人間の作業)。
**P4-16・P4-16b・P4-16c(オンライン MasterSource。ADR-0304)完了・main 統合済み(PR #128・#134・#153)**:
`createOnlineMasterSource`(持ち物・性格を全件取得、種族は検索専用インターフェース)、`MasterData.capabilities`
(公開 API に無い機能を明示的に無効化)、画面側(CalcScreen・ReverseScreen は技・持ち物候補比較が無効なとき
disabled+案内、種族一覧が無効なとき検索欄 `SpeciesSearchField`。BalanceScreen は speciesList・moves が両方
そろうまで画面ごと無効化)、検索欄のキーボード操作(WAI-ARIA "List Autocomplete with Automatic Selection")と
CSS。3段階とも critic 1回目 FAIL→修正→2回目 PASS で完了(重大バグ・文言バグ・IME変換中のキー横取りなど、
いずれも critic が発見)。既存752件は無変更のまま最終907件。技の ID→実体化(`getSpecies.learnset`)は公開 API
に手段が無く、データ/API レーンへ既定案付きで提案済み(DECISIONS.md 2026-09-23、未回答・急ぎではない)。
**P4-19(issue #110 セキュリティ。ADR-0300 §10)も完了・main 統合済み(PR #142)**: 持ち物候補を配列を作る
最終地点で64件に決定的に絞り込み、観測は16件で disabled+案内。critic PASS(境界値の網羅探索と変異テストで
上限超過が起きないことを確認)。データレーンの engine/wasmapi 側(ADR-0108・PR #138)も main 統合済み。
issue #110 は iOS の追従待ちで Web 単独ではクローズしない。
**P4-18 issue #99(ライトテーマの danger コントラスト不足)完了・issueクローズ済み**: Web 分(PR #164)は
danger のライト値を `#E5484D`→`#CD1D23` に変更(WCAG 2.2 SC 1.4.3 の4.5:1を bg.base・bg.glass 合成後の
両方で満たす)。`web/src/test/colorContrast.ts`・`web/src/styles/contrast.test.ts` を新規追加。critic PASS
(独立実装での検算・変異テストで確認)。iOS 側も完了(`fix: ライトテーマの danger コントラスト不足を修正
(issue #99)`。`PokeCalcDesign.swift`・`ColorContrast.swift`・`DangerContrastTests.swift`)。issue #99 は
クローズ済み。
**issue #113(逆算の古い計算要求の抑止・キャンセル)は Web 分完了・main 統合済み(PR #174)**: 観測のテキスト
編集のみ200msのtrailing debounce、確定操作は待たない。`CalcEngine` に任意引数 `signal?: AbortSignal` を
追加、取り消しは `REQUEST_ABORTED_CODE` で区別(ADR-0300 §11)。critic PASS(非同期・競合状態を重点検証。
mutation テスト9件で確認)。iOS 側も完了(`fix: iOS の計算・逆算で古い計算要求をキャンセル・観測入力を
debounce (issue #113)`。`LatestTaskRunner.swift`・`CalcInput.swift`)。**issue #113 自体はまだ open**
(API レーンの「クライアントのcancel伝播」連携分が残っているかは未確認。Web・iOS とも自レーン分は完了)。
P4-17(技の ID 解決)は、API レーンが判定レーン JD4 向けに `GET /api/pokedex/moves/{key}`(getMove)を
main 統合したが(PR #161)、種族1体あたり技20〜30件ぶんのラウンドトリップが要るため ADR-0304 §3 の欠落は
**まだ解消していない**(API レーン自身が ADR-0304 に追記済み)。案A(`learnset` を `Move[]` にする)か
`getMove` のバッチ解決化が API レーンへの未決の提案のまま。
**P4-21 issue #67(2xxの契約外JSONでAPIクライアントが例外を投げる)完了・main 統合済み(PR #189)**:
`apiEngine.ts`・`balanceClient.ts` の `postJson` を型ガード経由にし(`as Schemas[...]` の型アサーションを
除去)、契約外の2xxで例外を投げず `engine_unavailable`/`balance_unavailable` を返すようにした(ADR-0301 §4・
ADR-0303 §6)。calc側は写像関数が読む全フィールドを再帰的に検査、balance側は画面がたどる形だけを検査
(leafスカラー・enumは見ない。契約の二重管理を避けるため)。critic PASS(mutation テスト12件で型ガードの
過不足なしを確認)。既存1034件は無変更・新規143件追加(1166件)。
Next: (1) P4-21 の残り: #98(モバイル幅で計算・逆算画面が横に溢れる)。CSS の狭幅 media query + Playwright の
320/375px 回帰テストが必要。(2) P4-17: 技の ID 解決の欠落が解消されたら技を復活。(3) P4-20: issue #148
(アクセス境界・認証方針)。Web 側は既にコード上で条件を満たしていることを確認済み(apiBaseUrl の既定値は
同一オリジン、CORSはgateway側の設定)。実際のtailnet名が決まってから運用レーンより連絡が来る想定。
(4) 続いて P5-5(構築ビルダー等)は record/team の API 待ち(M2。人間の /phase キックオフ待ち)。
(5) 人間へのお願い: docs/verify-m1.md §4 を Safari で確認(P4-5)

## iOS
Lane: iOS(`ios/`。M3 の Phase 6。どの AI が進めてもよい)
Active: なし(P6-6 完了・main 統合済み。残る P6-7 は record/team の API 待ち)
Branch: feat/ios-p6(作業ディレクトリ ~/MyDamageCalcurater-ios)
Status: **M3(iPhone で使える)は完了**。P6-1(ADR-0500)・P6-2a 計算画面・契約追従・P6-2b 逆算画面・P6-2c 構築ビルダー
(一覧・編集・ニックネーム)・P6-2d(構築から個体を呼び出す配線)・P6-3・P6-4(手順書 `docs/runbooks/ios-device-install.md`)・
生成の internal タグ除外・DOC-ios は main に統合済み(PR #31・#53・#91・#119・#122)。
続けて Codex レビュー issue のうち iOS 主担当分を修正・main 統合済み: #100(種族変更後の特性ID残留)・#101(負のSPの
検証漏れ、PR #131)、#68(検索上限200件。種族・技ピッカーを `Menu` 一括取得から `.searchable()` 検索UIへ変更。
Web の ADR-0304 と同じ方針。PR #136)、#113(Web/iOS/API共同主担当。入力操作ごとの計算Taskを最新の1つだけ保持し
新入力・画面破棄で先行Taskをcancel、逆算の観測文字入力に200msのtrailing debounce。`CancellationError`は画面
エラーにしない。PR #166。1周目critic FAIL→2周目PASS)、#99(ライトテーマの danger コントラスト不足。
`ColorToken.danger`のライト値を`#E5484D`→`#CD1D23`に変更。Web PR #164 と同じ値。PR #170。issue #99 は
Web・iOS 両方完了でクローズ済み)、P6-6(issue #110の iOS側追従。ADR-0501「issue #110」章。`RequestLimits`/
`RequestLimitLabels` を新設し、観測16件・持ち物候補/比較64件(null込み。選べるのは63件)の上限を実装。
観測は追加ボタンを無効化、持ち物候補・比較トグルは上限到達中のON操作だけ拒否(OFFは常時可。Webの
決定的切り捨てとはあえて変えた判断はADR参照)。critic指摘でguardの位置(`beginInput()`より前)を固定する
回帰テストを追補。引き継ぎ検証で契約との同期検査 `ios/scripts/check-request-limits.sh` と観測上限の XCUITest を
追加。**PR #186 で main 統合済み**)。`make ios-test`(gen-check・件数上限の同期検査・XCTest 340件〈xcresult 集計353件〉・XCUITest 17件・Info.plist 検査)が緑。
issue #68 は既知の制約(一度も検索結果に出ていない技IDは名前解決できない)をコメントで記録した上でクローズせず残す。
issue #110 は API・データ・Web・iOS すべて完了したためクローズ済み(2026-09-24)。
Next: (1) P6-7(issue #103・ADR-0209 §8の削除UI。record-svc/team-svc実装待ち、急ぎではない)。#71(攻撃側プリセット
単一化)は engine 側の `AttackerPreset` カタログ新設(データレーン)が前提のため iOS からは未着手。将来の候補:
engine の Champions マスタが pokedex-svc 経由になったら iOS のモック/実マスタの差し替え動作を再確認、Web の
record/team-svc(M2)が進んだら iOS の構築を端末内保存から API 保存へ移行するかを検討。

## Type Balance Checker
Lane: タイプバランス(どの AI が進めてもよい。COORDINATION.md)
Active: なし(TB6・issue #105 対応 完了・main 統合済み。次はユーザー指示待ち)
Branch: 次は main から feat/tb-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-tb。git worktree)
Status: 設計書(docs/type-balance-design.md)の TB0〜TB6 はすべて main に統合済み(TB6: 技範囲チェッカー、PR #65)。P2-3b(特性の無効・吸収)の実データ確認を完了(2026-09-23): データレーンが再生成した export(348 pokemon・moves・216 abilities)で `make balance-k3d-deploy-readmodel && make balance-smoke-readmodel` を実行し、`POST .../team-balance/analyze` でチリーン(levitate)への ground 攻撃が `{"category":"immune","effect":"immune","multiplier":"0","source":"ability"}` になること、`POST .../move-range/analyze`(thunderbolt)の `walledByAbility` にエモンガ(motordrive)が正しく含まれることを実データで確認済み。メガフォームの nameJa が英語表記のままの件はデータレーンへ確認候補として残る(ブロッカーではない)。
**Codexレビュー issue #105(Argo CD導入のハッシュ・digest固定)対応も完了**(2026-09-23。ADR-0405。PR #140): `scripts/argocd-bootstrap.sh`(balance/speed共有)を新設し、balance・speed 両runbookの生URL直applyを置き換えた。実クラスタ(k3d-pokecalc)で実行し、argocd-server/dex/redisの3イメージがdigest参照に切り替わること・既存Applicationが無傷であることを確認済み
Next: (Web レーンは `make gen-ts` 実行済み。`web/src/api/balance.gen.ts` に move-range の型が反映済みであることを確認した)設計書の TB0〜TB6・issue #105 はすべて完了・実データ/実クラスタ確認済み、以後はユーザーからの新規要望待ち
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
Next: 特に無し。#105(Argo CD導入・digest固定の共有スクリプト化)はタイプバランスレーンが完了し、docs/runbooks/speed.md 節5を
scripts/argocd-bootstrap.sh の呼び出しに差し替え済み(2026-09-24 確認・追加対応不要)。#108(read model のdataVersion・rollout一本化)は
データレーンが主担当で、連絡が来たら合わせる(今は着手不要)。他は balance-registry → pokecalc-registry への改名提案(タイプバランス
レーンへ既定案で提示済み。DECISIONS.md 2026-09-23)かユーザーからの新規要望待ち。

## Judge
Lane: 判定(素早さ×ダメージ連動。`services/judge/`・`web/src/judge/`。どの AI が進めてもよい)
Active: なし(**judge-design.md §3 が定めた JD0〜JD5 すべて完了・main 統合済み**。次のユーザー要望待ち)
Branch: 次は main から feat/judge-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-judge)
Status: JD0(基盤。PR #92)・JD1(判定API本体。PR #118)・JD2(場の効果。PR #127)・JD3(複数の相手候補。PR #143)・
JD4(返り討ち判定。PR #169)・JD5(Web の画面。PR #182。ADR-0705)まで全段階が完了。`POST /api/judge/v1/outspeed-and-ko`
は自分1体対相手1〜6体の素早さ判定・場の効果(トリックルーム・追い風)・返り討ち判定まで対応し、`web/src/judge/`
(`/judge` タブ)から呼べる。技はID自由入力(ADR-0304 §3の技一覧APIの欠落を踏襲)、相手側の追い風は全候補共通の
1チェックボックス(ADR-0703 §5)、送信ボタンでのみ呼ぶ(1回で上流最大27回)。
Next: 新規要望待ち。軽微な積み残し: `attacker`単数の`Individual`にも`defenders`候補と同じ大文字小文字厳密な
キー検査を広げると契約全体で一貫する(plan.md 参照)。iOS版JD5は要望が出たら判断(ADR-0705 却下案)

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- balance の type chart: P1-13 の `testdata/golden/typechart.json` をバイト複製して同梱(ADR-0015)。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
