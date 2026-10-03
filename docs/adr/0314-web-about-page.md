# ADR-0314: 「このアプリについて」情報ページ(issue #328 の Web 分)

- 状態: 採用
- 日付: 2026-10-02
- レーン: Web
- 関連: issue #328、docs/ai-shared/DECISIONS.md 2026-09-26「P6-18」、ADR-0501「P6-18」(iOS)、ADR-0002(出典とライセンス)、
  ADR-0300 §1(URL とパスの連動)、ADR-0308(タブの入力の保持)

## 背景

issue #328 のユーザー決定(2026-09-25)は「非公開・私的利用のまま。LICENSE は置かない。アプリ内に第三者データの出典と
非公式の表示を入れる」。iOS が先に既定の文言を決め(DECISIONS.md 2026-09-26)、Web はそれに従う。

## 決定

1. **文言**: `web/src/i18n/ja.ts` の `aboutText` に1か所だけ持つ。非公式の注記は DECISIONS.md の確定文言と完全一致、
   データの出典は ADR-0002「責務の分離」表と同じ順の4件(`@smogon/calc`〈MIT License〉・Pokémon Showdown〈MIT License〉・
   PokeAPI・Pokémon HOME / Champions の公式情報)。ライセンスは ADR-0002 に書かれているものだけで、増減しない。
2. **入口**: アプリ下部の `<footer>`(main の外)に文字リンク「このアプリについて」(`href="/about"`)。タブ(`SCREEN_ROUTES`)には
   入れない(タブは計算の道具、これは補足情報のため。タブにするとロービング tabIndex・矢印キーの対象と
   `usesMaster` の表も増える)。フッターはマスタの読み込み中・失敗中も出す。
3. **ルート**: `/about`(ADR-0300 §1 と同じ History API。ライブラリなし)。リンクは pushState、戻る/進む(popstate)で往復、
   直接開ける。末尾 `/` 1つは同じ画面、深いパス・大文字小文字違いは未知のパスとして `/calc` に置き換える(既存の流儀)。
   `/about` はマスタ・engine を使わない(読み込み中・失敗中でも出す)。文書のタイトルは「このアプリについて | pokecalc」。
4. **画面の構造**: タブ列は出さない。他タブの画面は hidden で DOM に残す(ADR-0308。往復で入力を失わない)。
   見出しは h1 → h2(このアプリについて)→ h3(非公式表示・データの出典)。出典は `<ul>`(名前「データの出典」)。
   リンクで開いたら h2 にフォーカスを移す(SPA の画面遷移の作法)。「計算に戻る」は `/calc` へ pushState。
5. **a11y の検査**: axe(`@axe-core/playwright`。最新安定版を完全固定)で `/about`・フッター付きの `/calc`・ダークの `/about` を
   検査する。新しい依存なので、既存の `a11y.spec.ts` ではなく新規 `e2e/a11y-about.spec.ts` に置く。

## 検討した代替

- タブに入れる: 上記のとおり見送り(タブの意味を薄め、タブ列が7つになる)。
- モーダル/ダイアログ: URL・戻る/進むと連動せず、ADR-0300 §1 の流儀と合わない。
- 「計算に戻る」を直前の画面へ戻す: 「戻る」はブラウザの戻るが担う。ラベルと動作を一致させるため `/calc` 固定。

## 結果

実装済み。画面は `web/src/AboutScreen.tsx`、文言は `i18n/ja.ts` の `aboutText`、パス判定は `app/routes.ts` の
`isAboutPath`/`pathForAbout`/`aboutDocumentTitle`(SCREEN_ROUTES には入れていない)。App は `aboutOpen` の state を
持ち、タブ(tab)の state はそのまま保つ。他タブの画面は `<div hidden>` で DOM に残す。フォーカスの移動は
リンクで開いたときだけ(直接開く・popstate では移さない)。`@axe-core/playwright` は 4.13.0 で完全固定。
main の `min-height: 100vh` を外した(フッターが画面外へ押し出されないため)。
