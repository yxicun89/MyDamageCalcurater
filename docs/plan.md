# 開発計画と進行状況

凡例: `[ ]` 未着手 / `[~]` 作業中 / `[x]` 完了 / `[!]` ブロック中
作業する AI は、タスク開始時に `[~]`、完了時に `[x]` へ更新してからコミットする。

## マイルストーン

| ID | ゴール | 人間の確認方法 |
|---|---|---|
| **M1** | **ブラウザで計算できる**(Mac上のk3d) | `make up` → http://localhost:8080 で計算・一括表示・逆算を触る |
| M2 | 計算の自動保存・よく使う・構築 | 計算後に履歴と「よく計算する相手」が出る |
| M3 | iPhone で使える | Xcode から実機に入れて Tailscale 経由で計算 |
| M4 | 運用(監視・SLO・GitOps) | Grafana でSLOダッシュボードを見る |
| +α | クラウド移行 / ダブル / 推薦ML | |

**キックオフでは M1 完了まで自動で進める。** M2 以降は人間が `/phase M2` などで開始する。

---

## M1: ブラウザで計算できる

### Phase 0 土台
- [x] P0-1 `scripts/doctor.sh` を実行し不足ツールを報告(入れられるものは brew で入れる)
- [x] P0-2 Go workspace(go.work)、Makefile、.gitignore、各ディレクトリの雛形
- [x] P0-3 `api/openapi.yaml` の初版(pokedex検索 / 計算 / 一括計算 / 逆算)と `make gen`
- [x] P0-4 k3d クラスタ定義(`deploy/k3d.yaml`)と `make up/down`、Kustomize base/overlays
- [x] P0-5 `scripts/codex-review.sh` の動作確認(Codex 未ログインならスキップして記録)

### Phase 1 計算エンジン
- [x] P1-1 データモデル(種族・技・持ち物・特性・性格・フィールド状態・format)
- [x] P1-2 実数値計算(SP・性格・ランク)+ 全ポケモン網羅テスト(実数値)
- [x] P1-3 ダメージ計算コア(基本式・乱数16段階・一致・相性・急所・やけど)
- [x] P1-4 補正(天候・フィールド・壁・持ち物・特性)※マスタの補正定義から適用
- [x] P1-5 確定数/乱数n発の算出
- [x] P1-6 `tools/golden` でテストベクタ生成、`make test-golden` 全件一致
- [x] P1-7 一括計算(防御側の代表調整すべてに対する結果を一度に返す)
- [x] P1-8 逆算(観測ダメージ→調整候補、複数観測で絞り込み)+ 再現率テスト
- [x] P1-9 WASM ビルド(`make wasm`)と Go/WASM の結果一致テスト

### Phase R コーディング規約の策定とリファクタリング(2026-09-21 のユーザー依頼)
規約は [docs/coding-rules.md](coding-rules.md)(Claude Code と Codex 共通。公開できる状態を保つ・ハードコードしない・読みやすいコード)。Phase 1b より先に行う。
- [x] R-0 規約の策定と Codex レビュー(v2。Codex は条件付き承認で、条件2点を反映。反映後の Codex の再確認は未実施)
- [x] R-1 監査: 規約違反の洗い出し(読み取り専用)。結果は [docs/audit-r1.md](audit-r1.md)(条件付きで公開可。重大な違反なし)
- [ ] R-2 是正(挙動を変えない。`make test` / `make test-golden` / `make test-wasm` を維持。塊ごとに1コミット。詳細は audit-r1.md):
  - [x] R-2-1 シェルを `set -euo pipefail` に統一(単独コミット。`doctor.sh` は挙動を確認)
  - [x] R-2-2 fixture の公式日本語名を架空名に置換
  - [x] R-2-3 `tools/golden/package.json` を `0.10.0` に完全固定、`.gitignore` に `*.wasm` `*.pem` `*.key` `*.p12` を追加
  - [x] R-2-4 ドメイン定数に名前を付けて集約(75/20/2048/6144/8192、プリセットの 32)
  - [x] R-2-5 小さな可読性の是正(`Version` の重複、空コメント、`real` の改名、`AllStatKeys` の可変性)
  - [x] R-2-7 ADR の追記・修正(タイプ相性表=ADR-0013、engine の Label=ADR-0009 §1-a、内輪の表現の修正、ADR-0002 の第三者データ抜粋の削除)
  - [x] R-2-8 Go の module path を公開用プレースホルダ(`example.com/pokecalc/...`)に置換(3つの go.mod・import・go.work・テストの import 検査。Codex の `services/balance` は取り込み時に合わせる)。ユーザー決定: アカウント名は公開しない
  - [ ] R-2-9 公開用クリーンコピーの作成(ユーザー決定: 履歴を書き換える。方式は**今のリポジトリと worktree には触れず、書き換えたコピーを別に作る**)。`scripts/make-public-copy.sh`(仮): 公開したいブランチをローカル clone → `git filter-repo` で作者名・メールを公開用 identity(`pokecalc-dev <noreply@example.com>`)に置換(`--mailmap` を既存の作者から動的に生成するので、実名をスクリプトに書かない)、履歴内の `github.com/<アカウント名>/pokecalc` を `example.com/pokecalc` に置換(正規表現)→ コピー側で `make check-publishable-full`(Git 作者の許可リストを作成)と履歴全体の検査(絶対パス・メール・秘密)。実行は**公開するとき**。前提ツール: `git-filter-repo`(`brew install git-filter-repo`。`make doctor` の任意ツールに追加)。元のリポジトリ(Codex の `pokecalc-codex-tb0`・`pokecalc-main` を含む)は変更しない
- [x] R-3 `make check-publishable`(秘密情報・絶対パス・個人情報・追跡してはいけないファイルの検査。`make lint` から呼ぶ)

### Phase 1b 決定の反映(2026-09-21 のユーザー決定。ADR-0002 の確定方針・DECISIONS.md 参照)
- [x] P1-10 防御プリセットの再定義: `hb` = H32・B32・性格補正なし / `hd` = H32・D32・補正なし / `hb_boost` = H32・B0・B上昇性格 / `hd_boost` = H32・D0・D上昇性格 / **新設** `hb_full` = H32・B32・B上昇性格 / `hd_full` = H32・D32・D上昇性格。ADR-0009・`engine/bulk.go` のカタログと既定セット・テスト・`tools/golden`(旧 `hb`/`hd` の外部照合ベクタは性格補正ありなので `hb_full`/`hd_full` 相当。補正なしの `hb`/`hd` を追加)・`api/openapi.yaml`(`DefenderPreset` enum と例)を更新して `make gen`(絶対ルール1)。`hb_boost`/`hd_boost` の確認待ちは解消
- [x] P1-13 タイプ相性表のデータ化(ADR-0013): `TypeChart` 型を engine の入力(`DamageInput`)にし、`typechart.go` の埋め込み表を削除。テストは fixture、`tools/golden` が oracle の相性表を出力。`engine/wasmapi` のリクエスト・ADR-0011 を更新(ゴールデンの期待値は不変)。P1-11・P1-12 と独立に進められるが、`DamageInput` を触るので順番を調整する
- [x] P1-11 表示%の分離(ユーザー方針: **最小ダメージ側は切り捨て、最大ダメージ側は四捨五入**にして「最低これくらい入る」を保守的に示す。**乱数で何%で倒せるか**(`ko.chancePercent`。乱数n発の確率)も、結果に必ず表示できる形で出力する): アプリが表示する計算結果の%は**小数第1位**(例 73.4%)。HP 比率から直接求め、整数%に丸めない(整数演算で 0.1% 単位)。実機画面の観測%(整数%)を逆算の入力にする場合の丸め規則は**別関数・別概念**にし、同じ `DisplayPercent` に混在させない。engine・`engine/wasmapi`(`minPercent`/`maxPercent`)・`api/openapi.yaml`・ADR-0010/0011 を更新
- [x] P1-12 逆算の再設計: 固定プリセットからの選択をやめ、**H32 前提で B(D) SP を 0〜32 探索**、性格は「補正なし」「B(D) 上昇」の2通り(攻撃側の A/C も同様に0〜32×補正なし/上昇)。結果は「性格補正あり/なし × 持ち物」ごとの**SP の範囲**で、観測から区別できない候補は決め打ちせず残す。ADR-0010 を改訂、再現率テスト(Recall@5 の定義=真値の SP が範囲に入る等)を再設計し基準(1観測 ≥80%・2観測 ≥95%)は緩めない。P1-11 の後

### Phase 2 マスタデータ
- [x] P2-1 **データソース調査**: チャンピオンズの使用可能ポケモン・技・持ち物の取得元を調べ ADR-0002 に記録(調査完了。ADR-0002 は 2026-09-21 のユーザー決定で方針確定。残る確認事項はブロッカー節)
  - 確定後に、ゴールデンの種族集合(現在は gen9 参考集合1392種)を差し替えて `make golden-generate` で再生成し、`metadata.json` の `speciesScope` を更新する
  - HP=1 のヌケニンはゴールデンから除外している。チャンピオンズの実数値式(HP = 種族値+75+SP)では HP=76 になるため、扱いを決める
- [x] P2-1b ゴールデンの oracle を `@smogon/calc@0.12.0` の Champions へ切り替え: `tools/golden/package.json` を `0.12.0` に**完全固定**(`^` 不可)。**先に旧ゴールデンと 0.12.0 Champions の結果を diff し、差分を確認してから**更新する(いきなり上書きしない)。種族集合を Champions 集合へ、SP の換算(`8×SP−4`)が不要になる。`known_diffs.yaml` への追加は ADR 付きで人間レビュー(CLAUDE.md)。`make test-golden` 全件一致。ADR-0002 §決定 5
- [x] P2-1c 技の使用可否の調査: calc のみが持つ 11 技、Showdown のみの 1 技、タイプの食い違い 2 件(個別名は載せない。A と B の差分スクリプトで `data/generated/` に出す)を、GameWith のポケモンチャンピオンズのデータやポケモン徹底攻略など**公式以外の攻略サイト**(規約・アクセス頻度に配慮し、必要最小限の取得)で調べ、出典 URL と確認日付きで**結論だけ**を ADR-0002 に追記(第三者データの一覧は載せない)。断定できない項目は「未確認」と書き、最終裁定はユーザー
- [ ] P2-2 MySQL スキーマ(migrate)と importer(`types` / `type_chart` テーブルを含む=ADR-0013。**実マスタ・スナップショットは Git にコミットしない**: `data/generated/` は .gitignore、Git には schema・importer・架空データの example・README・データの版 metadata のみ。使用可能集合・持ち物候補は**レギュレーション(v1 は M-C)依存のデータ**で、M-C を直書きしない。日本語名は PokeAPI+ローカル override。ADR-0002)
  - [x] P2-2a スキーマと migrate(golang-migrate・sqlc)。`types` / `type_chart`・種族(メガの `is_mega` / `base_species_key` / `required_item_id`、性能が同じ見た目違いフォームは1件)・技・持ち物・特性・効果定義(ADR-0005)・レギュレーション(使用可能集合)・日本語名・データの版。k3d の MySQL と `make` からの migrate
  - [x] P2-2b importer の取得・変換(calc 0.12.0 Champions・Showdown champions mod・PokeAPI の日本語名+override → `data/generated/`)と DB への投入(冪等)
  - [x] P2-2c 照合と差分報告(calc と Showdown の差分、P2-1c の裁定の反映)
  - [x] P2-2d CronJob(週1回・版に変化が無ければ取り込まない)と `make import`
- [x] P2-3 pokedex-svc(検索・詳細・持ち物/技一覧、日本語名で前方一致)
  - calc-svc 向けの内部 API `GET /internal/pokedex/master`(契約は api/openapi.yaml の MasterExport。ADR-0204。API レーンの依頼): Ingress に出さずクラスタ内の Service だけ、DB に未投入なら 503 `master_unavailable`、使用可能集合で絞らない、effect は item_effects / ability_effects の JSON をそのまま、species に showdownId を含める
  - 性格のマスタ `natures`(id, name_ja, plus, minus)を追加する(API レーンの依頼。ADR-0100 の「性格は engine の固定」を改める。新しい migration と importer の取得・変換。日本語名は PokeAPI+override)
  - balance 向けの read model の出力(`pokedex export`。ADR-0100 §8)。タイプバランスレーンの依頼(TB5。ADR-0401 §5 の形、schemaVersion 1 のまま省略可能な項目を足す): 各ポケモンに `nameJa` と `abilityIds`(隠れ特性を含む)、出力を既定のレギュレーションの使用可能集合に絞る、特性の read model(ADR-0017 の正規化された効果)も同じ export で出す。出力は `services/balance/schema/` の JSON Schema(ADR-0402)に合うことをテストで確かめる(整数は `2` の形。`2.0` は不可)

### Phase 3 API
- [x] P2-3b 無効・吸収の特性(ふゆう・ちょすい等)を engine の効果定義(AbilityEffect)と DB・importer・export に足す。ダメージ計算でも 0 になるようにし、ゴールデン(oracle)と照合する(2026-09-22 ユーザー決定。タイプバランスの判定にも反映される。ADR-0106)
- [x] P3-1 calc-svc(起動時にマスタをメモリへ読み込み)。契約の変更・マスタ境界・受け入れ条件は ADR-0200(critic PASS。マスタは暫定の `Store` と架空データ。共通マスタ P2-2a が main に入ったら差し替え)
  - 一括計算(`/api/calc/bulk`)の対応: API の `presets`(enum 配列)→ engine の `PresetKeys`。`presets: []` と省略はどちらも既定セット
  - `api/openapi.yaml` の description を先に直して `make gen`(絶対ルール1): 「変化技は none/hp の2件のみ返す」「行の順序はプリセット優先(presets × itemVariants)」「`presets: []` は省略と同じ」。`BulkCalcRow.preset` は enum のみ(engine のカスタム `Presets` は API に出さない)
  - 逆算(`/api/calc/reverse`)の対応(ADR-0010 §R8 の持ち越し。engine と WASM 境界は P1-12 で新仕様済み・API 未変更): `api/openapi.yaml` を先に直して `make gen`(絶対ルール1)。`ReverseRequest.attacker` / `defenderSpeciesKey` を `known` / `unknownSpeciesKey` に改名(`side=attacker` のとき既知側=自分=防御側)、`itemCandidates: [ItemId]` を追加。`Observation` は `percent`(整数%)/ `percentTenths` / `damage` のちょうど1つ(整数でなければ 400)。`ReverseCandidate` を P1-12 の形(`natureClass` / `nature`(calc-svc が性格 ID に写像)/ `itemId` / `ranges:[{min,max}]` / `spCount` / `exact` / `mismatch` / `support` / `minPercent` / `maxPercent`)にし、結果に `assumedHpSp` を足す。旧 `archetypeKey` / `presetLabel` / `matchScore` は使わない。`CalcResult.minPercent` の description に残る `ObservedPercent` への言及を直す(ADR-0010 §R8)
  - WASM 境界との契約差分の解消(ADR-0011 §10 の持ち越し。P1-9 では `api/openapi.yaml` を変更していない): `api/openapi.yaml` を先に直して `make gen`(絶対ルール1)。`CalcResult.minPercent/maxPercent` は P1-11 で解消済み(小数第1位の表示%。`ko.displayChancePercent` も追加済み)、`CalcResult` に `category` を足すか(Web は要求から知っているので落とすか)を決める、`BulkCalcRow` に防御側の `defender{sp,nature,stats}`(SP・性格・実数値)を足す、エラーの `code` 語彙を WASM 境界(ADR-0011 §5 の `invalid_json` / `unknown_field` / `invalid_enum` / `invalid_input` / `unknown_preset` / `duplicate_preset` / `invalid_preset` / `invalid_reverse_side` / `no_observation` / `invalid_observation` / `internal`)と共通化し、同じ失敗が HTTP と WASM で同じ `code` になるようにする
- [x] P3-2 gateway(ルーティング・端末ID/セッションID・/assets・CORS)。設計・受け入れ条件は ADR-0202(calc-svc の重複ヘッダは `invalid_header` に統一)
- [x] P3-3 契約テスト(OpenAPI 準拠)と k3d 上のスモークテスト。gateway 経由の契約表・マニフェストの静的検査・`make api-k3d-deploy` / `make api-smoke`・`make dev`(ADR-0203)
- [x] P3-4 calc-svc のマスタを pokedex-svc の内部 API(`GET /internal/pokedex/master`・MasterExport)から受け取る形に変更(ユーザー決定 2026-09-22。ADR-0204)。k3d と dev は同じ形のファイル(架空データ)。pokedex-svc(P2-3)のデプロイ後に local overlay を URL 方式へ切り替える(API レーンの後続)
- [x] P3-5 gateway の `GATEWAY_WEB_URL`(Web レーンの依頼。設定時は `/api`・`/assets`・`/healthz`・`/internal` 以外への GET/HEAD を Web の Service へ転送。k3d の local は `http://web`。ADR-0205)
- [x] P3-6 calc・gateway を pokedex-svc につなぐ(データレーンからの依頼。ADR-0206)。base に `CALC_MASTER_URL=http://pokedex` / `GATEWAY_POKEDEX_URL=http://pokedex`、local overlay の Component から calc へのファイル方式の patch・ConfigMap を削除、`services/gateway/scripts/smoke.sh` が `/api/pokedex/*` から計算に使う ID を実際に引くように変更。k3d(`make api-k3d-deploy && make api-smoke`)で pokedex-svc(投入済み)への接続を確認済み(`master=pokedex species=0003-000 move=highhorsepower nature=bashful` / `pokedex=200`)

### Phase 4 Web
- [x] P4-1 デザイントークン(docs/design.md)を CSS 変数に実装(ADR-0300 §4。web/src/styles/tokens.css)
- [x] P4-2 計算画面(左右カード・持ち物・技・結果の一括表示・攻守入れ替え)。WASM で計算、マスタは架空の例データ(ADR-0300)
- [x] P4-3 プリセット選択(自分側: A特化/A振り/無振り)。定義は web/src/domain/attackerPresets.ts(ADR-0300 §5。engine への移設は DECISIONS.md で提案)
- [x] P4-4 逆算画面(観測ダメージ入力→候補リスト)。与えた/受けたダメージ・観測の追加・SP 範囲と目安の名前(ADR-0300 §7。型名でまとめる表示と絞り込みの演出は持ち越し)
- [!] P4-5 API / WASM 切り替え(WASM ならバックエンド無しで動く)。実装・自動テスト済み(ADR-0301。critic PASS)。ブラウザ実機確認: **Chrome は確認済み**(2026-09-22 ユーザー。`make web-dev` で良好)、**Safari は未確認**
  - WASM 境界との契約差分の解消(ADR-0011 §10 の持ち越し): Web に「ID → 実体(種族・技・持ち物・特性)」の解決層を1つ置き、オンライン(API に ID を送る=`moveId` など)とオフライン(WASM に解決済みの `move` / `Individual` を渡す)で同じ型を共有する。`openapi-typescript` の生成型と ADR-0011 §3 の DTO の対応表を作る。`minPercent`/`maxPercent` の整数化・`category`・`BulkCalcRow.defender`・エラー `code` 語彙の共通化(P3-1 で契約側を直した後)に Web 側を追従させる。WASM の遅延ロード(オンラインは API、オフラインだけ WASM)にするかを決める(ADR-0011 §11)
  - **ブラウザ実機確認**(P1-9 は Node + wasm_exec.js までの確認。仕様ブロッカーではない): Chrome と Safari で、`.wasm` の MIME type / `WebAssembly.instantiateStreaming` / キャッシュ / Service Worker との干渉 / 初回ロード(約4.6MB・gzip 1.3MB)/ メモリ を確認する
- [x] P4-6 Playwright E2E(主要フロー)。`make web-e2e`(オフライン)/ `make web-e2e-online`(calc-svc)。Web のテストを `make test` / `lint` / `build` に組み込み(ADR-0300 §9)
- [x] P4-7 **M1 完了報告**: 動作確認手順を `docs/verify-m1.md` に書く。P2-2c/d・P2-3・P3-3 が main に入り、自動テスト一式・k3d(gateway 経由の `http://localhost:8080`)での計算・逆算・タイプバランス(仮想敵・おすすめタイプ含む)を実地確認して完成版にした。
  実マスタでの計算・Web のオンライン用 MasterSource は M1 の定義(ブラウザで計算できる)の外側の作業として verify-m1.md の「未完了」に記録(gateway の pokedex 配線は API レーンの依頼 d 待ち)

- [x] P4-8 design.md「動き」の演出(操作したときだけ): 確定数が変わった瞬間にバッジが弾む、攻守入れ替えでカードが入れ替わる(0.35秒)、選択中のカード1枚だけのホロ(ポインタ位置に連動)、逆算で観測を追加したときの絞り込みの動き、ダメージバーの spring。OS の「視差効果を減らす」で全演出を無効化(ユーザー決定 2026-09-22)

- [x] P4-9 P4-8 の軽微な改善(ユーザー指示 2026-09-22): ホロの pointermove で画面全体を再レンダーしない(カード内に閉じる・rAF で間引き、cancel と対)、touch ではホロを出さない(pointerType が mouse / pen のときだけ)、確定数バッジの弾みと逆算の絞り込みにも最大待ちのタイマーを付ける(animationend が来なくても外す)

- [x] P4-10 URL で画面を切り替える(`/calc`・`/reverse`。タブと連動し、ブラウザの戻る・進む・直接開くが効く。以後の画面も同じ形)(ユーザー要望 2026-09-22)
- [x] P4-11 Web をコンテナで動かす(ADR-0302。gateway の転送は API レーン待ち、それまでは `make web-k3d-open`): nginx の静的配信イメージ(engine.wasm を含む)、Kustomize(base/web と local の Component)、`make web-k3d-deploy`。
  入口は gateway の後ろ(localhost:8080 だけで画面も API も使える。gateway が /api 以外を Web に転送する変更は API レーンに依頼)。それまでは port-forward で開く(ユーザー決定 2026-09-22)
- [x] P4-12 タイプバランスの画面 `/balance`(balance API をそのまま使う。ADR-0303)(ユーザー要望 2026-09-22)
  - [x] P4-12a メンバー選択(最大6体・特性・技)、防御相性(analyze)、攻撃範囲(coverage)、balance の read model への例データの書き出し(ADR-0303。critic PASS)
  - [x] P4-12b 仮想敵(threats)、おすすめタイプ(recommendations)(ADR-0303 §P4-12b。critic PASS)
- ~~P4-13 素早さ比較の画面~~ → 取り消し(ユーザー決定 2026-09-22: 素早さレーンの SP3 のまま。Web は P4-10 の URL の仕組みで `/speed` を足せる形を用意する)
- [x] P4-14 手順書の書き方の改善: docs/verify-m1.md を上から順に実行するだけで済む形にし、各コマンドの塊は必ずリポジトリのルートへの `cd` から始める(make の実行場所で迷わない)。k3d(コンテナ)で動かす手順を主にする(ユーザー要望 2026-09-22)
- [x] P4-15 `make gen-ts` の再生成: main に別レーンの TB6(技範囲チェッカー)の openapi 追加が入っており、`web/src/api/balance.gen.ts` がまだ反映していなかった(P4-12b の範囲外と確認済み・critic PASS の指摘事項)。move-range の型が追加されただけで typecheck/test/lint は変化なし
- [x] P4-16 Web のオンライン MasterSource の基盤(ADR-0301 §4・ADR-0304 A-1〜A-8。critic PASS。範囲は ADR-0304 A-7 の
  段階分割どおり): `MasterData.capabilities?`(省略時は既存どおり全機能あり)、`createOnlineMasterSource`(持ち物・
  性格を全件取得、種族は `searchSpecies`/`getSpecies` を都度引く検索専用インターフェース、技・持ち物候補比較・特性一覧は
  公開 API に無いデータに依存するため capabilities で明示的に無効を伝える)、`App.tsx`/`main.tsx` の配線。
  実装時に見つけた重大バグ(`baseUrl: "/"`(本番の既定)で `new URL(path, baseUrl)` が Invalid URL 例外を投げ、
  オンラインモードが常に失敗する)を修正し回帰テストを追加(critic 指摘。文字列連結に変更)。画面側は未着手
- [x] P4-16b Web のオンライン MasterSource の画面側(ADR-0304 A-5・A-7・「追記2」A-9〜A-11。critic 2回目 PASS 相当:
  1回目 FAIL の指摘のうち必須分〈重要1・重要4・軽微1・軽微2〉は修正済み、重要2・3・軽微3・4 は下記 P4-16c へ分離
  〈critic 許容範囲〉): 種族の検索コンボボックス(ADR A-4・A-10)、技選択・持ち物候補比較が使えないときの無効化と
  案内表示(`i18n/ja.ts` の `masterOnlineText`)、BalanceScreen は `speciesList`・`moves` が両方そろうまで画面ごと
  無効(案内 + 入力を全部 disabled + balance API を呼ばない。A-9)。検索を画面へ渡す経路は `ScreenProps.masterSearch?`
  (A-10)。P4-16 の積み残し(1)(2)(response.json 側の AbortError 再送出)も本タスクで解消・回帰テスト追加。
  (3)(4)は影響が無い軽微事項のため P4-16c にまとめて送る
- [x] P4-16c P4-16b の critic 指摘で見送った残り(ADR-0304 A-12): 種族の検索候補を↑↓/Enter/Escape で操作できる
  WAI-ARIA "List Autocomplete with Automatic Selection" パターンを実装(`web/src/screens/SpeciesSearchField.tsx`)、
  `SpeciesSearchField.css` を design.md トークンのみで新規作成、`aria-controls`/`aria-activedescendant` は候補
  非表示時に属性ごと外す。加えて(3)`onlineSource.test.ts` の origin 未検査、(4)入力を空に戻した直後の遅延応答、
  (6)`BalanceScreen.online.test.tsx` の A-9 ガードが coverage/threats を検知していなかった点、(7)truncated
  肯定側テストの非対称、をテスト強化で解消。critic 1回目 FAIL(IME変換中のEnter・矢印キーを誤って候補選択に
  使ってしまう退行を発見)→ `isComposing` ガード追加・`preventDefault()` は処理したときだけに修正・回帰テスト
  2件追加(907件)→再確認予定
- [ ] P4-17 技の ID 解決(データ/API レーンへの依頼。DECISIONS.md 2026-09-23 提案・未回答)が入ったら
  `capabilities.moves` を true にして技を復活させる
- [ ] P4-18 Codex コードレビューの issue(Web レーン主担当。タイプバランスレーンから 2026-09-23 に連絡・`gh issue view <番号>`)。
  優先: #99(bug, accessibility)ライトテーマのエラー文字色がコントラスト基準未達(iOS と共有デザイントークン同期が必要)、
  #113(improvement)逆算の数値入力で古い計算要求を抑止・キャンセル(200ms debounce・AbortSignal。iOS・API と連携)。
  次点: #98(bug)モバイル幅で計算・逆算画面が横に溢れる、#67(bug)2xx の契約外 JSON で API クライアントが例外を投げる(防御的処理)。
  連携(他レーン主担当。Web は連携のみ): #71(データ+Web+iOS 攻撃側プリセット単一化)・#72(API+Web ルート make e2e を Playwright へ)・
  #78(API+Web 特性の無効・吸収の境界反映)・#110(Web の担当分は P4-19 へ分離。DECISIONS.md 2026-09-23 参照)・
  #103(主担当 API・データ。M2保存データの保持期間。ユーザー決定 2026-09-23 で needs-decision は解消済み。
  DECISIONS.md参照。Web は連携のみで主担当ではない)
- [x] P4-19 issue #110(セキュリティ。ADR-0300 §10。critic PASS: 境界値の網羅探索〈約1.2万ケース〉と変異テスト5件で
  `itemVariants`/`itemCandidates` が常に64以下・`observations` が17件目を作れないことを確認済み)。
  `domain/requestLimits.ts` に上限3定数(`api/openapi.yaml` の `maxItems` との同期をテストで検査)と
  `limitToMax()`。`defenderItemVariants`・`reverseItemCandidates` が配列を作る最終地点で決定的に絞り込み、
  選んだ持ち物は落とさない。逆算の「観測を追加」は16件で disabled + `role="status"` の理由表示。
  絞り込みが起きたら計算・逆算の両画面に文言を明示(`requestLimitText`)。
  残る軽微(ブロッカーではない。次に触るときに拾う): (1) `addObservation()` 自体のガード(ボタンの disabled とは
  別の多層防御)を直接検証するテストが無い。(2) 観測上限到達時の `role="status"` 要素が条件付きマウントで、
  常時マウント+中身の出し入れの方が読み上げが安定する可能性。(3) `ReverseScreen.tsx` の `move === null` 分岐に
  「先頭は必ず null」の知識の小さな複製がある(実際には使われない経路)。
  issue #110 は engine/WASM・iOS の追従待ちで、Web 単独ではクローズしない(DECISIONS.md 参照)
- [ ] P4-20 issue #148(クラウド公開前のアクセス境界・認証方針。主担当 API・Web・iOS・運用。ユーザー決定
  2026-09-23「私設サービスを維持する」。DECISIONS.md参照): 中心は Ingress・TLS・overlay・gateway の運用レーン
  作業で `web/` 本体への変更は今のところ見込み薄。Web の分担は ADR に記録する「Web の接続方法」の節への記載と、
  端末IDを認証であるかのように誤解させる表示・文言が無いことの確認(現状そのような UI は無いはず)。
  ADR 作成が API/運用レーンで進んでから、依頼が来た時点で着手する(今は着手しない)

## M2: 保存・構築

**P5-1〜P5-4 の前提(先に決めた設計。issue #103・ADR-0209「M2 保存データの保持・削除・端末 ID 境界」に従う)**:
端末 ID は認証ではなくデータの分割キー / 生の計算イベントは作成から90日・構築とお気に入りは `max(devices.last_seen_at, 行.updated_at)` から540日で失効 /
端末単位の全削除はサービスごとに1本(`DELETE /api/record/device-data`・`DELETE /api/team/device-data`。冪等・`partial` の繰り返し)/
削除の墓石(`devices.purged_at`)で JetStream の遅延イベントの復活を防ぐ。受け入れ条件は ADR-0209 の AC-D / AC-P / AC-R / AC-L。

- [ ] P5-1 TiDB(tiup playground で開発、k3d は TiDB Operator 最小構成)。
  スキーマは ADR-0209 §3 に従う(`devices` テーブル〈`last_seen_at`・`purged_at`〉、**purge journal テーブル〈#5b。
  DB 側とは別に DB 外の独立した保存先〈P7-4 が決める〉にも同時に追記する〉**、全表に `device_id`、
  `favorites` は `calc_events` を参照せず個体スナップショットを自分で持つ)。保持日数は環境変数で渡し、起動時に検証する
- [ ] P5-2 NATS JetStream と calc-svc からのイベント発行(失敗しても計算は成功)。
  ストリームの `max_age` は7日、イベントに発生時刻(`occurred_at`)を載せる(ADR-0209 §7・#6)。
  record-svc と team-svc(P5-4)は**別々の durable consumer**を持つ(同じ consumer を共有すると配送が分かれ
  record-svc が計算イベントを取りこぼす。ADR-0209 §4)
- [ ] P5-3 record-svc(保存・よく使う集計: 頻度×時間減衰)。
  ADR-0209 §5.3 の契約を `api/openapi.yaml` に入れて `make gen`(`store_unavailable` の追加を含む)→
  分離(§6)・全削除(§5)・失効ジョブ(§4)・ログ(§3)を実装。時間減衰の半減期は保持期間90日より短くする。
  gateway に `/api/record/*` のルーティングと CORS の `DELETE` 許可を追加(ADR-0209 §10・ADR-0202 への追記)
- [ ] P5-4 team-svc(構築 CRUD、Showdown 形式入出力)。
  ADR-0209 §5.3 の `deleteTeamDeviceData` と §6 の分離規則(他端末のリソース ID は 404 `not_found`)を含む。
  **P5-2 のイベントを購読し、自分の DB の `devices.last_seen_at` だけを更新する**(計算 API だけを使い続ける端末の
  構築が誤って失効しないため。ADR-0209 §4。イベントの中身〈個体・計算結果〉は保存しない)。
  gateway に `/api/team/*` のルーティングと CORS の `DELETE`/`PUT` 許可を追加(ADR-0209 §10・ADR-0202 への追記)
- [ ] P5-5 Web: 履歴・よく計算する相手・構築ビルダー。ADR-0209 §8 の文言と「この端末のデータを削除」の UI を含む
- [x] P5-6 技の追加効果(使用者自身のランク変化。例: ニトロチャージで自分の素早さ+1)を engine の Move・マスタ・importer・export に足す(判定レーンからの提案。DECISIONS.md 2026-09-22。ADR-0005 に沿い、追加効果の対象=self/target・確率・ランク変化量をデータとして持つ。ADR-0107。critic PASS。engine は乱数を持たず「発動した場合の値」だけを返す。ゴールデン不変。公開APIへの露出は判定レーンの要件確定後)

## M3: iOS
- [x] P6-1 Xcode プロジェクト、swift-openapi-generator、デザイントークン(ADR-0500。`make ios-test` = 生成物の一致・XCTest・XCUITest・Info.plist の接続先。critic PASS)
- [x] P6-2 計算画面・逆算・構築(構築は端末内に保存、Showdown 形式は後回し。2026-09-21 ユーザー回答)。P6-2a 計算画面・契約追従・P6-2b 逆算画面・P6-2c 構築(一覧・編集画面・ニックネーム・XCUITest)・P6-2d(構築から呼び出す配線)は完了(critic PASS)
- [x] P6-2d 構築から個体を呼び出す配線(`TeamMemberConverter.makeIndividual` を `CalcViewModel` / `ReverseViewModel` の
  「構築から呼び出す」ボタンとして実際につなぐ。requirements.md「自分側のプリセット」。ADR-0501「P6-2c」4章で範囲外と
  明記し、ここに積んだ。`BuildSource<Preset>` で自分側の出どころを直和にし、計算画面・逆算画面の両側に配線。
  `swift test`(275件)・`make ios-test`(unit 284件・XCUITest 12件)成功。実装時に見つけたバグと回避は
  ADR-0501「P6-2d」9〜11章に記録。critic PASS。PR #119 で main に統合済み)
- [x] P6-3 シミュレータテスト(`make ios-test`。gen-check・XCTest・XCUITest・Info.plist の検査を1コマンドで実行し、
  P6-1〜P6-2d の各タスクで継続して緑を確認済み。iPhone 18 Pro シミュレータ)
- [x] P6-4 Tailscale serve の手順書 `docs/runbooks/ios-device-install.md` を作成 → **人間が実機インストール**(署名・
  Tailscale ログイン・実機への配線・外出先での確認は手順書どおり人間が行う。AI が代行しない)
- [ ] P6-5 ADR-0209 §8 の文言と「この端末のデータを削除」の UI(issue #103。record-svc / team-svc の全削除 API 実装後)

## TB: タイプバランスチェッカー(タイプバランスレーン。設計は docs/type-balance-design.md)
- [x] TB0 基盤(型・相性コア・HTTP・Docker/Kustomize・Argo CD・単体テスト)。Argo CD の実同期もローカル k3d で確認済み(ADR-0018: Git 変更 32fbb9e → manual sync → Pod の image digest 一致)
- [x] TB1 防御タイプバランス(最大6体 × 18タイプの防御倍率、攻撃タイプごとのチーム集計。総合点は作らない。ADR-0014)
- [x] TB1b 相性表を P1-13 のデータ(`testdata/golden/typechart.json`)から読み、TemporaryTypeChart を削除(ユーザー決定。ADR-0015)
- [x] TB2 攻撃範囲(ADR-0016)
- [x] TB3 特性(正規化された効果データ経由。タイプ由来/特性由来の区別。ADR-0017)
- [x] TB4 仮想敵診断(ADR-0400)
- [x] TB5 おすすめタイプと該当ポケモン(2026-09-22 ユーザー要望。ADR-0401): チームの穴(TB1 で弱点持ちが多く耐性・無効が少ない攻撃タイプ、TB2 で有効打が無い防御タイプ)をふさげるタイプの候補を出し、
  そのタイプを持つ**使用可能なポケモン全員**(レギュレーション依存。日本語名付き)を一覧にする。特性で穴をふさげるポケモンは別枠。他のサイトを見に行かずに候補が分かることが目的。詳細は着手時に ADR
- [x] TB 整備(2026-09-22): HTTP の 500 テスト、typed nil の provider の正規化、read model の JSON Schema(ADR-0402)、HTTP 層の検証の共通化、おすすめの穴を既存の集計から導出
- [x] TB 実データの配線(2026-09-22。データレーンの依頼): pokedex export の read model を ConfigMap で k3d の balance に読ませる(ADR-0403)、abilityIds の上限を 4 に
- [x] TB6 技範囲チェッカー(2026-09-22 ユーザー要望。ADR-0404): 技 ID(最大4つ)から18タイプの一貫判定を出し、その技構成を半減以下で受けられる実在ポケモンを図鑑から具体名で列挙する。特性で半減以下になるポケモンは別枠
- [x] Codexレビュー issue #105 対応(2026-09-23。ADR-0405): Argo CD 導入物(install.yaml・同梱3イメージ)をコミットSHA・SHA-256・digestで固定する `scripts/argocd-bootstrap.sh` を新設し、balance/speed 両runbookの重複した生URL直apply手順を1本化。自動テスト `scripts/argocd-bootstrap_test.sh`(`make test-scripts`)。

### ブロッカー(タイプバランスレーン)
(なし。Argo CD の実同期は 2026-09-22 に解消)

## SP: 素早さ比較(素早さレーン。設計は docs/speed-design.md。2026-09-22 ユーザー要望)
ユーザーの仕様(確定):
- 3つ目の機能・サービスとして独立して作る(`services/speed/`。ダメージ計算・タイプバランスと並ぶ)。Web に独立した画面(タブ)を置く。iOS は後で
- 画面は左右に配置する。**左 = 全体の表**(最初から速い順に並べた表)、**右 = 自分のポケモン**。自分の実数値が左の表のどこに入るかを視覚的に示す
- 左の表は、使用可能な各ポケモンについて **6 行**: 無振り / 準速(素早さ SP 32・補正なし)/ 最速(SP 32・素早さ上昇性格)/ 最速+こだわりスカーフ / 最速+1(ニトロチャージ等)/ 最速+2(こうそくいどう等)。道具・ランクで絞り込める
- 右の入力は**最小の選択で計算できる**こと: ポケモンを選び、「無振り / 準速 / 最速」を選んで、こだわりスカーフの on/off を切り替えるだけ。**オプション**で好きな数値でも算出できる
- 実数値は Champions の式(その他 = floor((種族値 + 20 + SP) × 性格補正))、Lv50・個体値31固定。スカーフ・ランクの掛け方は engine / Showdown の規則に従う
- 2026-09-22 ユーザー回答(確定): 右のオプションは素早さ SP 0〜32・性格の補正3通り・ランク -6〜+6・スカーフ on/off を自由に選ぶか、実数値を直接入力して位置だけを見る。同じ実数値は同速としてまとめて表示(同速の中は図鑑番号順)。表に載せるのは既定のレギュレーションの使用可能集合。Web の骨組み(`web/`)が無い間は `web/src/speed/` の画面部品とテストだけ先に作り、タブ登録は骨組みができてから1項目足す
- [x] SP0 基盤(ADR-0600。critic PASS。GitOps の overlay と Argo CD Application は digest が決まる SP4 へ): docs/speed-design.md と ADR-0600、services/speed(純粋な Go のコア・HTTP API・`services/speed/api/openapi.yaml`・Kustomize・Argo CD の定義はタイプバランスに倣う)、架空データの read model
- [x] SP1 素早さの表(ADR-0601。critic PASS。6行の生成・速い順の並び・同速の扱い・絞り込みの API)
- [x] SP2 自分のポケモンの位置(ADR-0602。critic PASS。preset/custom/raw の3モード・faster/slower/tie・POST /api/speed/v1/position)
- [x] SP3 Web の素早さ画面(ADR-0604。critic PASS。左右の配置・自分の位置の強調・表の絞り込み。`web/src/speed/`)
- [x] SP4 pokedex の read model(データレーン P2-3)への切り替えと k3d の疎通(ADR-0603。critic PASS。2026-09-24 ユーザーが実データで確認:
  `make pokedex-export`(348 pokemon)→ `make speed-k3d-deploy-readmodel` → `make speed-smoke-readmodel` が
  `speed readmodel smoke: pokemon=0003-000 list=200 table=200` で成功)
- [x] SP5 GitOps(ADR-0605。critic PASS。digest 固定の overlay・Argo CD Application・balance-registry と Argo CD を共有。
  `speed-gitops-template-check` まで実行して確認済み。クラスタへの実際の適用〈speed-argocd-app・registry-push・sync〉は
  人間の確認のもとで別途。手順は docs/runbooks/speed.md の節5〜10)

## JD: 判定(判定レーン。設計は docs/judge-design.md。2026-09-22 ユーザー要望)
「ニトチャ+メイン技で素早さ抜ける+そのポケモンを倒せるか」を1回の入力で確認する。engine を直接呼び、pokedex-svc と calc-svc の公開 API だけに依存する(speed-svc には依存しない)。
- [x] JD0 基盤(ディレクトリ構成・pokedex-svc/calc-svc への HTTP クライアント・ヘルスチェック)。受け入れ条件と契約は ADR-0700(docs/judge-design.md §4 の未決事項はここで全部決めた)。critic PASS(3回目。1・2回目 NG は上流エラー文面への URL/host:port/ホスト名の漏洩を修正)
- [x] JD1 抜けるか+倒せるかの最小構成(自分と相手の Individual・使う技 → outspeeds・speedTie・ko)。endpoint(POST /api/judge/v1/outspeed-and-ko)を services/judge/api/openapi.yaml に追加(ADR-0701)。critic PASS(2回目。1回目 NG は上流エラーのログ未記録・pokedex 400の扱いがADR未記載・defender側スカーフ/種族差の未検証を修正)
- [x] JD2〜JD5 の範囲・順序をユーザーに確認(2026-09-22)。4項目すべて対象(複数の相手候補・相手の技を含めた返り討ち・場の効果・Web/iOS 画面)、
  技の追加効果の自動反映は対象外のまま。順序は docs/judge-design.md §3(JD2 場の効果 → JD3 複数の相手候補 → JD4 返り討ち判定 → JD5 画面)
- [x] JD2 場の効果(トリックルーム・追い風)。judge だけが解釈する `speedField` を outspeed-and-ko に追加(ADR-0702)。
  丸め方(4096基準で連結してから1回だけ五捨五超入)は @smogon/calc 0.12.0 の実装を読んで確認・独立検算した。critic PASS(1回目)
- [x] JD3 複数の相手候補を一度に判定(攻撃側1つ・相手候補の配列 → 候補ごとの判定結果の配列。ADR-0703)。
  request の defender(単数)を defenders(1〜6件)に、response を matchups(配列)に破壊的変更(クライアント未着手のため安全)。critic PASS(1回目)
- [!] JD4 相手の技を含めた返り討ち判定。技の優先度を pokedex-svc から引く endpoint(`GET /api/pokedex/moves/{key}`)が無いため、
  API レーンへ依頼済み(DECISIONS.md 2026-09-22 に既定案)。**ブロック中**: ユーザーが「API レーンの実装を待つ」を選択(2026-09-23)。
  API レーンが endpoint を実装したら着手する
- [ ] JD5 Web/iOS の画面(judge-svc を呼ぶ。担当は着手時に判断)

## DOC: 文書(全レーン。docs/coding-rules.md §8。2026-09-22 ユーザー要望)
各レーンが自分の範囲の README(何をするか・mermaid の構成図・ディレクトリ・コマンド・関連 ADR。80 行以内)と、動かして確かめられるレーンは手順書(`docs/runbooks/<レーン>.md`。AGENTS.md「手順書の書き方」に従う)を書く。全体図は `docs/architecture.md`。
- [x] DOC-data: `engine/README.md`・`services/pokedex/README.md`・`tools/importer/README.md`・`tools/golden/README.md`、手順書 `docs/runbooks/data.md`(migrate・import・dry-run の確認)
- [x] DOC-api: `services/calc/README.md`・`services/gateway/README.md` を §8 の形に、手順書 `docs/runbooks/api.md`(k3d での疎通)。critic PASS
- [x] DOC-web: `web/README.md`、手順書(`docs/verify-m1.md` の画面の部分と重複させない。M1 の完了報告は verify-m1.md にまとめる)
- [x] DOC-tb: `services/balance/README.md` を §8 の形に、手順書 `docs/runbooks/balance.md`
- [x] DOC-speed: `services/speed/README.md` を §8 の形に、手順書 `docs/runbooks/speed.md`(k3d での疎通を確認済み)
- [x] DOC-ios: `ios/README.md`(coding-rules §8 の形)、手順書 `docs/runbooks/ios.md`(シミュレータでの確認。実機インストールは P6-4)。受け入れ条件・判断は ADR-0501 へ移動
- [ ] DOC-arch: `docs/architecture.md` を各レーンの変化に合わせて保つ

## M4: 運用
- [ ] P7-1 kube-prometheus-stack / Loki、各サービスのメトリクス
- [ ] P7-2 SLO(計算API p99 < 100ms、可用性)とダッシュボード
- [ ] P7-3 ArgoCD(GitOps)
- [ ] P7-4 MySQL/TiDB バックアップと復元テスト(ADR-0209 §9 を要件に含める: バックアップに `devices`〈墓石〉を含める /
  purge journal(#5b。世代取得後の削除要求。保持90日)をバックアップ世代と別に保持し復元時に再適用 /
  Ready の前に墓石の再適用・purge journal の再適用・失効ジョブの強制実行 / JetStream は再生しない / 世代30日。
  受け入れ条件は AC-B1〜B3・AC-B2b)

## ブロッカー
(ここに止まった理由と試したことを書く)

**判定レーン(2026-09-23)**: JD4(相手の技を含めた返り討ち判定)は、技の優先度(priority)を pokedex-svc から個別取得する
endpoint(`GET /api/pokedex/moves/{key}`)が無いと実装できない。API レーンへ依頼済み(DECISIONS.md 2026-09-22 に既定案付き)だが
未着手。ユーザーに「両者優先度0の限定で先に進める」か「API レーンの実装を待つ」か確認し、**待つ**を選択した(2026-09-23)。
判定レーンは API レーンが endpoint を実装するまで新規実装を止める(`feat/judge-jd4` は作成済み・空。作業ディレクトリ
~/MyDamageCalcurater-judge はこの間、他の判定レーンのタスクが無ければアイドル)。

**【人間の確認待ち】(Web レーン、2026-09-22 深夜に記載)**
- **P4-5 のブラウザ実機確認**(仕様ブロッカーではない。作業は止めない。**Chrome は 2026-09-22 に確認済み**、残りは Safari): `make web-dev` で開き、Chrome と Safari で計算・逆算が動くこと、
  `.wasm` の MIME type(`application/wasm`)・`WebAssembly.instantiateStreaming`(失敗時は arrayBuffer にフォールバックする実装)・
  キャッシュ・初回ロード(約4.6MB / gzip 1.3MB)・メモリを確認する。既定案: 確認できるまで P4-5 は「実装・自動テスト済み、実機未確認」として扱う。
- **PR #22(Web P4-1〜P4-4)のマージ**: 深夜のため作成のみ(作業はブランチで続けられるので止まらない)。朝に確認してマージする。

**解決済み(2026-09-21 のユーザー決定。詳細は DECISIONS.md / ADR-0002 / requirements.md)**
- `hb_boost` / `hd_boost` の定義 → `boost` = H32 + B(D)0 + 上昇性格、`full` = H32 + B(D)32 + 上昇性格、`hb`/`hd` = 性格補正なし。P1-10 で反映
- マスタデータの取得元 → 責務分離(calc=oracle 0.12.0 固定 / Showdown=照合 / 公式情報=レギュレーション基準 / PokeAPI+override=日本語名)、v1 は M-C のみ(差し替え可能な構造)、実マスタ・スナップショットは Git に置かない
- requirements.md との食い違い → こだわり系・とつげきチョッキ・しんかのきせきは M-C 向け候補から除外(requirements.md 修正済み)。ヌケニンは v1 で考慮不要
- 計算結果の表示% → 小数第1位。P1-11 で反映
- 逆算の Recall の新定義(返した SP 範囲が総当たりの正解と完全一致。基準 80%/95% は据え置き)→ 承認(2026-09-21)。P1-12 で反映
- 見た目違いフォーム → 性能が同じなら1件、性能が違えば別登録。マスタ更新 → CronJob で定期取込(既定は週1回・版に変化が無ければ取り込まない)。技の使用可否 → 既定案で進めて後で裁定(2026-09-21)。P2-1c の調査で12件すべて結論が出て、未確認の技は無かった(ADR-0002 追記 P2-1c)
- P2-2b の3点 → 効果定義 `data/importer/effects.json` はコミットする / 日本語名は ja(漢字混じり)→ ja-Hrkt の順 / レギュレーションの日本語ラベルはコミットする(2026-09-21 ユーザー回答。ADR-0101)
- P2-2c → 習得技は進化前から継がない(Champions のルール。Showdown に合わせる。当初の「世代で絞る」案 b は実データで不十分と判明し改めた)/ P2-1c の裁定の件数・集合が実データと食い違ったら取り込みを止める / ADR 番号のレーンごとの帯を承認(2026-09-22 ユーザー回答)
- P2-2d → 新しい版は成功のまま知らせるだけ(版を上げるのは人が PR)/ 実行は毎週土曜 12:00(日本時間)(2026-09-22 ユーザー回答。ADR-0104)
- 運用 → レーン制(どちらの AI もどちらのレーンを進めてよい)、main へは PR で統合(2026-09-21。COORDINATION.md)
- ブラウザでの WASM 実動作 → 仕様ブロッカーではない。P4-5 の確認項目

**解決済み(追加。2026-09-21 のユーザー回答。audit-r1.md のユーザー判断6件を含む)**
- `testdata/golden` の扱い → 数値と英語識別子だけなのでコミットを続ける(ADR-0002 §追加の回答。実マスタの代替を入れない)
- 技の使用可否の食い違い → 公式以外の攻略サイト(GameWith・ポケモン徹底攻略など)も Web から調べてよい。P2-1c で調査
- メガ石 ↔ メガフォーム → メガ後の姿を別ポケモンとして登録し、専用のメガストーンを持ち物に固定(変更不可)。P2-2 のスキーマに `is_mega` / `base_species_key` / `required_item_id`
- 公開に向けた判断(audit-r1.md): LICENSE は現時点で置かない(法的に面倒なものは公開しない)/ module path のアカウント名はプレースホルダに(R-2-8)/ 履歴は書き換える(R-2-9)/ ADR-0002 の抜粋は削除済み/ タイプ相性表はデータ化(ADR-0013、P1-13)/ engine の日本語ラベルは持ってよい(ADR-0009 §1-a)

**【人間の確認待ち】残り**
- **観測%の丸め方**(逆算の入力側。ADR-0010 §R2): 実機の相手 HP の減少は**整数%**で表示される(2026-09-21 ユーザー確認)。**丸め方(切り捨てか四捨五入か)だけが未確認**。確認できるまでは、切り捨て・四捨五入・切り上げのどれでも真値を落とさない区間で照合する(P1-12 で実装済み。作業は止まらない)。確認できたら `Observation.Matches` の1か所で区間を狭める(逆算の精度が上がる)。確認方法の例: HP が分かっている自分のポケモンで、実点数のダメージと画面の%を見比べる
- **公開のタイミング**(R-2-9・LICENSE): 公開するときに、クリーンコピーの作成と、第三者データを含まない状態の確認、LICENSE の決定を行う。それまでは今のリポジトリで開発を続ける(Codex の並行作業に影響しない)

## 改善要望(/improve で追加)
(ここに要望と対応状況を書く)
- [x] issue #110(セキュリティ。Codex レビュー)の API レーン担当分: `POST /api/calc/bulk`・`/api/calc/reverse` の候補・観測配列に件数上限が無く、1MiB未満の小さな本文で計算量を増幅できた(2,000×2,000 で約9.4秒)。契約(`maxItems`/`uniqueItems`/`maximum`。ADR-0208)を追加し、calc-svc の生成ラッパは検証しないため(実測確認済み)自前検証をID解決・engine呼び出しより前に実装。critic PASS、実HTTPで境界値と再現手順の解消(0.9ms・engine未到達)を確認。engine/wasmapi(データレーン)・Web・iOSへの追従は DECISIONS.md に既定案付きで依頼(issue はレーンの完了までクローズしない)
- [x] issue #110 のデータレーン担当分: `engine.CalcBulk`/`CalcReverse` と `engine/wasmapi` に ADR-0208 §1 と同じ件数・範囲の上限(presets 8・itemVariants 64・itemCandidates 64・observations 16・maxCandidates 0..128)を追加(ADR-0108)。HTTP を経由しない直接呼び出し・WASM でも計算量を増幅できないようにした。wasmapi は DTO 変換より前に同じ検査を重ねて置き、複数の違反が重なっても HTTP と同じ `invalid_input` が先に出るようにした(parity)。`MaxCandidates` の負の値は、従来「無制限」扱いだったのを ADR-0208 の契約(`minimum: 0`)に合わせて拒否するよう変更(既存テストの期待値を更新。理由は ADR-0108 決定4)。critic PASS(1往復)。Web・iOS の追従(観測16件でUI無効化・持ち物候補64件超の扱い)は ADR-0208 §4 のまま未着手
- MySQL の manifest に MYSQL_DATABASE が無く、初回起動時に pokedex DB が自動作成されない実バグを発見(データレーンが k3d に初めて実デプロイした際に発生)。deploy/k8s/overlays/local/mysql/statefulset.yaml に MYSQL_DATABASE: pokedex を追加し、layout_test.go に検知テストを追加して修正(2026-09-22)。**新規クラスタでは直るが、この修正前にすでに初期化済みの PVC は MYSQL_DATABASE の効果を受けない**(コンテナ起動時にしか実行されない仕様のため)。既存の PVC に対しては CREATE DATABASE を手動実行するしかない。docs/runbooks/data.md に一言注記するとよい
- P2-3 の critic の軽微(2026-09-22。未反映の4件): `check-publishable.sh` の `B_KEYVALUE_ALLOW` を self-test の基準リポジトリにも播く / `maxCatalogAbilityCount` が balance の schema・loader と三重管理(テストで検出はできる) / natures-mismatch のエラー案内が Showdown 側だけを見て `make import-fetch` の案内が出ないことがある / `TestPublicInputValidation` の 400 応答を契約検証(kin-openapi)に通す

- P2-2d の critic の軽微(2026-09-22): `cronjob_layout_test.go` の「消さない」検査を secret・statefulset・configmap にも広げる / `make lint` が kubectl に依存する(kubectl の無い環境では失敗する)/ upstream の `checkedAt` が未来でも fresh 扱い / **コンテナの中で取得スクリプト(Showdown の build 等)を実際に流した記録が無い。初回の `make import-k8s` で確かめる**

- `services/pokedex/db/mysql_test.go` に「species_abilities.slot = 4 が入る」ことを確かめるケースを足す(P2-2c の critic の軽微。000005 は使い捨てコンテナで手動確認済み)

- [x] `scripts/check-publishable.sh --self-test` の既存の失敗2件を MT-2 で修正し、`make lint` に自己テストを追加(2026-09-22)

P1-6 独立レビューで出た軽微・任意の指摘(コードは未変更。次の engine タスクに合わせて対応を検討):
- `engine/golden_test.go`: `speciesCount` の下限アサート追加(現在は 0 だけ検査。少数種で再生成しても通ってしまう)
- `engine/golden_test.go`: `DamageInput` の json タグ明示または `DisallowUnknownFields`(フィールド改名でフィクスチャ値が黙ってゼロ値になる)
- `Makefile`: `go vet -tags golden` を lint に追加 / `golden-generate` は説明どおり `npm ci` を実行するか未導入で明示的に失敗させる
- `tools/golden/package.json`: `^0.10.0` を `0.10.0` に完全固定
- `engine/damage.go` `chainMods`: @smogon/calc はクランプ(41/410〜131072/2097152)を持つ。現在の補正集合では到達しないが、補正追加時に再確認
- ゴールデン未カバー: リフレクターとオーロラベールの同時成立、`Effectiveness` / `STAB` の直接照合(L1 では確認済み)
