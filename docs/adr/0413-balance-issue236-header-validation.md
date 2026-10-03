# ADR-0413: balance の端末ID・セッションIDの検証を gateway と揃える(issue #236)

- 状態: 採用(2026-10-02。タイプバランスレーンの判断。素早さレーンの ADR-0606 を踏襲)
- 日付: 2026-10-02
- 関連: ADR-0606(素早さ側の同じ変更。背景・方針の詳細はそちら)、ADR-0202 §4(gateway の検証。
  `services/gateway/internal/httpapi/headers.go`)、ADR-0016 §4・ADR-0400(balance の判定順)

## 背景
`X-Device-Id`・`X-Session-Id` の検証が、gateway(正準 UUID・`missing_header`/`invalid_header`)と balance(非空のみ・
`missing_request_context`)で食い違っていた。方針は ADR-0606 と同じ(共通パッケージを作らず、`httpmetrics` の前例どおり
サービスごとに複製)。

## 決定
1. `services/balance/internal/httpapi/requestctx.go` に gateway の `checkAPIHeaders`・`headerStatus`・`isCanonicalUUID`・
   `isHexDigit` を複製する(gateway は変更しない)。`requireRequestContext` はこれを呼ぶ。
2. `services/balance/api/openapi.yaml`(0.8.0)の `ErrorCode` に `missing_header`・`invalid_header` を追加し、
   **`missing_request_context` を廃止する**(今後どこからも返さない)。`invalid_request` は header 以外の不正だけに使う。
   `web/src/api/balance.gen.ts` を再生成した。
3. 判定順は不変(ヘッダー → body → 503 → 422 → 200)。ヘッダー内の優先は欠落・空 > 不正・重複。

| 状態 | 旧 | 新 |
|---|---|---|
| 欠落・空 | `missing_request_context` | `missing_header` |
| UUID でない値 | 通る | `invalid_header` |
| 同名ヘッダの重複 | `invalid_request`(生成バインドが返す) | `invalid_header` |

これは意図的な契約の破壊的変更。既存テストの期待値の変更はこの表の範囲だけ:
`missing_request_context` → `missing_header`、空白だけの値(httptest で `http.Header` を直接組み立てたとき。実通信では
net/http が前後の空白を取るので欠落になる)は `invalid_header`、重複ヘッダは `invalid_header`。テストの ID 値(`test-device` 等)は
正準 UUID に替えた(正常系の期待値は変えていない)。smoke スクリプトも同様。
Web(`balanceClient.ts`)は端末ID・セッションIDを正準 UUID で送っており変更不要。code に依存する日本語の写像
(`web/src/i18n/ja.ts` の `balanceErrorText`)は `missing_request_context` を `missing_header` に替え、`invalid_header` を足した。

## 影響
balance API を直接叩く外部ツールは正準 UUID 以外のヘッダー値を使えなくなる。
