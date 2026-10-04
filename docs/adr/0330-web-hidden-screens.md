# ADR-0330: Web の画面の非表示(hidden)と判定タブの非表示(F-07 / I-web-5)

- 状態: 採用
- 日付: 2026-10-04
- レーン: Web(ユーザー決定: 判定は目的を作り直すので Web のタブを非表示にする。サービス・コード・`web/src/judge/` と単体テストは残す)
- 関連: ADR-0323(画面レジストリ)、ADR-0300(URL で画面を切り替える。未知のパスは既定へ置き換え)、ADR-0308(訪問済みタブの保持)、
  ADR-0304 追記6(マスタ不要の画面)、ADR-0705(判定)、`docs/usability-round2.md` F-07

## 決定(既定案)

1. `defineScreen` の定義に省略可の `hidden?: boolean`(既定 `false`)を足す。`RegisteredScreen` は `hidden: boolean` を持つ。
2. `buildScreenRegistry` は hidden の画面も **登録・検証の対象**にする(id・segment・order の重複、segment の形、予約語を同じ検査。
   `SCREENS` には hidden も入る)。
3. `app/screens.tsx` に `visibleScreens(screens)`(hidden を除き、順序を保つ)を足す。
   - `SCREEN_ROUTES` は `visibleScreens(SCREENS)` から導く。よって (a) タブ列に出ない、(b) 矢印キー・Home/End の巡回(`TAB_ORDER`)の対象外、
     (c) `screenFromPath("/judge")` は null になり、ADR-0300 の未知のパスと同じく `replaceState` で既定の画面(計算)へ置き換わる
     (pushState しない・履歴を増やさない・popstate で戻っても同様)。判定のブックマークが残っていても壊れない。
   - `instantiateScreens` は hidden の画面の `createClient` を呼ばず、描画の口も返さない。(d) マウントしない=判定 API・素早さへの通信が起きない。
4. `web/src/judge/judge.screen.tsx` は `hidden: true` を1行足すだけ(これは判定レーンの持ち物への最小の追記として許可する。コードは残す)。
5. 再表示は `hidden: true` を外すだけで元に戻る(タブ・URL・巡回・マウントがすべて復活する。テストはダミー画面の `hidden: false` で確認)。

## 期待値を更新するテスト(仕様変更。弱めずに、判定を除いた期待に直す)

- `web/src/app/routes.test.ts`: ルート表の並びから判定を除く。`/judge`・`/judge/`・`/app/judge` は「画面」の行から「null(未知)」の行へ。
- `web/src/App.test.tsx`: Home/End の巡回(End → 調整 → お気に入り → 構築 → 素早さ → …)から判定を除く。
- `web/src/App.masterSources.test.tsx`: タブ一覧と `toHaveLength(8)` を判定なしの7に。
- `web/src/App.about.test.tsx`: タブ数 8 → 7。
- `web/src/App.tabPersistence.test.tsx`: タブ往復の操作から判定のクリックを除く。
- `web/src/App.onlineMasterScreens.test.tsx`: 「判定の画面も包まれている」は「判定のタブが出ない」に置き換え
  (判定の `OnlineMasterGate` の包みの検証は、非表示の間は無い=マウントされないため。**再表示するときに App レベルのテストを復活させる**。タイプバランスの包みは App のテストが担う)。
- `web/src/App.routing.test.tsx`: JD5「判定のタブ」2件(タブ・/judge で判定を開く)を、非表示の4件(タブ無し・/judge と /judge/ は /calc へ置き換え・popstate)に置き換え。
- `web/src/app/screenRegistry.test.ts`: instantiateScreens の比較先を `visibleScreens(SCREENS)` に。hidden の新規テストを追加。
- `web/e2e/a11y.spec.ts`: 矢印キー巡回から判定を外す。`web/e2e/routing.spec.ts`: /judge 直開きが /calc に置き換わる1本を追加。

## 実装者向けのメモ

- `docs/design.md` にはタブ一覧の記述が無い(判定への言及は P5-5 の説明のみ)。タブの一覧を書く箇所ができたら「判定は非表示(ADR-0330)」を1行足す。
- 触るのは `app/screenDefinition.ts`・`app/screens.tsx`・`app/routes.ts`・`judge/judge.screen.tsx` の1行・テスト。`App.tsx` は `SCREEN_ROUTES`/`instantiateScreens` 経由で変更不要の見込み(訪問済みタブ・マスタ不要画面の処理は `SCREENS` ではなく可視の画面だけを見ること。`SCREENS` を直接 map している箇所があれば `visibleScreens` を使う)。
- engine・api・services・`web/src/judge/` のコードは変更しない。

## 結果

実装済み(2026-10-04)。`defineScreen` に `hidden?`(`RegisteredScreen.hidden`、既定 false)、`visibleScreens`(app/screens.tsx)を追加。
`SCREEN_ROUTES` と `instantiateScreens` は `visibleScreens` 経由なので、判定はタブ・巡回・URL・クライアント生成・マウントのすべてから外れる。
`isRegisteredScreen` は `hidden` が無い旧形式でも通す。`judge.screen.tsx` は `hidden: true` の1行のみ。App.tsx は変更不要だった。
再表示は `hidden: true` を消すだけ。
