# ADR-0016: Web(P4)の構成 — WASM 先行・計算の差し替え口・架空マスタ・攻撃側プリセット

- 状態: 採用(Web レーン、2026-09-21。§5 の攻撃側プリセットの置き場と §9 の `make test` への組み込みは DECISIONS.md に既定案付きで提案)
- 日付: 2026-09-21
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
- **持ち越し**: design.md「画面: 逆算」の「候補を型の名前でまとめる」表示と、観測を追加したときの絞り込みの演出は P4-4 では入れていない
  (候補は engine の順序のまま1件ずつ表示し、目安の名前を併記するだけ)。M1 の動作確認(P4-7)の後に、必要なら改善要望として足す。

### 8. テストの層

| 層 | 対象 | 実行 |
|---|---|---|
| 単体・画面 | 純粋ロジック(リクエストの組み立て・候補の抽出・表示の書式)と画面(engine は fake) | `make web-test`(Vitest + jsdom) |
| WASM 結合 | Web が組み立てたリクエストを本物の `engine.wasm` に通し、成功の封筒と結果の形を確かめる | `make web-test-wasm`(`make wasm` に依存) |
| E2E | ブラウザでの主要フロー | P4-6(Playwright) |

- WASM 結合テストは前提(`engine.wasm` / `wasm_exec.js`)が無ければ**スキップせず失敗**する(CLAUDE.md)。
- 計算の正しさ(数値)は Web では見ない(ゴールデンと Go/WASM 一致テストの役割)。Web が見るのは
  「正しい入力を組み立て、返ってきた値を加工せずに表示する」こと。

### 9. Makefile への組み込み

- ルートの `Makefile` には `include web/Makefile` の1行だけを足し、ターゲットは `web-` 接頭辞(COORDINATION.md の共有ファイル規約)。
- `make test` / `make lint` には**まだ含めない**。含めると、`web/node_modules` の無い他のレーンの作業ディレクトリで
  ルートの `make test` が失敗する。含めるかどうかは DECISIONS.md に提案する(既定案: P4-6 で E2E を入れるときに、
  `node_modules` が無ければ `npm ci` してから実行する形で `make test` / `make lint` に加える)。

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
