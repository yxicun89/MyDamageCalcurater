# タイプバランスチェッカー 設計書(タイプバランスレーン)

- 更新日: 2026-10-01
- 状態: TB0〜TB6 すべて完了(main 統合済み)。メトリクス・GitOps(Argo CD、手動 sync)・recommendations の同時実行上限まで実装済み。
  設計の正は **`services/balance/api/openapi.yaml`(契約)**・この文書・`docs/adr/0014〜0018`・`0400〜0409`(balance の帯)
- ユーザーの仕様: `docs/plan.md` の「TB」と `docs/ai-shared/DECISIONS.md`(2026-09-21〜22)
- この文書は旧版(2026-09-21 の設計レビュー依頼文書)を、実装済みの現在の設計に書き換えたもの。役割分担・レビュー依頼・AI 間の共有ルール・
  旧版の未決事項の決着は履歴として `docs/adr/0410-type-balance-design-history.md` に移した(旧版の節番号との対応もそこ)

## 1. 目的

最大 6 体のパーティについて、防御のタイプ相性・攻撃範囲・特性による変化・仮想敵との相性を一覧にし、穴をふさぐおすすめタイプと
該当ポケモンまで返す。他のサイトを見に行かずに「次に何を足せばよいか」が分かることが狙い。
あわせて、ダメージ計算と同じ k8s クラスタ上で独立したサービスとして動かし、Argo CD による GitOps の運用経験を得る(学習目的)。

**総合点・ランキング・独自スコアは作らない**(TB1 の方針。以後も作っていない)。

## 2. 位置づけ

- ダメージ計算(damage-calc)・素早さ(speed)・判定(judge)と並ぶ独立サービス `services/balance/`(ADR-0012 のサービス境界)。
  他サービスの実行時 API に依存しない。engine も import しない(自前の純粋な Go のコア)。DB を持たない。
- マスタ(ポケモンのタイプ・技・特性)は pokedex の export を **read model(JSON)として起動時に 1 回だけ読む**(§7)。
- 画面は Web の「タイプバランス」タブ(`web/src/screens/BalanceScreen.tsx`、ADR-0303)。iOS の画面は第1〜3段を実装済み(シミュレータ確認は未実施。§11・ADR-0415)。
- 認証なし。端末 ID・セッション ID のヘッダを必須とする(§5)。

## 3. 構成

```text
services/balance/
├─ api/openapi.yaml        # balance 自身の API 契約(ルートの api/openapi.yaml とは別。speed・judge と同じ)
├─ cmd/api/                # 起動・環境変数の読み込み・graceful shutdown
├─ cmd/checkreadmodel/     # export の read model をサービスと同じ loader で検証するツール(ADR-0402・0403)
├─ internal/api/           # oapi-codegen の生成物(手で編集しない。make balance-gen)
├─ internal/balance/       # 純粋な Go のコア(I/O なし。相性・集計・おすすめ・技範囲)
├─ internal/httpapi/       # Echo の HTTP アダプタ(検証・判定順・エラー変換・同時実行上限)
├─ internal/httpmetrics/   # GET /metrics(他サービスと同一の複製。ADR-0406 §2)
├─ internal/master/        # read model と同梱の相性表の loader(検証に失敗したら起動しない)
├─ schema/                 # read model の JSON Schema(pokedex export 向け。ADR-0402)
├─ testdata/               # 架空データの example
├─ scripts/                # smoke(架空データ版・実データ版)
└─ deploy/
   ├─ k8s/base                     # Deployment(replicas 1)・Service(Ingress は無い。gateway が転送)
   ├─ k8s/overlays/local           # 架空データの read model(ConfigMap)
   ├─ k8s/overlays/local-readmodel # pokedex export の実データ(ADR-0403)
   ├─ k8s/overlays/gitops          # digest 固定(ADR-0018)
   ├─ argocd/                      # Argo CD Application pokecalc-balance
   └─ local-registry/              # クラスタ内レジストリ(PVC。ADR-0408)
```

- 計算コアは HTTP・k8s から分離している。ハンドラにタイプ計算を書かない。単体テストは k8s を起動せずコアを直接検証する。
- balance は Ingress を持たない。クライアントは gateway(`/api/balance/*`)経由で届き、gateway が Service `balance` へ転送する
  (`GATEWAY_BALANCE_URL=http://balance`。issue #284・ADR-0414)。端末ID・セッションIDの検証は gateway でも掛かる。
- リソースは requests 10m/16Mi・limits 100m/64Mi、`GOMEMLIMIT=56MiB`(§8)。HPA は付けていない(軽量 API のため。必要になってから)。

## 4. 計算の考え方(`internal/balance`)

- **倍率は整数**(基準 4 = 等倍。0・1・2・4・8・16)。float は使わない。特性込みの倍率は**既約分数**で持つ(×3/4・×5/4 など。ADR-0017)。
- 分類: ×4 弱点 / ×2 弱点 / ×1 等倍 / ×1/2 耐性 / ×1/4 耐性 / ×0 無効。倍率の `source` は `type`(タイプ由来)か `ability`(特性由来)を区別する。
- 相性表は**コードに持たない**。ダメージ計算レーンの `testdata/golden/typechart.json` の複製(`internal/master/data/typechart.json`。
  `make balance-sync-typechart`)から読む(ADR-0013・0015)。
- 特性は正規化された効果データ(無効・吸収・特定タイプの倍率変更・効果抜群の軽減)を介して計算する。ID の巨大な switch は作らない(ADR-0017)。

| 機能 | 内容 | ADR |
|---|---|---|
| analyze(防御) | メンバーごとの 18 攻撃タイプへの倍率と、攻撃タイプごとのチーム集計(弱点数・4 倍弱点数〈弱点の内数〉・耐性数・無効数・等倍数)。メンバーに任意の特性 | 0014・0017 |
| coverage(攻撃範囲) | 変化技を除く技のタイプから、18 の単防御タイプへの最大倍率。**有効打 = 等倍以上**。チームの集計はメンバー数で数える。防御側は単タイプ | 0016 |
| threats(仮想敵) | 仮想敵(最大 6 体・技最大 4・特性任意)ごとに、各メンバーが受ける最大倍率と与える最大倍率、安全に受けられる人数。タイプ相性と特性だけを見る(STAB・持ち物・威力・場の効果は見ない) | 0400 |
| recommendations | 防御の穴(耐性も無効も持たない攻撃タイプ)と攻撃範囲の穴をふさげるタイプ集合(18 単 + 153 複合)の候補と、そのタイプを持つ使用可能なポケモン全員。特性で穴をふさげるポケモンは別枠 | 0401 |
| move-range | 技構成(技 ID 1〜4 件のみ)の攻撃範囲と、それを半減以下で受けられる実在ポケモン(タイプだけ・特性ありは別枠) | 0404 |

## 5. API(契約の正は `services/balance/api/openapi.yaml`)

すべて JSON の POST。`X-Device-Id`・`X-Session-Id` を必須とする。`GET /healthz`・`GET /api/balance/healthz`・`GET /metrics` はヘッダ不要。

| path | operationId |
|---|---|
| `POST /api/balance/v1/team-balance/analyze` | analyzeTeamBalance |
| `POST /api/balance/v1/team-balance/coverage` | analyzeTeamCoverage |
| `POST /api/balance/v1/team-balance/threats` | analyzeTeamThreats |
| `POST /api/balance/v1/team-balance/recommendations` | recommendTeamTypes |
| `POST /api/balance/v1/move-range/analyze` | analyzeMoveRange |

- request は **`pokemonId`(と技 ID・特性 ID)だけ**を送り、タイプは balance が read model から引く(クライアントごとのタイプの食い違いを防ぐ。ADR-0014 §1)。
- 判定順: ヘッダ(400)→ 本文(400 / 413。本文は 16 KiB まで)→ read model 未設定(503)→ 未知の ID(422)→ 200。それ以外は 500。
- エラー形式は `{code, message}`。code は `missing_header`・`invalid_header`・`invalid_request`・`request_too_large`・`unknown_pokemon`・
  `unknown_move`・`unknown_ability`・`master_unavailable`・`overloaded`・`internal_error`。
- 配列は map ではなく**安定した順序**(メンバーは request の順、タイプは正準順 normal … fairy)で返す。
- 上限: メンバー 1〜6、メンバーの技 4、特性は abilityId 1 つ(`abilityIds` は read model 側で最大 4)、move-range の技 1〜4、recommendations の `limit` は既定 10・最大 20。

## 6. 段階(2026-10-01 時点で TB0〜TB6 すべて完了)

| 段階 | 内容 | ADR |
|---|---|---|
| TB0 | 基盤(型・相性コア・HTTP・Docker・Kustomize・Argo CD・単体テスト)。Argo CD の実同期も k3d で確認済み | 0018 |
| TB1 | 防御タイプバランス(最大 6 体 × 18 タイプ・チーム集計)。相性表は P1-13 のデータから(TB1b) | 0014・0015 |
| TB2 | 攻撃範囲 | 0016 |
| TB3 | 特性による防御相性の変化 | 0017 |
| TB4 | 仮想敵診断 | 0400 |
| TB5 | おすすめタイプと該当ポケモン(使用可能な全員。日本語名付き) | 0401 |
| TB6 | 技範囲チェッカー | 0404 |
| 整備・運用 | read model の JSON Schema・実データの配線・Argo CD 導入物の固定・AppProject とレジストリ永続化・メトリクス・recommendations のメモリ削減と同時実行上限 | 0402・0403・0405・0406・0408・0409 |

## 7. データ(read model)

- 3 つの JSON を**起動時に 1 回だけ**読む: ポケモン(タイプ・日本語名・特性の候補。`BALANCE_POKEMON_TYPES_PATH`)、
  技(タイプ・分類。`BALANCE_MOVES_PATH`)、特性の効果(`BALANCE_ABILITIES_PATH`)。形式は `schema/` の JSON Schema が正(ADR-0402)。
  不正なファイルは起動時エラーで終了する(黙って一部だけ使わない)。
- 環境変数が未設定・空の機能は、その機能の API が 503 `master_unavailable` を返す(health は 200)。特性は、特性を指定したときだけ必要になる。
- 本番相当は pokedex の export(`make pokedex-export`)の出力を ConfigMap にして読ませる(`make balance-k3d-deploy-readmodel`。ADR-0403)。
  ConfigMap は実データのため Git に置かない。Git に置くのは schema と**架空データの example**(ID 9001-000 以降)だけ(ADR-0002)。
- 外部サイト・外部 API には、計算のたびにも起動時にもアクセスしない。使用可能なポケモンの範囲(レギュレーション)は export 側のデータで決まり、balance は直書きしない。

## 8. 運用(メモリ・同時実行・メトリクス)

- **recommendations の同時実行上限**: 同時に計算してよい数を `BALANCE_MAX_CONCURRENT_RECOMMENDATIONS`(正の整数。既定 4)で制限する。
  枠が無ければ**待たせず**即 503 `overloaded` と `Retry-After: 1` を返す。入力検証のエラーや他のエンドポイント・`/healthz`・`/metrics` は満杯でも応答する。
  1 リクエストのメモリも削減した(1,500 件のカタログで約 3.1 MB → 約 0.53 MB)。`GOMEMLIMIT=56MiB` と合わせて、同時 30 でも OOMKill しない(ADR-0409)。
  他のエンドポイントへの上限は、問題が出てから広げる(別 issue)。
- **メトリクス**: `GET /metrics`(Prometheus 形式。`http_requests_total`・`http_request_duration_seconds`)。
  ServiceMonitor は `deploy/k8s/base/observability/servicemonitors/balance.yaml`(ADR-0406)。SLO・ダッシュボードは calc(ADR-0407)と balance(ADR-0420)。

## 9. GitOps(Argo CD)

- Git 上の Kustomize(`services/balance/deploy/k8s/overlays/gitops`。イメージは digest 固定)を、Argo CD の Application `pokecalc-balance` が見る。
  Application は AppProject `pokecalc` に限定されている(ADR-0408)。
- **同期は手動 sync**。`automated`(自動 sync・prune・selfHeal)は意図的に無効で、共通スクリプト `scripts/gitops/check-gitops.sh` が有効化を検出して失敗する(ADR-0408 §4)。
  main へのマージで差分は検知されるが、反映は人が sync する。
- 共有の Argo CD は導入物をコミット SHA・SHA-256・digest で固定して入れる(`scripts/argocd-bootstrap.sh`。ADR-0405)。
  クラスタ内レジストリは PVC(`local-path`・2Gi)で、push 済みイメージが Pod の再作成で消えない(ADR-0408)。
- 手順は `docs/runbooks/balance.md`。ローカル k3d での実同期(Git 変更 → manual sync → Pod の image digest 一致)は確認済み(ADR-0018)。
- manifest は Kustomize に統一(DECISIONS 2026-09-21)。Application はサービス単位(`pokecalc-balance`・`pokecalc-speed`)で分け、judge・calc 系は未対応(§11)。

## 10. UI(Web)

- 画面: `web/src/screens/BalanceScreen.tsx`(ADR-0303)。最大 6 体のメンバー、18 タイプの防御相性表、チーム集計、おすすめ、技範囲。
- 倍率は**色だけで表現しない**。書式は「×4 弱点」「×2 弱点」「×1 等倍」「×1/2 耐性」「×1/4 耐性」「×0 無効」(`web/src/domain/balanceLabels.ts`)。
- 画像は必須にしない。無ければタイプ色のエンブレムで成立させる。常時動くアニメーションは入れない。

## 11. 未対応

| 項目 | 状態 |
|---|---|
| iOS のタイプバランス画面 | 第1段(防御相性・集計)+第2段(攻撃範囲)は ADR-0415 / P6-21、第3段(仮想敵・おすすめタイプ・技範囲チェッカー)は ADR-0415 §8 / P6-22 で実装済み(`swift test`・simulator ビルドのみ確認。**シミュレータ・実機での見た目の確認は未実施**) |
| gateway 経由への一本化 | balance は完了(直結 Ingress を撤去し `GATEWAY_BALANCE_URL` を配線。ADR-0414)。speed・judge の直結 Ingress は各レーンで残り |
| 自動 sync・prune・selfHeal | 意図的に無効。有効化は未決 |
| ApplicationSet・App-of-Apps・judge/calc 系の Application | なし |
| クラウドへのデプロイ(EKS / GKE の選択・クラウドのレジストリ・実データの GitOps 配布) | 未決(クラウド公開はしない方針。ADR-0210) |
| recommendations 以外の同時実行上限・HPA | なし(問題が出てから) |
| balance の SLO・アラート | SLO(p99 < 500ms・可用性)とダッシュボードは ADR-0420 で追加済み(実クラスタ確認は未実施)。アラートは作らない(ADR-0407 §2) |
| 実データ・永続化(パーティ保存・お気に入り) | balance は DB を持たない。保存は team-svc の責務 |
| 他のレーンに依存する部分 | レギュレーションの使用可能集合や日本語名は pokedex export の内容で決まる(データレーン) |

## 12. 関連

- テストは [type-balance-test-strategy.md](type-balance-test-strategy.md)、サービスの README は `services/balance/README.md`、手順は `docs/runbooks/balance.md`。
- 履歴(旧版の役割分担・レビュー依頼・AI 間の共有ルール・未決事項の決着): [ADR-0410](adr/0410-type-balance-design-history.md)。
