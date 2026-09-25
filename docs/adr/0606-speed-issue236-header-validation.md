# ADR-0606: 端末ID・セッションIDの検証を gateway と揃える(issue #236)

- 状態: 採用(2026-09-25。素早さレーンの判断。タイプバランスレーン・API レーンとの調整結果を踏襲)
- 日付: 2026-09-25
- 関連: ADR-0202 §4(gateway の検証。`services/gateway/internal/httpapi/headers.go`)、ADR-0600 §5(speed の既存の判定順)、
  `services/internal/httpmetrics`(サービスごとに複製する前例。cross-module import にしない)

## 背景
全体レビュー issue #236: `X-Device-Id`・`X-Session-Id` の検証基準とエラーの `code` が、gateway(正準 UUID・`missing_header`/
`invalid_header`)と balance・speed・judge(非空のみ・`missing_request_context`/`invalid_request`)で食い違っている。
Traefik が `/api/balance`・`/api/speed`・`/api/judge` を gateway を経由せず各サービスへ直接届けるため(ADR-0700 §4-3)、
gateway の検証はこの3サービスには効いていない。

タイプバランスレーン経由で API レーン(gateway の持ち主)に相談した結果、共通パッケージ(`services/internal/requestctx` 等)を
新設せず、`services/internal/httpmetrics` と同じ前例(cross-module import ではなく各サービスへの複製)に揃えることで合意した。

## 決定

### 1. gateway の判定をそのまま複製する
`services/speed/internal/httpapi/requestctx.go`(新設)に、`services/gateway/internal/httpapi/headers.go` の
`checkAPIHeaders`・`headerStatus`・`isCanonicalUUID`・`isHexDigit` を、gateway の `services/internal/api`(gateway 専用の
生成型)への依存を除いて移植する。判定基準(正準形 `8-4-4-4-12` の16進・欠落は `missingHeader`・不正/重複は `invalidHeader`と
いう2つの bool を返す形)は一字一句 gateway と同じにする。**gateway のコードは変更しない**(コピー元として読むだけ)。

### 2. エラーコードを追加する(契約の破壊的変更)
`services/speed/api/openapi.yaml` の `ErrorCode` に `missing_header`・`invalid_header` を追加する。
既存の `invalid_request` は、header 以外の理由(body の JSON 不正・クエリ不正など)にだけ使う。
判定順は変わらない(ヘッダー → body/クエリ → 503 → 422 → 200)が、ヘッダーの 400 の `code` が変わる:

| 状態 | 旧 | 新 |
|---|---|---|
| 欠落・空 | `invalid_request` | `missing_header` |
| UUID でない・重複 | `invalid_request` | `invalid_header` |

これは意図的な契約の破壊的変更(CLAUDE.md 絶対ルール6の「期待値の変更は理由をコミットメッセージに書く」に従う)。
speed の API は Web だけが呼んでおり(judge のように他サービスへ転送しない)、同一リポジトリ内なので影響範囲は把握できる。
Web レーンへの追従依頼は本 ADR の「他レーンへの依頼」に書く。

**注**: 旧実装は `strings.TrimSpace` で空白だけの値も「空」扱いにしていたが、gateway の `headerStatus` は
`""` だけを missing とし、空白だけの値(例: `"  "`)は「空でなく UUID でもない」ので `invalid_header` になる。
「一字一句 gateway と同じ」を優先し、speed でも `TrimSpace` はしない(空白だけの値は `invalid_header` に変わる)。
ただし実際の HTTP 通信では net/http がヘッダー値の前後の空白を受信時に取り除くため、この差は httptest 等で
`http.Header` を直接組み立てたときだけ見える。実運用で空白だけの値を送っても `""` になり `missing_header` を返す。

### 3. `requireRequestContext` ミドルウェアを置き換える
`services/speed/internal/httpapi/server.go` の `requireRequestContext` を、`strings.TrimSpace` による非空検査から
`checkAPIHeaders(c.Request().Header)` の呼び出しに置き換える。エラーは `missingHeader`/`invalidHeader` の bool から
`api.MissingHeader`/`api.InvalidHeader` を選んで返す。

### 4. 他レーンへの依頼
- Web レーンへ: `web/src/speed/speedClient.ts` が `code` 文字列に依存する箇所があれば確認してほしい(現状は
  `SpeedError.code` をそのまま透過するだけで、`invalid_request` を特別扱いしていないので追従は不要と見込む。DECISIONS.md に記録)。
- 変更範囲はこの ADR 内(`services/speed/`)に閉じる。gateway・balance・judge は変更しない(それぞれ自分のレーンで対応)。

## 却下した案
- `services/internal/requestctx` のような共通パッケージを新設する: API レーンとの相談の結果、`httpmetrics` の前例
  (複製方式)に揃えることで合意した。
- gateway の `services/internal/api` を speed から import する: speed は自分の契約(`services/speed/api/openapi.yaml`)を
  持つ独立サービスなので、gateway 専用の生成型に依存させない(ADR-0012)。
