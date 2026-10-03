# ADR-0323: Web の画面レジストリ(画面・タブを足しても共有ファイルを編集しない)

- 状態: 採用
- 番号: 当初 0173 で起票したが、Web の ADR は Web 帯(0300〜)に集める規約(COORDINATION.md)に合わせて 0323 に付け替えた
- 日付: 2026-10-03
- レーン: Web(ユーザー決定 2026-10-03「画面レジストリ化」。iOS は別 PR で同じ考え方を取る)
- 関連: ADR-0300 §1(URL で画面を切り替える。ルーターのライブラリは入れない)、ADR-0304 追記6(issue 308: マスタ不要の画面)、
  ADR-0308(訪れたタブを hidden で残す)、ADR-0411(API 専用の画面はオンラインのマスタを使う)、
  ADR-0303・0604・0705・0309・0319(各レーンの画面とクライアント)、`docs/ai-shared/COORDINATION.md`

## 背景

画面(タブ)を足すたびに、各レーンが同じ4ファイルを編集していた。

- `web/src/app/routes.ts` の `SCREEN_ROUTES`(id・segment・表示名・usesMaster)
- `web/src/app/screens.tsx` の `SCREEN_COMPONENTS`(と、共有の `ScreenProps` に自分のクライアントのフィールド)
- `web/src/App.tsx`(クライアントの生成と `AppTabPanel` への個別のフィールドの受け渡し)
- `web/src/i18n/ja.ts`(タブの表示名と、そのレーンの画面・クライアントの文言のブロック)

並行するレーンの PR がこの4ファイルで毎回衝突した。

## 決定

1. **画面の登録ファイル `*.screen.tsx`**: 各画面は、自分のレーンのディレクトリに登録ファイルを1つ置く
   (`web/src/screens/calc.screen.tsx`・`reverse.screen.tsx`・`balance.screen.tsx`、`web/src/speed/speed.screen.tsx`、
   `web/src/judge/judge.screen.tsx`、`web/src/team/team.screen.tsx`、`web/src/adjust/adjust.screen.tsx`)。
   中身は `defineScreen({ id, segment, label, order, usesMaster, createClient, render })` の既定エクスポート。
   - `app/screens.tsx` が `import.meta.glob(["../**/*.screen.tsx", "!../**/*.test.screen.tsx"], { eager: true, import: "default" })` で集め、
     `buildScreenRegistry` で検証して `order` の昇順に並べる(`SCREENS`)。`app/routes.ts` の `SCREEN_ROUTES` はそこから導く。
   - **新しい画面を足すレーンは、自分のディレクトリに登録ファイルを置くだけ**(`App.tsx`・`app/screens.tsx`・`app/routes.ts`・
     `i18n/ja.ts` は触らない)。eager なので遅延読み込みにはならず、バンドルの分割・初期表示は今までと同じ。
   - `order` は既存の並びを再現する値を、間を空けて振る: 計算 100・逆算 200・タイプバランス 300・素早さ 400・判定 500・
     構築 600・調整 700。間に挿入するときは間の値(例 450)を使う。
   - **テスト用の fixture には `.screen.tsx` の名前を使わない**(glob に拾われて本番に取り込まれ、タブになる)。念のため `*.test.screen.tsx` は glob から除外している。
     テストで登録を試すときは、ファイルを作らず `buildScreenRegistry` に偽の登録を渡す。
   - `buildScreenRegistry` は、既定エクスポートの欠落・形の誤り(id・segment・label が文字列、order が数値、usesMaster が真偽値、
     instantiate が関数でない。defineScreen を使わずコンポーネントを既定エクスポートにした場合など)・id の重複・segment の重複・order の重複・segment の形
     (小文字英数字とハイフン)・予約済みの segment(`about`)・空の表示名を検出して例外を投げる(起動時に気付ける)。
     `app/screenRegistry.test.ts`(登録の整合テスト)が、実際の登録と偽の登録の両方でこれを確かめる。
   - 画面 ID(`ScreenId`)は `string` になる(以前は `SCREEN_ROUTES` の `as const` から導いた union)。足し忘れの型エラーは、
     「表に足し忘れる」こと自体が無くなる(登録ファイル1つが画面のすべて)ので不要になる。重複・欠落は上の検証とテストで守る。
2. **クライアントは登録ファイルが作る**: 登録ファイルの `createClient(deps)` が、App の渡す共通の材料
   (`ScreenClientDeps` = 基点 URL・fetch・端末 ID/セッション ID)から自分のクライアントを作る。App はマウント時に1回だけ
   全画面の `createClient` を呼び(`instantiateScreens`。今までの `useState(() => createXxxClient(...))` と同じ時点・同じ回数)、
   画面の `render(env, client)` に渡す。
   - `render` が受け取る `ScreenEnvironment` は**アプリ全体の値だけ**(engine・master・masterSearch・recordClient・
     reloadToken・onlineMasterSource)。画面ごとのクライアントは入れない。新しい画面のために `ScreenEnvironment`・
     `AppTabPanel` のフィールドを足す必要は無い。
   - 各画面の既存の Props(`CalcScreenProps` など)は変えない。登録ファイルの `render` がアダプタになり、
     `env` と `client` から Props を組み立てる。`defineScreen<C>` がクライアントの型 `C` を閉じ込める(any を使わない)。
   - マスタ不要の画面(`usesMaster: false`。今は素早さ)の `render` は `master`・`engine` を持たない
     `MasterlessScreenEnvironment` しか受け取れない(ADR-0304 追記6 の型の保証を、表ではなく登録ファイルの型で保つ)。
   - API 専用の画面(タイプバランス・判定・調整。ADR-0411)は、登録ファイルの `render` で `OnlineMasterGate`
     (`app/onlineMasterGate.tsx`。旧 `withOnlineMaster` の高階コンポーネントを、Props の型を崩さない render-prop に置き換えたもの。
     挙動は同じ)で包む。
   - 情報ページ(`AboutScreen`)はタブではないので登録の対象外。App が今までどおり持ち、データ削除用の record・team の
     クライアントも App が作る(team のクライアントは構築タブ用と別インスタンスになるが、どちらも状態を持たない fetch の薄い包みで、
     生成時に通信しない)。
3. **文言はレーン別のファイル**: `i18n/ja.ts` のうちレーン固有のブロックを `i18n/<レーン>.ts` に移した
   (`balance.ts`・`speed.ts`・`judge.ts`・`team.ts`・`adjust.ts`・`about.ts`)。`ja.ts` は `export * from "./<レーン>"` の1行/レーンで
   再エクスポートし、既存の `import { ... } from "../i18n/ja"` は変えない。
   - レーンをまたぐ語(欄のラベルの語・ステータスの1文字表記 `statLetterJa`)は `i18n/common.ts` に置き、`ja.ts` とレーンのファイルの両方が
     そこから読む(レーンのファイルが `ja.ts` を import すると循環 import になり、評価順で未初期化の値を読むため)。
   - **新しいレーンは `i18n/<レーン>.ts` を作り、画面・登録ファイルからそこを直接 import する**。`ja.ts` への再エクスポート行は足さない
     (足さなくても使える。再エクスポートは既存の import 先を変えないための互換)。よって新しいレーンが `ja.ts` を編集する必要は無い。
     `import.meta.glob` で `ja.ts` に集めることはしない(型付きの名前付きエクスポートを glob で再エクスポートできないため)。
   - 既存のタブの表示名(`appText.*TabLabel`)は `appText` に残す(テストと iOS との語の照合が参照しているため)。新しい画面の表示名は
     そのレーンの i18n ファイルに置く。
4. **共有ファイルに残るもの**: 画面を足すときに共有ファイルを編集する必要は無い。アプリ全体の値(全画面が使いうる値)を
   `ScreenEnvironment` に足すときだけ `app/screenDefinition.ts` と `App.tsx` を編集する(これはアプリの骨組みの変更なので Web レーンが行う)。
5. **他レーンの未マージ PR の移行手順**(App.tsx 等を編集している Web の PR など)(この PR のマージ後、`App.tsx`・`app/screens.tsx`・`app/routes.ts`・`i18n/ja.ts` を
   編集していた PR は衝突する):
   1. `origin/main` を自分のブランチに merge する。
   2. 4ファイルの衝突は、**main 側(この PR の形)を採る**。
   3. 自分の変更を移す: 新しいタブは `<自分のディレクトリ>/<id>.screen.tsx` に登録ファイルを足す(既存の登録ファイルを手本にする)。
      クライアントは `createClient`、Props の受け渡しは `render`。既存の画面の Props を変えた場合は、その画面の登録ファイルの `render` を直す。
      文言は `i18n/<レーン>.ts`(既存のレーンなら移動先のファイル、`calcScreenText` など計算・逆算の語は `ja.ts` のまま)に足す。
   4. `npm run lint && npm run typecheck && npm test` で確かめる(`app/screenRegistry.test.ts` が id・segment・order の重複を見る)。

## 結果

- 既存の挙動は変えない: タブの並び・URL セグメント・表示名・キーボード操作・aria・マスタの要否・計算モード・
  オンライン専用画面の扱い・各画面の Props は同じ。既存のテストの期待値は変えていない。
- 画面を足す PR は、自分のディレクトリの登録ファイル・画面・i18n ファイルだけで閉じる。
- 失うもの: `ScreenId` の union 型(タブ ID の綴り間違いはコンパイルで捕まらず、`screenFromPath` が null を返す形でしか現れない)。
  既存の routes のテストが全画面の id・segment・表示名を列挙して確かめているので、退行は検出できる。
