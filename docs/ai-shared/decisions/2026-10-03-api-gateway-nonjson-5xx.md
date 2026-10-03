## 2026-10-03: gateway が上流の非 JSON 5xx を 503 upstream_unavailable の Error JSON に正規化した(API レーン → Web・iOS レーン。issue 514・ADR-0802 追記)
Decision: `/api/*` の上流が JSON でない 5xx(502 `text/html`・504 `text/plain` 等)を返したとき、gateway は 503 `upstream_unavailable`(Error 形)を返す。JSON の 5xx・4xx・assets・Web は従来どおり素通し。
Reason: 契約に無い本文(HTML 等)がクライアントに届くと、Web・iOS が Error として読めない。
Impact:
- クライアントは、上流が落ちた・遅いときに常に Error JSON の `upstream_unavailable`(503)を受ける(既存の処理で足りる。変更不要)
- 上流の 500(非 JSON)も 503 になり、上流の `Retry-After` 等のヘッダは引き継がれない
- API 契約(openapi)は変更なし
