# Current State

## Damage Calculator
Lane: データ(engine・マスタ・pokedex。どの AI が進めてもよい。COORDINATION.md)
Active: Claude Code
Branch: feat/claude-p1-engine(作業ディレクトリ ~/MyDamageCalcurater)
Status: Phase 1・P2-1・P1-10・Phase R・P1-13・P1-11・P1-12・P2-1b・P2-1c・P2-2a・P2-2b・P2-2c・P2-2d・P2-3(pokedex-svc。内部 API・公開 API・natures・balance/speed 向け export。ADR-0105)は完了(critic レビュー済み)
Status(追記): P2-3b(無効・吸収の特性)も完了・main 統合済み(ADR-0106)。calc・gateway の pokedex-svc 接続(API レーンの依頼)も PR #87 で解決済み(api-smoke で master=pokedex 確認済み)。
Status(追記): P5-6(技の追加効果によるランク変化。ADR-0107)完了・critic PASS(1往復)・**main 統合済み(PR #132)**。engine は乱数を持たず「発動した場合の値」だけを返す。ゴールデン不変。`move_effects` 別表・`MasterMove.effect`(内部API)まで。公開APIへの露出(判定レーンが技IDからランク変化を引く経路)は判定レーンの要件確定後に別途対応。
Status(追記): issue #110 のデータレーン担当分(ADR-0108)完了・critic PASS(1往復)・**main 統合済み(PR #138)**。`engine.CalcBulk`/`CalcReverse` と `engine/wasmapi` に ADR-0208 §1 と同じ件数・範囲の上限を追加し、wasmapi は DTO 変換より前に検査して HTTP との parity を確保。issue #110 は Web・iOS レーンの追従が残っている限りクローズしない。
Next: (1) Codexレビュー issue #106(排他制御)を優先、続いて #104/#109/#112。(2) 他レーンからの依頼待ち。人間の確認待ち(plan.md ブロッカー): 観測%の丸め方(整数%表示は確認済み)、公開のタイミング(LICENSE・クリーンコピー)

## API
Lane: API(calc-svc・gateway・契約テスト。`api/openapi.yaml` の持ち主。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/api-issue110-limits(PR で main へ。作業ディレクトリ ~/MyDamageCalcurater-api)
Status: Phase 3 完了、P3-4〜P3-6・DOC-api は main に統合済み。issue #110(セキュリティ。Codex レビュー)の API レーン担当分(契約の maxItems/uniqueItems・calc-svc の自前検証。ADR-0208)は critic PASS。PR 作成待ち
Next: (1) issue #110 の PR を main へ(マージ後、他レーンへ依頼: engine/wasmapi に同じ防御上限、Web/iOS の観測16件・候補64件UI。DECISIONS.md に既定案あり。issue はレーンの完了までクローズしない)。(2) issue #103(M2保存データの保持・削除・端末ID境界。ユーザー決定=一定期間の自動失効。ADR作成。データレーンと調整)

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
**P4-16(オンライン MasterSource の基盤。ADR-0304)完了・main 統合済み(PR #128)**: `createOnlineMasterSource`
(持ち物・性格を全件取得、種族は `searchSpecies`/`getSpecies` の検索専用インターフェース)、
`MasterData.capabilities`(技選択・持ち物候補比較・特性一覧は公開 API の欠落により明示的に無効化)、
`App.tsx`/`main.tsx` の配線。critic 1回目 FAIL で重大バグ発見(`apiBaseUrl()` の既定値 `"/"` で
`new URL(path, baseUrl)` が例外を投げ、オンラインモードが常に失敗していた)→ 修正・回帰テスト追加 → 2回目 critic PASS。
既存のオフライン・全画面・既存752件のテストは無変更。技の ID→実体化(`getSpecies.learnset`)は公開 API に手段が無く、
データ/API レーンへ既定案付きで提案済み(DECISIONS.md 2026-09-23、未回答・急ぎではない)。
**P4-16b(画面側。ADR-0304 A-9〜A-11)も完了・main 統合済み(PR #134)**: CalcScreen・ReverseScreen は技・持ち物候補比較が
無効なとき disabled+案内、種族一覧が無効なとき検索欄(`SpeciesSearchField`)。BalanceScreen は `speciesList`・`moves`
が両方そろうまで画面ごと無効化し balance API を1本も呼ばない(A-9。技が空のまま誤解を招く診断を返さないため)。
critic 1回目 FAIL(検索候補が1件でも「候補が多い」と誤案内する文言バグ、BalanceScreen のガードが実質未検証だった点)
を修正・テスト強化して2回目 PASS。既存803件は無変更・新規30件追加(833件)。
キーボード操作・CSS 等の残りは P4-16c として plan.md に理由付きで分離(ブロッカーではない)。
**P4-19(issue #110 セキュリティ。ADR-0300 §10)も完了・main 統合済み(PR #142)**: 持ち物候補
(defenderItemVariants・reverseItemCandidates)を配列を作る最終地点で64件に決定的に絞り込み(選んだ持ち物は
落とさない)、観測は16件で「観測を追加」を disabled+role="status"案内。critic PASS(セキュリティ関連のため
境界値を重点検証: candidates 0〜80件×選択パターン×compareON/OFFの約1.2万ケース網羅探索と変異テスト5件で、
itemVariants/itemCandidates が常に64以下・observationsが17件目を作れないことを確認)。既存857件は無変更・
新規9件追加(866件)。データレーンの engine/wasmapi 側(ADR-0108・PR #138)も main 統合済み。issue #110 は
iOS の追従待ちで Web 単独ではクローズしない。
Next: (1) P4-16c(検索欄のキーボード操作・CSS 等。plan.md 参照)。
(2) P4-18(Codexレビュー issue。タイプバランスレーンから連絡): 優先 #99(アクセシビリティ)・#113(debounce/cancel)。
(3) P4-17: 技の ID 解決(データ/API レーンへの依頼。DECISIONS.md 2026-09-23 提案・未回答)が入ったら技を復活。
(4) 続いて P5-5(構築ビルダー等)は record/team の API 待ち。
(5) 人間へのお願い: docs/verify-m1.md §4 を Safari で確認(P4-5)

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
Next: 特に無し。他レーンからの依頼(Codexレビュー issue #105・#108。上記)かユーザーからの新規要望待ち。balance-registry →
pokecalc-registry への改名提案はタイプバランスレーンへ既定案で提示済み(DECISIONS.md 2026-09-23)。
Codexレビューissue(2026-09-23、タイプバランスレーンから連絡): #105(Argo CD導入・digest固定の共有スクリプト化)はタイプバランスレーンが
主担当で進め、できたら docs/runbooks/speed.md の該当節をその呼び出しに差し替えるだけになる見込み(今は着手不要)。#108(read model の
dataVersion・rollout一本化)はデータレーンが主担当で、連絡が来たら合わせる(今は着手不要)

## Judge
Lane: 判定(素早さ×ダメージ連動。`services/judge/`。どの AI が進めてもよい)
Active: Claude Code
Branch: feat/judge-jd3(作業ディレクトリ ~/MyDamageCalcurater-judge。PR 作成待ち。JD2 の feat/judge-jd2 は PR #127 で main に統合済み・削除)
Status: JD0(基盤。PR #92)・JD1(判定API本体。PR #118)・JD2(場の効果。PR #127)・JD3(複数の相手候補。ADR-0703)は完了。
`POST /api/judge/v1/outspeed-and-ko` の request の `defender`(単数)を `defenders`(1〜6件の配列)に、response を
`matchups`(配列。`defenderIndex`・`outspeeds`・`speedTie`・`attackerSpeed`・`defenderSpeed`・`ko`)に破壊的変更した
(クライアントがまだ無い=JD5未着手なので安全と判断。ADR-0703 §7)。上流は natures 1回+attacker種族1回+候補種族N回+
calcN回の逐次、最初に失敗した候補で全体を打ち切る(部分成功なし)。critic PASS(1回目)。`internal/judge` は無変更
Next: JD4(相手の技を含めた返り討ち判定)に着手。技の優先度(priority)が要るが pokedex-svc に個別取得endpointが無く、
API レーンへ依頼中(DECISIONS.md 2026-09-22)。依頼が通るまで「両者優先度0」の限定で進めるか待つかを最初に判断する

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- balance の type chart: P1-13 の `testdata/golden/typechart.json` をバイト複製して同梱(ADR-0015)。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
