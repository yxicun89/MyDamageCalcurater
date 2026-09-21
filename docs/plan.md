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
  - [ ] R-2-3 `tools/golden/package.json` を `0.10.0` に完全固定、`.gitignore` に `*.wasm` `*.pem` `*.key` `*.p12` を追加
  - [ ] R-2-4 ドメイン定数に名前を付けて集約(75/20/2048/6144/8192、プリセットの 32)
  - [ ] R-2-5 小さな可読性の是正(`Version` の重複、空コメント、`real` の改名、`AllStatKeys` の可変性)
  - [ ] R-2-7 ADR の追記・修正(タイプ相性表の所有者、engine の Label の扱い、内輪の表現。ユーザーの判断後)
- [ ] R-3 `make check-publishable`(秘密情報・絶対パス・個人情報・追跡してはいけないファイルの検査。`make lint` から呼ぶ)

### Phase 1b 決定の反映(2026-09-21 のユーザー決定。ADR-0002 の確定方針・DECISIONS.md 参照)
- [x] P1-10 防御プリセットの再定義: `hb` = H32・B32・性格補正なし / `hd` = H32・D32・補正なし / `hb_boost` = H32・B0・B上昇性格 / `hd_boost` = H32・D0・D上昇性格 / **新設** `hb_full` = H32・B32・B上昇性格 / `hd_full` = H32・D32・D上昇性格。ADR-0009・`engine/bulk.go` のカタログと既定セット・テスト・`tools/golden`(旧 `hb`/`hd` の外部照合ベクタは性格補正ありなので `hb_full`/`hd_full` 相当。補正なしの `hb`/`hd` を追加)・`api/openapi.yaml`(`DefenderPreset` enum と例)を更新して `make gen`(絶対ルール1)。`hb_boost`/`hd_boost` の確認待ちは解消
- [ ] P1-11 表示%の分離(ユーザー方針: **最小ダメージ側は切り捨て、最大ダメージ側は四捨五入**にして「最低これくらい入る」を保守的に示す。**乱数で何%で倒せるか**(`ko.chancePercent`。乱数n発の確率)も、結果に必ず表示できる形で出力する): アプリが表示する計算結果の%は**小数第1位**(例 73.4%)。HP 比率から直接求め、整数%に丸めない(整数演算で 0.1% 単位)。実機画面の観測%(整数%)を逆算の入力にする場合の丸め規則は**別関数・別概念**にし、同じ `DisplayPercent` に混在させない。engine・`engine/wasmapi`(`minPercent`/`maxPercent`)・`api/openapi.yaml`・ADR-0010/0011 を更新
- [ ] P1-12 逆算の再設計: 固定プリセットからの選択をやめ、**H32 前提で B(D) SP を 0〜32 探索**、性格は「補正なし」「B(D) 上昇」の2通り(攻撃側の A/C も同様に0〜32×補正なし/上昇)。結果は「性格補正あり/なし × 持ち物」ごとの**SP の範囲**で、観測から区別できない候補は決め打ちせず残す。ADR-0010 を改訂、再現率テスト(Recall@5 の定義=真値の SP が範囲に入る等)を再設計し基準(1観測 ≥80%・2観測 ≥95%)は緩めない。P1-11 の後

### Phase 2 マスタデータ
- [x] P2-1 **データソース調査**: チャンピオンズの使用可能ポケモン・技・持ち物の取得元を調べ ADR-0002 に記録(調査完了。ADR-0002 は 2026-09-21 のユーザー決定で方針確定。残る確認事項はブロッカー節)
  - 確定後に、ゴールデンの種族集合(現在は gen9 参考集合1392種)を差し替えて `make golden-generate` で再生成し、`metadata.json` の `speciesScope` を更新する
  - HP=1 のヌケニンはゴールデンから除外している。チャンピオンズの実数値式(HP = 種族値+75+SP)では HP=76 になるため、扱いを決める
- [ ] P2-1b ゴールデンの oracle を `@smogon/calc@0.12.0` の Champions へ切り替え: `tools/golden/package.json` を `0.12.0` に**完全固定**(`^` 不可)。**先に旧ゴールデンと 0.12.0 Champions の結果を diff し、差分を確認してから**更新する(いきなり上書きしない)。種族集合を Champions 集合へ、SP の換算(`8×SP−4`)が不要になる。`known_diffs.yaml` への追加は ADR 付きで人間レビュー(CLAUDE.md)。`make test-golden` 全件一致。ADR-0002 §決定 5
- [ ] P2-1c 技の使用可否の調査: calc のみが持つ 11 技(Anchor Shot・Astral Barrage・Blood Moon・Bolt Beak・Dragon Hammer・Fishious Rend・Gear Grind・Hyper Drill・Metal Claw・Revelation Dance・Triple Dive)、Showdown のみの Pound、Snap Trap・Growth のタイプを、GameWith のポケモンチャンピオンズのデータやポケモン徹底攻略など**公式以外の攻略サイト**(規約・アクセス頻度に配慮し、必要最小限の取得)で調べ、出典 URL と確認日付きで裁定案を ADR-0002 に追記。断定できない項目は「未確認」と書き、最終裁定はユーザー
- [ ] P2-2 MySQL スキーマ(migrate)と importer(**実マスタ・スナップショットは Git にコミットしない**: `data/generated/` は .gitignore、Git には schema・importer・架空データの example・README・データの版 metadata のみ。使用可能集合・持ち物候補は**レギュレーション(v1 は M-C)依存のデータ**で、M-C を直書きしない。日本語名は PokeAPI+ローカル override。ADR-0002)
- [ ] P2-3 pokedex-svc(検索・詳細・持ち物/技一覧、日本語名で前方一致)

### Phase 3 API
- [ ] P3-1 calc-svc(起動時にマスタをメモリへ読み込み)
  - 一括計算(`/api/calc/bulk`)の対応: API の `presets`(enum 配列)→ engine の `PresetKeys`。`presets: []` と省略はどちらも既定セット
  - `api/openapi.yaml` の description を先に直して `make gen`(絶対ルール1): 「変化技は none/hp の2件のみ返す」「行の順序はプリセット優先(presets × itemVariants)」「`presets: []` は省略と同じ」。`BulkCalcRow.preset` は enum のみ(engine のカスタム `Presets` は API に出さない)
  - 逆算(`/api/calc/reverse`)の対応(ADR-0010 §9 の持ち越し。engine は `CalcReverse` 済み・API 未変更): `api/openapi.yaml` を先に直して `make gen`(絶対ルール1)。`ReverseRequest.attacker` / `defenderSpeciesKey` を `known` / `unknownSpeciesKey` に改名(`side=attacker` のとき既知側=自分=防御側)、`itemCandidates: [ItemId]` と `Observation.observedDamage` を追加、`observedPercent` は整数でなければ 400(engine には `int` を渡す)、`matchScore` の意味(1.0 = 全観測に完全一致する格子点がある)を description に書き `exact` を足す、`ReverseCandidate` に `archetypeKey` を足し `presetLabel` に `Archetype.Label` を入れる、`natureId` は engine の性格構造値(代表性格クラス)→ 性格 ID に calc-svc が写像する(例 +B/-A → Bold)、`rangePercent` は `MinPercent`/`MaxPercent` を写す
  - WASM 境界との契約差分の解消(ADR-0011 §10 の持ち越し。P1-9 では `api/openapi.yaml` を変更していない): `api/openapi.yaml` を先に直して `make gen`(絶対ルール1)。`CalcResult.minPercent/maxPercent` を整数(`engine.DisplayPercent`)にして description に丸めを書く、`CalcResult` に `category` を足すか(Web は要求から知っているので落とすか)を決める、`BulkCalcRow` に防御側の `defender{sp,nature,stats}`(SP・性格・実数値)を足す、エラーの `code` 語彙を WASM 境界(ADR-0011 §5 の `invalid_json` / `unknown_field` / `invalid_enum` / `invalid_input` / `unknown_preset` / `duplicate_preset` / `invalid_preset` / `invalid_reverse_side` / `no_observation` / `invalid_observation` / `internal`)と共通化し、同じ失敗が HTTP と WASM で同じ `code` になるようにする
- [ ] P3-2 gateway(ルーティング・端末ID/セッションID・/assets・CORS)
- [ ] P3-3 契約テスト(OpenAPI 準拠)と k3d 上のスモークテスト

### Phase 4 Web
- [ ] P4-1 デザイントークン(docs/design.md)を CSS 変数に実装
- [ ] P4-2 計算画面(左右カード・持ち物・技・結果の一括表示・攻守入れ替え)
- [ ] P4-3 プリセット選択(自分側: A特化/A振り/無振り)
- [ ] P4-4 逆算画面(観測ダメージ入力→候補リスト)
- [ ] P4-5 API / WASM 切り替え(WASM ならバックエンド無しで動く)
  - WASM 境界との契約差分の解消(ADR-0011 §10 の持ち越し): Web に「ID → 実体(種族・技・持ち物・特性)」の解決層を1つ置き、オンライン(API に ID を送る=`moveId` など)とオフライン(WASM に解決済みの `move` / `Individual` を渡す)で同じ型を共有する。`openapi-typescript` の生成型と ADR-0011 §3 の DTO の対応表を作る。`minPercent`/`maxPercent` の整数化・`category`・`BulkCalcRow.defender`・エラー `code` 語彙の共通化(P3-1 で契約側を直した後)に Web 側を追従させる。WASM の遅延ロード(オンラインは API、オフラインだけ WASM)にするかを決める(ADR-0011 §11)
  - **ブラウザ実機確認**(P1-9 は Node + wasm_exec.js までの確認。仕様ブロッカーではない): Chrome と Safari で、`.wasm` の MIME type / `WebAssembly.instantiateStreaming` / キャッシュ / Service Worker との干渉 / 初回ロード(約4.6MB・gzip 1.3MB)/ メモリ を確認する
- [ ] P4-6 Playwright E2E(主要フロー)
- [ ] P4-7 **M1 完了報告**: 動作確認手順を `docs/verify-m1.md` に書く

## M2: 保存・構築
- [ ] P5-1 TiDB(tiup playground で開発、k3d は TiDB Operator 最小構成)
- [ ] P5-2 NATS JetStream と calc-svc からのイベント発行(失敗しても計算は成功)
- [ ] P5-3 record-svc(保存・よく使う集計: 頻度×時間減衰)
- [ ] P5-4 team-svc(構築 CRUD、Showdown 形式入出力)
- [ ] P5-5 Web: 履歴・よく計算する相手・構築ビルダー

## M3: iOS
- [ ] P6-1 Xcode プロジェクト、swift-openapi-generator、デザイントークン
- [ ] P6-2 計算画面・逆算・構築
- [ ] P6-3 シミュレータテスト(`make ios-test`)
- [ ] P6-4 Tailscale serve の手順書 → **人間が実機インストール**

## M4: 運用
- [ ] P7-1 kube-prometheus-stack / Loki、各サービスのメトリクス
- [ ] P7-2 SLO(計算API p99 < 100ms、可用性)とダッシュボード
- [ ] P7-3 ArgoCD(GitOps)
- [ ] P7-4 MySQL/TiDB バックアップと復元テスト

## ブロッカー
(ここに止まった理由と試したことを書く)

**解決済み(2026-09-21 のユーザー決定。詳細は DECISIONS.md / ADR-0002 / requirements.md)**
- `hb_boost` / `hd_boost` の定義 → `boost` = H32 + B(D)0 + 上昇性格、`full` = H32 + B(D)32 + 上昇性格、`hb`/`hd` = 性格補正なし。P1-10 で反映
- マスタデータの取得元 → 責務分離(calc=oracle 0.12.0 固定 / Showdown=照合 / 公式情報=レギュレーション基準 / PokeAPI+override=日本語名)、v1 は M-C のみ(差し替え可能な構造)、実マスタ・スナップショットは Git に置かない
- requirements.md との食い違い → こだわり系・とつげきチョッキ・しんかのきせきは M-C 向け候補から除外(requirements.md 修正済み)。ヌケニンは v1 で考慮不要
- 計算結果の表示% → 小数第1位。P1-11 で反映
- ブラウザでの WASM 実動作 → 仕様ブロッカーではない。P4-5 の確認項目

**解決済み(追加。2026-09-21 のユーザー回答)**
- `testdata/golden` の扱い → 数値と英語識別子だけなのでコミットを続ける(ADR-0002 §追加の回答。実マスタの代替を入れない)
- 技の使用可否の食い違い → 公式以外の攻略サイト(GameWith・ポケモン徹底攻略など)も Web から調べてよい。P2-1c で調査
- メガ石 ↔ メガフォーム → メガ後の姿を別ポケモンとして登録し、専用のメガストーンを持ち物に固定(変更不可)。P2-2 のスキーマに `is_mega` / `base_species_key` / `required_item_id`

**【人間の確認待ち】残り**
- **観測ダメージの入力と丸め**(逆算の入力側。ADR-0010 §3): ユーザーの回答は「最小ダメージ側は切り捨て、最大ダメージ側は四捨五入。最低これくらい入るを知りたい」。私の解釈は次のとおりで、**違っていたら訂正してほしい**。
  - 逆算の入力は、プレイヤーが実際のゲーム画面で読み取る「相手 HP バーの減少 %」(または自分の HP の減少量)。実機の表示が**整数%か小数付きか**が分からない(未確認。ユーザーが実機で確認できるなら確認してほしい)。
  - 確認できるまでは、観測は「精度付きの値」(整数%か小数第1位か)として受け、その精度が表す範囲に候補のダメージが入るかで照合する(特定の丸め規則に依存しない)。P1-12 で設計する。
  - アプリが**表示する**ダメージ%は、最小側を切り捨て・最大側を四捨五入した小数第1位(P1-11)。
- **公開に向けた判断**(audit-r1.md「ユーザーの判断が要る点」): LICENSE の方針 / Go module path の GitHub アカウント名 / Git 履歴の作者情報 / ADR-0002 の第三者データ抜粋 / タイプ相性表の正 / engine の日本語ラベル
- **技の使用可否の調査結果の裁定**(P2-1c の結果を見て決める)
- **見た目違いフォームの持ち方、importer の更新運用**(スナップショットが Git 管理外になったため CronJob の役割を再設計): P2-2 の設計で扱う

## 改善要望(/improve で追加)
(ここに要望と対応状況を書く)

P1-6 独立レビューで出た軽微・任意の指摘(コードは未変更。次の engine タスクに合わせて対応を検討):
- `engine/golden_test.go`: `speciesCount` の下限アサート追加(現在は 0 だけ検査。少数種で再生成しても通ってしまう)
- `engine/golden_test.go`: `DamageInput` の json タグ明示または `DisallowUnknownFields`(フィールド改名でフィクスチャ値が黙ってゼロ値になる)
- `Makefile`: `go vet -tags golden` を lint に追加 / `golden-generate` は説明どおり `npm ci` を実行するか未導入で明示的に失敗させる
- `tools/golden/package.json`: `^0.10.0` を `0.10.0` に完全固定
- `engine/damage.go` `chainMods`: @smogon/calc はクランプ(41/410〜131072/2097152)を持つ。現在の補正集合では到達しないが、補正追加時に再確認
- ゴールデン未カバー: リフレクターとオーロラベールの同時成立、`Effectiveness` / `STAB` の直接照合(L1 では確認済み)
