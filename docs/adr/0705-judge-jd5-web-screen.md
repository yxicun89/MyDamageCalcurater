# ADR-0705: 判定(素早さ×ダメージ連動)JD5 Web 画面

- 状態: 採用(2026-09-24。§0 はユーザー決定。§1 以降は判定レーンの判断)
- 日付: 2026-09-24
- 関連: ADR-0700(JD0 基盤。エラーの正規化・「真偽値 1 つに丸めない」立場・judge 自身の Ingress)、
  ADR-0701(JD1 の request/response)、ADR-0702(JD2 の `speedField`)、ADR-0703(JD3 の `defenders` / `matchups`・
  候補ごとに変えられないもの)、ADR-0704(JD4 の `DefenderCandidate`・行動順・`attackerKo` / `defenderKo`)、
  `services/judge/api/openapi.yaml`、docs/judge-design.md §3 JD5、
  ADR-0604(素早さレーンが `web/src/speed/` を自分で作った前例。ファイルの置き場所・タブ登録・クライアントの形)、
  ADR-0303(balance 画面。複数メンバーのフォーム)、ADR-0304(オンラインのマスタの制約。§3 の「技を ID から引く公開 API が無い」)、
  ADR-0300(画面の骨組み)、docs/ai-shared/DECISIONS.md 2026-09-24「JD5 の担当をユーザーが判定レーン自体に決定」

## 背景

JD0〜JD4 で `POST /api/judge/v1/outspeed-and-ko` は「自分 1 体が、相手候補 1〜6 体それぞれを素早さで抜けて、
自分の技で倒せるか + 相手の技で返り討ちに遭うか」を、場の効果(トリックルーム・追い風)込みで返せるようになった。
JD5 はこれを呼ぶ**画面**で、judge-svc に初めて付くクライアントになる。

担当は 2026-09-24 のユーザー決定で**判定レーン自体**(Web レーンへ依頼しない)。COORDINATION.md の
「素早さレーンが `web/src/speed/` を自分で作った」前例に倣う。まずは Web を作り、iOS は着手時に改めて判断する。

judge の response は意図的に「勝てる / 負ける」に丸められていない(ADR-0700 §6-1・ADR-0704 §3)。
画面はその材料(素早さ・優先度・行動順・双方向の確定数)をそのまま並べる側で、**Web で勝敗を判定しない**。

## 決定

### 1. ファイルの置き場所(レーン境界)

ADR-0604 §2 と同じ形にする。**判定レーンが持つのは `web/src/judge/` の中だけ**:

- `web/src/judge/judgeClient.ts` — judge API のクライアント(§3)
- `web/src/judge/judge.gen.ts` — 生成型(§2)
- `web/src/judge/JudgeScreen.tsx` / `JudgeScreen.css` — 画面(§4〜§7)
- `web/src/judge/judgeClient.test.ts` / `JudgeScreen.test.tsx` — テスト

共有ファイルへの追記は**4 か所だけ**(ADR-0604 §2 の 5 項目と同じ):

1. `web/src/app/routes.ts` の `SCREEN_ROUTES` に `{ id: "judge", segment: "judge", label: appText.judgeTabLabel }` を
   `speed` の後ろに 1 件
2. `web/src/i18n/ja.ts` に `appText.judgeTabLabel` の 1 行と、画面・クライアントの文言ブロック
   (`judgeClientText`・`judgeErrorText`・`judgeScreenText`)
3. `web/src/app/screens.tsx` の `SCREEN_COMPONENTS` に `judge: JudgeScreen`(import 1 行)と、
   `ScreenProps` に `judgeClient: JudgeClient` を 1 フィールド
4. `web/src/App.tsx` に `createJudgeClient` の `useState` 1 行と `<ActiveScreen ... judgeClient={judgeClient} />` の 1 引数

**他の画面のファイル(`CalcScreen.tsx`・`BalanceScreen.tsx`・`SpeciesSearchField.tsx` 等)は一切変更しない**。
`SpeciesSearchField` は import して使うだけで、中身には触らない。

### 2. 生成型(`judge.gen.ts`)は手で再生成してコミットする

ADR-0604 §2 と同じ。ルートの `make gen-ts`(Web レーンの持ち物)は変更せず、`web/` で

```
npx openapi-typescript ../services/judge/api/openapi.yaml -o src/judge/judge.gen.ts
npx prettier --write src/judge/judge.gen.ts
```

を手で実行し、生成物をコミットする(再生成コマンドはファイル冒頭のコメントに残す)。
`make gen-ts` へ組み込むかは Web レーンの都合に任せる(DECISIONS.md に提案として記録する)。

### 3. API クライアント(`judgeClient.ts`)

`speedClient.ts`(ADR-0604 §3)と同じ設計。**例外を投げず**、常に判別 union で返す。

```ts
export type JudgeResult<T> =
  | { readonly ok: true; readonly value: T }
  | { readonly ok: false; readonly error: JudgeError };

export interface JudgeClient {
  outspeedAndKo(
    request: Schemas["OutspeedAndKoRequest"],
  ): Promise<JudgeResult<Schemas["OutspeedAndKoResponse"]>>;
}
```

- パスは `api/judge/v1/outspeed-and-ko`(`services/judge/api/openapi.yaml` のまま)。judge は gateway を経由せず
  自分の Ingress を持つ(ADR-0700 §6-3)ので、Web から見た基点 URL は他の API と同じ `apiBaseUrl()` でよい。
- `web/src/api/config.ts`(`apiBaseUrl`)・`web/src/api/clientIds.ts`(`ClientIds`)は Web レーン共通のヘルパーを
  そのまま import する(変更しない)。`X-Device-Id` / `X-Session-Id` は契約上必須(judge は無ければ 400)。
- HTTP エラーで本文が `{code, message}` なら**その `code` / `message` をそのまま運ぶ**。
  通信できない・本文が JSON でない・エラー本文の形が不正なときは Web 側のコード `judge_unavailable`
  (自動の切り替え先は持たない。判定はサーバーでしか行わない)。
- 呼ぶ endpoint は判定の 1 本だけ。`/api/judge/healthz` は画面から呼ばない(疎通は判定の呼び出し自体が示す)。

### 4. 画面の構成(`JudgeScreen.tsx`)

3 つの領域に分ける(`<section aria-label>` = role `region`)。

- **自分のポケモン**(1 体固定。ADR-0703 §1): 種族・性格・SP 6 項目・ランク 5 項目・特性・持ち物・**使う技の ID**。
- **相手の候補**(1〜6 件。追加・削除できる): 各候補が自分と同じ入力一式 + **その候補自身の技の ID**。
  `BalanceScreen` の `MemberState` / `MAX_MEMBERS` と同じ形の配列 state にする(部品は共有せず `web/src/judge/` に持つ。§9)。
  候補ごとに `role="group"`・`aria-label="相手候補 N"` を付け、中の入力欄のラベルは自分側と同じ語を使う
  (`within(候補 N)` で引けるので、ラベルに番号を埋め込まない)。
- **判定結果**: `matchups` を `defenders` と**同じ順序**で並べる(§6)。

場の効果(`speedField`)は自分のポケモンの領域に置く。**対戦形式(`format`)は `single` を既定の `<select>`** で切り替える
(契約上必須で、judge が calc-svc へそのまま渡す)。

入力補助のマスタ(§5)を使うだけで、**`engine`(WASM)・`master` で判定の計算はしない**(ADR-0604 §5 と同じ立場)。
判定は judge-svc が行う。

### 5. マスタの使い方(入力補助だけ)

ADR-0304 の `MasterCapabilities` に素直に従う。

| 入力 | 取り方 | 根拠 |
|---|---|---|
| 種族 | `capabilities.speciesList` が true なら `master.species` の `<select>`、false なら `SpeciesSearchField` | ADR-0304 §1(オンラインは全件そろわない) |
| 性格 | `master.natures` の `<select>` | 常に全件そろう |
| 特性・持ち物 | `master.abilities` / `master.items` の `<select>`(**省略可**。未選択なら欄ごと送らない) | 常に全件そろう |
| 技 | **自由入力のテキスト欄(技 ID を直接入力)** | ADR-0304 §3(ID から技を引く公開 API が無い) |

- 種族は `SpeciesSearchField` の `onResolved` で `MasterSpeciesResolution` を受け、`species.key` を `speciesKey` に使う
  (種族値・learnset は judge が上流から引くので画面では使わない)。
- **技をドロップダウンにしない**理由は §9(却下した案)に書く。ADR-0304 §3 が既に記録している欠落なので、
  JD5 で新しい提案はしない(制約を踏襲するだけ)。
- 特性・持ち物の**効果**(`capabilities.effects`)は使わない。judge は ID をそのまま calc-svc に渡す(ADR-0701 §3)ので、
  画面も ID を選ばせるだけでよい。

### 6. request の組み立て(最小の request を送る)

- `format`: `<select>` の値(既定 `single`)。
- `attacker` / `defenders[i]`: `speciesKey` / `natureId` / `sp`(6 項目・常に送る)。
  - `ranks` は**5 項目すべてが 0 なら送らない**(契約上省略可で既定 0)。1 つでも 0 でなければ 5 項目すべてを送る
    (生成型の `RankBlock` は 5 項目必須)。
  - `abilityId` / `itemId` は**未選択なら欄ごと送らない**(`null` を明示的に送らない。最小の request にする)。
- `moveId`: 自分の技は request 直下、候補の技は候補の欄。両方とも前後の空白を落とした文字列。
- `speedField`: **3 つすべてが false なら送らない**(送らない request の挙動は JD1 と同じ。ADR-0702)。
  1 つでも true なら 3 項目すべてを送る(生成型の `SpeedField` は 3 項目必須)。
- `field`(天候・地形・壁)は **JD5 では送らない**(§9 の却下案)。

**相手側の追い風(`defenderTailwind`)はチェックボックス 1 つ**にし、候補ごとには持たせない。
ADR-0702 §1・ADR-0703 §5 が「`speedField` は 1 リクエストに 1 つだけで、すべての候補に同じように適用される」と
決めているため、候補ごとのチェックボックスを置くと**画面にしかない概念**が生まれ、契約に送れない入力を作ってしまう。
「すべての候補に同じように適用される」ことは文言で添える。

### 7. 送信のタイミングと、古い応答の扱い

- **送信ボタンを押したときだけ呼ぶ**。入力のたびに呼ぶ `SpeedScreen`(ADR-0604 §4)とは変える。
  1 リクエストが上流を最大 27 回(3+4N。ADR-0704 §5)逐次で叩くため、打鍵ごとの呼び出しは上流に重すぎる。
- 送信ごとに連番を持ち、**最後に送った request の応答だけを表示する**(古い応答を捨てる。`SpeedScreen` の
  `cancelled` フラグ・`Completed.key` と同じ考え方)。判定中は「判定中」の表示を出す。
- **送信前にクライアント側で検査する**(契約の範囲と同じ): SP は各 0〜32 の整数・合計 66 以下(CLAUDE.md ドメイン規約)、
  ランクは各 -6〜+6 の整数、種族・性格・技の ID が空でないこと。違反していれば**呼ばずに**理由を出す
  (無駄な往復を上流に流さない。judge も同じ理由で 400 を返す)。

### 8. 結果の表示(judge が返した値をそのまま出す)

`matchups` の行を `defenderIndex` の順に並べ、各行に次を出す。

| 出すもの | 出典 |
|---|---|
| 候補の種族名 | **request の入力(画面が持っている値)**。judge は種族名を返さない |
| 素早さ | `attackerSpeed` 対 `defenderSpeed` |
| 素早さの比較 | `outspeeds` / `speedTie`(同速は「同速」。真偽値 1 つに丸めない) |
| 優先度 | `attackerMovePriority` 対 `defenderMovePriority` |
| 行動順 | `attackerMovesFirst` / `turnOrderTie`(tie は「どちらが先か決まらない」) |
| 与える確定数 | `attackerKo`(`hits` / `guaranteed` / `displayChancePercent`) |
| 受ける確定数 | `defenderKo`(同上) |

- **画面も「勝てる / 負ける」に丸めない**(ADR-0700 §6-1・ADR-0704 §3 の立場を画面でも保つ)。
  確定 n 発・乱数 n 発(%)・倒せない(`hits === 0`)の 3 通りをそのまま出す。
- 行は必ず request の候補と対応づける(`defenderIndex` を `data-defender-index` に出し、種族名を
  その index の入力から引く)。**行の取り違えは表示だけでは気づけない**ので、候補ごとに違う値を返す
  fake でテストする(ADR-0704「テストの期待値」と同じ轍を踏まない)。
- エラーは `role="alert"` に、**コードごとの日本語の文言**(`judgeErrorText`)を出す。未知のコードは
  サーバーの `message` をそのまま出す。サーバーの `message` が表示した文言と違うときは、補助の行として
  併せて出す(`defenders[2]` のように**どの候補で失敗したか**が入っているため。ADR-0703 §3)。
- エラーでも入力は消さない(直して送り直せる)。

### 9. 個体の入力部品は `web/src/judge/` に新しく作る

`CalcScreen` の個体編集フォームは演出(ホロ効果・入れ替えアニメーション)と強く結合していて、そのままでは使えない。
共通部品へ切り出すと `web/src/screens/` の他の画面を変えることになり、レーン境界(§1)を越える。
`JudgeScreen.tsx` の中(または `web/src/judge/` 配下)に、判定に要る欄だけの小さな部品を作る。
`SpeciesSearchField` だけは既存のものを **import して使う**(変更しない)。

## 受け入れ条件(JD5)

1. **クライアント**: `createJudgeClient({baseUrl, fetch, ids})` が `${baseUrl}api/judge/v1/outspeed-and-ko` に
   `X-Device-Id` / `X-Session-Id` / `Content-Type: application/json` 付きで `OutspeedAndKoRequest` を POST し、
   200 は `{ok: true, value}`、`{code, message}` の HTTP エラーは**その code / message のまま** `{ok: false, error}`、
   通信不能・JSON でない・エラー本文の形が不正は `judge_unavailable` を返す。**例外を投げない**。
2. **相手候補のフォーム**: 初期は 1 件。追加で 6 件まで増え、**7 件目は追加できない**(ボタンが `disabled`)。
   削除で減り、**1 件のときは削除できない**。候補ごとに独立した入力(種族・性格・SP・ランク・特性・持ち物・技 ID)を持ち、
   ある候補の入力が他の候補に影響しない。
3. **request**: 送信ボタンでだけ `outspeedAndKo` を 1 回呼び、body は §6 のとおり。
   ランクが全 0 なら `ranks` を送らず、場の効果が全 false なら `speedField` を送らない。
   未選択の特性・持ち物の欄は送らない。`field` は送らない。技 ID は前後の空白を落として送る。
4. **場の効果**: トリックルーム・自分側の追い風・相手側の追い風の 3 つのチェックボックスが
   `speedField.trickRoom` / `attackerTailwind` / `defenderTailwind` に 1 対 1 で対応する。
   **相手側の追い風は候補ごとではなく 1 つだけ**(すべての候補に同じように適用される旨の文言が出る)。
5. **結果**: `matchups` の各行が対応する候補の**種族名**と一緒に出て、行の並びは `defenders` と同じ。
   各行に `outspeeds` / `speedTie` / 双方の素早さ・優先度・`attackerMovesFirst` / `turnOrderTie` /
   `attackerKo` / `defenderKo` が出る。**候補ごとに違う値を返す fake で、行の取り違えが起きないこと**を確かめる。
   `hits === 0` は「倒せない」、`guaranteed` は「確定 n 発」、それ以外は「乱数 n 発(x.x%)」。
   画面は「勝ち」「負け」に丸めた語を出さない。
6. **エラー**: `ok: false` のとき `role="alert"` にコードごとの文言が出て、サーバーの `message` も併せて出る
   (どの候補で失敗したかが読める)。エラーの後も入力は残る。`judge_unavailable` でも画面が壊れない。
7. **送信前の検査**: SP が範囲外・合計 66 超過、ランクが範囲外、種族・性格・技の ID が空のときは
   **`outspeedAndKo` を呼ばず**に理由を出す。
8. **古い応答**: 2 回続けて送ったとき、先に送った方の応答が後から届いても表示を上書きしない。
9. **画面登録**: `SCREEN_ROUTES` に `judge`(`/judge`・タブ「判定」)があり、`/judge` を直接開くと判定タブが選ばれる。
   他の画面のファイルは変更されていない。
10. `npm run typecheck` / `npm run lint` / `npm run test`(web)が通る。

## テストの期待値

- `judgeClient.test.ts` は `speedClient.test.ts` と同じ形(fake fetch・URL とヘッダーと本文の検査・
  エラー本文の解釈・例外を投げないこと)。request / response は生成型で書き、契約とのずれを typecheck で検出する。
- `JudgeScreen.test.tsx` は fake の `JudgeClient`(呼び出しを記録し、テストが応答を解決する)で、
  `SpeedScreen.test.tsx` と同じ形。**マスタは架空データ**(`9001-000` の形の種族・`test-*` の技 ID。
  実マスタ・実データは使わない。CLAUDE.md ドメイン規約・ADR-0002)。
- 行の取り違えを検出するため、候補ごとに `defenderIndex` / 素早さ / 優先度 / `attackerKo` / `defenderKo` を
  **すべて違う値**にした `matchups` を返す(全部同じ値だと取り違えが緑のまま通る)。
- `web/src/app/routes.test.ts` に `judge` タブの登録(`/judge`・タブ「判定」)のケースを足す。
  画面登録(`routes.ts` / `screens.tsx` / `App.tsx`)を入れるときに、ADR-0604 §6 の前例どおり
  `web/src/App.routing.test.tsx` に `/judge` の1ケースを足す(そこは実装と同じコミットで入れる。
  ルート表に `judge` が無い状態では `SCREEN_COMPONENTS` の型が通らないため、テストだけ先に置けない)。
- judge は engine の式を持たないので `make test-golden` の対象は増えない(ADR-0700〜0704 と同じ)。

## 却下した案

- **`field`(天候・地形・壁)を JD5 で扱う**: judge-design.md の JD1〜JD4 の範囲に画面の要件が無く、
  壁は「自分の側 / 相手の側」の向きの説明(ADR-0704 §4)まで画面に載せることになる。入力が一気に増えるわりに、
  判定の主要な問い(抜けるか・倒せるか・返り討ち)には効かない場面が多い。要望が出てから別タスクで足す
  (契約は既に `field` を受け取れるので、画面だけの追加で済む)。
- **技をドロップダウン(一覧)から選ばせる**: オンラインのマスタでは `capabilities.moves` が false で、
  技 ID から技を引く公開 API が無い(ADR-0304 §3 の既知の欠落)。`getSpecies.learnset` は ID の配列しか返さないので、
  名前で選ばせるには技の一覧 API(データ/API レーンへの新規依頼)が要る。JD5 はそれを待たず、
  **技 ID の自由入力**で出す。未知の ID は judge が 422 `unknown_move` で返し、画面はその理由を出せる
  (ADR-0704 §6)ので、黙って間違った判定を出すことはない。技の一覧 API が付いたら `<select>` に差し替える。
- **候補ごとに追い風のチェックボックスを置く**: §6 のとおり、契約(ADR-0703 §5)に送れない入力になる。
  候補ごとに変えたいなら `speedField` を候補ごとに持てるよう**契約から**変える話で、画面の都合で先に UI を作らない。
- **入力のたびに `outspeedAndKo` を呼ぶ(`SpeedScreen` と同じ形)**: §7 のとおり、1 回の判定が上流を最大 27 回
  逐次で叩く。打鍵ごとに投げると上流への負荷が桁違いになり、途中の不完全な入力で 400 / 422 が出続ける。
- **Web 側で「勝ち / 負け」を計算して 1 行にまとめる**: judge が意図的に丸めなかったもの(ADR-0700 §6-1・
  ADR-0704 §3)を画面で丸め直すことになる。乱数と確定の区別・同速・同優先度の不確定さが消える。
  まとめた見出しが欲しくなったら、材料を残したうえで別に足す。
- **`CalcScreen` の個体編集フォームを共通部品に切り出して使い回す**: §9 のとおり、他レーンが触っている画面を
  変えることになる。重複は承知のうえで `web/src/judge/` に小さく作り、共通化は 2 つ目の利用者が出てから考える。
- **`web/src/api/judgeClient.ts` に置く**(balance と同じ場所): ADR-0604 §2 と同じ理由で、
  ディレクトリによるレーン境界が崩れる。
- **iOS 画面を同じタスクでやる**: Web を先に出して形を確かめる(DECISIONS.md 2026-09-24)。
  iOS は `swift-openapi-generator` の生成物を judge の契約から作る必要があり、作業の性質が違う。
