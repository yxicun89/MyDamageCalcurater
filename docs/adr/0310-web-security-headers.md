# ADR-0310: Web 配信のセキュリティヘッダ(issue #219)

- 状態: 採用(Web レーン、2026-10-01)
- 関連: issue #219、issue #148(公開前のアクセス境界。本件はヘッダの話で別)、ADR-0011 §6(WASM)、
  requirements.md §4(gateway が `/` と `/api` を同じオリジンで配る)

## 決定

1. `web/nginx.conf` の配信に CSP・`X-Frame-Options`・`Referrer-Policy`・`Permissions-Policy`・
   `X-Content-Type-Options` を付ける。ヘッダ本体は `web/security-headers.conf` に1か所だけ書き、各 location が
   `include` する(nginx は location に `add_header` が1つでもあると server の分を継承しないため)。
2. CSP は `default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self'; img-src 'self' data:;
   font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'`。
   `'unsafe-inline'`・`'unsafe-eval'` は許さない。ビルド成果物はインライン script/style を持たず、React の
   `style` 属性は CSSOM 経由で CSP の対象外。WASM の実体化にだけ `'wasm-unsafe-eval'` が要る。
3. `connect-src` は `'self'`。API は gateway と同じオリジン(既定。`VITE_API_BASE_URL` 未設定)。
   別オリジンの API を指す構成にするときは `security-headers.conf` の `connect-src` にそのオリジンを足す
   (nginx は静的設定でビルド時の環境変数を知らないため、自動では追従しない。ファイル先頭のコメントに明記)。
4. HSTS は TLS 終端(Tailscale・クラウド)の持ち物なので付けない。
5. gateway は上流ヘッダを素通しする(`ModifyResponse` が触るのは `Access-Control-*` だけ)ので、gateway 側の
   変更は無い。

## 検証

`web/e2e/container.spec.ts`(`make web-e2e-container`): `/`・`/calc`・`/engine.wasm`・`/wasm_exec.js`・
`/static/*.js` の各応答にヘッダが付くこと、CSP の下で画面と WASM 計算が動き CSP 違反・コンソールエラーが
出ないこと。
