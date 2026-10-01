# ADR-0217: エラーの形と code 語彙(404/405・panic・上流の非 JSON・ヘッダ欠落/内部エラーの対応表)

- 状態: 採用(2026-10-02。API レーンの判断。issue #325)
- 日付: 2026-10-02
- 関連: ADR-0200(calc の語彙。メソッド違いに新しい code を足さない)、ADR-0202(gateway のエラー)、
  ADR-0606(speed のヘッダ検証)、ADR-0701/0014(judge/balance の内部エラー)、issue #236(ヘッダ検証の code)

## 背景
全サービスのエラー本文は `{code, message}`(`code` 必須)のはずだが、judge・balance・speed は
未知ルート/メソッド違いで echo の既定(`{"message":"Not Found"}`、`code` 無し)を返し、panic は回復されず
接続が切れ、gateway は上流の非 JSON 5xx(text/html の 502 等)をそのまま通していた。ヘッダ欠落と内部エラーの
code 名もサービスごとに違う。

## 決定(今回実装したもの)
1. **未知ルート・メソッド違いは 404 `not_found`**(judge・balance・speed)。calc・gateway が既に 405 を
   404 `not_found` に畳んでいる(ADR-0200)ので同じにする。`method_not_allowed` は足さない。
   各契約の `ErrorCode` に `not_found` を追加した(enum への追加のみ。既存値は不変)。
2. **panic は 500 `internal_error` の JSON**(judge・balance・speed)。メトリクス middleware の内側に
   回復 middleware を置き(回復した 500 も数える)、回復した panic はエラーとして `writeHTTPError` に渡す。
   応答を書き出し済みなら何も足さない(calc・pokedex と同じ流れ)。固定文 `internal error` だけを返し、
   panic の値は `slog.Error` にだけ出す。`http.ErrAbortHandler` は回復せず再 panic する。
3. **gateway は `/api/*` の上流が JSON でない 5xx を返したら、503 `upstream_unavailable` に正規化する**
   (`application/json`・`*+json` 以外、Content-Type 無しを含む)。
   上流の JSON エラー(5xx も含む)・4xx・assets/Web 上流の応答は素通しのまま。
   正規化では**上流の元のステータス(例 500)を 503 に書き換え、上流のヘッダ(`Retry-After` 等)は引き継がない**。
   上流の本文も出さない。元のステータスと Content-Type は gateway のログ(WARN)にだけ残す。
   judge が「5xx・契約に合わない応答」を `upstream_unavailable` にするのと同じ流儀。

## 決定(コードは変えない。対応表と方針)
### 現状の対応表(各セルは実装のコードで確認)
| 失敗 | calc / gateway / pokedex | balance | speed | judge |
|---|---|---|---|---|
| ヘッダ欠落・空 | `missing_header`(400) | `missing_request_context`(400) | `missing_header`(400) | `invalid_request`(400) |
| ヘッダ不正・重複 | `invalid_header`(400) | `invalid_request`(400。重複のみ。UUID 検証なし) | `invalid_header`(400。UUID でない・重複) | `invalid_request`(400。重複のみ) |
| 内部エラー・panic | calc・gateway・pokedex は `internal`(500) | `internal_error` | `internal_error` | `internal_error` |
| ルート無し・メソッド違い | `not_found`(404) | `not_found`(404) | `not_found`(404) | `not_found`(404) |
| 依存するマスタ・DB が使えない | calc・pokedex は `master_unavailable`(503) | `master_unavailable`(503) | `master_unavailable`(503) | (judge は上流を呼ぶ。下の行) |
| 上流サービスに接続できない・5xx・非 JSON | gateway が `upstream_unavailable`(503) | (上流は pokedex。`master_unavailable`) | (read model。`master_unavailable`) | `upstream_unavailable`(503) |

`upstream_unavailable` は gateway と judge の code で、pokedex・calc・balance・speed は使わない(マスタが使えないときは
`master_unavailable`)。gateway の `upstream_unavailable` は「gateway が上流に届かない・上流が契約外の応答」、
`master_unavailable` は「そのサービス自身のデータが使えない」で意味が違う。

### クライアントの扱い(現状。Web・iOS の grep 結果)
- Web は code の値で分岐せず、サービスごとの文言表を引く(`web/src/i18n/ja.ts` の `balanceErrorText`・speed の
  `errorByCode`・`judgeErrorText`)。表に無い code は各表の `fallback` か message を出す。`not_found` は
  どの表にも無く fallback になる(通常のユーザー操作では起きない = ルート誤りまたは古いクライアント)。
- iOS は `PokeCalcError.Code.notFound = "not_found"` を使う(ルート API の species/move の 404)。
  balance・speed・judge は iOS から呼ばない。
- したがってクライアントは **code を新規に分岐に使わず、未知の code は fallback の文言にする**こと。
  同じ失敗の別名(`internal`/`internal_error`、`missing_header`/`missing_request_context`/`invalid_request`)は
  このファイルの表を正として同値に扱う。

### 将来の統一方針(未実施。実施するときは別 ADR とクライアントの同時修正)
- 目標語彙: ヘッダ欠落=`missing_header`、内部エラー=`internal_error`(ルートの `internal` は後方互換のため残す)。
- 手順: (1) 各契約に新 code を**追加**(別名の併用期間。サーバーは旧 code を返し続ける)→ (2) Web・iOS が新旧両方を
  受け付ける版を出す → (3) サーバーが新 code を返す → (4) 旧 code を契約から削除。(2)〜(3) の間は最低でも
  1リリース(クライアントを配る期間)を空ける。enum への追加は openapi-typescript・swift-openapi-generator の
  生成型を変えるため、`make gen`・`make ios-gen-check` を同じ PR で通す。
- 今回は既存クライアントへの影響があるため見送り(後続)。

## 範囲外
- Traefik(ingress)が直接返す 502/504 のテキスト本文はクラスタ側の設定(deploy/k8s)の問題で、アプリの
  コードでは変えられない。gateway を経由しない失敗なので、このADRでは扱わない。必要なら ingress の
  エラーページ/middleware を別 issue で検討する。
- ルート API とレーン API の版の方針(`/api/calc` と `/v1`)は本件の対象外。

## 影響
- Web の生成型 `web/src/api/balance.gen.ts`・`web/src/speed/speed.gen.ts`・`web/src/judge/judge.gen.ts` の `ErrorCode` に
  `not_found` が増える(分岐を作っていないので挙動は不変)。
- 上流の非 JSON 5xx は 503 に書き換わり、上流の `Retry-After` 等は落ちる(上記)。
