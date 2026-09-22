# ADR-0700: 判定(素早さ×ダメージ連動)JD0 基盤

- 状態: 採用(2026-09-22)
- 日付: 2026-09-22
- 関連: docs/judge-design.md、ADR-0012(サービス境界)、ADR-0204(pokedex の内部 API とマスタの取得)、ADR-0600(speed SP0 基盤。サービスの骨格の前例)、
  ADR-0602(同速を真偽で丸めない前例)、ADR-0010(表示%の丸め。`displayChancePercent` の意味)

## 背景

ユーザー要望(DECISIONS.md 2026-09-22「判定レーンを新設」)で、「ニトチャ+メイン技で素早さ抜ける+そのポケモンを倒せるか」を
1回の入力で判定する 4 つ目の独立サービス `services/judge/` を作る。JD0 では、以後の段階(JD1 の判定 API・JD2 の複数候補や場の効果)が
乗る基盤として、サービスの骨格・上流サービスへの HTTP クライアント・最小の API を決める。判定そのもの(`outspeeds` / `ko`)は JD1。

judge は balance / speed と違って **read model(JSON ファイル)を持たない**。判定に必要な種族値もダメージも、すでに動いている
pokedex-svc・calc-svc の公開 API が持っているため、リクエストごとにそれを呼ぶ(judge-design.md §2)。したがって JD0 で一番決めるべきものは
「上流をどう呼び、上流の失敗をどう自分の言葉に直すか」になる。

## 決定

### 1. サービスの骨格(speed / balance に倣う)

- Go モジュール `example.com/pokecalc/services/judge`(独立モジュール。`go.work` に `use ./services/judge`)。
  engine へは `replace example.com/pokecalc/engine => ../../engine` で依存し、`GOWORK=off` でもビルド・テストできるようにする(ADR-0600 §2 と同じ)。
- ディレクトリ: `api/openapi.yaml`(judge 自身の契約)/ `cmd/api`(起動・環境変数を 1 か所で読む・graceful shutdown)/
  `internal/judge`(純粋なコア。I/O なし。JD1 で中身が入る)/ `internal/client`(pokedex-svc・calc-svc への HTTP クライアント)/
  `internal/httpapi`(Echo v5 のアダプタ)/ `internal/api`(oapi-codegen の生成物。`make judge-gen`)。
- Makefile は `services/judge/Makefile`(ターゲットは `judge-` 接頭辞)。ルートの `test` / `lint` / `build` に前提条件として含める(speed と同じ)。
- Ingress は `/api/judge` prefix の judge 自身の Ingress(`deploy/k8s/base/ingress.yaml`)。gateway は変更しない(下の §6-3)。
- 設定は環境変数(CLAUDE.md の技術規約): `PORT`(既定 8080)・`JUDGE_POKEDEX_BASE_URL`・`JUDGE_CALC_BASE_URL`・`JUDGE_UPSTREAM_TIMEOUT`(既定 3 秒)。
  上流の URL が未設定なら**起動はする**(ヘルスは 200、判定の API は 503)。設定されているのに不正(scheme・ホスト・タイムアウト)なら**起動を失敗させる**
  (speed の read model と同じ立場。ADR-0600 §4)。
- **judge は engine を直接呼ぶ**(素早さの実数値。JD1 で使う)。ダメージの式は呼ばない(calc-svc の `POST /api/calc` の結果をそのまま使う)。
  計算式を judge に複製しない、という点は speed(ADR-0600 §3)と同じ立場。

### 2. 上流の呼び方(judge が新しく決めること)

- 呼ぶのは **公開 API** だけ: `GET {POKEDEX}/api/pokedex/species/{key}`(種族値)と `POST {CALC}/api/calc`(ダメージ・確定数)。
  pokedex-svc の内部 API(`/internal/pokedex/master`。ADR-0204)は使わない。judge が要るのは 1 リクエストにつき数件の種族であって、マスタ一式ではない。
- **リクエストごとに呼ぶ**。calc-svc の `master.HTTPSource`(起動時に 1 回だけ取得し、指数バックオフで待つ)は流用しない。
  judge の上流呼び出しは 1 回のクライアント要求の中にあり、起動をブロックする無限リトライは不適切なため。
- 1 回の呼び出しは **request スコープの `context`** と、設定のタイムアウト(既定 3 秒)のうち早い方で打ち切る。JD0 ではリトライしない
  (上流は同じクラスタ内で、失敗の大半は再試行しても直らない。リトライが要ると分かった段階で別の ADR で足す)。
- クライアントは `X-Device-Id` / `X-Session-Id` を**呼び出し元のものをそのまま転送する**(CLAUDE.md の技術規約。judge が自分の ID を作らない)。
- 応答の本文には上限(1 MiB)を設ける。judge が読むのは 1 種族の詳細か 1 件の計算結果だけで、上流の異常時に無制限に読み込まないため
  (calc-svc の `readAllLimited` と同じ考え方)。

### 3. 上流のエラーの正規化(`internal/client`)

上流の事情(HTTP のステータス・接続エラーの文面・URL)を judge の外に漏らさず、`errors.Is` で判別できる 4 つの番兵エラーに畳む。

| 上流で起きたこと | judge のエラー | JD1 での HTTP 応答(予定) |
|---|---|---|
| 接続できない・タイムアウト・5xx | `ErrUpstreamUnavailable` | 503 `upstream_unavailable` |
| 200 だが本文が契約に合わない(壊れた JSON・必須欄の欠落・上限超過) | `ErrUpstreamInvalidResponse` | 503 `upstream_unavailable` |
| 404(種族が無い) | `ErrNotFound` | 422(JD1 で決める) |
| 400(上流が要求を受け付けない)、および judge 自身の事前検査(端末 ID・セッション ID が空) | `ErrInvalidRequest` | 400 `invalid_request` |

- 端末 ID・セッション ID が空の要求は、上流に届いてから 400 で跳ね返されるのを待たず、**上流を呼ぶ前に** `ErrInvalidRequest` で止める
  (無駄な往復をせず、原因も分かりやすい)。どちらも呼び出し側から見れば「要求が契約に合わない」で同じ扱いになるため、番兵は分けない。

- タイムアウトも `ErrUpstreamUnavailable` に畳む。呼び出し側から見れば「上流が使えない」で同じ扱いになり、判定の分岐が増えないため。
  原因を調べられるよう、元のエラー(`context.DeadlineExceeded` 等)は `%w` で包んで残す。
- エラーの文面に上流の URL・本文・ステータス行をそのまま入れない(ログには残す。ADR-0600 §5 の「500 は固定文言」と同じ立場)。
- コンストラクタは base URL(http/https の絶対 URL・ホストがある・クエリを含まない)とタイムアウト(正)を検証し、不正なら起動を失敗させる
  (`NewHTTPSource` と同じ検証。設定ミスを起動時に気づけるようにする)。

### 4. 上流の DTO は judge が自分で持つ

`services/internal/api`(共通の生成物)を judge から import せず、`internal/client` に**judge が読む欄だけ**の小さな型を置く。

- 読むのは種族値 6 つ・タイプ・ダメージ幅(`minDamage` / `maxDamage` / `defenderHP`)・`ko` だけ。送るのはルートの `CalcRequest` の
  必須欄(`format` / `attacker` / `defender` / `moveId`)。
- 理由: judge は薄いオーケストレーション層で、上の欄しか使わない。共通マスタの型一式に依存すると、
  モジュールをまたぐ結合が増え、`GOWORK=off` でのビルドに replace が 1 つ増える。
- 代わりに、**欄の意味の正はルートの `api/openapi.yaml`**(`SpeciesDetail`・`CalcRequest`・`KOChance`)であることを型のコメントに書き、
  必須欄が欠けた応答は `ErrUpstreamInvalidResponse` にして黙って 0 を使わない。

### 5. API(JD0)

- `GET /healthz`・`GET /api/judge/healthz` → 200 `{"status":"ok"}`。**上流の設定・疎通に依存しない**(ヘッダーも不要)。
  上流が落ちている間も pod 自身は生きており、readiness を落とすと judge まで巻き込んで落ちるため。上流の疎通は JD1 の判定 API が 503 で表す。
- `Error` / `ErrorCode`(`invalid_request` / `upstream_unavailable` / `internal_error`)は JD0 の契約に入れる。
  §3 の対応表で JD0 が決めたものなので、JD1 は paths を足すだけで済む。
- **JD1 の `POST /api/judge/v1/outspeed-and-ko` は JD0 の契約に入れない**。request の形(相手の指定・`ranks` の扱い・`field`)は JD1 で確定する項目で、
  先に器だけ置くと「契約にあるのに 404」になる。ADR-0600 §5 の「表(SP1)・自分の位置(SP2)の endpoint は各段階の ADR で足す」と同じ扱い。

### 6. judge-design.md §4 の未決事項の決定

1. **同速の扱い**: `outspeeds`(自分が相手より**厳密に**速いか)と `speedTie`(実数値が同じか)を**別のフィールドで返す**。
   根拠: 同速は「抜けている」でも「抜けられている」でもなく、真偽値 1 つに丸めると画面で区別できない。ADR-0602 の `tie`(同速の段を空配列で表し、
   真偽値に丸めない)と同じ立場をとる。
2. **相手の技を含めるか**: JD1 は**自分が攻撃する側だけ**を扱う(plan.md の JD1「自分と相手の Individual・使う技(単数)」の通り)。
   根拠: 返り討ち判定は相手の技構成(複数)と行動選択の仮定が要り、JD1 の「1回の入力」を大きくする。JD2 以降で扱う。
3. **gateway 経由か**: judge も `/api/judge` prefix の**自分の Ingress** を持つ(balance・speed と同じ)。
   根拠: gateway への追加は API レーンへの依頼になり、判定レーンが待たされる。前例が 2 つあり、クライアントから見た形も変わらない。
4. **ADR の帯**: `0700〜`(COORDINATION.md に登録済み)。JD0 はこの 0700。
5. **技の追加効果によるランク変化**: JD1 は request の `Individual.ranks`(ルートの `api/openapi.yaml` の既存の型)を**そのまま使う**。
   呼び出し側(将来の Web/iOS)が「技を撃った後のランク」を指定する(judge-design.md §3 JD1 の選択肢(a))。
   根拠: engine の `Move`、pokedex-svc・calc-svc の公開 API のいずれにも「技の追加効果(使用者自身のランク変化)」を表すデータが無いことを確認した。
   データ駆動で持たせるにはマスタの新しい欄が要り、データレーンへの新規依頼になる。判定レーンはそれを待たずに JD1 を出せる形を選ぶ。
   「技 ID から自動でランク変化を出す」は JD2 以降の課題とし、マスタへの追加は DECISIONS.md にデータレーンへの提案として記録する(今回は提案の追記のみ)。
   確率的な追加効果(命中率 100% でない追加効果)の扱いも、自動検出を入れるときに合わせて決める(JD1 は「指定されたランクで計算する」だけなので問題にならない)。

## 受け入れ条件(JD0)

1. `GET /healthz` と `GET /api/judge/healthz` が、上流の設定の有無によらず 200 `{"status":"ok"}` を返す(ヘッダー不要)。
2. `internal/client` のコンストラクタが、http/https でない・ホストが無い・クエリ付きの base URL と、0 以下のタイムアウトを拒否する。
3. pokedex クライアントが `GET {base}/api/pokedex/species/{key}` を呼び、200 の本文から種族値 6 つとタイプを取り出す。
   `X-Device-Id` / `X-Session-Id` は呼び出し元のものをそのまま転送する。
4. calc クライアントが `POST {base}/api/calc` に CalcRequest を送り、200 の本文から `ko`(hits・guaranteed・displayChancePercent)を取り出す。ヘッダーの転送は同じ。
5. 接続不能・タイムアウト・5xx は `ErrUpstreamUnavailable` に正規化され、上流の URL・本文・ステータス行を文面に含めない。
6. 200 だが本文が契約に合わない(壊れた JSON・必須欄の欠落・上限超過)は `ErrUpstreamInvalidResponse`、404 は `ErrNotFound`、400 は `ErrInvalidRequest`。
7. 1 回の呼び出しが設定のタイムアウトを超えて待たない(遅い上流に対し、所定時間内にエラーが返る)。
8. `make judge-test` / `judge-lint` / `judge-build` があり、ルートの `test` / `lint` / `build` の前提条件に入る。`go.work` に `./services/judge` がある。

## テストの期待値

- 上流はすべて `httptest.Server` の架空の応答で確かめる(実マスタ・実データを使わない。CLAUDE.md のドメイン規約)。
  種族は架空の `9001-000`、技は架空の ID を使う。
- タイムアウトは「上流が待たせる時間 > クライアントのタイムアウト」で確かめ、実時間の待ちは 100ms 程度に抑える。
- judge は計算式を持たないので、ゴールデンテスト(`make test-golden`)の対象は増えない。JD1 で `outspeeds` を出すときも、
  実数値は engine の式をそのまま使うため、judge 側で期待値を手計算するのは比較の向き(`>` か `>=` か)だけになる。

## 却下した案

- **pokedex-svc の内部 API でマスタ一式を起動時に取る**(calc-svc の `HTTPSource` と同じ形): judge が要るのは 1 リクエストにつき数件の種族だけで、
  マスタ一式を持つとメモリと鮮度の管理(再取得)まで背負う。read model を持たないという judge-design.md §2 の前提とも合わない。
- **calc-svc を呼ばず engine のダメージ計算を直接呼ぶ**: engine を直接呼ぶにはマスタ(種族・技・持ち物・特性・相性表)一式が要り、上と同じ問題になる。
  `KOChance` の意味(ADR-0006・ADR-0010)を judge が再実装する危険もある。
- **speed-svc を呼んで素早さを得る**: SP2(自分の実数値)は main にあるが表の位置を返す API で、judge が欲しい「2 体の実数値の比較」ではない。
  実数値は engine の `RealStats` を直接呼ぶ方が短く、素早さレーンの進み方にも縛られない(judge-design.md の却下案と同じ結論)。
- **上流のエラーをそのまま透過する**(ステータスと本文を中継する): どのサービスのどの呼び出しが失敗したかが client に漏れ、
  judge の契約が上流の契約に引きずられる。§3 の 4 つに畳む。
- **JD0 の契約に JD1 の endpoint の器を置く**: §5 の通り「契約にあるのに 404」を作らない。
- **ルートの `api/openapi.yaml` に judge を足す**: 独立サービスの契約はサービス内に置く(ADR-0012・balance・speed と同じ)。
