# ADR-0802: エラー本文の形・語彙をサービス間で揃える(issue #325)

- 状態: 採用(2026-10-02。API レーンの判断。issue #325 の既定案どおり)
- 日付: 2026-10-02
- 関連: ADR-0200(calc の 404/405 は not_found)、ADR-0606(speed のヘッダー検証)、ADR-0700・ADR-0701 §6(judge)、
  issue #236(ヘッダー検証)。並行 PR #484(httpguard が 503 の `overloaded` / `upstream_unavailable` を返す)・
  PR #485(speed 契約 0.5.0)とは、enum への追記 1 行ずつだけが接する(下の「競合の扱い」)。

## 背景
Web・iOS はエラーを `{code, message}`(`code` 必須)で扱うが、judge・balance・speed は未知ルート・メソッド違いで echo の既定
(`{"message":"Not Found"}`、`code` なし)を返し、panic の回復も無く接続が切れて空応答になっていた。

## 決定

### 1. 未知ルート・メソッド違いは 404 `not_found`(judge・balance・speed)
calc-svc と同じ扱い(ADR-0200: メソッド違いに新しい code を足さず `not_found` に畳む)。3サービスの `ErrorCode` に `not_found` を足し、
`writeHTTPError` が echo の 404/405 を `{code:"not_found", message}`(HTTP 404)に整える。既存の値は変えない(後方互換)。

### 2. panic は 500 `internal_error` の JSON(judge・balance・speed)
`recoverMiddleware` を各サービスの `internal/httpapi` に置く(`httpmetrics` と同じく、共通パッケージを作らず複製する前例に従う)。
スタック・panic の値は本文に出さず、`slog` にだけ残す。

### 3. 既存 code の改名はしない(語彙の対応表をここに置く)
公開済みの code を変えると既存クライアントに影響するため、本 ADR では改名しない。クライアントは次の表で読む。

| 失敗 | calc / gateway | pokedex | balance | judge | speed |
|---|---|---|---|---|---|
| ヘッダー欠落・空 | `missing_header` | `missing_header`(内部 API はヘッダー不要) | `missing_request_context` | `invalid_request` | `missing_header` |
| ヘッダー不正・重複 | `invalid_header` | `invalid_header` | `invalid_request` | `invalid_request` | `invalid_header` |
| 想定外の内部エラー | `internal` | `internal`(ルートの契約) | `internal_error` | `internal_error` | `internal_error` |
| 未知ルート・メソッド違い | `not_found`(404) | `not_found`(404) | `not_found`(404) | `not_found`(404) | `not_found`(404) |

方針: 新規・変更するサービスは `missing_header` / `invalid_header` / `internal_error` に寄せる。calc・gateway の `internal` と、
balance の `missing_request_context`、judge のヘッダー系 `invalid_request` の統一は、Web・iOS のクライアントを同時に直す別 PR で行う
(クライアントは未知の code を `message` 付きの一般エラーとして扱うこと)。

### 4. 版の方針
ルートの API(`/api/calc` 等)はパスに版を持たず、レーンの API は `/v1` を付ける。後方互換の原則は「既存 code・既存フィールドを変えず、
enum への追加だけ行う」とする。破壊的変更は新しい `/vN` を足す。

### 競合の扱い
並行 PR #484 は `ErrorCode` に `overloaded` / `upstream_unavailable` を、#485 は speed の契約を 0.5.0 に上げる。本 PR は各 `ErrorCode` の
末尾に `not_found` を足すだけで、どちらも後から取り込める(enum の追記行の衝突は両方を残して解決する)。

## 限界(対象外)
- Traefik 直結の 502/504(gateway を経由しない、クラスタ側の応答)は本 ADR では直さない(Traefik の errors middleware が要る)。
  gateway を経由する上流の非 JSON 5xx は、下の「追記(issue 514)」で正規化した。
- 上の表の code の統一(§3)は未実施。

## 追記(issue 514): gateway が上流の非 JSON 5xx を 503 upstream_unavailable に正規化する
`/api/*` の上流(calc・pokedex・record・team・balance・speed・judge)が JSON でない 5xx(Traefik 等の `text/html` の 502、
`text/plain` の 504 など)を返したら、gateway が 503 `upstream_unavailable` の Error JSON に書き換える(ADR-0202 §7 の本文)。
- 判定は `newReverseProxy` の `apiUpstream`(`/api/*` の上流か)。`ModifyResponse` で、ステータスが 5xx かつ
  Content-Type が JSON(`application/json`・charset 付き・`application/*+json`)でないときだけ正規化する。
  JSON の 5xx・4xx(非 JSON を含む)・assets・Web は素通し。
- 本文に上流の本文を出さない。**上流のステータスが 500 でも 503 に書き換わる**。上流が付けた `Retry-After` 等のヘッダも引き継がない
  (正規化後の応答は gateway が作り直すため)。CORS は既存の処理で付く。
- 正規化時は専用の WARN「上流が JSON でない 5xx を返した」に上流のステータス・Content-Type を残す(端末 ID は出さない)。
- 範囲外: Traefik が gateway より前で返す 502/504(クラスタ側)。
