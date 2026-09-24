# ADR-0300: Web(P4)の構成 — WASM 先行・計算の差し替え口・架空マスタ・攻撃側プリセット

- 状態: 採用(Web レーン、2026-09-21。§5 の攻撃側プリセットの置き場と §9 の `make test` への組み込みは DECISIONS.md に既定案付きで提案)
- 日付: 2026-09-21(2026-09-22 に ADR 番号の帯の規則(COORDINATION.md)に合わせて 0016 から 0300 に振り直した)
- 関連: plan.md P4-1〜P4-7、docs/design.md、docs/requirements.md「自分側のプリセット」「相手側の一括表示」「調整の推定」、
  ADR-0009(防御プリセット)、ADR-0010 §R(逆算)、ADR-0011(WASM 境界)、ADR-0013(相性表はデータ)、
  docs/ai-shared/COORDINATION.md「レーン間の依存と共有ファイル」、docs/coding-rules.md

## 背景

Phase 4 は Web(Vite + React + TypeScript)を作る。4レーン制(2026-09-21 ユーザー決定)で、Web レーンは
API レーン(`api/openapi.yaml`・calc-svc)とデータレーン(pokedex-svc・マスタ)を待たずに進める。
そのため、計算は WASM(ADR-0011 の JSON 契約)で先に作り、マスタは pokedex-svc ができるまで架空データで作る。
API 接続(P4-5)は API レーンが契約を main に入れてから追従する。決めるのは次の9点。

## 決定

### 1. 技術と依存(完全固定)

| 用途 | 依存 | ライセンス |
|---|---|---|
| 実行時 | react / react-dom 19.3.0 | MIT |
| ビルド | vite 8.3.0、@vitejs/plugin-react 6.1.1、typescript 7.0.2(別名 `typescript7`)| MIT / MIT / Apache-2.0 |
| テスト | vitest 5.0.1、jsdom 30.1.0、@testing-library/react 16.3.3・dom 10.4.2・user-event 14.6.7・jest-dom 7.0.1 | MIT |
| 整形・lint | eslint 10.11.0、typescript-eslint 8.70.0、eslint-plugin-react-hooks 7.1.1、prettier 3.9.8 | MIT |

- すべて `package.json` で完全固定し、`package-lock.json` をコミットする(コーディング規約 §1・§4、`check-publishable` の E 区分)。
- **版は最新の安定版にする**(ユーザー決定 2026-09-21。DECISIONS.md)。Node.js は 26.9.0 を `web/.node-version` と `engines` で固定する。
- 型検査は TypeScript 7.0.2(ネイティブ実装)。ただし typescript-eslint 8.70 は TS 7 の API に未対応(peer が `<6.1.0`)で、
  TS 7 は JS API を持たないため、ESLint が `require("typescript")` で読むパッケージだけ公式の互換パッケージ
  `@typescript/typescript6`(6.0.2)を `typescript` の別名で入れ、TS 7 は `typescript7` の別名で入れて `npm run typecheck` から直接呼ぶ。
  typescript-eslint が TS 7 に対応したら、別名をやめて `typescript` を 7 系1本にする。
- ルーター・状態管理・CSS フレームワークは入れない。画面は「計算」「逆算」の2つで、React の state で足りる
  (使われない汎用機構を作らない。コーディング規約 §3)。Playwright は P4-6 で入れる。
- **画面の切り替えは URL と連動させる**(2026-09-22 追記。P4-10。ユーザー要望): `/calc`・`/reverse` のように1画面1パス。ルーターのライブラリは入れず、
  History API(`pushState` / `replaceState` / `popstate`)と、画面 ID・パス・タブ名の対応を1か所に置いたルート表(`web/src/app/routes.ts`)で行う。
  画面を足すレーン(タイプバランスの P4-12、素早さの SP3)は、ルート表(`app/routes.ts`)・表示名(`i18n/ja.ts`)・画面のコンポーネントの対応(`app/screens.tsx`)に1件ずつ足す。
  `App.tsx` は触らない(`ScreenId` はルート表から導出し、対応は `Record<ScreenId, …>` なので足し忘れは型エラーになる)。
  nginx の SPA フォールバックは P4-11 で入れ、E2E で確かめる。`/` と未知のパスは `/calc` に置き換える(履歴を増やさない)。
  配信側(nginx・vite preview)は未知のパスで `index.html` を返す(SPA のフォールバック)。
- 初期ロードの予算(design.md: JS ≤ 300KB gzip、WASM 除く)は `web/scripts/check-bundle-size.mjs` が
  `npm run build` の最後に検査し、超えたら失敗する。

### 2. 計算の差し替え口 `CalcEngine`(WASM 先行、API は P4-5)

画面は計算の実装を知らず、`CalcEngine` インターフェース(`calc` / `calcBulk` / `calcReverse`)だけに依存する。

- 入出力の型は **ADR-0011 §3 の WASM 境界の DTO をそのまま TypeScript で書いたもの**(`web/src/engine/types.ts`)。
  WASM 境界は HTTP を通らず OpenAPI の契約ではないので、コーディング規約 §4「API の型は OpenAPI の生成型を正とする」の
  対象外(ADR-0011 §10)。ずれは §8 の結合テストが本物の `engine.wasm` で検出する。
- 戻り値は `{ok: true, value} | {ok: false, error: {code, message}}` の判別 union。境界のエラー封筒の `code`
  (ADR-0011 §5)を捨てずに画面まで運ぶ。
- WASM 実装(`createWasmEngine`)は `wasm_exec.js` と `engine.wasm` の読み込みを注入可能にし、ブラウザは
  `/wasm_exec.js`・`/engine.wasm`(`make wasm` が `web/public/` に出す)、Node の結合テストはファイルから読む。
  読み込みは初回の計算時に1回だけ行う(ページを開いた時点で 4.6MB を読まない)。
- P4-5 で API 実装を同じインターフェースの後ろに足す。「ID → 実体」の解決層もそこで置く(ADR-0011 §10 の持ち越し)。

### 3. マスタは `MasterData` の後ろに置き、いまは架空データ

- 画面が使うマスタ(種族・技・持ち物・特性・相性表)は `MasterData` 型1つにまとめ、`MasterSource` から受け取る。
  種族・技・持ち物・特性の形は WASM 境界の DTO と同じ(解決済みの実体をそのまま engine に渡せる)。
  種族が覚える技の一覧(`learnset`)だけは画面のための追加フィールド。
- いまの実装は **架空の例データ**(`web/src/master/example/`)。名前は `テスト` で始め、ID は `example-` で始め、
  図鑑番号は実在と重ならない 9001 以降にする(ADR-0002: 実マスタを Git に置かない)。
  効果の数値(倍率など)は engine の効果定義の形に合わせた架空の値で、実在の持ち物の再現ではない。
- **タイプ相性表だけは架空にしない**。架空の表では計算結果が画面の確認に使えないため。P1-13 のデータ
  `testdata/golden/typechart.json`(数値と英語 ID だけ。ADR-0002 §追加の回答でコミットが認められている)を、
  Vite の別名 `@typechart` で**複製せずに読む**(単一の正。ADR-0015 は balance に同梱するためバイト複製したが、
  Web はビルド時に読めるので複製しない)。Web レーンはこのファイルを読むだけで変更しない。
- レギュレーション(v1 は M-C)の絞り込みはマスタの責務で、画面は渡された一覧をそのまま出す(M-C をコードに書かない)。
- pokedex-svc ができたら `MasterSource` の API 実装に差し替える(P4-5 以降)。例データはテストの fixture として残す。

### 4. 画面の文言・タイプ表示名

- タイプの表示名(ノーマル・ほのお…)と画面の文言は `web/src/i18n/ja.ts` の文言資源に置く(コーディング規約 §2)。
  タイプの**一覧**は相性表のデータ(`types`)から引き、文言資源は ID → 表示名の対応だけを持つ。
- タイプ色は docs/design.md のトークン(P4-1 の CSS 変数 `--type-<id>`)が正。TypeScript 側に色の値を書かない。

### 5. 攻撃側(自分側)のプリセット(P4-3)

requirements.md「A特化 / A振り(補正なし)/ 無振り」を次の3件にする。X は技の分類で決まる関連ステータス
(物理 = `atk`、特殊 = `spa`)。ラベルと調整は ADR-0010 の attacker の表(順位 0〜2)と同じ呼び名・同じ調整にする。

| Key | 表示(物理 / 特殊) | SP | 性格 |
|---|---|---|---|
| `none` | 無振り | なし | 無補正 |
| `x_full` | A特化 / C特化 | X:32 | +X / −(`atk` なら `spa`、`spa` なら `atk`) |
| `x` | A振り(無補正)/ C振り(無補正) | X:32 | 無補正 |

- 下降補正の置き方は ADR-0010 §R1 の代表性格(`Minus: atk`、関連が atk のときだけ `Minus: spa`)と同じ。
  下降したステータスは使わないので、どちらでもダメージは変わらない。
- 残りの SP は振らない。攻撃側の HP・防御・素早さは engine のダメージ計算に使われないため(特性の HP 条件は engine の範囲外)。
- 置き場は Web(`web/src/domain/attackerPresets.ts`)。本来は ADR-0009 の防御プリセットと同じく engine が持つのが
  一貫するが、engine はデータレーンの範囲なので Web からは変えない。iOS(M3)が同じ定義を必要とする前に engine へ移すことを
  DECISIONS.md に提案する(既定案: データレーンが `AttackerPresetCatalog()` を engine と WASM 境界に足し、Web はそれに切り替える)。
- 「構築から個体を呼び出す」は team-svc(M2)が要るので P5-5 で行う。

### 6. 一括表示と持ち物の差し替え候補

- 防御側の行は engine の既定(`presetKeys` を省略すると技の分類で HB 系 / HD 系の5行。ADR-0009)をそのまま使い、
  Web でプリセットを定義し直さない。
- 「持ち物の差し替え候補」のトグルを入れると `itemVariants` に `[null, 候補...]` を渡す。候補はマスタの効果データから
  求める(ID・名前で選ばない): 技の分類に対応する防御側ステータスを上げる(`statMods.def` / `statMods.spd`)、または
  技のタイプを半減する(`resistBerryType` が技のタイプ)。requirements.md「逆算の持ち物候補」の防御側の分類と同じ。
- トグルが切りのとき、防御側のカードで持ち物を選んでいれば `itemVariants` に `[その持ち物]` を渡す(選ばなければ省略 = 持ち物なし)。
  トグルが入りのときは `[null, 選んだ持ち物(候補に無ければ), 候補...]`。選んだ持ち物を比較から落とさないため(重複はさせない)。

### 7. 逆算の画面(P4-4)

- 「与えたダメージ(相手の HP 減少%)」= `side: defender`、「受けたダメージ(自分の HP の減少量)」= `side: attacker`。
  観測は整数%(1〜100)か HP の実点数で入力し、`percent` / `damage` に入れる(小数の表示%を観測に使わない。ADR-0010 §R2)。
- 持ち物候補はマスタの効果データから requirements.md の分類で求める。防御側 = なし / 防御・特防を上げる / 技のタイプの半減きのみ、
  攻撃側 = なし / ダメージ倍率(`damageMod`)/ 分類の威力(`powerMod`)/ 技のタイプの強化(`boostType`)。
- 候補は engine の順序(ADR-0010 §R4)のまま表示し、並べ替えない。SP の範囲は `ranges` をそのまま全部出す(1区間に畳まない)。
- 目安の名前(ADR-0010 §R3「表示層が付ける」): 範囲に SP 0 / 32 が入っているとき、性格クラスとの組で名前を併記する。
  防御側(H32 前提): 0+補正なし = H振り、0+上昇 = H振り+B(D)補正、32+補正なし = HB(HD)振り、32+上昇 = HB(HD)特化。
  攻撃側: 0+補正なし = 無振り、0+上昇 = A(C)補正のみ、32+補正なし = A(C)振り、32+上昇 = A(C)特化。
  防御側は「H32 を仮定した結果」であることを `assumedHpSp` から画面に出す(ADR-0010 §R1)。
- `known` は常に自分、技は常に攻撃側のもの(engine/reverse.go)。「与えたダメージ」では自分の攻撃側を §5 のプリセットで作り、
  技は自分の覚える技。「受けたダメージ」では自分の防御側を SP 0・無補正に固定し(P4-4 の限界。構築の個体を使うのは P5-5)、技は相手の覚える技。
- 観測の単位の既定は、与えたダメージ = %、受けたダメージ = HP の実点数。空の行は送らず、不正な行が1つでもあれば計算しない。
  対象側を切り替えたら観測は1行の空に戻す(単位の意味が変わるため)。
- 推定はボタンを置かず、入力が有効になるたびに自動で行う(逆算1回は数 ms。ADR-0011 §9)。古い応答は捨てる。
- `maxCandidates` は送らない。候補は 2 × 持ち物候補数(十数件)で、切る必要が無い。
- 攻撃側の持ち物候補には、技の分類の攻撃ステータスを上げる持ち物(`statMods.atk` / `statMods.spa`)も含める。
  M-C には無いこだわり系が別レギュレーションで追加されたとき、マスタの差し替えだけで候補に戻るようにするため(requirements.md「逆算の持ち物候補」)。
  防御側の分類は §6 の一括表示の候補と同じ関数を使う(定義を1か所にする)。
- **型でまとめない**(2026-09-22 ユーザー決定。design.md を改めた): 型は SP 範囲から一意に決まらないので、候補は engine の順序のまま1件ずつ表示し、
  目安の名前を併記する。観測を追加したときの絞り込みの演出は P4-8 で入れる。

### 8. テストの層

| 層 | 対象 | 実行 |
|---|---|---|
| 単体・画面 | 純粋ロジック(リクエストの組み立て・候補の抽出・表示の書式)と画面(engine は fake) | `make web-test`(Vitest + jsdom) |
| WASM 結合 | Web が組み立てたリクエストを本物の `engine.wasm` に通し、成功の封筒と結果の形を確かめる | `make web-test-wasm`(`make wasm` に依存) |
| E2E | ブラウザでの主要フロー | P4-6(Playwright) |

- WASM 結合テストは前提(`engine.wasm` / `wasm_exec.js`)が無ければ**スキップせず失敗**する(CLAUDE.md)。
- 計算の正しさ(数値)は Web では見ない(ゴールデンと Go/WASM 一致テストの役割)。Web が見るのは
  「正しい入力を組み立て、返ってきた値を加工せずに表示する」こと。

### 9. Makefile への組み込み(2026-09-22 追記: ユーザー決定で改訂。P4-6)

- ルートの `Makefile` には `include web/Makefile` の1行だけを足し、ターゲットは `web-` 接頭辞(COORDINATION.md の共有ファイル規約)。
- **`make test` / `make lint` / `make build` に Web を含める**(ユーザー決定 2026-09-22。当初の既定案「まだ含めない」を改める)。
  `web/Makefile` が `test: web-test` などの前提条件を足す(balance と同じ形)。`web/node_modules/.package-lock.json` が
  `package-lock.json` より古い・無いときだけ `npm ci` する(`web-deps`)ので、他のレーンの作業ディレクトリでも初回は自動で依存が入る。
- E2E(Playwright)は `make web-e2e`(オフライン)/ `make web-e2e-online`(例データを書き出して calc-svc を起動。要 Go)。
  ブラウザの起動が要るので `make test` には含めない。初回は `make web-e2e-install` で chromium を入れる。ルートの `make e2e`(`scripts/e2e.sh`、k3d のスモーク)へのつなぎ込みは DECISIONS.md に提案する。

### 10. 候補・観測の件数上限の扱い(2026-09-23 追記。P4-19。issue 110 / ADR-0208 への追従)

API レーンが `api/openapi.yaml` に上限を入れた(`itemVariants` / `itemCandidates` は null を含めて 64、
`observations` は 16。ADR-0208、DECISIONS.md 2026-09-23)。engine(WASM)にも同じ上限が入るので、
オフラインでも上限超過は失敗する。Web は**上限を超える入力をそもそも作らない**。

- **値の正は `api/openapi.yaml`**。実行時に YAML は読めないので `web/src/domain/requestLimits.ts` に写し、
  `requestLimits.test.ts` が openapi.yaml を読んで一致を検査する(コーディング規約 §2 の「同期を検査するテスト」)。
  画面・ドメインの他の場所に 64 / 16 を直書きしない。
- **絞り込みは決定的に、先頭から**。§6・§7 の「マスタの順序をそのまま使う(並べ替えない)」を崩さず、末尾から落とす
  (`limitToMax`)。同じマスタ・同じ技からは常に同じ候補になる。
- **黙って切り捨てない**。落とした候補があるかを `truncated` で返し、画面が文言(`requestLimitText`)で明示する。
  P4-16b の `speciesSearchTruncated` と同じ UX(絞り込んだ事実を必ず見せる)。
- **上限を適用するのは最終的な配列を作る1か所**: 一括計算は `defenderItemVariants`(null と「選んだ持ち物」を足して
  配列を確定させるのはここだけなので、分類だけを行う `defensiveItemCandidates` には上限を置かない。
  上限を2か所に分けて「64 − 予約枠」を見積もるより、確定した配列を1か所で切るほうが境界を間違えない)。
  逆算は `reverseItemCandidates`(null を先頭に付けて配列を確定させるのがここ)。
- **利用者が選んだ持ち物は落とさない**。`defenderItemVariants` が切るときは、選んだ持ち物が落ちる位置にあれば
  最後に残る1枠をその持ち物に使う(マスタの順序は崩さない)。選んだ行が比較表から消えるほうが実害が大きいため。
- **観測は 16 件で「観測を追加」を `disabled` にする**(`canAddObservation`)。理由は `role="status"` の live region で
  読み上げ、ボタンからも `aria-describedby` で指す(無効なボタンは焦点を取れず説明が読まれないことがあるため、
  両方を使う)。1行削除すれば再び追加できる。

### 11. 入力の途中で計算を始めない・先行の計算を取り消す(2026-09-24 追記。P4-18。issue 113)

タイプバランスレーンの Codex レビュー(issue 113)。逆算の観測欄は「45」と打つ途中の「4」も有効な観測なので、
1文字ごとに `calcReverse` が走り、送信済みの古い要求も最後まで走り切っていた(cleanup の `cancelled` は
**応答の反映を捨てるだけ**で、計算・通信そのものは止めていない)。結果は正しいが、端末の計算・通信・calc-svc の CPU を
無駄に使う。#110 の「1要求の上限」とは別の問題(正常な大きさの古い要求が短時間に並ぶ)なので、別に抑える。

- **待つのは「数値テキストの編集」だけ、200ms の trailing debounce**(`OBSERVATION_INPUT_DEBOUNCE_MS`。
  値の置き場は `web/src/domain/observations.ts` = 観測の入力を所有するモジュール1か所)。
  **表示と入力の検証は待たない**: 打った文字はその場で出し、不正(1〜100 の外など)もその場で知らせ、
  「計算中」もその場で出す。待つのは engine の呼び出しだけ。
- **確定した操作は待たない**: 種族・持ち物・技の選択、観測の単位の切り替え、観測行の追加・削除、対象側の切り替え。
  待機中のテキストがあれば、その確定操作と一緒に反映して**1回だけ**計算する(待機中のタイマーは必ず解除する)。
  画面が消えるときもタイマーを片付ける(回しっぱなしにしない。§7 の絞り込みの最大待ちと同じ作法)。
- **`CalcEngine` の3つの計算に任意の第2引数 `signal?: AbortSignal` を足す**(`web/src/engine/types.ts`)。
  第2引数の任意引数にすると、既存の実装・呼び出し側を変えずに足せて、`MasterSpeciesSearch.searchSpecies(query, signal)`
  (ADR-0304)と同じ形になる。オプションのオブジェクトにはしない(今のところ渡すものが1つしかないため)。
- **取り消しは `engine_unavailable` と区別する**(`REQUEST_ABORTED_CODE = "request_aborted"`)。
  取り消しは engine・API の失敗ではないので、同じ code にすると「API に接続できません」を出しかねない。
  画面は先行の応答を `cancelled` で捨てる(既存)ので利用者には見えないが、**境界が嘘の理由を返さない**ようにする
  (コーディング規約 §3「エラーは握りつぶさず、原因が分かる文脈を付けて返す」)。code の正は `engine/types.ts` の1か所。
- **API 実装**は `signal` を `fetch` の init にそのまま渡し、呼ぶ前に abort 済みなら fetch せずに `request_aborted` を返す。
  fetch / 本文の読み取りが abort で失敗したときも `request_aborted`(abort していない通信失敗は従来どおり `engine_unavailable`)。
- **WASM 実装は「始める前」にだけ signal を見る**。境界関数はブラウザのメインスレッドで同期実行するので、
  始まった計算は JS からは止められない。**取り消せるふりをしない**: 始める前(読み込みの前と、読み込みが終わった直後)に
  abort 済みなら計算せずに `request_aborted`、返ってきた結果を後から取り消し扱いに書き換えることはしない。
  無駄な計算を実際に減らすのは debounce の役目で、signal は「待っている間に要らなくなった計算」を止める役目。
- **画面は最新の計算を1つだけ持つ**: effect ごとに `AbortController` を作り、cleanup で `abort()` する
  (`cancelled` は最後の砦として残す)。
- **対象は逆算の画面だけ**(issue 113 の主眼)。`CalcScreen` の `calcBulk` はインターフェースが任意引数なので今までどおり動く。
  計算画面の debounce 化・abort 化は別タスク(この ADR の対象外)。
- **iOS・API レーンは別担当**。同じ 200ms の契約に各レーンの区切りで追従する(issue 113 の受け入れ条件)。
- **演出・通知は変えない**: 絞り込み(§7)・視差効果を減らす設定・`role="status"` の読み上げの挙動は据え置き。
- **実装(`ReverseScreen.tsx`)は観測を2つの state に分ける**: `observations`(即時。表示・入力検証用)と
  `requestRows`(計算のトリガー用。テキスト編集は debounce の発火時にだけ追いつく)。両者が同じ参照かどうかで
  「待機中(debouncePending)」を判定する(値の比較ではなく参照の比較。確定操作は両方を同じ参照で同時に
  更新するので、待機中でなければ常に同じ参照になる)。「計算中」表示は `debouncePending` も条件に含める
  (前の結果が出ている状態で新しい値を打った直後、古い結果を出し続けない)。

## 却下・保留

- **Web で engine を TypeScript に書き直す / 一部の計算を JS で持つ**: 計算は WASM(同じ engine)か API だけ。
  表示のための加工(書式・ラベル)以外の計算を Web に持たない。
- **相性表を web/ に複製する**: ビルド時に読めるので不要。複製すると同期の検査が要る。
- **架空の相性表**: 画面の数値が確認に使えなくなる。§3 のとおり P1-13 のデータを読む。
- **状態管理ライブラリ(Redux 等)・ルーター**: 2画面では不要。必要になったら ADR を追加する。

## 影響

- `web/` を新設(Vite + React + TS)。ルートの `Makefile` に `include web/Makefile`。
- engine・`api/openapi.yaml`・他レーンのファイルは変更しない。
- DECISIONS.md に2件を提案(§5 攻撃側プリセットの engine への移管、§9 `make test` への組み込み)。
