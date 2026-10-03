# ADR-0319: Web の調整タブ(AJ6。指数・16n・SP 配分・最小 SP・技を覚えるポケモン)

- 状態: 採用(2026-10-02。AJ6 の仕様。spec 段階)
- 日付: 2026-10-02
- レーン: ダメージ計算(Web 帯 `0300〜`。`0318` までは他ブランチが使用済みのため `0319`)
- 関連: plan.md「AJ: 調整」AJ6、ADR-0150(指数・16n・探索・配分の定義)、ADR-0250(調整 API の契約)、
  ADR-0251(技の逆引き API)、ADR-0300(Web の構成・画面の足し方)、ADR-0301 §4(計算モード)、
  ADR-0304(オンラインのマスタ・SpeciesSearchField)、ADR-0411(API 専用画面は常にオンラインのマスタ)、
  ADR-0705(判定画面: 送信ボタンでだけ呼ぶ・レーンの境界)、ADR-0604(素早さ画面: 専用クライアント)、
  ADR-0123 / ADR-0215(未対応の印)、docs/design.md「入力のラベル」・ブレークポイント 600px・アニメーション、
  requirements.md「プリセットを選ぶだけ」

## 背景

AJ0〜AJ5 で調整の engine・API(`POST /api/calc/adjust/*`)・技の逆引き(`GET /api/pokedex/moves/{key}/learners`)が
そろった。AJ6 はこれを Web の新しいタブ「調整」に出す。ユーザー要望(2026-10-01)は次のとおり。

- 火力指数・耐久指数・HP の 16n / 16n-1 を見て、耐久調整の数値を出せる(機能 2)
- 固定したい SP を決め、残りを「攻撃側(S と A/C)」か「耐久側(H/B/D の配分は自分の意思で。素早さは見ない)」に回すと、
  効率の良い振り方を提示する(機能 3)
- 相手のポケモン・技を指定すると「倒せる/耐える最小 SP」を提示する(機能 4)
- 技から、その技を覚えるポケモンの一覧を開ける(機能 1)
- 常時最大振りにせず、調整したいときだけ使う。送信ボタンでだけ API を呼ぶ(打鍵ごとに探索しない)

## 決定

### 1. API 専用の画面にする(WASM は使わない)。タブは末尾に1件足す

- 調整の画面は判定・タイプバランスと同じ **API 専用の画面**とし、`withOnlineMaster`(ADR-0411)で包む。
  ヘッダーの「ダメージ計算の実行場所」(オフライン = WASM / オンライン = API)には従わない。
- ADR-0250 §結果 は「Web(AJ6)は WASM」と書いたが、AJ6 では採らない。理由:
  1. 機能 1(技を覚えるポケモン)は pokedex の API でしか引けず、どのみちオンラインが要る。
  2. WASM で呼ぶには `CalcEngine`(engine/types.ts)に4操作を足し、`wasmEngine.ts`・`apiEngine.ts`・テスト用の fake engine を
     すべて変えることになる。並行して動く Web レーンの共有ファイルを広く触る(ADR-0705 §1 と同じ「レーンの境界」の考え方)。
  3. オフライン(架空の例データ)の ID でオンラインの探索をすると 422 になる問題(ADR-0411 の背景)を避けられる。
  4. HTTP と WASM のパリティは calc-svc のテストが固定している(ADR-0250 §結果)ので、あとで WASM に切り替えても数値は変わらない。
  WASM 関数(`adjustIndices` など)は AJ4 で登録済みのまま残す。オフラインで調整を使いたい要望が出たら、別タスク(AJ6b 候補)で
  `CalcEngine` に足して切り替える。
- 画面の中身は `web/src/adjust/` に置く(`AdjustScreen.tsx`・`adjustClient.ts`・`adjustRequest.ts`・`adjustFormat.ts`)。
  共有ファイルへの追記は次に限る(ADR-0705 §1 と同じ):
  - `app/routes.ts` の `SCREEN_ROUTES` に `{ id: "adjust", segment: "adjust", label: appText.adjustTabLabel, usesMaster: true }`
    を**末尾**(構築の後ろ)に1件
  - `app/screens.tsx` の `SCREEN_COMPONENTS` に `adjust: withOnlineMaster(AdjustScreen)` を1件、`ScreenProps` に
    `readonly adjustClient: AdjustClient` を1件(既存画面の Props の型は変えない。構造的に無視される)
  - `App.tsx` で `createAdjustClient({ baseUrl: apiBaseUrl(), fetch, ids: clientIds })` を judge と同じ形で作って渡す
  - `i18n/ja.ts` に `appText.adjustTabLabel`・`adjustClientText`・`adjustErrorText`・`adjustScreenText`
- タブが 7 件になるので、タブの並び・件数を固定している既存テスト(`app/routes.test.ts`・`App.test.tsx` の End・
  `App.masterSources.test.tsx` の件数・`e2e/a11y.spec.ts` の矢印キー)は「末尾に調整」へ期待値を変える(意図の変更)。

### 2. 画面の構成(入力 → モード → 相手・目標 → 送信 → 結果)

上から1列(狭い幅 600px 未満は縦積み。広い幅でも入力カードは縦に並べてよい。docs/design.md のブレークポイント)。

| 領域(h2 の見出し = region の名前) | 中身 |
|---|---|
| 自分 | ポケモン(`speciesList` が false なら SpeciesSearchField)・性格・特性(任意)・持ち物(任意)・技(任意。learnset の攻撃技だけ)+「覚えるポケモン」ボタン・固定する能力ポイント 6 欄と合計「合計 n / 66」 |
| 調整の内容 | モードの radiogroup(下表)と、モードごとの欄 |
| 相手 | ポケモン・調整(プリセット)・技(相手が攻撃する側のときだけ。+「覚えるポケモン」ボタン)。相手が要るモードだけ出す |
| 目標 | 発数(1〜10 の select)・確率(確定 / 90% / 75% / 50% の select)。相手が要るモードだけ出す |
| (ボタン)調整する | |
| 調整の結果 | 今の振り方の指数と 16n(h3)+ モードの結果 + 未対応の印 |
| この技を覚えるポケモン | 「覚えるポケモン」を押したときだけ出す(§7) |

モード(radio。既定は「指数と 16n を見る」):

| モード | 呼ぶ API | 必須 | モードの欄 |
|---|---|---|---|
| 指数と 16n を見る(`indices`) | indices | 自分のポケモン・性格 | なし |
| 耐久に振る(`bulk`) | indices + allocation(mode=bulk) | 同上 | 耐久の基準(物理 / 特殊 / 両方。既定「両方」を見える形で選択済み)、H・B・D の上限(既定 32)、「目標を指定する」 |
| 攻撃と素早さに振る(`offense`) | indices + allocation(mode=offense) | 同上 | 攻撃の分類(既定は物理。自分の技を選ぶとその分類に合わせる)、A(C)・S の上限、素早さの目標(実数値。空 = 目標なし)、「目標を指定する」 |
| 倒せる最小の振り方(`minKo`) | indices + min-sp-to-ko | + 自分の技・相手のポケモン | — |
| 耐えられる最小の振り方(`minSurvive`) | indices + min-sp-to-survive | + 相手のポケモン・相手の技 | — |

- 「固定する能力ポイント」は ADR-0150 §8 の**下限**。探索系(min-sp-to-*)では探索する能力の値をサーバーが上書きする(ADR-0250 §2)。
  素早さは耐久側では見ない(上限の欄を出さない)。
- 相手の SP・性格は**数値入力させずプリセットから選ぶ**(requirements.md)。相手が攻撃するとき(`minSurvive`・`bulk` の目標)は
  攻撃側プリセット(`domain/attackerPresets.ts`。無振り / A(C)特化 / A(C)振り)、相手が受けるとき(`minKo`・`offense` の目標)は
  防御側プリセット(`domain/defenderPresets.ts` の `defenderPresetKeysFor(自分の技の分類)`)。プリセットの性格(plus/minus)は
  マスタの性格から ID を引く(無補正は plus・minus とも null の先頭)。見つからなければ送らずに理由を出す(別の性格で代えない)。
- モードを切り替えても入力は消さない。相手の持ち物・特性・場の状態・急所・ダブルは AJ6 では入力させない(必要になったら足す。
  ダブルは判定画面と同じく壁・全体技の補正が未実装のため出さない。issue 288)。

### 3. API クライアント(`web/src/adjust/adjustClient.ts`)

- `createAdjustClient({ baseUrl, fetch, ids })` → `AdjustClient { indices, minSpToKo, minSpToSurvive, allocation, moveLearners }`。
  judge/judgeClient.ts と同じく**例外を投げず** `AdjustResult<T>`(判別 union)で返し、生成型(api/openapi.gen.ts)のまま運ぶ。
- POST 4本は `api/calc/adjust/{indices,min-sp-to-ko,min-sp-to-survive,allocation}`、本文は request のまま(既定を補わない)。
  逆引きは `GET api/pokedex/moves/{encodeURIComponent(key)}/learners?limit=&offset=`。全リクエストに X-Device-Id・X-Session-Id。
- 失敗の写像: HTTP エラーで `{code, message}` → そのまま運ぶ / 通信できない・JSON でない・形が不正 → `adjust_unavailable`。
- `AbortSignal` を全操作で受ける(api/apiEngine.ts と同じ扱い): 呼ぶ前に abort 済みなら fetch せず `request_aborted`、
  通信中の abort も `request_aborted`(`adjust_unavailable` にしない)。signal を渡さなければ init に付けない。

### 4. 送信・取り消し

- API を呼ぶのは「調整する」を押したときと「覚えるポケモン」「続きを読み込む」を押したときだけ。マウント・入力の変更では呼ばない。
- 1回の送信で indices とモードの操作を並行して呼び(`Promise.all` 相当)、**両方そろってから**結果を出す。どれか1つでも失敗したら
  結果を出さずにエラーを出す(片方だけの結果で判断させない)。
- 送り直したら前の送信の `AbortController` を abort し、遅れて届いた古い応答で上書きしない(連番でも捨てる)。
  画面を閉じたら(unmount)計算中の呼び出しを abort する。`request_aborted` はエラーとして出さない。
- 計算中もボタンは押せる(押し直し = 取り消して送り直す)。結果の領域に `aria-busy="true"` と「計算中」を出す。
- 送信前の検査(違反なら API を呼ばず role=alert で理由): 自分のポケモン・性格、固定 SP の 0〜32・合計 66 以下、
  モードごとの必須(自分の技・相手のポケモン・相手の技)、上限 ≥ 固定 SP、`offense` の目標ありで攻撃の分類と自分の技の分類が一致、
  素早さの目標が 0 以上の整数、相手のプリセットに合う性格がマスタにあること。

### 5. リクエストの組み立て

- 個体は `{ speciesKey, level: 50, natureId, sp }` に、選んだときだけ `abilityId`・`itemId`。`format` は常に `single`。
- 省略可の欄は既定と同じなら送らない: `thresholdPercent` は 100(確定)なら省略、`modifier` は 4096 なら省略、
  `damageModifier` は送らない(等倍)。`ceiling` は回す能力だけ(bulk: hp/def/spd、offense: atk か spa と spe)を常に送る。
  `minSpeed` は生成型で必須なので常に送る(空欄は 0)。`goal` は「目標を指定する」のときだけ。
- **火力指数の補正はタイプ一致だけ**: 技のタイプが自分の種族のタイプに含まれれば `modifier = 6144`(×1.5)。持ち物・特性・テラスタル・
  天候は含めない(画面に注記する)。ADR-0250 §2 は補正の組み立てをクライアントの責務としたが、持ち物・特性の効果の倍率は
  オンラインのマスタに無い(`capabilities.effects` が false)ため、確実に組み立てられるタイプ一致だけを入れる。倒せる/耐えるの判定は
  探索 API がダメージ式で行うので、この省略で判定はぶれない(ADR-0150 §結果)。
- 技の選択肢は選んだ種族の learnset(オンラインは `resolveSpecies` が解決した技)のうち、変化技を除いたもの
  (変化技では指数・探索とも `invalid_input` になるため、選べないようにする)。

### 6. 表示とエラー

- 文言はすべて `i18n/ja.ts` の `adjustScreenText` から組み立て、画面のコードに日本語を直書きしない。
- 指数: 「実数値 H … / S …」、火力指数(技なしは「技を選ぶと出します」)・物理耐久指数・特殊耐久指数を `<ラベル> <値>` で出す。
- 16n: 「HP n(16n / 16n-1 / どちらでもない)」と、次・前の 16n / 16n-1 を「<ラベル>: HP n(H sp、±差)」、無ければ「<ラベル>: なし」。
- 最小 SP: 満たせるとき「A に n 振れば m発で倒せます」「H に a・B に b 振れば m発耐えます」、満たせないとき「振っても…」と、
  そのときの確率。配分: 残り SP、「指数が最大になる振り方」と「目標を満たす最小の振り方」(h3 の小見出し・region)を
  能力ポイント・合計・実数値・耐久指数・目標を満たすか(確率)・素早さの目標を満たすか(offense のみ)で出す。
  目標なしは「目標を指定すると、目標を満たす最小の振り方も出します」。
- 確率は engine の生値(ADR-0250 §6)を **0.1% 単位で切り捨て**て表示する(99.99% を「100%」と出して確定に見せない)。
- 未対応の印(ADR-0123・ADR-0215)は、既存の `unsupportedMarkLabels` で名前をマスタから引き、結果の先頭に `unsupportedText.notice`
  を1回出す。数値は印の有無で変えない。
- エラーは **code から `adjustErrorText` の日本語**を引き、未知の code は汎用の「調整に失敗しました」。サーバーの message
  (英語の内部メッセージを含みうる)は出さない(ADR-0411 §3 と同じ。判定画面の補助行の方式は採らない)。入力は消さない。

### 7. 技を覚えるポケモン(機能 1)

- 自分の技・相手の技の欄の横に「覚えるポケモン」ボタン(accessible name は「自分の技を覚えるポケモン」「相手の技を覚えるポケモン」。
  見える文字を含む)。技を選ぶまで押せない。押すと領域「この技を覚えるポケモン」を開き、見出し「<技名>を覚えるポケモン」と一覧を出す。
- 1ページ 50 件(`LEARNERS_PAGE_SIZE`。契約の既定 limit)を `offset` で読む。返った件数が 50 ちょうどなら「続きを読み込む」を出して
  次の offset を足していく(総数は返らない。ADR-0251 §1)。一致なしは「この技を覚えるポケモンはいません」、エラーは §6 と同じ写像を
  パネルの中の role=alert に出す(続きの読み込みの失敗は一覧を残し、「続きを読み込む」をもう一度押せる)。別の技で開き直したら前の呼び出しを abort し、一覧を置き換える。
- 逆引きは1技あたり1回の GET で、`getMovesByIds` の 64 件分割(ADR-0304 §3 追記)は関係しない。
- 一覧の行は種族名だけ(行から自分のポケモンに設定する操作は AJ6 では持たない)。

### 8. a11y・見た目

- 各領域は `section` + 見える見出し h2(region の名前と同じ語)。結果の中の小見出しは h3(region)。技を覚えるポケモンのパネルは兄弟領域と並ぶので h2。
- 欄の accessible name は「<領域の見出し>の<ラベル>」(例「自分のポケモン」)、見えるラベルは短い語(「ポケモン」)で、
  見えるラベルの文字を必ず含む(SC 2.5.3)。未選択の select は「ポケモンを選ぶ」「性格を選ぶ」「技を選ぶ」を出す。
- モードは `role="radiogroup"`(名前「調整の内容」)。固定 SP の欄と素早さの目標は説明の文を `aria-describedby` で結び付ける。
- エラーは role=alert。送信後もフォーカスは「調整する」に残す(結果へ勝手に移さない)。
- アニメーションは入れない(常時も操作時も。結果は差し替えるだけ)。画像は使わない。

## 却下した案

- **WASM で動かす**: §1 の理由。ADR-0250 §結果 の記述は本 ADR で置き換える(WASM の登録は残す)。
- **打鍵ごとに探索する(debounce)**: 耐久側の配分は最大 33³ 候補で数百 ms かかり、入力途中の値で探索しても使えない。ADR-0705 と同じく送信ボタンにする。
- **相手の SP を数値入力させる**: requirements.md の「プリセットを選ぶだけ」に反する。代表的な調整(プリセット)で足りる。
- **持ち物・特性の補正を指数に含める**: オンラインのマスタに効果の倍率が無い(`capabilities.effects` false)。含められないものを含めたように見せない。
- **サーバーの message を補助行に出す(判定画面の方式)**: 英語の内部メッセージが出る(ADR-0411 の背景と同じ問題)。

## 受け入れ条件と担当テスト

| # | 受け入れ条件 | テスト |
|---|---|---|
| AC1 | クライアントは契約どおりのパス・メソッド・ヘッダー・本文で呼び、失敗を `{code,message}` / `adjust_unavailable` / `request_aborted` に写し、例外を投げない | `web/src/adjust/adjustClient.test.ts` |
| AC2 | タブ「調整」が末尾にあり `/adjust` で開け、開いただけでは調整 API を呼ばない | `app/routes.test.ts`・`App.routing.test.tsx`・`App.test.tsx`・`App.masterSources.test.tsx`・`e2e/a11y.spec.ts` |
| AC3 | 「調整する」でだけ呼び、モードごとに契約どおりの request(下限・上限・プリセット・タイプ一致の補正・省略可の欄の省略)を組み立てる | `AdjustScreen.test.tsx` S1・S2・S3、`adjustRequest.test.ts` |
| AC4 | 送信前の検査で違反を日本語で止め、API を呼ばない | `AdjustScreen.test.tsx` S4 |
| AC5 | 指数・16n・最小 SP・配分・未対応の印を出し、確率を切り捨てで出す | `AdjustScreen.test.tsx` S5、`adjustFormat.test.ts` |
| AC6 | エラーは code から日本語で、サーバーの message を出さない。入力は消えない | `AdjustScreen.test.tsx` S6、`adjustFormat.test.ts` |
| AC7 | 送り直し・画面を閉じたときに前の呼び出しを abort し、古い応答で上書きしない | `AdjustScreen.test.tsx` S7 |
| AC8 | 技から覚えるポケモンの一覧を開け、ページング・空・エラーを扱う | `AdjustScreen.test.tsx` S8 |
| AC9 | 見える見出し・見えるラベル・未選択の文言・radiogroup・aria-describedby・aria-busy | `AdjustScreen.test.tsx` S9 |

## 結果

- Web は計算をしない(指数・探索・配分はすべて calc-svc の engine)。補正の組み立てはタイプ一致だけで、Web と iOS(AJ7)は同じ規則にそろえる。
- オフライン(WASM)で調整を使う道は残る(AJ6b 候補)。そのときは本 ADR の §1 を改める ADR を足す。
- E2E の実データ(k3d)での確認は calc-svc と pokedex-svc が要るため、AJ6 では a11y.spec.ts のタブ順だけを足し、
  画面の操作の E2E は足さない(オフラインの E2E では API 専用画面がマスタを読めないため。判定画面と同じ扱い)。
