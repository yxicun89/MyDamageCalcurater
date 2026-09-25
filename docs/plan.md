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
- [x] P3-7 `GET /api/pokedex/moves/{key}`(`getMove`)を追加(判定レーンの JD4 の依頼。2026-09-22・2026-09-23の DECISIONS.md。ADR-0105 §3 追記): 技1件を ID で引く。`getSpecies` と同様に使用可能集合で絞らない。マスタ未投入(0件)は`GetDefaultRegulation`を経由しないため 503 ではなく 404 `not_found`(searchMoves 等の一覧系と異なる。契約に明記)。`services/pokedex/`(データレーンの範囲)への実装まで API レーンが一括して行った理由: `api.ServerInterface` にメソッドが増えるため、スタブだけ置いて main に入れると「200 を約束する契約なのに実装が無い」状態になり、完全な実装より悪いと判断(既存の GetSpecies/GetItem パターンの写し。新規の設計判断はしていない)。データレーンへ触ったファイルの一覧を添えて再レビューを依頼(DECISIONS.md)。critic PASS(3往復)。**main 統合済み(PR #161)**

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
- [x] P4-17 技の ID 解決(データ/API レーンへの依頼。DECISIONS.md 2026-09-23 提案)— **完了。critic PASS**。
  2026-09-24 に API レーンが `GET /api/pokedex/moves/batch`(`getMovesByIds`)で回答・実装済み
  (ADR-0105 §3・ADR-0304 §3・DECISIONS.md 2026-09-24 参照)。Web 側は ADR-0304 **A-13** の設計どおり実装:
  **`capabilities.moves` は true にしない**(true にすると BalanceScreen が「有効なのに技が選べない」壊れた
  状態になるため)。技は `resolveSpecies` が種族・特性と一緒に解決して返し(`MasterSpeciesResolution.moves`)、
  技セレクトの disabled は「いま技の候補があるか」で決める。`learnset` は64件ずつに分割して `getMovesByIds`
  を並列に複数回呼ぶ(API レーンが実データで確認: 349種族中151種族・43%が64件超、最大106件。稀な例外では
  なく主経路として実装・テスト)。
  critic レビュー: チャンク分割・結合順序・1回でも失敗したら全体失敗、を変異テストで確認(全滅)。
  重要指摘1件(攻守入れ替え・与えた/受けた切り替え後の**選択中の技**〈候補一覧だけでなく実際にリクエストに
  乗る技〉がテストされていなかった。`<select>` の DOM 値は state が壊れていても先頭候補にフォールバック
  表示するため見逃しやすい経路)を受け、実際のリクエストを検査する回帰テストを2件追加(CalcScreen の
  攻守入れ替え・ReverseScreen の与えた/受けた切り替え)し、指摘された3つの変異すべてで実際に落ちることを
  確認。既存1192件は無変更・新規28件追加(1220件。うち2件は critic 指摘を受けて既存アサーションに追記して強化)。
  BalanceScreen の種族検索・技選択は P4-17b として積み残し(下記)
- [x] P4-17b BalanceScreen(タイプバランス)の種族検索・技選択をオンラインでも使えるようにする
  (ADR-0304 A-9 の申し送り・A-13.5 の積み残し)。パーティ・仮想敵の各枠(6枠 × 2)に A-10 の種族検索を広げ、
  枠ごとに解決した learnset から技を4つまで選べるようにする。spec-writer が ADR-0304 追記5(A-14)に
  設計を記録し、失敗するテスト13件を追加: 可否の判定を `speciesList && moves` から「入力の口があるか」
  (`(speciesList || masterSearch) && (moves || (!speciesList && masterSearch))`)に置き換え、12枠で
  `useSpeciesResolutions` を共有し、`moveById` の「実体不明の技 ID を攻撃技と誤判定する」不具合を直す。
  implementer が `web/src/screens/BalanceScreen.tsx` を A-14 のとおり実装(ゲート条件・
  `useSpeciesResolutions()` の12枠共有・`MemberFields` のドロップダウン/検索欄の出し分け・
  `moveById`/`hasDamagingMove` の fail-closed 化・結果表の名前解決)。critic PASS(mutation testing で
  ゲート条件・fail-closed 判定・`registerSpeciesResolution` 呼び出し漏れ等の主要な変異を全て検知することを確認)。
  critic 指摘の軽微3件はその場で直接修正: (1) A-14.1 の表7行のうち未カバーだった2行
  (`NO_SPECIES_LIST`+検索口あり・容量そろい+検索口ありの2組み合わせ)のテストを追加、
  (2) `useMemberListActions.resolveSpecies` を index ではなく `member.id` で引くように変更
  (検索の解決を待つ間に他の枠が削除されると index が別の枠を指しうる競合を根治)、
  (3) `MemberFields` 内の特性名解決の重複ロジックを `findAbilityName` 呼び出しに統一。
  `cd web && npx vitest run` は1243/1243 green(新規15件)、`npx tsc --noEmit`・`npx eslint` ともにエラー無し。
- [x] P4-18(Web 分。Codex コードレビューの issue。タイプバランスレーンから 2026-09-23 に連絡・
  `gh issue view <番号>`)。**#99(bug, accessibility)ライトテーマのエラー文字色がコントラスト基準未達 —
  Web 分・iOS 分とも完了、issue クローズ済み**: danger のライト値を `#E5484D`→`#CD1D23`(WCAG 2.2 SC 1.4.3
  の4.5:1を bg.base・bg.glass 合成後の両方で満たす。色相・彩度は変えず明度だけ下げた)。
  `web/src/test/colorContrast.ts`・`web/src/styles/contrast.test.ts` を新規追加。critic PASS(独立実装での
  検算・変異テストで確認)。iOS 側も `PokeCalcDesign.swift`・`ColorContrast.swift`・`DangerContrastTests.swift`
  で対応済み(commit `23e5c5e`)。
  **#113(improvement)逆算の数値入力で古い計算要求を抑止・キャンセル — Web 分・iOS 分とも完了(critic PASS。
  ADR-0300 §11)**: 観測のテキスト編集のみ200msのtrailing debounce(確定操作は待たない)、`CalcEngine` に
  任意引数 `signal?: AbortSignal` を追加(API実装はfetchへ配線、WASM実装は開始前にのみ検査)、取り消しを
  `REQUEST_ABORTED_CODE` で `engine_unavailable` と区別。既存929件は無変更・新規26件追加(953件)。
  iOS 側も `LatestTaskRunner.swift`・`CalcInput.swift` で対応済み(commit `492b6d3`)。**issue 自体はまだ
  open**(API レーンの「クライアントのcancel伝播」連携分が残っているか未確認)。
  残る軽微(次に触るときに拾う。ブロッカーではない): 種族・技・プリセット選択でも確定操作として即座に
  flush することの回帰テストが無い(実装は正しいがテスト未カバー。`ReverseScreen.debounce.test.tsx`)。
  連携(他レーン主担当。Web は連携のみ): #71(データ+Web+iOS 攻撃側プリセット単一化)・#72(API+Web ルート make e2e を Playwright へ)・
  #78(API+Web 特性の無効・吸収の境界反映)・#110(Web の担当分は P4-19 へ分離。DECISIONS.md 2026-09-23 参照)・
  #103(主担当 API・データ。M2保存データの保持期間。ユーザー決定 2026-09-23 で needs-decision は解消済み。
  DECISIONS.md参照。Web は連携のみで主担当ではない)
- [x] P4-21 Codex コードレビューの次点 issue(P4-18 で優先分が完了したため分離)。
  **#67(bug)2xx の契約外 JSON で API クライアントが例外を投げる — 完了(critic PASS)**: `apiEngine.ts`・
  `balanceClient.ts` の `postJson` を、写像関数/画面が実際に読むフィールドを検査する型ガード経由にし
  (`response.value as Schemas[...]` の型アサーションを除去)、契約外の2xxは例外を投げず `engine_unavailable`/
  `balance_unavailable` を返すようにした(ADR-0301 §4・ADR-0303 §6 に追記)。calc側は写像関数が読む全
  フィールドを再帰的に検査、balance側は画面がたどる形(オブジェクト性・必須配列フィールド)だけを検査
  (leafスカラー・enum は見ない。契約の二重管理を避けるため。ADR-0303 の「応答をそのまま表示に使う」設計に
  合わせた)。critic レビュー: mutation テスト12件で型ガードの過不足なし・reject しないこと・abort との
  混同なしを確認。軽微指摘1件(`ko.chancePercent` の null 許容が ADR 文言と食い違っていた)を修正済み。
  既存1034件は無変更・新規143件追加(1166件)。
  **#98(bug)モバイル幅で計算・逆算画面が横に溢れる — 完了(critic PASS)**: ブレークポイント600px(design.md
  「幅への対応」に根拠付きで記録)。600px未満はmobile-firstで縦積み(計算: 攻撃側→攻守入れ替え→防御側、
  逆算: 自分側→相手側。DOM順・フォーカス順は変えない)、600px以上は従来の左右配置。伸縮列を`minmax(0, 1fr)`
  にし、カード・selectに`min-width: 0`/`width: 100%`を追加して内容の固有幅で列が広がらないようにした。
  critic が実ブラウザ(vite build + preview)で13幅×5画面を実測し横溢れゼロ・599/600pxで正確に切り替わる
  ことを確認、縦中央揃えも壊れていないことを確認。既存1166件は無変更・新規34件追加(1200件。単体18+E2E16)。
  軽微な積み残し(次に触るときに拾う。ブロッカーではない): 600〜699px帯のE2Eが無い(単体テストでは
  カバー済み)、結果一覧の行(`*-results__row`)は今回対象外(実マスタの長い名前が入る段で見直す余地あり)。
  作業中に発見した無関係の既存退行(JD5の判定タブ追加で `a11y.spec.ts` のタブ回り込みが壊れていた)は
  別途修正・main統合済み(PR #191)。
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
- [ ] P4-20 issue #148(クラウド公開前のアクセス境界・認証方針。ADR-0210。API レーン担当分は完了・main 統合済み
  〈PR #157〉。DECISIONS.md 2026-09-23 参照)。API レーンからの具体的な依頼2件(ADR-0210 §4・§7):
  (1) API の base URL を tailnet の MagicDNS 名にし、public な既定値を持たないこと (2) CORS 許可オリジンも
  tailnet 上の名前だけにすること。
  現状確認済み: `web/src/api/config.ts` の `apiBaseUrl()` の既定値は同一オリジン `"/"`(public な固定値ではない。
  `VITE_API_BASE_URL` 環境変数で上書きする設計。ADR-0301 §4)なのでコード自体は既に条件を満たしている。
  CORS の許可オリジン一覧は gateway(Go・API レーンの持ち物)側の設定で、Web 側にハードコードは無い(確認済み)。
  残るのは実際の tailnet MagicDNS 名を `VITE_API_BASE_URL` にデプロイ時設定するという**運用/設定の話**で、
  運用レーンが到達経路(Tailscale Operator の ingressClass か subnet router + tailscale serve か)を選び、
  実際の名前が決まってから。今はコード変更不要。着手のタイミングは運用レーンの選定後
- [x] P4-22 issue #72(ルートの `make e2e` が未実装スタブのまま「テスト0件で成功」する)。**完了(2026-09-25。Web レーン。API+Web 共同担当)**: ADR-0306 に従い `scripts/e2e.sh` を実装(常時3件 `web-e2e`/`web-e2e-online`/`web-e2e-balance` を必ず実行し、kubectl のコンテキストが `k3d-$CLUSTER` のときだけ `api-smoke`/`web-k3d-smoke`/`web-k3d-e2e` を追加実行。`E2E_REQUIRE_K3D=1` あり)。ルート Makefile の `e2e` を `CLUSTER=$(CLUSTER) ./scripts/e2e.sh` に配線、`docs/test-strategy.md`・`README.md` を実装内容に合わせて更新。`bash scripts/e2e_test.sh` は80/80 passed
- [x] `make e2e` の `web-e2e-online` 修復・PR1(2026-09-25。Web レーン。issue 無し。ブランチ
  `fix/web-online-e2e-master-export`)。**完了・critic PASS。ただし `make e2e`(`web-e2e-online`)は
  まだ緑にならない(PR2 で対応。下記)**。
  P4-22 で常時実行になった3件のうち `web-e2e-online` が main で壊れていた(calc-svc が起動できない)。原因は
  ADR-0204(相性表は MasterExport 本体に含める・`CALC_TYPECHART_PATH` は廃止)に Web 側の書き出しが
  追従していなかったこと: (1) `web/playwright.online.config.ts` が廃止済みの `CALC_TYPECHART_PATH` を渡す
  → calc-svc が起動を拒否する (2) `web/src/master/exportSnapshot.ts` の `toCalcSnapshot()` の出力が
  ADR-0204 以前の暫定スキーマのままで、`MasterExport`(`dataVersion`/`types`/`typeChart`、`MasterSpecies` の
  `showdownId`・`type1`/`type2`・`abilities[{slot, abilityId}]`、`MasterMove.effect`、効果のキーの
  PascalCase)を満たさない (3) 例データの技・持ち物・特性の ID がハイフンを含み、共通マスタの
  `codeIDPattern`(`^[a-z0-9]+$`)で弾かれる。calc-svc の契約を正として Web 側を写像する形で修正
  (契約は緩めていない。`services/`・`api/openapi.yaml`・`testdata/golden/typechart.json` は無変更)。
  `node scripts/export-example-master.mjs` の出力を実際に `go run ./calc/cmd/calc` に読ませ、
  `/healthz` 200・`/api/calc` が契約レベルのバリデーションまで到達することを確認済み(=マスタのロード自体は成功)。
  critic PASS(mutation testing 4件、うち2件〈defAbsorbTypesの入れ子変換・相性表の欠けた組の既定値〉は
  未到達分岐だったため専用のテストを追加してから確認)。
  **PR2 に持ち越し(最優先。#308 以降はPR2が緑になるまで着手しない。オーケストレーター決定 2026-09-25)**:
  `npm run e2e:online` はまだ2件ともタイムアウトする。原因は今回の修正とは別の、P4-16 以降の既存の設計不整合
  (オンラインモードの `main.tsx` は pokedex-svc の公開API に依存するが、`web-e2e-online` は calc-svc しか
  起動しない。calc-svc は pokedex ルートを意図的に404で返す設計。`registerPokedexNotFoundRoutes`、
  critic指摘R1で確定済み)。方針は e2e専用の軽量 pokedex フィクスチャサーバーを Web 側に新設し、
  `API_PROXY_TARGET` を pokedex パスと calc パスで振り分ける(calc-svc/pokedex-svc 本体には触れない。
  詳細は ADR-0301 §5 追記)。
  **申し送り(J1。critic指摘、ブロッカーではない)**: `web/src/master/exportSnapshot.ts` の
  `CalcSnapshot`/`CalcSnapshotSpecies`等は生成済みの契約型(`web/src/api/openapi.gen.ts` の
  `components["schemas"]["MasterExport"]`)の手書きの写し。`exportSnapshot.contract.test.ts` が
  毎回 `api/openapi.yaml` を読んで突き合わせるのでズレは検知できるが、`type CalcSnapshot =
  Schemas["MasterExport"]` に寄せるか型レベルの一致アサーションを足すと、より一枚岩になる
  (次に触るときの検討事項)。
- [x] issue #71 の Web 側(攻撃側プリセットの単一化。ADR-0114)。**完了(2026-09-25。Web レーン)**: データレーン
  が `engine/presets/attacker.json`(embed)を唯一の正にした(PR #346)のを受け、
  `web/src/domain/attackerPresets.contract.test.ts` を新規追加。ハードコードした期待値と比較する既存の
  `attackerPresets.test.ts`(Web の今の挙動を固定)とは別に、こちらは毎回 `engine/presets/attacker.json` を
  読み、カタログの順序・既定値・`relevantStat`/`boostMinus`/`relevantSp`/`nature` から導いた期待値と
  `resolveAttackerPreset` の実際の出力を突き合わせる契約テスト(現状の値は一致済み)。JSON の値を書き換える
  mutation で実際に検知することを確認(3件 fail)、確認後に復元。新規17件追加(1262件)。`web/src/domain/
  attackerPresets.ts` 自体は変更していない(engine への実装移管は別タスク)。
- [x] issue #333(375px幅でタブの名前が1文字ずつ縦に折り返す)。**完了(2026-09-25。Web レーン。critic 1回目
  FAIL→修正→PR #356 で検証済み)**: `App.css` の `.app-tabs__list` に `overflow-x: auto`、`.app-tabs__tab` に
  `white-space: nowrap`・`flex-shrink: 0` を追加(既定案どおり)。**critic 1回目FAIL**: `justify-content: center`
  のままだとはみ出した先頭タブ(「計算」)がscrollLeft=0でも画面外に残ったまま戻れない
  (centered flexbox overflow clipping。320pxで実測再現)。`justify-content: safe center` に修正
  (docs/design.md「幅への対応」に記録。`safe`キーワードのSafari対応はP4-5のSafari確認〈人間の作業〉と
  合わせて確認)。回帰テスト(`web/e2e/mobile.spec.ts`、320・375px)を2件に強化: (1)各タブのテキストノードを
  `Range.getClientRects()`で数えて**ちょうど1行**(`toBe(1)`。0〈ラベル消失〉も1未満として弾く)、
  (2)タブ列を左端・右端までスクロールし、先頭・末尾のタブが表示領域に収まることを確認(centered flexbox
  overflow clippingの回帰ガード)。CSSを戻すと両方とも実際に検知することを確認(確認後に復元)。
  `web/e2e/a11y.spec.ts`(タブのキーボード操作)・vitest 1262件・lintは無回帰(22/22 green)。
- [x] issue #306(計算画面のタイプ名のコントラストとダメージバーの読み上げ名。Web レーン)。**完了・critic PASS
  (2026-09-25)**: ブランチ `fix/web-issue-306-type-badge-contrast`。
  タイプ名は文字色にタイプ色を使うのをやめてバッジ化し(背景 = タイプ色、文字 = `--type-<id>-ink`)、
  ダメージバーは %幅 を同じ行に併記済みなので装飾(`aria-hidden`、`role="meter"` を外す)にする方針を
  `docs/design.md`(「タイプバッジ」の新設・「画面: ダメージ計算」への追記)に記録。
  失敗するテストを先に追加(`web/src/styles/typeBadgeContrast.test.ts` 新規、`tokens.test.ts`・
  `CalcScreen.test.tsx`・`web/e2e/calc.spec.ts` を更新)。
  実装: `web/src/styles/tokens.css` に18タイプ分の `--type-<id>-ink`(design.md 転記、ダークで上書きしない)を追加、
  `web/src/screens/CalcScreen.tsx` のタイプ名 `<li>` を背景 `var(--type-<id>, var(--border-hairline))`・
  文字色 `var(--type-<id>-ink, var(--text-primary))` のバッジに変更(未知タイプIDは既定値にフォールバック)、
  `CalcScreen.css` に `.calc-card__type` のピル装飾を追加、ダメージバーの `div` から
  `role="meter"`/`aria-value*` を外し `aria-hidden="true"` + `data-testid="damage-bar"`(fill 側は
  `data-testid="damage-bar-fill"`)に変更。critic指摘の軽微1件(ダメージバーの技タイプ色`var(--type-${moveType})`
  にフォールバックが無かった。今回の差分ではなく既存コードだが、ついでに`var(--type-${moveType},
  var(--text-secondary))`へ修正)は直接修正。`cd web && npx vitest run` 1341 passed(0 failed)、
  `npx tsc --noEmit` エラー無し、`npm run lint` エラー無し。テストファイル・design.md は変更していない。
  **iOSレーンへの申し送り(critic指摘)**: design.mdの「タイプバッジ」節はiOSも同名トークン(`typeInk(_:)`)
  を持つ前提で書かれているが、2026-09-25時点でiOS側にタイプ色自体がまだ実装されていない
  (`ios/`にタイプ色の16進値0件)。iOS側でタイプ名を表示する画面ができたときに、design.mdの18タイプ分の
  文字色の表を移植する必要がある
- [x] issue #275(重大度 high。逆算の「受けたダメージ」で自分の耐久が無振り固定・画面にも出ない)。
  **完了・critic PASS(2026-09-25。Web レーン。ブランチ `fix/web-issue-275-reverse-defender-preset`)**:
  自分側カードに防御側プリセット(ADR-0009 §1 のカタログ8件)を出し、既定は `none`(無振り)のまま = 既定時の
  リクエストは今までと同じ(回帰無し)。選択肢は技の分類で絞り(物理 = B 系、特殊 = D 系、変化技 = none/hp)、
  分類が変わったら対のプリセットへ読み替える(`defenderPresetForCategory`。state は元のキーを保持したまま
  表示だけ読み替えるので、物理→変化→物理と戻すと元のプリセットが復活する)。新しいテスト:
  `web/src/domain/defenderPresets.test.ts`(挙動の固定)・`defenderPresets.contract.test.ts`
  (`engine/bulk.go` の `DefenderPresetCatalog()` と `api/openapi.yaml` の enum を読む契約テスト)・
  `ReverseScreen.test.tsx`(UI とリクエストの回帰)。engine への一本化
  (`engine/presets/defender.json`)は DECISIONS.md 2026-09-25 でデータレーンへ申し送り済み、
  `defenderPresetForCategory`(engine に無いWeb限定の読み替え規則)は ADR-0009 §1 に追記して根拠を残した。
  critic PASS(mutation testing 2/3 kill。`flushObservationDebounce()` を守るテストが無いのは
  `selectAttackerPreset` 側にも元々あった既存の穴で、今回の退行ではない。次に触るときに攻撃側・防御側
  両方へテストを足すとよい)。`cd web && npx vitest run` 1419/1419 green、tsc・lintエラー無し。
- [x] issue #304(重大度 medium。計算・逆算・タイプバランスの入力に見えるラベルが無く、初見でどれが
  攻撃側・防御側・技・観測値か分からない。WCAG 2.2 SC 3.3.2 / SC 2.5.3)。**完了・critic PASS
  (2026-09-25。Web レーン。ブランチ `fix/web-issue-304-visible-labels`)**: 受け入れ条件と失敗する
  テストを先に用意(`web/src/screens/visibleLabels.test.tsx`・`web/src/i18n/visibleLabels.test.ts`・
  `web/src/styles/inputTokens.test.ts`、共通の道具 `web/src/test/accessibleName.ts`)。
  設計は docs/design.md「入力のラベル」に記録済み(領域の見える見出し・h2/h3 の階層・`label for` の
  見えるラベル・accessible name は「<見出しの語>の<ラベルの語>」を文言資源で組み立てる・未選択 option の
  文言・観測欄の説明文・入力/ピルの角丸トークン)。実装完了(1534/1534 green)。
  **critic 1回目FAIL→修正**: (1) `web/e2e/mobile.spec.ts` の見出しレベル検査(h2→h3。calc.spec.ts と
  同じ形)が未追従だったので修正。あわせて同ファイルの `focusedCalcZone` ヘルパーが `[aria-label="攻撃側"]`
  という属性セレクタで区画を判定していたが、計算画面のカードは `aria-labelledby`(見える h2 を参照)に
  変わったため引けなくなっていたのを修正(`aria-labelledby` の参照先の文字も見るように)。
  (2) 逆算の観測欄で、見えるラベルと説明文(hint)がどちらも grid の全幅行を要求するため、`grid` の
  自動配置で入力欄と単位ラジオ・削除ボタンが別々の行に分かれてしまっていた(jsdom では検出できない
  レイアウト崩れ。実ブラウザで確認)。hint の DOM 位置を単位ラジオの後ろへ移し(`aria-describedby` は
  DOM順に依存しないので支援技術への影響なし)、`.reverse-observation__label` に `grid-column: 1 / -1`
  を追加して解消。`e2e/reverse.spec.ts` の検証メッセージの accessible description の期待値も、
  hint が `aria-describedby` に加わった分を反映して更新(検証メッセージ自体の確認は維持。弱めていない)。
  (3) オンラインの種族検索欄(`SpeciesSearchField.tsx`)がplaceholderだけで見えるラベルが無く、
  issue #304 の症状が本番のオンラインモードに残っていたのを、短い見えるラベル(「ポケモン」)を追加して解消。
  `npx playwright test`(オフライン、35/35)・vitest(1534/1534)は無回帰。
  **申し送り(critic指摘の重要事項)**: `web/src/styles/inputTokens.test.ts` は「design.mdの値から
  `:root` で最初にその値を持つ変数名を逆引きする」設計で、`--radius-input`(12px)と
  `--font-size-caption`・`--space-3`(いずれも12px)が値を共有するため、`:root` の宣言順に依存する
  脆さを持つ(今回は宣言順の入れ替え〈値は不変〉で対応。critic が宣言順を戻すと実際に失敗することを
  確認済み)。次にトークンを足すときに同じ値の衝突が再発しうるので、触るときは
  `inputTokens.test.ts` の逆引きを変数名ベースに直すことを検討する。

## M2: 保存・構築

**P5-1〜P5-4 の前提(先に決めた設計。issue #103・ADR-0209「M2 保存データの保持・削除・端末 ID 境界」に従う)**:
端末 ID は認証ではなくデータの分割キー / 生の計算イベントは作成から90日・構築とお気に入りは `max(devices.last_seen_at, 行.updated_at)` から540日で失効 /
端末単位の全削除はサービスごとに1本(`DELETE /api/record/device-data`・`DELETE /api/team/device-data`。冪等・`partial` の繰り返し)/
削除の墓石(`devices.purged_at`)で JetStream の遅延イベントの復活を防ぐ。受け入れ条件は ADR-0209 の AC-D / AC-P / AC-R / AC-L。

- [ ] P5-1 TiDB(tiup playground で開発、k3d は TiDB Operator 最小構成。バージョン・リソース上限・DB/ユーザーの
  分け方・migration ツールの流用は ADR-0211 で確定)。**このタスクの範囲は `devices`〈`last_seen_at`・`purged_at`・
  `orphaned_since`〉と purge journal の2表とプロビジョニングまで**(`favorites`・`teams`・`team_members`・
  `calc_events` 等の業務テーブルは P5-3/P5-4 で追加する。ADR-0211「背景」で当初案から縮小)。
  purge journal〈#5b。DB 側とは別に DB 外の独立した保存先〈P7-4 が決める〉にも同時に追記する〉は、
  その独立保存先が無い P5-3〜P7-4 の間は未充足のままになる既知のギャップ(ADR-0209 追記・ADR-0211 §6 参照)。
  保持日数・墓石猶予の環境変数名と既定値は ADR-0211 §7 で確定し、起動時検証コード自体は P5-3/P5-4 で書く。
  **実装状況(2026-09-25)**: `services/internal/dbmigrate` の切り出し・`services/record`・`services/team`
  のスキーマ/migrate CLI(PR #205)、TiDB Operator の k8s マニフェスト(TidbCluster・TidbInitializer)・
  `up.sh` 配線・回帰テストまで実装済み(critic 2ラウンドで裏取り。TiDB Operator v1.6.6 の実ソースまで
  確認して `passwordSecret` のキー名・初期化用イメージ・namespace・資格情報境界の誤りを修正済み)。
  **残作業**: 共有 k3d クラスタへの実適用(AC-T3・AC-T8)は未実施(他レーンが使う共有クラスタへの影響を
  先に確認する必要があるため、このセッションでは意図的に見送った。次に `make up` を実行するときに
  TidbCluster・TidbInitializer が実際に Ready/Completed になることを確認する)。TidbInitializer の
  `tnir/mysqlclient` イメージは amd64 専用(上流がそれしか提供していない)で、Apple Silicon の k3d
  ノードでの起動可否(QEMU エミュレーション経由)も未確認
- [ ] P5-2 NATS JetStream と calc-svc からのイベント発行(失敗しても計算は成功)。
  ストリームの `max_age` は7日、イベントに発生時刻(`occurred_at`)を載せる(ADR-0209 §7・#6)。
  record-svc と team-svc(P5-4)は**別々の durable consumer**を持つ(同じ consumer を共有すると配送が分かれ
  record-svc が計算イベントを取りこぼす。ADR-0209 §4)。
  バージョン固定・ローカル/k3d 導入・ストリーム設定(Retention は Limits。ADR-0209 §3 #6 の
  文言からの意図的な逸脱で理由は ADR-0212 §4)・イベントのワイヤフォーマット(`services/internal/calcevents`。
  `calc` は個体・技・状況・ダメージ幅まで、`calcBulk`/`calcReverse` は envelope のみ)は ADR-0212 で確定
  (critic 3ラウンド)。実装はこれから
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
- [x] P6-5 issue #113(Web/iOS/API共同主担当)の iOS 側: `ReverseViewModel`/`CalcViewModel` が最新の入力 Task を1つ
  (`LatestTaskRunner`)保持し、新入力時・画面破棄時(`.onDisappear` → `cancelPendingWork()`)に先行 Task を cancel
  する。`ReverseScreenObservations` の観測文字入力に 200ms の trailing debounce(`CalcInput.debounceInterval`)を
  適用(同期的な TextField 表示・入力検証は即時のまま)。方針は ADR-0501「issue #113」に確定(受け入れ条件
  A1〜A7・追加する API・View の置き換え先)。`CancellationError` は画面 error にしない catch を、`reverse`/
  `calcBulk` を包む catch だけでなく `species(key:)`(learnset の読み直し)を包む catch も含めて全経路に適用
  (1周目の critic 指摘で漏れを修正。各 ViewModel の private `handleInputFailure(_:)` に集約)。
  `swift test` 323件・`make ios-test`(unit 332件・XCUITest 16件・Info.plist 検査)成功。判断: debounce は逆算の観測欄
  だけ(計算画面は Task 管理のみ。自由文字入力が無いため)、`MasterSearchField`(issue #68)は今回統合しない
  (理由は ADR 7章)。1周目の critic FAIL(A5 未達)は修正済み・2周目 critic PASS。main の `getMove` 追加に追従し
  iOS 生成物も再生成済み。PR #166 で main に統合済み)
- [x] P6-6 issue #110(API レーン主担当。PR #130 で契約に上限追加済み: presets 8+unique・itemVariants/
  itemCandidates 64+unique・observations 16・maxCandidates 上限128)の iOS 側追従。API レーンから 2026-09-23 に
  依頼: 観測追加UIを16件で無効化(理由表示)、持ち物候補が64件を超える場合の扱いを決める(Web レーンの対応
  〈黙って切り捨てず明示的なエラーか決定的な絞り込み〉に揃える)。DECISIONS.md 2026-09-23「calc の候補・観測件数
  に上限を置く」参照。方針は ADR-0501「issue #110 の受け入れ条件(iOS 側)」に確定(受け入れ条件 A1〜A8):
  件数の定数は `PokeCalcCore` の `RequestLimits`(+ 理由文言 `RequestLimitLabels`)に1か所で持つ。観測は 16 行で
  追加ボタンを無効化(`observationsReachedLimit`)。持ち物候補は Web の「送信直前に切り捨て」ではなく
  **上限到達中の ON トグルを拒否**する(選んだのに送られない状態を作らないため。ADR 5章)。送る配列の先頭の
  null(持ち物なし)も1件と数えるので選べるのは 63 件。計算画面の `itemVariants`(比較トグル)も同じ規則
  (iOS は `itemVariants` を使っている)。`presets`/`maxCandidates` は iOS から上限を踏めないので対応不要。
  implementer 実装完了(`RequestLimits`/`RequestLimitLabels` 追加、`ReverseViewModel`/`CalcViewModel` の
  `…ReachedLimit` + guard、View 側の disabled とヒント文言・`ChipButton.isEnabled`)。critic 指摘で、
  上限到達中の ON 拒否 guard が `beginInput()` より前にあることを固定する回帰テストを
  `CalcViewModelLimitTests`/`ReverseViewModelLimitTests` にそれぞれ1本追加(guard を後ろに動かすミューテーションで
  実際に red になることを確認済み)。`swift test` 340件・`make ios-test`(unit 340件・XCUITest 16件・
  Info.plist 検査)成功。critic PASS。引き継ぎ時の検証(2026-09-24)で、iOS の写し `RequestLimits` と契約の
  ずれを検出する検査が無い(Web の `requestLimits.test.ts` は Web の定数しか見ない)ことに気づき、
  `ios/scripts/check-request-limits.sh`(`make ios-check-request-limits`。`make ios-test` に組み込み)を追加。
  観測上限の XCUITest `testAddObservationButtonIsDisabledWithReasonAtTheLimit`(ボタン無効+理由表示。
  ライト/ダークのスクリーンショットで崩れ・文字切れなしを目視確認)も追加。`make ios-test`(unit 353件〈xcresult 集計。XCTest 実行数は340件〉・XCUITest
  17件・Info.plist 検査)成功
- [x] P6-9 issue #68 の残り(iOS): 選択中の技が検索結果に一度も現れていないとき、`getMove`(PR #161)で名前を解決する。
  計算・逆算は起動時・種族変更・構築からの呼び出しで「選ぶべき技」だけを解決(learnset 全件は解決しない。1操作
  最大 `MasterSearch.maxMoveLookupsPerSelection` = 4 回)、構築は保存済みの技の名前を `load()` で解決。失敗は今日の
  振る舞い(`moveUnavailable` / ID のまま)。持ち物は一覧のまま、上限到達で `itemOptionsReachedLimit` と案内。
  方針は ADR-0501「issue #68 の残り」。implementer 実装完了 → critic 1回目 FAIL(`ReverseViewModel.recalculateIfPossible`
  の古いエラーの消し方が `selectedMove != nil` だけでは不十分で、直前の種族の解決済みの技が辞書に残っているせいで
  `moveUnavailable` を誤って消す回帰があった。ADR-0501「issue #68 の残り」11章)。判定を「`selectedMove` + いまの
  攻撃側の learnset の ID 集合 + ダメージ技」の3条件に直し、回帰・変更理由・stage-2 解決ループの世代保護の回帰テストを
  4本追加(いずれも該当箇所を戻すと実際に red になることを確認済み)。`swift test` 383件・`make ios-test`
  (unit 396件・XCUITest 17件・Info.plist 検査)すべて成功。**issue #68 は iOS 側を閉じてよい**(ADR 7章)
- [x] P6-10 `TeamEditViewModel.load()` の技解決を `getMovesByIds`(main に追加済み。web は P4-17 で採用済み)の
  一括呼び出しに切り替えた(旧実装は `move(id:)` をメンバーごとに個別呼び出し)。`PokeCalcService.moves(ids:)` を
  追加(`APIPokeCalcService`: `RequestLimits.maxMoveBatchIds` = 64 件ずつに分割・重複除去・空配列で無呼び出し・
  404 は無い写像、`MockPokeCalcService`: 未知 ID を省く)、`load()` は全メンバーの `species(key:)` 後に未知の技を
  集めて1回で解決する2段構成に書き換えた(`withTaskGroup` は不要になった)。Calc/Reverse の `move(id:)` は
  変えていない(stage-2 の短絡評価の意味が変わるため。理由は ADR 4章)。方針・受け入れ条件は
  ADR-0501「getMovesByIds による構築編集の技の一括解決」。既存の `TeamEditViewModelMoveLookupTests`(7件)は
  1行も編集せず、`StubPokeCalcService` 側の拡張(`moveLookups` の共有記録・`moveBatchModeOverride` が未設定なら
  `moveLookupMode` に従う近似。ADR 3章・3.1章)だけで green を保った。実装完了後の critic 1回目は FAIL
  だったが、指摘はテストの網羅不足のみ(実装は問題なしと判定): 分割の境界テストを `RequestLimits.maxMoveBatchIds`
  基準の表駆動に直し(ミューテーションテストで実際に red になることを確認)、`load()` の Task を保留中の一括解決
  ごと cancel するテストを追加、`species(key:)` が途中で失敗したときの挙動差分を ADR に明記、古くなったテストの
  ヘッダーコメントを修正(ADR 8章)。`swift test`(399件・0失敗)・`make ios-test`
  (`ios-test-unit` 412件・`ios-test-ui` 17件・Info.plist 検査、すべて成功。終了コード0)ともに green
- [ ] P6-7 ADR-0209 §8 の文言と「この端末のデータを削除」の UI(issue #103。record-svc / team-svc の全削除 API 実装後)
- [x] P6-11 issue #334: 攻撃側プリセットの表示名(`AttackerPreset.label`)が技の分類(物理/特殊)に追従せず、
  特殊技でも「A特化」のままになる不具合を直した。表記は Web(`attackerPresetText`)の出荷済み文言に揃えた
  (「A振り(無補正)」等)。受け入れ条件・判断・実装結果は ADR-0501「P6-11」。
  `AttackerPreset.label(for:)`(`relevantStat(for:)` の atk/spa から文字 A/C を引く。文字・接尾辞は1箇所にまとめた)を実装し、
  `CalcScreenView.presetSegmentedRow`・`ReverseScreenView.presetSegmentedRow`(`case .defender:`)を
  `label(for: category)` に切り替えた。既存の分類なし `label` は削除していない(旧テスト維持)。
  `swift test`(PokeCalcKit)400件0失敗、`make ios-test`
  (`ios-test-unit` 413件・`ios-test-ui` 18件(新規 `testSelectingSpecialMoveShowsCLetterPresetLabel` を含む)、
  すべて成功。終了コード0)ともに green
- [x] P6-12 issue #71 の iOS 側(ADR-0114「Web・iOS への依頼」): `AttackerPreset` を `engine/presets/attacker.json` に揃える。
  並び順を 無振り → 特化 → 振り に、既定を無振り(JSON の `default`)にし、JSON のキーとの対応を `catalogKey` の1か所に書く。
  JSON を直接読む契約テスト(`AttackerPresetCatalogContractTests`。シミュレータでも読めることを実測済み)を追加。
  受け入れ条件・判断・変えた既存テストの期待値・実装結果は ADR-0501「P6-12」。
  `swift test` 406件0失敗、`make ios-test`(`ios-test-unit` 419件・`ios-test-ui` 20件)すべて成功
- [x] P6-13 issue #274(iOS 側): 計算画面に「詳細」(既定は閉じる)を足し、急所・攻撃側のやけど・天候・フィールド・防御側の壁・
  攻撃側のランク(技の分類の関連ステータス)・攻撃側の特性を選べるようにした。既定は今の要求と同じ。防御側のランク・特性・状態異常は
  bulk の契約に無いので対象外(API レーンへ提案。DECISIONS.md 2026-09-25「計算条件の入力 UI」)。受け入れ条件・判断・文言・実装結果は
  ADR-0501「issue #274」(9章)。テスト先行(`CalcViewModelConditionsTests`・`CalcConditionsDomainTests`・`APIPokeCalcServiceConditionsTests`・
  `MockPokeCalcServiceConditionsTests`・XCUITest `CalcConditionsUITests`)で実装し、critic PASS。実装中に accessibilityIdentifier が
  コンテナに飲まれる不具合と、XCUITest の前方スクロールがランクの +/− ボタンに届かない不具合を見つけて直した(ADR 9章に詳細)。
  ランクの +/− ボタンに VoiceOver ラベル、「詳細」の各行に Dynamic Type(アクセシビリティ文字サイズで2行に切り替え)を追加。
  `swift test` 436件0失敗、`make ios-test`(`ios-test-unit` 449件・`ios-test-ui` 22件)すべて成功(並行セッションが同じ
  シミュレータを使っていた回はブートストラップ失敗になったが、P6-12 と同じ既知の環境要因と確認済み)
- [ ] P6-14 最大の文字サイズ(accessibility-extra-extra-extra-large)で計算画面の全体が横にはみ出し、左端が切れる(既存の不具合。
  P6-13 の確認中に発見。2026-09-23 のスクリーンショットでも同じ)。原因の特定と修正、XCUITest か撮影での確認
- [x] P6-8 issue #99(ライトテーマの danger コントラスト不足)の iOS 側。Web レーンから 2026-09-24 に依頼された
  内容どおり `ColorToken.danger` のライト値を `0xE5,0x48,0x4D` → `0xCD,0x1D,0x23` に更新し、
  `DesignTokenTests.swift` の旧値も書き換えた。`ios/PokeCalcKit/Tests/PokeCalcDesignTests/ColorContrast.swift`
  (WCAG相対輝度・コントラスト比。`web/src/test/colorContrast.ts` と同じ算出式の独立実装)・
  `DangerContrastTests.swift`(ライト・ダーク × bg.base・bg.glass合成の4組。Web の `contrast.test.ts` と同じ
  組み合わせ)を新規追加。旧値に戻すと `3.59:1`/`3.82:1` で実際に red になることを確認済み(有効なテスト)。
  `swift test`(PokeCalcDesignTests 13件・PokeCalcCoreTests 323件)・`make ios-test`(unit 336件・XCUITest 16件・
  Info.plist 検査)・`make lint`(check-publishable 含む)すべて成功。軽微な作業のため spec-writer/critic の
  サブエージェントは使わずメインで実施(CLAUDE.md「軽微な作業はメインのみでよい」)

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
- [x] JD4 相手の技を含めた返り討ち判定(正は ADR-0704)。
  技の優先度を引く endpoint(`GET /api/pokedex/moves/{key}`)は API レーンが実装し main 統合済み(2026-09-23。P3-7・PR #161)。
  決定: `defenders` の要素を `DefenderCandidate`(`Individual` + 必須 `moveId`)へ・`ko` を `attackerKo` に改名して
  `defenderKo`・`attackerMovePriority`/`defenderMovePriority`/`attackerMovesFirst`/`turnOrderTie` を追加(破壊的変更。
  クライアント未着手のため安全)/ 先制判定は優先度優先(トリックルームは優先度に影響しない)で `internal/judge` に
  `CompareTurnOrder` を新設 / 逆方向の calc では `field` の screens を入れ替える / 検査順に attacker と候補の技の解決を挿入し
  `unknown_move`(422)を追加(攻撃側の未知の技も JD4 からは 422)。critic PASS(1回目)
  - [x] 軽微な積み残し(critic 指摘)を解消(2026-09-25): `attacker`(単数の `Individual`)の欄名にも
    `defenders` の候補と同じ `individualWireKeys` allow-list による厳密な大文字小文字検査を適用
    (`parseIndividualWire` を新設。`candidateWireKeys` は `individualWireKeys` + `moveId` から導出する形に統一)。
    `TestOutspeedAndKoRejectsUnknownAttackerField` を追加、fix 前に戻して失敗することを確認済み(mutation test)
- [x] JD5 Web の画面(judge-svc を呼ぶ。ADR-0705。critic PASS〈2回目。1回目 NG は古い応答〈A8〉テストが
  実際にはレースを検証していなかった点を、送信ボタンの disabled が反映される前に2回叩いて実際に2本
  同時に送る形へ修正〉)
  - 担当は**判定レーン自体**(2026-09-24 ユーザー決定。DECISIONS.md)。素早さレーンが `web/src/speed/` を自分で作った前例に倣い、
    持ち物は `web/src/judge/`(`judgeClient.ts`・`judge.gen.ts`・`JudgeScreen.tsx`)だけ。共有ファイルへの追記は
    `app/routes.ts` 1件・`app/screens.tsx`(import と `ScreenProps.judgeClient`)・`i18n/ja.ts` の文言・`App.tsx` の client 受け渡しに限る
  - 設計の正は ADR-0705: 送信ボタンでだけ判定を呼ぶ(1回で上流 3+4N 回のため打鍵ごとに呼ばない)/ 技は **ID の自由入力**
    (ADR-0304 §3: ID から技を引く公開 API が無い)/ 相手側の追い風は**全候補共通の1つ**(ADR-0703 §5)/
    `field`(天候・地形・壁)は JD5 の対象外 / 画面も「勝ち・負け」に丸めない(ADR-0700 §6-1・ADR-0704 §3)
  - 失敗するテストを先に置いた(spec-writer): `web/src/judge/judgeClient.test.ts`・`web/src/judge/JudgeScreen.test.tsx`・
    `web/src/app/routes.test.ts`(judge タブの登録)。`judge.gen.ts` は生成済み
  - iOS は Web を出してから改めて判断する(DECISIONS.md 2026-09-24)
- [x] issue #234 moveId/natureId の形式検証が無く、制御文字などが上流 URL にそのまま埋め込まれ、503
  `upstream_unavailable` + 誤警告ログになる(全体レビュー指摘。2026-09-24)
  - 設計の正は ADR-0706: `moveId` / `natureId` を `^[a-z0-9]+(-[a-z0-9]+)*$`・1〜64 文字で検査し
    (`services/balance` の `MoveId` / `AbilityId` と同じ綴り。ADR-0016 §2・ADR-0017 §2)、
    **上流を呼ぶ前に** 400 `invalid_request` を返す(`writeUpstreamError` に到達させない)/
    message は `attacker` / `defenders[<index>]` を示す(ADR-0703 §3)/ `internal/client` は
    key を `url.PathEscape` で埋める(二重の守り)/ pokedex-svc 側の key 検証と `abilityId` / `itemId` は範囲外
  - 契約は先に更新した(spec-writer): `services/judge/api/openapi.yaml` に `MoveId` / `NatureId` を新設し、
    `moveId` / `natureId` の 4 か所をその `$ref` に。**`make judge-gen` は implementer が実行する**
  - 失敗するテストを先に置いた(spec-writer): `internal/httpapi` に `TestOutspeedAndKoRejectsInvalidIDFormat`・
    `TestOutspeedAndKoIDLengthLimit`・`TestOutspeedAndKoIDFormatCheckOrder`・`TestOutspeedAndKoAcceptsValidIDFormat`、
    `TestOutspeedAndKoRejectsInvalidRequest` に形式の行を 4 件追加、`internal/client` に
    `TestPokedexEscapesKeyInPath`・`TestPokedexDoesNotEscapeValidKey`。
    現行コードでは 5 つの Test が失敗し、それ以外の既存テストは全件通ることを確認済み
  - 実装(implementer、1 回目): `internal/httpapi/outspeed.go` の attacker 本体・defenders 候補それぞれで
    `moveId`・`natureId` の計 3 箇所に ID 形式検査を追加(`writeUpstreamError` より前・attacker → defenders を
    index 昇順)、`internal/client/pokedex.go` の `Species` / `Move` は key を `url.PathEscape` で path 要素に
    埋めるよう変更(二重の守り)。`make judge-test` 全件通過
  - critic 1 回目 NG: Go 側のロジック・テストは適合だが、`web/src/judge/judge.gen.ts` が契約変更
    (`MoveId` / `NatureId` の追加)後に再生成されておらず stale(絶対ルール1「API 変更は openapi.yaml から。
    変更後は make gen」違反)。2 回目(implementer)で `web/` にて手動再生成
    (`npx openapi-typescript ../services/judge/api/openapi.yaml -o src/judge/judge.gen.ts &&
    npx prettier --write src/judge/judge.gen.ts`。ADR-0604 §2 の素早さレーンの前例に倣う)、
    差分が契約変更相当の型・doc コメントのみであることを確認、`npm run lint`・`npm test`(1262 件)が通ることを確認。
    ADR-0706 の受け入れ条件7に `judge.gen.ts` の手動再生成を明記して再発防止
- [x] issue #213(重大度 high)上流(pokedex-svc/calc-svc)が遅いと judge は1リクエスト全体の期限を持たず、
  逐次呼び出し(最大27回・1回3秒)を律儀に最後まで続け、クライアントは HTTP 000(空応答)を受け取る
  (JSON の 503 が返らない。全体レビュー指摘。2026-09-24)
  - 設計の正は ADR-0707: `JUDGE_REQUEST_TIMEOUT`(既定 12 秒)を新設し、`writeTimeout`(15 秒)未満であることを
    起動時に検証する。ハンドラ(`outspeedAndKo`)の先頭で `ctx` を 1 回だけ `context.WithTimeout` でラップし、
    以降のすべての上流呼び出しに使い回す(呼び出し順序・逐次であることは変えない。ADR-0703 §3 の維持)。
    `internal/client` は変更不要(既に `http.NewRequestWithContext` を使っており、`net/http` の context 統合が
    「進行中の呼び出しを打ち切る」「未着手の呼び出しは即座に失敗する」の両方を自動で満たす)
  - 失敗するテストを先に置いた(spec-writer): `services/judge/internal/httpapi/outspeed_deadline_test.go`
    (`TestOutspeedAndKoOverallDeadline`・`TestOutspeedAndKoWithinDeadlineUnaffected`)、
    `outspeed_test.go` に `upstreams.delay`・`sleepOrCancel` を追加、`cmd/api/config_test.go` に
    `TestRequestTimeoutFromEnv`。実装前は `go vet` が `deps.RequestTimeout undefined` /
    `undefined: requestTimeoutFromEnv` の 2 件で失敗する状態だった
  - 実装(implementer): `cmd/api/config.go` に `requestTimeoutEnv`・`defaultRequestTimeout`(12秒)・
    `requestTimeoutFromEnv(lookup, writeTimeout)`(`upstreamTimeoutFromEnv` と同じ形 + `writeTimeout` 以上は
    起動失敗)を追加。`cmd/api/main.go` で呼び出し、`httpapi.Dependencies.RequestTimeout` に渡す(パース失敗は
    他の設定エラーと同じく `os.Exit(1)`)。`internal/httpapi/server.go` の `Dependencies` に
    `RequestTimeout time.Duration` を追加(既定 0 は「期限なし」で既存テストに影響しない)。
    `internal/httpapi/outspeed.go` の `outspeedAndKo` で `ctx := c.Request().Context()` の直後に
    `deps.RequestTimeout > 0` のときだけ `context.WithTimeout` でラップ。`README.md` に
    `JUDGE_REQUEST_TIMEOUT` の行を追加。`go vet`・`go test ./...`(新規テスト含め全件)・`gofmt -l`・
    `make judge-lint`・`make judge-build`・`bash scripts/check-publishable.sh` すべて成功を確認
    (`TestOutspeedAndKoOverallDeadline` は `-count=5` でも安定して ~0.22s で 503 を返すことを確認済み)
- [x] issue #329(重大度 low)SP 合計超過(67)の拒否を確かめる回帰テストが無く、`validateSP` の
      `> engine.MaxSPTotal` を `> engine.MaxSPTotal+1` に変える退行を検出できない(既存の唯一のケースが
      合計96で境界〈67〉から遠い。全体レビュー第3回指摘。2026-09-25)。テストのみの変更(実装は無変更):
      `internal/judge/speed_test.go` に合計ちょうど66(受け付ける)・ちょうど67(拒否する。各欄は
      MaxSPPerStat=32以下のまま)を追加、`internal/httpapi/outspeed_test.go` にも合計67の境界値ケースを
      追加。mutation test で検証: `validateSP` を `> engine.MaxSPTotal+1` に一時的に変えて新規テスト
      (`TestSpeedRejectsOutOfRangeInput`・`TestOutspeedAndKoRejectsInvalidRequest` の追加分)が実際に
      失敗することを確認、復元して `go test ./...`・`gofmt -l`・`make judge-lint`・`make judge-build`・
      `bash scripts/check-publishable.sh` すべて成功を確認(軽微な作業のため /phase の quick-scanner〜critic
      は使わずメインで対応。CLAUDE.md「軽微な作業はメインのみでよい」)
- [x] issue #257(重大度 low)`services/judge/scripts/smoke.sh` が healthz しか叩かず、k3d 上で
      pokedex-svc・calc-svc への疎通(業務エンドポイント)を検証していない(balance・speed の smoke と
      深さが揃っていない。全体レビュー指摘。2026-09-24)。`services/gateway/scripts/smoke.sh` の ID
      取得部分(`API_URL`=gateway 経由で性格・種族・物理技の実IDを引く。pokedex 未投入なら例の架空ID
      にフォールバックし「未投入」として区別)を流用し、`JUDGE_URL`(judge 自身の Ingress)へ
      `POST /api/judge/v1/outspeed-and-ko` を実際に送って 200(`matchups[0].attackerKo.hits` を含む)・
      ヘッダなし 400 `invalid_request`・未知 speciesKey 422 `unknown_species`(pokedex 到達時のみ)・
      7候補(上限6超過)400 `invalid_request` を確認するよう拡張。`Makefile` に `API_URL` を追加し
      `judge-smoke` に配線。README の古い「JD0完了」表記も直した(coding-rules §8)。
      実クラスタ(k3d-pokecalc。実データ)で `make judge-smoke` を実行して確認済み
      (`judge smoke: master=pokedex species=0003-000 move=highhorsepower nature=bashful` /
      `health=200 outspeed=200(hits=4) missing_headers=400 unknown_species=422 too_many_defenders=400`)。
      `JUDGE_URL` を到達不能にすると exit 1 になることも確認(回帰検出力)。`pokedex-svc` が未投入時の
      example フォールバック分岐は、共有クラスタの pokedex-svc を落とさずに済ませるため、
      `services/gateway/scripts/smoke.sh` の同じロジックを流用したことによる構成の裏取りで代える
      (実際に落として確認はしていない)。軽微な作業のため /phase の quick-scanner〜critic は
      使わずメインで対応
- [~] issue #260(担当: タイプバランス・判定。重大度 low)`docs/judge-design.md`・`docs/type-balance-design.md` が
      実装の後追いになっていない(全体レビュー指摘。2026-09-24)。**判定レーンの分だけ対応**:
      `judge-design.md` の状態を「起草」→「完了(JD0〜JD5・main統合済み)」に、JD5節を「着手する」から
      実際の完了内容(ADR-0705・PR #182・担当決定)へ更新、JD1の麻痺の記述(JD2で扱う、が誤り。JD2でも
      見送りを継続したのが正しい)を訂正、新設の §5「未対応(既知の制限)」に状態異常・素早さ関連特性・
      ダブルの全体技/壁減衰(issue #288)を明記。`docs/README.md` の目次は既に judge-design.md を指しており
      変更不要。`type-balance-design.md` はタイプバランスレーンの持ち物のため対象外(DECISIONS.mdへ)。
      `bash scripts/check-publishable.sh`(0件)成功を確認。軽微な作業のためメインで対応

## DOC: 文書(全レーン。docs/coding-rules.md §8。2026-09-22 ユーザー要望)
各レーンが自分の範囲の README(何をするか・mermaid の構成図・ディレクトリ・コマンド・関連 ADR。80 行以内)と、動かして確かめられるレーンは手順書(`docs/runbooks/<レーン>.md`。AGENTS.md「手順書の書き方」に従う)を書く。全体図は `docs/architecture.md`。
- [x] DOC-data: `engine/README.md`・`services/pokedex/README.md`・`tools/importer/README.md`・`tools/golden/README.md`、手順書 `docs/runbooks/data.md`(migrate・import・dry-run の確認)
- [x] DOC-api: `services/calc/README.md`・`services/gateway/README.md` を §8 の形に、手順書 `docs/runbooks/api.md`(k3d での疎通)。critic PASS
- [x] DOC-web: `web/README.md`、手順書(`docs/verify-m1.md` の画面の部分と重複させない。M1 の完了報告は verify-m1.md にまとめる)
- [x] DOC-tb: `services/balance/README.md` を §8 の形に、手順書 `docs/runbooks/balance.md`
- [x] DOC-speed: `services/speed/README.md` を §8 の形に、手順書 `docs/runbooks/speed.md`(k3d での疎通を確認済み)
- [x] DOC-ios: `ios/README.md`(coding-rules §8 の形)、手順書 `docs/runbooks/ios.md`(シミュレータでの確認。実機インストールは P6-4)。受け入れ条件・判断は ADR-0501 へ移動
- [x] DOC-arch: `docs/architecture.md` を全レーンの現行構成(gateway・pokedex・calc・balance・speed・judge・record/team〈計画中〉・
  データの流れ・WASM・Kustomize overlay)に合わせて更新(2026-09-24。素早さレーンが担当。PR #168・#194)。以後も各レーンが自分の
  変化に合わせて保つ(単発の完了ではなく継続する運用)

## M4: 運用
- [x] P7-1 kube-prometheus-stack / Loki、各サービスのメトリクス
  - [x] メトリクス計測(ADR-0406 §1〜3。2026-09-24): gateway・pokedex・calc(`services/internal/httpmetrics`)、
    balance・speed・judge(各自 `internal/httpmetrics` に複製)すべてに `GET /metrics`(Prometheus text format、
    `http_requests_total{method,path,status}` / `http_request_duration_seconds{method,path}`、path はルーティング
    パターン)を追加。`github.com/prometheus/client_golang` v1.24.1 を4 go.mod に追加。`make test`/`make lint`/
    `make build`/`make check-publishable` 確認済み
  - [x] kube-prometheus-stack / Loki / Alloy の導入(ADR-0406 §4〜5。2026-09-24): `scripts/observability-bootstrap.sh`
    (取得→3チャート分ハッシュ検証→namespace冪等作成→`helm upgrade --install`→ServiceMonitor適用の順。ADR-0405 と同じ
    「取得→検証→適用」)、`deploy/k8s/base/observability/`(values 3つ・ServiceMonitor 6つ・kustomization)を実装。
    `scripts/observability-bootstrap_test.sh`(全192チェック)・`make test`/`make lint`/`make build`/
    `make check-publishable`(0件)確認済み。手順書 `docs/runbooks/observability.md` を追加。
    独立レビュー(critic)で、k3d(servers:1/agents:0)では chart 既定の chunks-cache/results-cache(memcached。
    memory request 各9830Mi/1229Mi)と `monitoring.lokiCanary`(chart に存在しないキーで無効化できていなかった)
    が Pending のまま残る指摘を受け、`loki.yaml` に `chunksCache.enabled: false`・`resultsCache.enabled: false`・
    トップレベル `lokiCanary.enabled: false` を追加(`helm template` で StatefulSet/DaemonSet が描画されなくなることを
    確認)。あわせて `check-publishable` が実際には B(秘密らしき文字列の誤検知4件)で失敗していたのを
    `scripts/check-publishable.sh` の許可リストとコメント・runbook の書き方を直して解消。
    Grafana に Loki データソース(`grafana.additionalDataSources`)を追加、runbook のパスワード一時ファイルに
    `umask 077`+`mktemp -d` を適用(他ユーザーに読める権限で残らないように)。
    **実クラスタ(k3d)で `scripts/observability-bootstrap.sh` を実行して確認済み(2026-09-25)**: 全8 Pod が
    Pending なく Running(chunks-cache/results-cache/lokiCanary は描画されない)、`storage-loki-0` PVC が Bound、
    6つの ServiceMonitor が適用され、balance・calc・gateway(直近に再デプロイ済みで `/metrics` を持つイメージ)への
    scrape が Prometheus targets で `up`(judge・pokedex・speed は `/metrics` 追加前の古いイメージのままのため
    404。各レーンが次に再デプロイすれば up になる見込み)。Grafana は Loki・Prometheus・Alertmanager の3
    データソースを認識、Loki への クエリで `pokecalc` namespace の実ログ(Alloy 経由)を取得できることを確認
- [x] P7-2 SLO(計算API p99 < 100ms、可用性)とダッシュボード(ADR-0407。実装・自動テスト済み、実クラスタ未確認)。
  P7-1(ADR-0406)の `http_requests_total`・`http_request_duration_seconds`(job="calc"、計算3エンドポイント
  `/api/calc`・`/api/calc/bulk`・`/api/calc/reverse`)だけで計算でき、calc-svc へのコード追加は無し。
  `deploy/k8s/base/observability/prometheusrules/calc-slo.yaml` に記録ルール2つ(p99・可用性。アラートは
  作らない)、`deploy/k8s/base/observability/dashboards/`(`calc-slo.json` + `kustomization.yaml` の
  `configMapGenerator`、`grafana_dashboard: "1"` ラベル)にダッシュボード(p99/可用性の時系列2つ+直近値の
  stat 2つ、すべて記録ルール参照で生クエリなし)。`values/kube-prometheus-stack.yaml` に
  `ruleSelectorNilUsesHelmValues: false` を追加(ServiceMonitor と同じ理由)。親 kustomization の resources に
  `prometheusrules/calc-slo.yaml`・`dashboards/` を追加。新規 `scripts/observability-slo_test.sh`(48件)が
  全件成功、既存 `scripts/observability-bootstrap_test.sh`(192件)も壊れていない。`make test`・`make lint`・
  `make build`・`make check-publishable` 成功、`kubectl kustomize deploy/k8s/base/observability` の描画結果に
  PrometheusRule 1つ・`grafana_dashboard: "1"` の ConfigMap 1つを確認。**実クラスタ(k3d-pokecalc)で確認済み**
  (2026-09-25。独立レビュー時): `scripts/observability-bootstrap.sh` を再実行し、`/api/v1/rules` で記録ルール
  2本とも `health: ok`(p99 ≈ 5ms、可用性 = 1)、`/api/calc` へ不正な入力を30件送っても4xxは可用性に数えられない
  ことを確認(ADR §1どおり)。Grafana sidecarがダッシュボード(uid `calc-slo`)を読み込み、パネル4つが表示された
- [ ] P7-3 ArgoCD(GitOps)
- [ ] P7-4 MySQL/TiDB バックアップと復元テスト(ADR-0209 §9 を要件に含める: バックアップに `devices`〈墓石〉を含める /
  purge journal(#5b。世代取得後の削除要求。保持90日)をバックアップ世代と別に保持し復元時に再適用 /
  Ready の前に墓石の再適用・purge journal の再適用・失効ジョブの強制実行 / JetStream は再生しない / 世代30日。
  受け入れ条件は AC-B1〜B3・AC-B2b)

## ブロッカー
(ここに止まった理由と試したことを書く)

**【人間の確認待ち】(Web レーン、2026-09-22 深夜に記載)**
- **P4-5 のブラウザ実機確認**(仕様ブロッカーではない。作業は止めない。**Chrome は 2026-09-22 に確認済み**、残りは Safari): `make web-dev` で開き、Chrome と Safari で計算・逆算が動くこと、
  `.wasm` の MIME type(`application/wasm`)・`WebAssembly.instantiateStreaming`(失敗時は arrayBuffer にフォールバックする実装)・
  キャッシュ・初回ロード(約4.6MB / gzip 1.3MB)・メモリを確認する。既定案: 確認できるまで P4-5 は「実装・自動テスト済み、実機未確認」として扱う。
  **追加(issue #333、2026-09-25)**: 375px幅未満でタブ列を左端までスクロールし、先頭の「計算」タブが
  読める・押せることも合わせて確認する(`justify-content: safe center` を使っており、`safe`キーワードの
  Safari対応をPlaywrightで自動確認できていない。docs/design.md「幅への対応」参照)。
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
- [x] issue #148(クラウド公開前のアクセス境界・認証方針。ユーザー決定「私設サービスを維持する」)の API レーン担当分: `deploy/k8s/overlays/cloud` から gateway の Ingress を削除 patch で除去し、public Ingress/LoadBalancer/NodePort/externalIPs/hostNetwork/hostPort が無いことを構造検査+`kubectl kustomize`実描画検査の2層で固定(ADR-0210)。TLS 終端は gateway/クラスタの Ingress では行わず Tailscale(`tailscale serve`)に任せる方針を決定。端末IDが認証として機能しないこと・CORSが到達制御でないことの回帰テストを追加(`TestDeviceIDIsNotAuthentication`・`TestCORSIsNotAccessControl`・`TestContractHasNoAuthentication`)。`base`のgateway Ingress本体は local(k3d)専用として残し、先頭コメントで明記。ADR-0209 §1(クラウド公開へ進む判断)は「公開しない」で確定した旨を追記。critic PASS。運用(tailnet ACL・失効手順のrunbook)・Web/iOS(接続先をtailnet名に)への依頼はDECISIONS.mdに既定案付きで記録(issue はレーンの完了までクローズしない)
- [x] issue #106(データ・運用レーン。Codex レビュー)手動 import Job(`make import-k8s`)と定期 CronJob が同時実行できる問題: `concurrencyPolicy: Forbid` は同じ CronJob が作る Job 同士にしか効かず、`kubectl create job --from=cronjob/...` が作る独立した手動 Job とは排他しないため、共有 PVC(`pokedex-import-cache`)上の取得キャッシュ・DB 投入が競合しうる実バグだった。`tools/importer/cronjob.sh` に busybox の `flock`(非ブロッキング)を `fetch.mjs` 呼び出しより前に追加し、取得〜投入の全工程をアプリ側で排他(ADR-0109)。ロック取得失敗は既存の終了コード規約どおり終了コード1(再試行可能)にし、`cronjob-import.yaml`(podFailurePolicy・concurrencyPolicy とも既存のまま)・`services/pokedex/cmd/import`(Go CLI)・Makefile は無変更。2プロセス同時起動の統合テスト(`cronjob_lock_test.go`)を追加し、Docker(Linux・busybox flock)で実際にロックが機能することを確認済み(macOS はローカルに flock が無いため自動 Skip)。critic PASS。k3d での手動確認手順は docs/runbooks/data.md §6 に追記し、2026-09-23 に実クラスタで実施: 2つの手動 Job を同時作成し、片方が「別の import が実行中」のログで即座に終了コード1、`backoffLimit` の再試行で成功したことを確認(秘密は出力に含まれない)
- [x] issue #104(データ・運用レーン。Codex レビュー)pokedex の DB 資格情報を用途別の最小権限へ分離する: server(検索API)・importer(CronJob)・migrate(Job)がすべて root 相当の同じ資格情報(Secret `mysql-auth`/`pokedex-dsn`)を使っており、公開 HTTP Pod が侵害されると DDL・ユーザー管理まで可能だった実リスクだった。`pokedex_reader`(SELECT専用)・`pokedex_importer`(SELECT/INSERT/UPDATE/DELETE)・`pokedex_migrator`(+ CREATE/ALTER/DROP/INDEX/REFERENCES)の3ロールを作り、server/importer/migrateそれぞれに最小限のDSNだけを渡す(ADR-0110)。`services/pokedex/db.Provision`が冪等・ローテーション対応で3ユーザーを作成・GRANT(接続前に正規表現でパスワード・ユーザー名・DB名・権限を検証し、root自身を対象にする入力は拒否)。`services/pokedex/cmd/migrate`の`up`は`POKEDEX_PROVISION_DSN`があるときだけプロビジョニングしてから実際のmigrationを行う(無ければ後方互換で直接migration。ローカルmake dev/make test-dbは対象外)。`scripts/up.sh`は新規クラスタで4DSNを一度に作成、既存クラスタは無いキーだけ`kubectl patch`で追記(値をargv/ログに出さない設計に修正)。critic PASS(1往復。指摘は軽微5件、うちroot保護の抜け穴を塞ぐテスト追加・check-publishable.shの過剰な許可パターン修正・up.shのエラー握り潰し修正の3件を反映)。**実クラスタ(k3d)で実際に`make up`を実行し、`SHOW GRANTS`で3ユーザーの権限がADR決定1と過不足なく一致することを確認済み**。既存クラスタからの無停止移行(Secretへのキー追記のみ)も実地確認済み
- [x] issue #109(データ・運用レーン。Codex レビュー)pokedex HTTPサーバーにタイムアウトとgraceful shutdownを追加する: `services/pokedex/cmd/pokedex/main.go`の`runServe`が`http.ListenAndServe`を直接呼ぶだけでタイムアウト(ReadHeaderTimeout等)を一切設定せず、SIGINT/SIGTERMも購読しないため、遅い・不完全な接続がリソースを無期限に保持し、Kubernetesのrollout・node drainで処理中リクエストが即座に打ち切られていた実リスクだった(calc/gateway/balance/judgeは既に対応済みでpokedexだけが欠けていた)。`services/balance`と同じ値(readHeaderTimeout=5s・readTimeout=10s・writeTimeout=15s・idleTimeout=60s・maxHeaderBytes=16KiB・shutdownTimeout=10s)で`newHTTPServer`/`serve`/`runServe`の3層に分離(ADR-0111)。既存の`run(args) int`(サブコマンド振り分け)との名前衝突を`runServe`への改名と`runServeCmd`の新設で解消。`deployment.yaml`に`terminationGracePeriodSeconds: 30`を追加し、main.goの`shutdownTimeout`定数より長いことをハードコードせず不等式でmanifestテストに固定。critic PASS(1往復。指摘なし)。ヘッダ未完了接続の切断(生TCP接続で実測)・shutdown中のin-flightリクエスト完了・DSN非露出をすべてテストで固定し、`-race`・`-count=3`でも安定を確認。**実クラスタ(k3d)でpokedexを再ビルド・再デプロイし、`terminationGracePeriodSeconds`が実際に30になっていること・`api-smoke`が正常応答することを確認済み**
- [x] issue #112(データ・運用レーン。Codex レビュー)pokedexのDB接続プールに上限と寿命を設定する: `services/pokedex/cmd/pokedex/main.go`が`sql.Open`後に`SetMaxOpenConns`等を一度も呼ばず(Go標準の既定は無制限)、突発的な同時要求がそのままMySQL接続数に転嫁され、1 Podでも接続枠を占有しうる実リスクだった。4環境変数(`POKEDEX_DB_MAX_OPEN_CONNS`=10・`POKEDEX_DB_MAX_IDLE_CONNS`=5・`POKEDEX_DB_CONN_MAX_IDLE_TIME`=5m・`POKEDEX_DB_CONN_MAX_LIFETIME`=30m)を追加し、`services/pokedex/db.OpenPool`(プール生成を1か所に集約。P7-1のメトリクス化に備える)経由で適用(ADR-0112)。検証(open/idleは正の整数・idle<=open・durationは正値)は`sql.Open`より前、エラー文にDSNを含めない。`export`サブコマンドは`(PoolConfig).ForExport()`で`MaxOpenConns=1`に上書き(逐次処理の実態に合わせる)。`deployment.yaml`に4環境変数を既定値のまま明示し、`docs/runbooks/data.md`にreplica数を増やすときの接続予算の注記を追加。critic PASS(1往復。軽微指摘1件〈idle==openの境界値テスト追加〉を反映)。実MySQLで同時クエリがMaxOpenConnsを超えないこと(直列化の実測込み)を確認。**実クラスタ(k3d)でpokedexを再ビルド・再デプロイし、4環境変数が実際に設定されていること・`api-smoke`が正常応答することを確認済み**
- [x] issue #69(データ/APIレーン)技・持ち物検索の並びがOpenAPI契約と一致しない: `api/openapi.yaml` の `searchMoves`/`searchItems` の description が「並びは ID 順」としていたが、`services/pokedex/db/query/pokedex.sql` の `SearchMoves`/`SearchItems` は導入時(P2-3)から一貫して `ORDER BY <table>.name_ja, <table>.id`(日本語名の照合順序が正。ADR-0105 §3 に「技・持ち物は name_ja, id」と既に明記されており、SQL 側もこれに一致していた)。つまり誤っていたのは契約の説明文だけで、SQL・ADR は無変更(新規 ADR は不要)。`searchSpecies`(`dex_no, form`。SpeciesKey が固定幅ゼロ埋めのためこれは文字列としての ID 順と一致)と `listNatures`(`ORDER BY id`)は元から契約どおりで対象外。契約の description を実態(名前順・同順位は ID)に訂正して `make gen`・`make ios-gen`(絶対ルール1。iOS 生成物は getMove〈P3-7〉分も含めて追従していなかったため合わせて解消)。DB 層(`db.TestSearchMovesAndItemsOrderIsNameJaNotID`。実 MySQL で確認、`-tags mysql`)と httpapi 層(`TestSearchMovesAndItemsPreserveGivenOrderAndLimitCutsThatOrder`。ハンドラが並べ替えず、`limit` がその並びの先頭から切ることを固定。ID 順に並べ替えてから切ると集合自体が変わることを変異テストで確認済み)の両方にテストを追加。critic PASS(1往復)
- [x] issue #73(API/データレーン)OpenAPIとengineの防御プリセット集合を同期検査する: `api/openapi.yaml` の `DefenderPreset` enum と `engine.DefenderPresetCatalog()` は1対1対応が前提(`services/calc/internal/httpapi/convert.go` の `presetKeysFrom` は変換テーブルを持たず契約の列挙値をそのまま `engine.PresetKey` に型変換するだけ)だが、それを固定するテストが無かった。`services/calc/internal/httpapi/preset_sync_test.go`(`TestDefenderPresetEnumMatchesEngineCatalog`)を追加: ハードコードした一覧同士を比較する既存の `vocabulary_test.go` の流儀ではなく、契約(埋め込まれた spec。`loadContract` 経由の kin-openapi)から `DefenderPreset` の enum を直接読み、`engine.DefenderPresetCatalog()` のキー集合・順序(契約の description が「耐久が上がる順」と明記。ADR-0009 §1)と比較する(ハードコードした一覧は「足し忘れ」自体を検出できないため避けた)。変異テストで両方向(engineだけに追加・openapi.yamlだけから削除・件数一致のまま重複させて列としてだけ崩す)を実際に検知することを確認済み(確認後 revert)。critic 1回目FAIL(件数不一致を黙ってskipすると重複を見逃す穴。修正: 件数不一致を明示的な失敗にしてから列を比較)→ 修正 → 2回目相当でPASS
- [x] issue #113(Web/iOS/APIレーン)入力変更時の古い計算要求を抑止・キャンセルする、のAPIレーン連携分(「クライアントのcancel伝播」): Web/iOSは自レーン分(200ms debounce・AbortSignal/Task cancel)を完了済み(PR #174・iOS側コミット)だったが、実測で調べたところ gateway 側に見落としがあった。クライアントが要求を中断すると Go の `http.Server` が `r.Context()` を `context.Canceled` で終えるが、`services/gateway/internal/httpapi/proxy.go` の `ReverseProxy.ErrorHandler` はこれを区別せず「上流に到達できない」WARN ログを出し 503 `upstream_unavailable` を返していた(実際は上流もgatewayも正常で、クライアントが単に離脱しただけ)。`errors.Is(err, context.Canceled)` のときだけ特別扱いし、WARN ログを出さず(Debug に留める)応答も書かない(相手は既に居ない)ように修正。自前のタイムアウト(`net/http: timeout awaiting response headers`)は別のエラー文言になるため混同しないことを実測で確認。`TestClientCancelIsNotUpstreamUnavailable`(wall-clock sleep 不使用。フェイクRoundTripper版・実`http.Transport`版の両方)を追加、変異テストで実効性を確認。ADR-0202 §5 に追記(AC-G10)。**限界**: gateway→calc-svcへのcontextキャンセル伝播自体は効くが、calc-svcのハンドラ・engineはcontextを見ない(engineを純粋に保つ絶対ルール2)ため、issue本文の「calc-svc CPU消費も止める」は本修正の範囲では未達成(中断された逆算は完走する。ADR-0208の上限で最悪計算量は有界なので実害は限定的。詳細はDECISIONS.md)。issue本文の受け入れ条件・対象範囲はいずれもWeb/iOS固有かgateway/calcのtimeout値等を明示的に除外しており、この限界を残したままissue #113はWeb/iOS/APIすべてのレーン分が完了としてクローズ可
- [x] P4-17(Web/APIレーン)技のID解決の欠落を解消(ADR-0304 §3): `GET /api/pokedex/moves/batch?ids=...`(`getMovesByIds`)を新設。ADR-0304が当初推していた案A(`getSpecies.learnset`をID配列からMove実体配列に変える)は採らなかった。理由: iOS(M3)が`SpeciesDetail.learnset`を`string[]`のまま前提にした機能(CalcViewModel・ReverseViewModel・TeamEditViewModelがlearnsetをID集合として扱い、検索結果との積集合を取る)を既に出荷済みで、案Aはその完成済み機能を壊す破壊的変更になり「契約変更が小さい方」の基準に反すると判断。新設したエンドポイントは既存のlearnsetを無変更のまま、1回の呼び出しでIDの配列をMove実体の配列に解決する。ids 1〜64件(ADR-0208の前例。生成ラッパは配列のmaxItemsを検証しないため`services/pokedex/internal/httpapi/search.go`で自前検査)、見つからないIDは黙って省く、応答順はidsと同じ(DBのIN句は順序を保証しないためハンドラで並べ替え)、`getMove`と同様に既定のレギュレーションで絞らない。ルーティング(`/moves/batch`が`/moves/:key`に食われないこと)を含めテスト済み(`TestGetMovesByIds`)。ADR-0105 §3・ADR-0304 §3に追記。**ids 64件超の実データ確認はまだ行っていない**(1種族のlearnsetが64件を超える場合はWeb側で分割呼び出しが必要。詳細はADR-0304 §3・DECISIONS.md)。critic 1回目FAIL(コメント・ドキュメントの事実誤り3件。実装・テストへの指摘なし)→ 修正・推奨事項も反映 → 2回目FAIL(1: 64件で1回に収まるという未検証の約束をしていた。実データ確認できず〈k3dクラスタ停止中〉、分割呼び出し前提に書き換えて対応 2: calc-svcのエラーコードのコメントが実際と不一致〈missing_headerが正・invalid_inputは誤り〉 3: 契約のmaxItemsとGo定数64の同期テストが無かった。`api.GetSwagger()`から契約のmaxItemsを読んで期待値にする`contractQueryParamMaxItems`ヘルパーを追加し、変異テスト〈契約だけ32に変更〉で同期の実効性を確認)→ 修正済み → **3回目PASS**(重要2件を反映してcommit: Web欄のP4-17の行が「未回答」のまま古くなっていたのを解決済みに更新、`TestGetMovesByIds`に未検証だった3つの契約どおりの挙動〈マスタ未投入→200 []・重複ids→重複したまま返る・空要素は黙って省く〉のテストを追加。軽微なコメントの言い回しの訂正も反映)。Webレーンへ実装完了を連絡(learnsetの解決に使ってP4-17の技オンライン未対応を解消できる。64件超は分割呼び出しが必要である旨も伝える)
- MySQL の manifest に MYSQL_DATABASE が無く、初回起動時に pokedex DB が自動作成されない実バグを発見(データレーンが k3d に初めて実デプロイした際に発生)。deploy/k8s/overlays/local/mysql/statefulset.yaml に MYSQL_DATABASE: pokedex を追加し、layout_test.go に検知テストを追加して修正(2026-09-22)。**新規クラスタでは直るが、この修正前にすでに初期化済みの PVC は MYSQL_DATABASE の効果を受けない**(コンテナ起動時にしか実行されない仕様のため)。既存の PVC に対しては CREATE DATABASE を手動実行するしかない。docs/runbooks/data.md に一言注記するとよい
- P2-3 の critic の軽微(2026-09-22。4件。#74): 未反映は `check-publishable.sh` の `B_KEYVALUE_ALLOW` を self-test の基準リポジトリにも播く、の1件。
  pokedex 側の3件は対応済み(2026-09-25): readmodel の `maxCatalogAbilityCount` を balance の schema の maxItems と直接比べる同期テスト(balance の loader 側の定数はタイプバランスレーン) /
  nature-mismatch の Blocker の Detail に `make import-fetch` を含む復旧案内 / `TestPublicInputValidation` の 400 応答を契約検証(kin-openapi)に通す

- P2-2d の critic の軽微(2026-09-22): `cronjob_layout_test.go` の「消さない」検査を secret・statefulset・configmap にも広げる / `make lint` が kubectl に依存する(kubectl の無い環境では失敗する)/ upstream の `checkedAt` が未来でも fresh 扱い / **コンテナの中で取得スクリプト(Showdown の build 等)を実際に流した記録が無い。初回の `make import-k8s` で確かめる**

- [x] issue #76(データレーン)`services/pokedex/db/mysql_test.go` に「species_abilities.slot = 4 が入る」ことを確かめるケースを足す(P2-2c の critic の軽微。000005 は使い捨てコンテナで手動確認済みだったが自動テストが無かった): `TestConstraintsRejectInvalidRows` のスロット5拒否・特性重複拒否は負方向だけだったため、`TestSpeciesAbilitiesSlot4RoundTrip` を追加し slot 4 への挿入成功と読み戻し(`SELECT ... ORDER BY slot`)を固定。確認・ロールバックはトランザクション内(コミットして残すと、他テストの `freshDB` が呼ぶ `DownAll` が migration 000005 の down〈CHECK を 1..3 へ戻す〉で失敗するため)。migration 000005 の CHECK を一時的に `(1,2,3)` に戻して新テストが失敗することを確認した上で revert(退行検知の実効性を確認)。`-tags mysql`(使い捨て MySQL コンテナ、pin 済み `mysql:9.7.2`)・`make test`・`make lint` 成功

- [x] `scripts/check-publishable.sh --self-test` の既存の失敗2件を MT-2 で修正し、`make lint` に自己テストを追加(2026-09-22)

P1-6 独立レビューで出た軽微・任意の指摘(コードは未変更。次の engine タスクに合わせて対応を検討):
- `engine/golden_test.go`: `speciesCount` の下限アサート追加(現在は 0 だけ検査。少数種で再生成しても通ってしまう)
- `engine/golden_test.go`: `DamageInput` の json タグ明示または `DisallowUnknownFields`(フィールド改名でフィクスチャ値が黙ってゼロ値になる)
- [x] `Makefile`: `go vet -tags golden` を lint に追加 / `golden-generate` は説明どおり `npm ci` を実行するか未導入で明示的に失敗させる(#77。vet は allspecies も。`golden-generate` は `npm ci` してから生成)
- `tools/golden/package.json`: `^0.10.0` を `0.10.0` に完全固定
- [x] `engine/damage.go` `chainMods`: @smogon/calc はクランプ(41/410〜131072/2097152)を持つ。現在の補正集合では到達しないが、補正追加時に再確認(#77。同じクランプを実装し境界テストを追加)
- ゴールデン未カバー: リフレクターとオーロラベールの同時成立、`Effectiveness` / `STAB` の直接照合(L1 では確認済み。壁の同時成立は #77 で L1 の回帰テストを追加)
