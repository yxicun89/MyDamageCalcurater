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
**P4-17(技の ID 解決。ADR-0304 A-13)完了・main 統合済み(PR #202)**: API レーンが新設した
`GET /api/pokedex/moves/batch`(`getMovesByIds`)を使い、オンラインモードの技選択を復活させた。
`resolveSpecies` が learnset を技の実体に解決して返す設計(`MasterSpeciesResolution.moves`)。`learnset` が
64件を超える場合はチャンク分割して並列に複数回呼ぶ(API レーンが実データで確認: 349種族中151種族・43%が
64件超・最大106件。稀な例外ではなく主経路として実装・テスト)。`capabilities.moves` の意味は変えずオンラインで
`false` のまま(`true` にすると BalanceScreen が「有効なのに技が選べない」壊れた状態になるため)。技セレクトの
disabled 判定は「今の種族の技候補があるか」に変更。critic レビュー: チャンク分割・結合順序・全体失敗の扱いを
mutation テストで確認(全滅)。重要指摘1件(攻守入れ替え・与えた/受けた切り替え後の選択中の技〈候補一覧だけで
なく実際にリクエストに乗る技〉が未検証。`<select>` の DOM 値は状態が壊れていても先頭候補にフォールバック表示
するため見逃しやすい)を受け、実際のリクエストを検査する形に既存テスト2件を強化。既存1192件は無変更・新規
28件追加(1220件)。BalanceScreen 自体の種族検索・技選択は P4-17b として積み残し(下記で完了)。
**P4-17b(BalanceScreen のオンライン対応。ADR-0304 A-14)完了・main 統合済み(PR #207)**: P4-17 で
`capabilities.moves` が永続的に false のままと決まった結果、従来のゲート `speciesList && moves` では
BalanceScreen がオンラインで永久に使えなかった。可否の判定を「一覧がそろっているか」から「入力の口が
あるか」(`(speciesList || masterSearch) && (moves || (!speciesList && masterSearch))`)に置き換え、
パーティ・仮想敵の12枠(6枠×2)それぞれで種族検索→技解決(`useSpeciesResolutions` を1画面で共有)を
独立に行えるようにした。`moveById`/`hasDamagingMove` を fail-closed に直し、実体不明の技 ID を攻撃技と
誤判定して誤解を招く診断(coverage の誤呼び出し)を出さないようにした。critic PASS(mutation testing で
ゲート条件・fail-closed 判定・種族解決の登録漏れ等の主要な変異を全て検知)。critic 指摘の軽微3件は
その場で直接修正: 種族解決の適用を index ではなく枠の id で引くよう変更(検索解決を待つ間に他の枠が
削除されると index が別の枠を指しうる競合の根治)、A-14.1 の境界表(7パターン)の未カバー2行のテスト追加、
特性名解決の重複ロジックの統一。新規15件追加(1243件)。
**P4-21(issue #67・#98)完了・main 統合済み(PR #189・#192)。Codexレビューissue(P4-18・P4-21)はこれで
すべて完了**:
- #67(2xxの契約外JSONでAPIクライアントが例外を投げる): `apiEngine.ts`・`balanceClient.ts` の `postJson` を
  型ガード経由にし(`as Schemas[...]` の型アサーションを除去)、契約外の2xxで例外を投げず
  `engine_unavailable`/`balance_unavailable` を返すようにした(ADR-0301 §4・ADR-0303 §6)。calc側は写像関数が
  読む全フィールドを再帰的に検査、balance側は画面がたどる形だけを検査(leafスカラー・enumは見ない)。
  critic PASS(mutation テスト12件で型ガードの過不足なしを確認)。新規143件追加。
- #98(モバイル幅で計算・逆算画面が横に溢れる): ブレークポイント600px(`docs/design.md`「幅への対応」に記録)。
  600px未満はmobile-firstで縦積み(DOM順・フォーカス順は不変)、600px以上は従来の左右配置。伸縮列を
  `minmax(0, 1fr)` に、カード・selectに `min-width: 0`/`width: 100%` を追加。critic が実ブラウザで13幅×5画面を
  実測し横溢れゼロを確認。新規34件追加(単体18・E2E16)。
  作業中に発見した無関係の既存退行(JD5の判定タブ追加で `a11y.spec.ts` が壊れていた)を別途修正・main統合済み
  (PR #191)。
  最終テスト数: 既存1166件は無変更のまま vitest 1184件・Playwright 31件、すべて green。
**issue #268(gateway経由:8080の白画面)の検証テスト修正・main統合済み(PR #338。実装は別セッション)**:
`services/gateway/scripts/smoke.sh` に追加された「index.htmlが読むJSを実際に取得して200」の検査により、
`TestSmokeScriptAcceptsWebDeployed` のfixtureがscript srcの無いHTMLを返していて落ちていた(タイプバランス
レーンが検証・指摘)。fixtureにscript srcとその配信先を持たせ、JSが404のケースで検知できることの回帰テスト
(`TestSmokeScriptFailsWhenEntryJSMissing`)も追加。
**issue #334(相性表記の倍率併記。iOSとの語の統一)完了・main統合済み(PR #341)**: iOSのDisplayLabels.swiftの語
(「ばつぐん(×2)」「いまひとつ(×0.5)」)にWeb側を揃えた(「効果は」接頭辞は削除)。format.test.tsの0.25/0.5・
2/4のケースが倍率を書き分けるようになり検証強化。新規0件(既存アサーション4件の強化)。
**issue #72(ルートmake e2eが未実装スタブ)完了・main統合済み(PR #350。ADR-0306)**: k3dクラスタ不要な3件
(`web-e2e`→`web-e2e-online`→`web-e2e-balance`)を必ずこの順で実行し、kubectlの現在のコンテキストが
`k3d-$CLUSTER`のときだけ`api-smoke`→`web-k3d-smoke`→`web-k3d-e2e`を追加実行するよう`scripts/e2e.sh`を実装。
クラスタが無ければ黙らずスキップを明示、`E2E_REQUIRE_K3D=1`でスキップさせない逃げ道も用意。critic PASS
(mutation testing 4件で全て検知)。`scripts/e2e_test.sh`(新規80件)を`make test-scripts`に追加。
**issue #71のWeb側(攻撃側プリセット単一化。ADR-0114)完了・main統合済み(PR #352)**:
`web/src/domain/attackerPresets.contract.test.ts`を新規追加。毎回`engine/presets/attacker.json`を読み、
カタログ(順序・既定値・relevantStat・boostMinus・relevantSp・nature)から導いた期待値と
`resolveAttackerPreset`の実際の出力を突き合わせる契約テスト(現状の値は一致済み、実装変更なし)。
JSONを一時的に書き換えるmutationで実際に検知することを確認済み。新規17件追加。iOSの追従が済めば
データレーンが#71をcloseする想定(2026-09-25時点、Web側は完了を連絡済み)。
**issue #333(375px幅でタブの名前が1文字ずつ縦に折り返す)PR #356オープン中(2026-09-25、マージは
オーケストレーターが検証後に実施)**: `App.css`の`.app-tabs__list`にoverflow-x: auto・safe center、
`.app-tabs__tab`にwhite-space: nowrap・flex-shrink: 0。**critic 1回目FAIL**: 素のcenterのままだと
はみ出した先頭タブがscrollLeft=0でも戻れない(centered flexbox overflow clipping。320pxで実測再現)→
`justify-content: safe center`に修正、design.mdに記録。safeキーワードのSafari対応はP4-5のSafari確認
(ブロッカー節)に追記。回帰テスト2件(1行であることの直接確認・スクロールで先頭末尾に到達できることの確認)。
**2026-09-25、オーケストレーター(damage calculation bug resolution)から13件のissue消化を依頼された**
(open 113件中、優先度順): (1) bug: #333(完了・main統合済み。PR #356)・#306(タイプ名コントラスト・
ダメージバー読み上げ名。完了・main統合済み。PR #362)・#275(逆算「受けたダメージ」で自分の耐久が無振り固定。
high severity。完了・main統合済み。PR #366)。
(2) ready-for-implementation: #304(完了・main統合済み。PR #375)・#308・#305・#248・#218・
#219(APIレーン連携)・#211(APIレーン連携)・#332(devDependencies更新)・#226(README等の実装状況)は
#304以外未着手。(3) needs-decisionだが「要望済み機能は実装しきる」方針で既定案付きで実装: #272(特性選択)・
#274(急所・やけど・天候・フィールド・ランク・壁・特性の指定。iOSへも連絡済み)・#210(オフライン実データ)は
未着手。P5-5はAPIレーンの契約が出たら最優先。
**#71のWeb側(攻撃側プリセット単一化)完了・main統合済み(PR #352)**。
**`make e2e`のweb-e2e-onlineがmainで壊れていた件(廃止済みCALC_TYPECHART_PATHをcalc-svcが拒否)を発見・
2PRに分けて修理・両方main統合済み**: PR1(#382、MasterExport追従。ADR-0204/ADR-0301§5追記)で
`web/src/master/exportSnapshot.ts`をMasterExportの形に全面書き換え(types/typeChart本体化、
species/moves/items/abilitiesの明示フィールド化、効果のPascalCase変換)、例データのID(move/item/ability)から
ハイフンを除去(codeIDPattern対応)。PR2(#385、pokedexフィクスチャ。ADR-0307)でpokedex-svc(MySQL必須)の
代わりにWeb例データから公開API応答を返す軽量フィクスチャ(`e2e/support/pokedexFixture.ts`+
`pokedexFixtureServer.mjs`)を追加し、`/api/pokedex`を`vite.config.ts`でそちらへ転送。
両PRとも`npm run e2e:online`実弾実行(3/3 pass)まで確認済み。critic指摘(playwright.container.config.tsの
testMatch漏れ、POKEDEX_PROXY_TARGETのvite proxyルーティングが無検査だった点等)は全て修正・検証済み。
**issue #308(オンラインでマスタ読み込み失敗時に再試行・オフライン切替ができずタブ一覧ごと消える)
完了・main統合済み(2026-09-25。PR #390)**: 失敗時もタブ一覧を残し、マスタを使わない「素早さ」画面
(ADR-0604 §5)は選んで使えるようにした。マスタを使う4画面(計算/逆算/タイプバランス/判定)が選ばれている
ときは、原因(Errorのmessage)・「再試行」・(オンラインのときだけ)「オフラインに切り替える」を出す。
自動フォールバックはしない(ADR-0301 §4の既定方針を維持)。`app/routes.ts`の`SCREEN_ROUTES`に
`usesMaster: boolean`を追加し、`app/screens.tsx`の`MASTERLESS_SCREEN_COMPONENTS`(`ScreenProps`から
`master`・`engine`を除いた`MasterlessScreenProps`)経由で描画(設計判断はADR-0304追記6)。
spec-writer→implementer→critic(PASS、mutation testing 7件で全て検知)を経て、critic指摘のうち
マスタ不要画面へengineが静かに伝播するリスクは直接修正、残り2件(再試行中のフィードバック欠如・
素早さタブでの通知非表示)はplan.mdに申し送り。`npx vitest run App`101/101・`npm test`1605/1605・
typecheck/lint無回帰。
**issue #305(逆算で観測を説明できる候補が1つも無くても、その旨が出ず「近い候補」とSP範囲が並ぶだけ)
完了・main統合済み(2026-09-25。PR #392)**: `result.exactCount === 0 && result.candidates.length > 0`
のときだけ、結果の先頭に`role="status"`の案内(「入力した観測を説明できる調整がありません」相当)を出し、
各候補のSP範囲に「参考」の印をテキストで添える(候補一覧自体は消さない)。各候補の%欄には常時「予測」の
ラベルを添える。一致判定はengineが返す`ReverseResult.exactCount`をそのまま使い、TS側での再判定は追加して
いない(ADR-0300 §8)。spec-writer→implementer→critic(PASS、mutation testing 5件で全て検知)。
critic指摘の軽微な1件(role="status"テストの頑健性)は直接修正、他は非ブロッキングのため見送り
(候補0件時の文言欠如は別issue候補として観察のみ)。`npx vitest run ReverseScreen`94/94・
`npm test`1611/1611・typecheck/lint無回帰。
**issue #248(計算画面のオンライン計算にAbortSignalを渡していない)完了・main統合済み(2026-09-25。
PR #396)**: `ReverseScreen.tsx`(issue #113)と全く同じ形で`CalcScreen.tsx`のcalcBulk用`useEffect`に
`AbortController`を追加(effectごとに作り、cleanupで`abort()`)。WASM(オフライン)モードは`signal`を
無視するだけなので挙動不変。`web/src/test/fakeEngine.ts`の`PendingBulk`に`signal`フィールドを追加
(`PendingReverse`と対称)。severity低・既存承認済みパターンの横展開のため、CLAUDE.md「軽微な作業は
メインのみでよい」に従いメインセッションで直接実装し、独立criticでレビュー(1回目はopusのセッション
利用枠上限で失敗、CLAUDE.mdのモデル割り当て方針に従いsonnetで再実施してPASS。mutation testing 2件・
WASM無回帰を確認)。`npx vitest run CalcScreen.test`33/33・`npm test`1612/1612・typecheck/lint無回帰。
**APIレーンのPR #372(issue #271/#270のunsupported: UnsupportedMark[]追加。CalcResult/ReverseCandidate)が
main統合済み**: Web側の対応は不要(`apiEngine.ts`のmapCalcResult/mapReverseCandidateが明示的フィールド
写像のため増えたフィールドは自動的に無視される。issue #67の前方互換どおり)。印を画面に表示するかどうかは
Webレーンの判断(DECISIONS.md 2026-09-25参照)。
**issue #218(タブを切り替えると計算・逆算の入力状態が消える)完了・main統合済み(2026-09-25。PR #407)**:
ADR-0308に沿って実装。(1) lazy-mount-then-keep-alive(一度選ばれたタブだけmount、以後unmountしない。
SpeedScreenのマウント時eager fetchを避けるため全画面の先読みはしない) (2) `role="tabpanel"`は1つのまま、
非選択画面はネイティブ`hidden`属性で隠す (3) 計算モード切り替え(マスタ入れ替え)ではタブの殻ごと
リセットしてよい(古いマスタの計算結果が残るより安全。ADR-0304 A-6の既存unmount挙動を利用) (4) リロードは
初期状態(storage不使用)。`visitedTabs`はマスタ取得口が変わるたびに作り直される`AppTabPanel`子コンポーネント
自身のstateに置き、モード切替時に隠れた素早さタブが余分にAPIを叩かない設計。
**critic 1回目FAIL(2点、実測込み)**: `masterEpoch`がデッドコードでADR決定3が未検証/マスタ再読み込みの
たびに隠れた素早さタブが再マウントしてspeed APIを二重に叩く実害(2件→4件を実測)。implementerが
`masterEpoch`削除・`visitedTabs`の置き場所変更で対応、**critic 2回目PASS**(mutation testing 5種・
実測プローブで両問題の解消を確認)。`npx vitest run`1625/1625・`make web-e2e`37/37・typecheck/lint無回帰。
**運用インシデント**: 実装1回目の際、worktree競合で実装者エージェントが`git update-ref`でブランチ参照を
強制移動する場面があった(データ損失は無し、コーディネーターが検証済み)。次回以降はスキル間で
worktreeを都度削除してから次段階へ進む運用に修正済み(メモリに記録)。
**issue #271・#270(計算・逆算・bulkの結果に「未対応」の印を表示)完了・main統合済み(2026-09-25。
PR #412)**: ADR-0123に沿って`unsupported: UnsupportedMark[]`(target/reason/id)を表示。全行(全候補)に
共通する印は結果一覧の先頭に1回、一部の行(候補)だけの印はその行だけ(`splitUnsupportedMarks`。
`web/src/domain/unsupportedLabels.ts`)。色は`--danger`でなく`--text-secondary`(エラーではなく目安の
ため警告色にしない)。spec-writer→implementer→**critic 1回目PASS**→**レビュー直後にiOSレーンが
DECISIONS.mdへクロスプラットフォームの文言・配置・色の決定を追加**したため追加のimplementerラウンドで
整合(iOSの`DisplayLabels.swift`と文言を1件ずつ突き合わせ完全一致)→**critic 2回目PASS**。
JudgeScreen(JD5)は別contract(`attackerKoUnsupported`/`defenderKoUnsupported`)のため対象外、別タスクとして
plan.mdに記載。`npx vitest run`1674/1674・`make web-e2e`37/37・typecheck/lint無回帰。opusのセッション
利用枠上限で両criticともsonnetで代替実施(CLAUDE.mdのモデル割り当て方針どおり)。
**P5-5a(構築ビルダーの骨格。一覧・新規作成・名前変更・削除)完了・main統合済み(2026-09-25。PR #417)**:
新規タブ「構築」(`/team`、末尾、`usesMaster: true`)。`web/src/team/teamClient.ts`(専用`.gen.ts`は作らず
ルート共有の`openapi.gen.ts`を使う。team/recordはルート契約に同居しgateway経由のため)。削除確認は
`window.confirm`を使わず行内の2段階ボタン。書き込み後は`list()`を呼び直さず応答の`Team`で手元を書き換える。
**メンバー編集(種族・技・持ち物・特性・性格・SP・テラスタイプ)は次のPR(P5-5b)で別途**(ADR-0309「却下した案」)。
spec-writer→implementer→**critic 1回目PASS**(重要指摘2件: 名前変更の送信前検査漏れ・list()応答と
create/update/removeのレースコンディションで作成直後の構築が消えて見える不具合)→implementer(修正)→
**critic 2回目PASS**。`npx vitest run`1745/1745・`make web-e2e`37/37・typecheck/lint無回帰。
判定レーンがShowdown形式インポート/エクスポートをブランチ`feat/web-team-showdown-format`(`web/src/team/`
配下)で並行して進めている(分担合意済み。member editorとファイルが重ならないよう次のPR着手前に確認)。
Next(P8-1c 画像表示 Web 分): 実装済み・コミット前(ADR-0325 採用)。残りは PR 化のみ。判定画面の画像は判定レーンが `PokemonImage` を使えば足せる(`web/src/judge/` は未編集)。
Next: **P5-5(構築ビルダー・Showdown 形式の入出力を含む)は全子項目が完了**(P5-5e = ADR-0321。履歴一覧は record-svc に API が無く対象外)。
**Web レーンの実装は完了**(2026-10-03。#451・#459・#462・#481・#486・#489・#525 を ADR-0803 の手順〈CI 全件成功+`--match-head-commit`〉でマージ済み)。
2026-10-01〜03 に消化した issue/タスク(1 issue = 1 PR): #218(PR #407)・#219(#420・ADR-0310。nginx のセキュリティヘッダ)・#272 Web 分(#430・ADR-0311。特性セレクト)・
#274 Web 分(#433・ADR-0312「詳細」、#462・ADR-0315 防御側ランク)・#210(#451・ADR-0313。**既定の計算モードをオンラインに変更**+IndexedDB キャッシュのオフライン)・
#332 Web 分(#454)・#328 Web 分(#459・ADR-0314。情報ページ)・P5-5b(#481・ADR-0316。メンバー編集)・P5-5c(#486・ADR-0317。よく計算する相手)・
P5-5d(#489・ADR-0318。この端末のデータを削除。issue #103 の Web 側)。#226 は D29 でクローズ済み、#271/#270 の Web 分は #412 で完了。
2026-10-03 追加: #515(メガ種族を選ぶと持ち物をメガストーンに固定)の Web 分が完了・main 統合済み(#535 公開 API の SpeciesDetail に isMega/requiredItemId〈Web レーンが越境して実装。ユーザー決定〉・
#537 PR-A〈共通ドメイン+計算・逆算〉・#540 PR-B〈構築の編集・判定・古い保存データの補正〉。ADR-0320)。残りはデータレーン(メガ名の生成・検索)と iOS レーン(同じ挙動。文言は `megaItemText` に揃える)。
**待ち(他レーン)**: (1) #211: Web 分は完了(ADR-0322。オンラインは `effect` を camelCase に写し、効果ありの持ち物が1件でもあれば `effects:true`。オフライン〈キャッシュ〉の「持ち物の候補も比較」は無効のまま)。残りは iOS 分(decisions/2026-10-03-web-211-online-effects.md)。
(2) M2 の実機確認: `make deploy-latest` に record・team・TiDB・NATS が無い(API レーン。決定ファイル `decisions/2026-10-03-214-web-m2-k3d-deploy.md`)。入ったら `docs/verify-m2.md` §2 を実機で確認し前提の注記を直す。
(3) #332 の残り(`services/pokedex/Dockerfile` の Node〈データ〉・golang タグ統一・定期検出〈運用〉)。(4) #271/#270 の判定画面での表示(判定レーン)。
(5) iOS へ防御側ランク文言の統一提案(ADR-0315・`decisions/` の該当ファイル)。
(3) P4-20: issue #148(アクセス境界・認証方針)。Web 側は既にコード上で条件を満たしていることを確認済み
(apiBaseUrl の既定値は同一オリジン、CORSはgateway側の設定)。実際のtailnet名が決まってから運用レーンより
連絡が来る想定。(5) 人間へのお願い:
docs/verify-m1.md §6(ブラウザ確認)を Safari で確認(P4-5。issue #333のsafeキーワード確認も合わせて)

**画面レジストリ化(2026-10-03。ADR-0323。ブランチ feat/web-screen-registry)**: 画面・タブは各レーンのディレクトリの登録ファイル
`*.screen.tsx`(`defineScreen`)で足す。`App.tsx`・`app/screens.tsx`・`app/routes.ts`・`i18n/ja.ts` は画面の追加では触らない。
App.tsx・app/screens.tsx・app/routes.ts・i18n/ja.ts を編集している未マージの Web の PR などは、main を merge して変更を登録ファイル・`i18n/<レーン>.ts` に移す(ADR-0323 §5)。

**持ち物の役割とメガストーン表示(2026-10-03。ADR-0326。ブランチ feat/web-item-roles)**: ユーザーの実使用の不具合報告(メガストーンの英語表記・攻撃側に意味のない持ち物)の修正。
持ち物欄は `web/src/domain/itemRoles.ts` の `itemsForRole` で絞る(`roles` が無い持ち物は絞らない・メガストーンはどの欄にも出さない)。
固定中の表示は「{基本種名}のメガストーン」。文言は `web/src/i18n/items.ts`。マスタのキャッシュ版は 3。iOS の同じ語での追従は iOS レーンに残る。
