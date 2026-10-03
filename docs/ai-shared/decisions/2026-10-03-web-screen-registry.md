## 2026-10-03: Web の画面・タブは登録ファイルで足す(画面レジストリ。Web レーン → 全レーン。ADR-0323)
Decision: Web に画面・タブを足すときは、自分のレーンのディレクトリに登録ファイル `<id>.screen.tsx`(`defineScreen({ id, segment, label, order, usesMaster, createClient, render })` の既定エクスポート)を置くだけにした。
`web/src/app/screens.tsx` が `import.meta.glob` で集めて order の昇順に並べる。クライアントは登録ファイルの `createClient`、Props の受け渡しは `render`。
レーン固有の文言は `web/src/i18n/<レーン>.ts`(既存の balance・speed・judge・team・adjust・about は移動済み。`ja.ts` は再エクスポートで互換を保つ)。
Reason: 各レーンが `App.tsx`・`app/screens.tsx`・`app/routes.ts`・`i18n/ja.ts` を毎回編集して PR が衝突していた(ユーザー決定 2026-10-03)。
Impact:
- **新しい画面**: 共有ファイルを触らない。order は既存の 100 刻み(計算 100 … 調整 700)の間か後ろの値。文言は `i18n/<レーン>.ts` を作って直接 import する(`ja.ts` に行を足さない)。
- **未マージの PR**(App.tsx 等を編集している Web の PR など。素早さ・判定の Web 分も同じ): `origin/main` を merge → 4ファイルの衝突は main 側を採る →
  自分の変更を登録ファイル(`render` で Props を組み立てる)・`i18n/<レーン>.ts` に移す → `npm run lint && npm run typecheck && npm test`。
  `calcScreenText` など計算・逆算の語は `ja.ts` のまま。手順の詳細は ADR-0323 §5。
- アプリ全体の値(全画面が使いうる値)を足すときだけ `app/screenDefinition.ts` の `ScreenEnvironment` と `App.tsx` を Web レーンが編集する。
