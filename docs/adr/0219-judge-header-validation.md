# ADR-0219: judge の端末ID・セッションID検証を gateway と揃える(issue #236 の judge 分)

- 状態: 採用(2026-10-02。API レーンが判定レーンの範囲を実施)
- 日付: 2026-10-02
- 関連: ADR-0202 §4(gateway の検証)、ADR-0606(speed。同じ変更の先行)、ADR-0217(エラーの形と code 語彙。PR #472)、
  ADR-0700 §2(judge が ID を calc-svc・pokedex-svc へ転送する)、ADR-0701 §5(judge の判定順)、issue #236、
  balance 分は PR #458(ADR-0413。タイプバランスレーン)に委ねる

## 背景
Traefik は `/api/judge` を gateway を経由せず直接届けるため(ADR-0700 §4-3)、gateway の `checkAPIHeaders`(各1つ・非空・
正準 UUID)が効かない。judge は非空のみ(`invalid_request`)で、受けた値をそのまま calc-svc・pokedex-svc へ転送するため、
非 UUID の ID が上流まで届いていた。speed は ADR-0606 で揃え済み。balance は PR #458 / ADR-0413 が別に扱う
(`missing_request_context` の扱いもそちらの決定に従う。ADR-0217 の対応表の balance セルは本 ADR では触らない)。

## 決定
1. **判定を gateway・speed と同じにする**: 欠落・空 → 400 `missing_header`、正準形 8-4-4-4-12 の UUID でない・同名ヘッダの
   重複 → 400 `invalid_header`、両方あれば `missing_header` 優先。大文字の UUID は通し、そのまま上流へ転送する。
   ヘッダ検査は body・上流呼び出しより先(非 UUID は calc へ転送されない)。healthz のヘッダ免除は既存のまま。
2. **実装は複製方式**(`services/judge/internal/httpapi/requestctx.go`)。ADR-0606 が API レーンとの合意で `httpmetrics` と
   同じ複製方式を選んでおり、speed と同じ関数を同じ形で置くのが最小。戻り値が各サービスの生成型 `api.Error` に依存するため
   共通化しにくい。同期は `requestctx_test.go` のテーブル(gateway の `headers_test` と同じケース)が守る。
3. **契約**: judge `Error.code` に `missing_header`・`invalid_header` を追加(version 0.2.0)。`invalid_request` は body 不正・
   calc が受け付けなかった場合に残し、ヘッダ起因では返さない。これは ADR-0217 の段階手順(1)追加 →(2)クライアント対応 →
   (3)サーバーが新 code を返す を**同時に行う意図的な逸脱**。理由: クライアントは同じリポジトリの Web だけで、iOS は judge を
   呼ばず、表に無い code は Web で fallback になる。
4. **ADR-0217 の対応表**: judge の「ヘッダ欠落・空」は `missing_header`(400)、「ヘッダ不正・重複」は `invalid_header`(400)に
   なる(ADR-0217 は PR #472 のため本文は書き換えない)。
5. 実通信では net/http がヘッダ値の前後空白を除くため、空白だけの値は `missing_header` になる(httptest で直接組むと
   `invalid_header`。ADR-0606 §2 の注と同じ)。

## クライアント
`judgeErrorText`(`web/src/i18n/ja.ts`)に新 code の文言を追加(additive)。`web/src/judge/judge.gen.ts` を再生成した。

## 影響
judge を直接叩く外部ツールは正準形 UUID 以外のヘッダー値を使えなくなる。judge の openapi は #457 とも触るため、取り込み時の
衝突は取り込み側で解消する。
