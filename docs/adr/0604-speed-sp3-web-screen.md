# ADR-0604: 素早さ比較 SP3 Web 画面

- 状態: 採用(2026-09-22。§1 はユーザー決定・回答。§2 以降は素早さレーンの判断)
- 日付: 2026-09-22
- 関連: docs/speed-design.md §5・§6、ADR-0600〜0603、ADR-0300(P4-10 画面の骨組み)、ADR-0303(balance 画面。同じ形を倣う)、
  docs/design.md(デザイントークン)

## 決定

### 1. ユーザー決定・回答(2026-09-22)
- 画面は左右に配置。左 = 速い順の全体の表、右 = 自分のポケモン。自分の実数値が左の表のどこに入るかを視覚的に示す。
- 素早さの画面は素早さレーンの SP3 のまま(Web の P4-13 は取り消し)。タブ・URL は Web の P4-10 のルート表に1項目足すだけ。

### 2. ファイルの置き場所(レーン境界)
COORDINATION.md は `web/src/speed/` を素早さレーンの持ち物と定め、タブ登録は「自分の1項目を足すだけ」とする。この境界を保つため:
- **`web/src/speed/` の中だけに置く**: `SpeedScreen.tsx`・`SpeedScreen.css`・`SpeedScreen.test.tsx`・`speedClient.ts`・生成型 `speed.gen.ts`。
  balance の `web/src/api/balanceClient.ts`・`balance.gen.ts` とは違うディレクトリにする(`web/src/api/` は Web レーンの持ち物として扱う)。
- **3か所 + 2か所の最小の追記**(Web レーンと合意。2026-09-22):
  1. `web/src/app/routes.ts` の `SCREEN_ROUTES` に `{ id: "speed", segment: "speed", label: appText.speedTabLabel }`
  2. `web/src/i18n/ja.ts` の `appText` に `speedTabLabel`
  3. `web/src/app/screens.tsx` の `SCREEN_COMPONENTS` に `speed: SpeedScreen`
  4. 同じ `screens.tsx` の `ScreenProps` に `speedClient: SpeedClient` を1フィールド追加(既存の `client: BalanceClient` は触らない。
     P4-12a で `client` が balance 専用の型になったため、素早さは別フィールドで持つ)
  5. `web/src/App.tsx` に `speedClient` の作成(`useState` 1行)と `<ActiveScreen ... speedClient={speedClient} />` の1引数を追加
- 型生成(`speed.gen.ts`)は `make gen-ts`(ルート Makefile。Web レーンの持ち物)を変更せず、`npx openapi-typescript
  ../services/speed/api/openapi.yaml -o src/speed/speed.gen.ts` を手動で実行してコミットする(再生成コマンドはファイル冒頭にコメントで残す)。
  `make gen-ts` へ組み込むかは Web レーンの都合に任せる(DECISIONS.md に提案として記録)。

### 3. API クライアント(`speedClient.ts`)
balance の `balanceClient.ts` と同じ設計(ADR-0303 §1・§5・§6 を踏襲): 例外を投げず `SpeedResult<T> = {ok:true,value} | {ok:false,error}`。
`web/src/api/config.ts`(`apiBaseUrl`)・`web/src/api/clientIds.ts`(`ClientIds`)は共通の Web レーンのヘルパーをそのまま import して使う
(変更しない)。呼ぶ endpoint:

```ts
interface SpeedClient {
  pokemon(): Promise<SpeedResult<Schemas["PokemonListResponse"]>>;
  table(presets?: readonly Schemas["PresetId"][]): Promise<SpeedResult<Schemas["TableResponse"]>>;
  position(request: Schemas["PositionRequest"]): Promise<SpeedResult<Schemas["PositionResponse"]>>;
}
```
パスは `api/speed/v1/pokemon`・`api/speed/v1/table`・`api/speed/v1/position`(services/speed/api/openapi.yaml のまま)。

### 4. 画面の構成(SpeedScreen.tsx)
- マウント時に `pokemon()` と `table()`(presets 省略 = 全6行)を1回ずつ呼ぶ。両方が揃うまで **左の表だけ** ローディング表示にする
  (balance と同じく、複数呼び出しは独立でエラーを混ぜない。ADR-0303 §9)。
- **左(表)**: `table()` の `tiers`(段)をそのまま速い順に描画する。1段 = 1行。同じ段に複数 `entries` があれば「同速」のバッジを付け、
  段の中の各エントリを横に並べる(pokemonId 昇順・プリセット順はサーバー側で確定済みなので並べ替え直さない)。
  各エントリは、CalcScreen の `type-emblem`(`data-testid="type-emblem"`。`backgroundColor: var(--type-<id>)`)と同じ形の
  タイプ色エンブレムを使う(画像は必須にしない。CLAUDE.md ドメイン規約)。行にはポケモンの日本語名・プリセットの表示名(文言資源。
  ADR-0601 §2 の6つを i18n に持つ)・素早さの実数値を出す。
- **右(自分)**: `mode` の選択(セグメントコントロール。`preset` が既定)。
  - `preset`: ポケモンの `<select>`(pokemon() の一覧)+ `uninvested`/`neutral-max`/`max` のラジオ + スカーフの checkbox。
  - `custom`: ポケモンの `<select>` + SP(0〜32 の数値入力)+ 性格(`minus`/`neutral`/`plus`)+ ランク(-6〜+6)+ スカーフ。
  - `raw`: 実数値の数値入力(ポケモンの `<select>` は任意。表示用)。
  - 入力の変更ごとに `position()` を呼ぶ(useEffect + key の変更検知。BalanceScreen と同じ形。cancelled フラグで古い応答を無視)。
  - 結果: 実数値、`faster`/`slower` の件数、`tie` があれば「同速」の一覧。
- **左右の連動(視覚的な強調)**: `position()` の結果が届いたら、左の表で「自分と同じ段」(`tie` の `speed` と一致する段)があれば
  その段を強調(枠線・背景)し、無ければ `faster` 件数と `slower` 件数から「この位置に自分の行が挟まる」境界線を、該当する段の間に
  引く(`faster` 行目の段の直後、`slower` 行目の段の直前)。強調・境界線はどちらも CSS の `--danger` は使わず、控えめな強調色トークン
  (`--border-hairline` を濃くした程度)にする(常時アニメーションは入れない。CLAUDE.md ドメイン規約)。

### 5. マスタ・engine は使わない
SP0〜SP2 で決めたとおり、素早さの計算は speed-svc(engine を呼ぶ)が行う。Web は結果を表示するだけで、`engine`(WASM)・
`MasterData`(pokedex の Web 側マスタ)を使わない。`ScreenProps.engine`・`ScreenProps.master` は構造的に無視する(CalcScreen/ReverseScreen が
`client` を無視するのと同じやり方)。

### 6. テスト
- `SpeedScreen.test.tsx`: BalanceScreen.test.tsx と同じパターン(fake `SpeedClient`、呼び出し履歴の検証、古い応答の無視)。
  同速のケース・境界線のケースを1つずつ。
- `web/src/App.routing.test.tsx` と `web/e2e/routing.spec.ts` に `/speed` の1ケースを足す(既存のパターンのまま)。
- `speedClient.test.ts`: balanceClient.test.ts と同じ形(fake fetch、ヘッダーの付与、エラー本文の解釈)。

## 却下した案
- `ScreenProps.client` を `BalanceClient | SpeedClient` の union にする: 各画面で毎回型を絞り込む必要があり、balance 側の変更が要る
  (レーン外)。別フィールドの追加の方が影響範囲が小さい。
- speed の型を `web/src/api/` に置く: balance と同じ場所にすると、ディレクトリでのレーン境界(`web/src/speed/`)が崩れる。
- 左右の連動を「自分の行を表に挿入して描画し直す」形にする: サーバーの `tiers` をそのまま描画する方針(§4)と矛盾しない範囲で、
  強調・境界線だけで表現する方が実装が単純で、表の並びの正本(サーバー)を1つに保てる。
