# ADR-0011: WASM 境界(JSON 文字列 API)とビルド・Go/WASM 一致テスト

- 状態: 採用(ただし §9 の「ブラウザでの実動作確認」は未実施。P4-5 で人間が確認する)
- 日付: 2026-09-21
- 関連: P1-9、docs/requirements.md「オフライン: ブラウザ版はWASMでバックエンドなしでも計算可能」
  (非機能要件・アーキテクチャ図・技術スタック「オフライン計算 Go → WASM」)、
  docs/test-strategy.md「E2E: WASM モードでバックエンドを止めても計算できる」、
  ADR-0005(データ駆動の効果定義)、ADR-0009(一括計算のプリセット)、ADR-0010(逆算)、
  CLAUDE.md 絶対ルール1(API は openapi.yaml から)・2(engine は純粋)・3(ゴールデン全件一致)・
  6(テストを弱めない)、ドメイン規約(持ち物・技・ポケモンをハードコードしない)

## 背景

engine(P1-1〜P1-8)はできている。要件は「ブラウザ版はバックエンドなしでも計算できる」ことなので、
同じ engine を WASM にして JS から呼べるようにする。決めるべきは次の6点。

1. JS と Go の間で何をどう渡すか(境界の形)
2. その変換コードをどこに置くか(engine の純粋性を壊さず、かつテストできる場所)
3. エラーをどう返すか(panic を JS に漏らさない)
4. 「Go と WASM が同じ結果を返す」を何で確かめるか
5. ビルド成果物をどう作り、どこに置き、コミットするか
6. 逆算(格子 3,267 点)が WASM で実用的な速度か

`api/openapi.yaml` は**今回変更しない**。WASM は HTTP を介さないので API 契約ではなく、
契約とのズレは §10 に記録して P3-1 / P4-5 で解消する。

## 決定

### 1. 境界の変換は `engine/wasmapi`(build タグ無しの純粋パッケージ)に置く

`engine/wasmapi` を新設し、DTO 定義・JSON エンコード/デコード・列挙の検証・エラー整形を全部ここに置く。

```
engine/wasmapi/          ← 変換・検証・エラー整形(build タグ無し。ネイティブ Go でテストできる)
engine/cmd/wasm/         ← syscall/js での登録だけ(//go:build js && wasm)
engine/cmd/wasmexpect/   ← 一致テストの期待値をネイティブ Go で生成する道具
```

置き場所の理由:

- **`engine` 本体パッケージに入れない**。DTO を入れると `encoding/json` 依存とフィールドの
  JSON 表現が engine の関心事になり、「engine は純粋」(絶対ルール2)の線が濁る。
  また engine の型に `json` タグを足すのは**却下**(後述)。
- **`engine/cmd/wasm` の中に入れない**。`//go:build js && wasm` の下にあるコードは
  `go test`(ネイティブ)から見えず、`make test` で守れない。境界のバグは静かに残る。
- **`engine` モジュールの中に置く**。`services/` から見れば calc-svc は同じ engine を直接呼ぶので
  このパッケージは要らないが、モジュールを分けると `go.work` と Makefile の配線が増える。
  `engine/wasmapi` は engine と標準ライブラリにしか依存せず、その不変条件はテストで固定する
  (`TestBoundaryPackageImportsAreMinimal`)。

`engine/wasmapi` は engine の公開 API(`CalcDamage` / `CalcBulk` / `CalcReverse` / `RealStats` /
`DisplayPercent`)を呼ぶだけで、独自のダメージ式・独自の丸めを持たない。
これは「DTO 出力と engine 直呼びの結果が一致する」テストで固定する(§8)。

絶対ルール2(engine は純粋)が守る対象は **engine のライブラリパッケージ**(`engine` 本体と
`engine/wasmapi`)で、`cmd/` 配下の開発用ツール(`engine/cmd/wasmexpect` は `os` / `flag` を使う)は対象外。
ライブラリ側は `TestBoundaryPackageImportsAreMinimal` で「engine と標準ライブラリだけ」を固定している。

### 2. 境界は「JSON 文字列 → JSON 文字列」の3関数

WASM エントリ(`engine/cmd/wasm`)は起動時に次を登録し、`select{}` でブロックする。

```js
globalThis.pokecalc = {
  calc(requestJSON)        // → responseJSON
  calcBulk(requestJSON)    // → responseJSON
  calcReverse(requestJSON) // → responseJSON
}
globalThis.pokecalcReady = true   // 登録が終わった目印(最後に立てる)
```

- 3関数を1つのグローバル名前空間 `pokecalc` にまとめる。グローバルを3つ生やすより衝突しにくく、
  「未登録」を1か所で判定できる。`pokecalcReady` は Node/ブラウザ双方で起動待ちに使う。
- **引数と戻り値は文字列1つだけ**。`js.Value` のオブジェクトを組み立てて返すと、
  境界の形が Go のコードに散らばり、ネイティブ Go でテストできなくなる。
  文字列なら `engine/wasmapi` の関数シグネチャがそのまま境界の契約になる。
- リクエストは**解決済みの engine 入力**(種族・技・持ち物・特性の実体と効果定義)。
  WASM 内にマスタは持たない。マスタ解決は Web 側(P4-5)が API / アセットから行う。

### 3. JSON 契約(フィールド名は lowerCamelCase)

engine の型に `json` タグを足さず、`engine/wasmapi` の DTO で定義する。
engine のフィールドを改名しても、DTO のタグが変わらない限り境界は壊れない(逆も同じ)。

共通:

```jsonc
Stats   = {"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0}
Nature  = {"plus":"atk","minus":"spa"}          // "" は無補正。HP は不可(engine が拒否)
Ranks   = {"atk":0,"def":0,"spa":0,"spd":0,"spe":0}
Screens = {"reflect":false,"lightScreen":false,"auroraVeil":false}
Species = {"key":"snorlax","dexNo":143,"form":0,"nameJa":"カビゴン",
           "types":["normal"],"baseStats":Stats,"abilities":["thickfat"]}
Move    = {"id":"bodyslam","nameJa":"のしかかり","type":"normal","category":"physical",
           "power":85,"priority":0}
Item    = {"id":"lifeorb","nameJa":"いのちのたま","effect":ItemEffect}      // 持ち物なしは null
Ability = {"id":"thickfat","nameJa":"あついしぼう","effect":AbilityEffect}  // 効果なしは effect:null
ItemEffect    = {"statMods":{"atk":6144},"damageMod":0,"powerMod":0,"powerCategory":"",
                 "onlySuperEffective":false,"boostType":"","boostTypeMod":0,"resistBerryType":""}
AbilityEffect = {"stabMod":0,"offBoostType":"","offBoostTypeMod":0,
                 "defResistType":{"fire":2048},"reduceSuperEffective":0,"ignoresBurn":false}
Individual = {"species":Species,"level":50,"nature":Nature,"ability":Ability,"item":Item,
              "sp":Stats,"ranks":Ranks,"teraType":"","status":"none"}
Field   = {"weather":"none","terrain":"none",
           "attackerScreens":Screens,"defenderScreens":Screens}
```

リクエスト:

```jsonc
// calc
{"format":"single","attacker":Individual,"defender":Individual,"move":Move,
 "field":Field,"critical":false}

// calcBulk(ADR-0009)
{"format":"single","attacker":Individual,"defenderSpecies":Species,"move":Move,
 "field":Field,"critical":false,
 "presets":[{"key":"hb","label":"HB特化","sp":Stats,"nature":Nature,"applies":"physical"}],
 "presetKeys":["none","hp","hb"],
 "itemVariants":[null, Item, Item]}

// calcReverse(ADR-0010)
{"format":"single","side":"defender","known":Individual,"unknownSpecies":Species,"move":Move,
 "field":Field,"critical":false,
 "itemCandidates":[null, Item],
 "observations":[{"percent":45,"damage":0,"note":"1発目"}],
 "maxCandidates":10}
```

レスポンス(成功は `result`、失敗は `error` のどちらか一方だけ):

```jsonc
// calc
{"result":{"rolls":[16個],"minDamage":208,"maxDamage":246,"minPercent":63,"maxPercent":75,
           "defenderHP":330,"effectiveness":1,"stab":true,"category":"physical",
           "ko":{"hits":2,"guaranteed":true,"chancePercent":0}}}

// calcBulk
{"result":{"defenderSpeciesKey":"blissey","rows":[
  {"preset":"hb","presetLabel":"HB特化","itemId":"eviolite",
   "defender":{"sp":Stats,"nature":Nature,"stats":Stats},   // stats は engine.RealStats
   "result":{ /* calc と同じ CalcResult */ }}]}}

// calcReverse
{"result":{"side":"defender","stat":"def","exactCount":15,"candidates":[
  {"archetype":{"key":"hfull-bfull-neutral","label":"HB振り(無補正)","side":"defender",
                "stat":"def","hpBucket":"full","statBucket":"full","natureClass":"neutral"},
   "itemId":"","sp":Stats,"nature":Nature,
   "matchScore":1,"exact":true,"minPercent":40,"maxPercent":48,
   "points":9,"exactPoints":3}]}}

// エラー
{"error":{"code":"invalid_enum","message":"天候 \"sunny\" は未知"}}
```

決めたこと:

- **数値は整数のまま**渡す。`minPercent` / `maxPercent` は `engine.DisplayPercent`(整数%)を使い、
  float にしない(ドメイン規約)。float で出るのは `effectiveness`(0/0.25/0.5/1/2/4)、
  `ko.chancePercent`、`matchScore` の3つだけで、いずれも engine が float64 で持っている値。
- **レスポンスは改行で終わらせない**(`json.Encoder` ではなく `json.Marshal`)。
  Go/WASM のバイト一致比較で末尾改行の有無に足を取られないため。
- 一括の `rows[].defender` は SP・性格・実数値だけを返す。個体を丸ごと返すと種族データを
  行数ぶん繰り返すことになり、画面が必要とするのはこの3つだけ。

### 4. 未知フィールドは拒否、列挙は境界で検証する

- デコードは `json.Decoder` + `DisallowUnknownFields`。誤記(`"attackerr"`, `"hiddenPower"`)が
  黙って無視され、無補正の結果が返るのを防ぐ。
- 列挙(タイプ・分類・天候・フィールド・状態異常・ステータスキー・形式)は **engine の定数と
  同じ文字列**で受け、一致しないものは `invalid_enum` にする。engine は未知の天候を
  「補正なし」として素通しするので、境界で止めないとタイプミスが静かに間違った答えになる。
  対象は `format` / `status` / `weather` / `terrain` / `move.category` /
  `presets[].applies` / すべての Type(`species.types`, `move.type`, `teraType`,
  `boostType`, `resistBerryType`, `offBoostType`, `defResistType` のキー)/
  すべての StatKey(`sp`・`ranks` のキー、`nature.plus/minus`、`statMods` のキー)。
- **空文字 `""` の扱い**(実装の実測どおり。空を「既定値」として受ける列挙と、拒否する列挙を分ける)。

  | 項目 | `""` | 意味 |
  |---|---|---|
  | `format` | 許す | `single` |
  | `field.weather` / `field.terrain` | 許す | `none` |
  | `attacker.status` などの `status` | 許す | `none` |
  | `teraType` | 許す | テラスタルなし(TypeNone) |
  | `nature.plus` / `nature.minus` | 許す | 補正なし |
  | `move.type` | 許す | TypeNone(タイプなし) |
  | `item.effect.powerCategory` | 許す | 全分類 |
  | `item.effect.boostType` / `resistBerryType` | 許す | 対象なし |
  | `ability.effect.offBoostType` | 許す | 対象なし |
  | `presets[].applies` | 許す | 全分類 |
  | `move.category` | **拒否**(`invalid_enum`) | 分類の無い技は計算の意味が決まらない |
  | `species.types[]` の各要素 | **拒否**(`invalid_enum`) | 空要素は「タイプなし」ではなく入力ミス。個数の誤りは `invalid_input` |
  | `statMods` のキー / `defResistType` のキー | **拒否**(`invalid_enum`) | 空キーは意味を持たない |

  省略(フィールド自体が無い)は `""` と同じ扱い。未知の文字列だけが `invalid_enum` になる。
  `move.category` を省略すると `""` になって拒否されるので、技は分類を必ず渡す。
- **`side` と `presetKeys` は境界で列挙検証しない**。engine が sentinel
  (`ErrInvalidReverseSide` / `ErrUnknownPreset`)を持っているので、そちらの写像に一本化する。
  検証を二重に持つと、どちらが先に鳴るかで code が変わる。

### 5. エラーは封筒で返す。panic は JS に漏らさない

`{"error":{"code":"...","message":"..."}}`。`code` は engine の sentinel を写した安定した文字列で、
画面とテストが依存する。`message` は日本語の説明(Go のランタイム情報を含めない)。

| code | 起点 |
|---|---|
| `invalid_json` | JSON として壊れている / 型が合わない |
| `unknown_field` | 契約にないフィールド(`DisallowUnknownFields`) |
| `invalid_enum` | §4 の列挙値が未知 |
| `invalid_input` | `engine.Individual.Validate`(SP 範囲・合計、ランク、性格が HP、タイプ数、レベル) |
| `unknown_preset` | `engine.ErrUnknownPreset` |
| `duplicate_preset` | `engine.ErrDuplicatePreset` |
| `invalid_preset` | `engine.ErrInvalidPreset` |
| `invalid_reverse_side` | `engine.ErrInvalidReverseSide` |
| `no_observation` | `engine.ErrNoObservation` |
| `invalid_observation` | `engine.ErrInvalidObservation` |
| `internal` | 上記以外(回復した panic を含む) |

写像は `errors.Is` で行う(engine 側はすべて `%w` でラップ済み)。
`js.FuncOf` のコールバックと `engine/wasmapi` の各関数の両方で `recover` し、
panic は `internal` 封筒にして返す。JS 側に Go の panic を投げない
(投げると `go.run` の Promise が reject して以後すべての呼び出しが死ぬ)。

### 6. `make wasm` は `web/public/` に2ファイルを出す

`scripts/wasm.sh`(`set -euo pipefail`)が次を行う。

1. `cd engine && GOOS=js GOARCH=wasm go build -trimpath -o ../web/public/engine.wasm ./cmd/wasm`
2. `wasm_exec.js` を Go の配布物からコピーする。
   **Go 1.24 以降は `$(go env GOROOT)/lib/wasm/wasm_exec.js`**(このリポジトリの実測:
   `/opt/homebrew/Cellar/go/1.27.1/libexec/lib/wasm/wasm_exec.js`、16,992 bytes)。
   1.23 以前の `$(go env GOROOT)/misc/wasm/wasm_exec.js` にもフォールバックし、
   どちらも無ければ**エラーで終了する**(黙って古いファイルを使わない)。

サイズ実測。次の表は**設計時のプローブ値**(engine 全体 + `encoding/json` + `syscall/js` を含む
使い捨てプログラム、go1.27.1 darwin/arm64)で、最終成果物の実測値は表の下に書く:

| ビルド | raw | gzip -9 |
|---|---|---|
| 既定 | 4,666,353 B (4.45 MiB) | 1,276,463 B (1.22 MiB) |
| `-trimpath` | 4,653,842 B | 1,276,046 B |
| `-trimpath -ldflags="-s -w"` | 4,569,554 B | 1,252,610 B |

**最終成果物の実測**(実装後の `make wasm`、`-trimpath`、go1.27.1 darwin/arm64):
`web/public/engine.wasm` = **4,851,816 B(4.63 MiB)/ gzip -9 で 1,323,675 B(1.26 MiB)**、
`wasm_exec.js` = 16,992 B。プローブより約 0.2 MB 大きい(境界の DTO・列挙検証・`errors`/`strconv` 等が加わるため)。
以降のサイズの議論(§11)はこの実測値を使う。

- **`-trimpath` は採用**(ビルド環境の絶対パスをバイナリに入れない。再現性)。
- **`-ldflags="-s -w"` は却下**。gzip 後で 23 KB(1.8%)しか減らないのに、
  ブラウザのコンソールでスタックトレースの関数名が読めなくなる。
  配信サイズが問題になるのは gzip 1.2 MB の側なので、効くのは tinygo か分割配信であって
  シンボル削除ではない(§11)。
- 生成物(`web/public/engine.wasm` / `web/public/wasm_exec.js`)は**コミットしない**。
  `.gitignore` には P0 時点で既に両方入っている(追加作業なし)。

### 7. テストは2段。ネイティブは `make test`、一致は `make test-wasm`

| テスト | 何を見るか | 実行 |
|---|---|---|
| `engine/wasmapi/*_test.go`(build タグ無し) | DTO 変換・列挙・未知フィールド・エラー写像・engine 素通し・ステートレス | `make test` |
| `scripts/wasm-conformance.mjs` | ネイティブ Go と WASM の出力が**バイト一致**すること | `make test-wasm` |

`make test-wasm` の仕様(implementer が Makefile に追加する):

```make
.PHONY: test-wasm
test-wasm: wasm ## Go と WASM の結果一致テスト(Node)
	@node scripts/wasm-conformance.mjs
```

- `wasm` に依存させ、常に最新のバイナリで照合する。
- `make test` には**含めない**(WASM ビルドと Node が要る。`make test` < 1分 の目標を守る)。
  代わりに `engine/wasmapi/wiring_test.go` が「`make test-wasm` と `scripts/wasm.sh` が
  本実装されていること」を `make test` 側で固定し、配線忘れが緑にならないようにする。
- `scripts/wasm-conformance.mjs` は前提(`engine.wasm` / `wasm_exec.js` / `go` / ベクタ)が
  欠けたら**スキップせず exit 1**。「未実装ターゲットやスキップの正常終了を成功と数えない」
  (CLAUDE.md / docs/development-workflow.md)。

**期待値はコミットせず、実行のたびに生成する。**
`engine/cmd/wasmexpect` がベクタを `engine/wasmapi` に通して
`{"<ベクタ名>":"<レスポンス JSON>"}` を一時ファイルに書き、Node 側がそれと WASM の出力を比較する。

- コミットする方式(SHA マニフェスト + 再生成手順)は**却下**。この一致テストが見たいのは
  「同じコードが2つの実行環境で同じ結果を出すか」であって、期待値そのものの正しさではない。
  正しさは `make test-golden` が @smogon/calc と照合して担保している(ADR-0008)。
  期待値をコミットすると、engine を直すたびに再生成が要り、陳腐化検知という二つ目の仕事が増える。
- Node 側は期待値が `result` を持たないベクタがあれば先に落ちる。
  ネイティブ側が未実装のまま WASM 側と「同じエラー」で一致して緑になるのを防ぐ。

ベクタは `engine/wasmapi/testdata/vectors.json`(schemaVersion 1)に 32 件。

| 系統 | 件数 | 中身 |
|---|---|---|
| ダメージ | 24 | `testdata/golden/fixed.json` から 20 件(天候4・フィールド・壁2・持ち物4・特性3・急所2・やけど・確定数)+ 手書き4(既定値省略・タイプ無効・変化技・半減相性) |
| 一括 | 4 | 既定プリセット(物理/特殊)・`presetKeys` × `itemVariants`(null 込み)・カスタムプリセット |
| 逆算 | 4 | defender%1回 / defender%2回 / defender 実点数 + 持ち物候補 / attacker 実点数 |

ダメージ系はゴールデンの固定ケースから機械変換した(フィールド名を lowerCamelCase にするだけ)。
入力の出どころを1つにしておくと、P2 で種族集合を差し替えたときに同じ変換で作り直せる。

### 8. 「engine の素通しである」ことをネイティブ側で固定する

`engine/wasmapi` のテストは、同じベクタを

- `engine/wasmapi` の関数(JSON 文字列)
- `engine.CalcDamage` / `CalcBulk` / `CalcReverse` の直呼び(Go の値)

の両方に通し、**全フィールドの数値が一致する**ことを確かめる。
DTO が独自に丸めたり、ロールを並べ替えたり、`minPercent` を float で作ったりしたら落ちる。

### 9. 性能と堅牢性(実測)

go1.27.1 / Node v26.4.0 / darwin arm64、`web/public` と同じ標準 wasm ビルドで測定:

| 項目 | 実測 |
|---|---|
| `WebAssembly.instantiate` | 6.0 ms |
| `calc` 1回 | 約 25 µs(1,000回で 25.2 ms) |
| `calcReverse` 1回(格子 3,267 点 × 持ち物2 = 6,534 回の CalcDamage) | 初回 33.8 ms / 2回目以降 16〜17 ms |

受け入れ基準「ブラウザで対話的に使える = 逆算1回 1 秒以内」に対して 2 桁の余裕がある。
`scripts/wasm-conformance.mjs` は `--max-reverse-ms`(既定 1000)を超えたら失敗する。

**浮動小数点のバイト一致**: `ko.chancePercent` は `koProbability` の畳み込み(float64)で、
arm64 では `a + b*c` が FMA に融合されうるため wasm(FMA 命令なし)と差が出る恐れがある。
ゴールデンの固定ケース 203 件すべてでネイティブと WASM の JSON を比較したところ
**203/203 がバイト一致**し、うち 59 件は小数付きの `chancePercent`
(例 `49.58343505859375`, `96.62551879882812`)だった。
理由は `dist[s] * p` の `p = 1/16` が2の冪で乗算が厳密なため、融合しても結果が変わらないこと。
この性質に依存しているので、**engine の確率計算を変えたらこの一致テストが番人になる**。
もし将来ずれたら、テストを緩めるのではなく DTO 側で量子化するかを ADR で決め直す(絶対ルール6)。

**ステートレス性**: 一致テストは全ベクタを順方向・逆方向の2周呼び、2回目が1回目と
バイト一致することを確かめる。ネイティブ側でも不正入力を挟みながら同じ入力を繰り返す
テストを置く。境界にグローバル状態・キャッシュを持たせない。

### 10. `api/openapi.yaml` との差分 — P1-9 では API を変更しない

WASM は HTTP を通らないので API 契約ではない。ズレを記録して P3-1(calc-svc)/ P4-5(Web)で解消する。

| 契約(openapi.yaml) | WASM 境界 | 解消 |
|---|---|---|
| `CalcRequest.moveId: string`(ID をサーバが解決) | `move: Move`(解決済み) | P4-5。Web は「オンライン=ID を送る / オフライン=解決済みを渡す」の2経路を持つ。マスタ解決は Web の責務 |
| `Individual` は ID 参照(speciesKey / itemId / abilityId) | 種族・持ち物・特性の実体 | 同上。Web に「ID → 実体」の解決層を1つ置き、両経路で共有する |
| `CalcResult.minPercent/maxPercent: number` | 整数(`DisplayPercent`) | P3-1。calc-svc も整数で返し、description に丸めを書く |
| `CalcResult` に `category` が無い | `category` を返す | P3-1 で契約に足すか、Web が要求から知っているので落とすかを決める |
| `BulkCalcRow` に防御側の SP/性格/実数値が無い | `defender{sp,nature,stats}` | P3-1 で `BulkCalcRow` に足す |
| 逆算の契約差分 | ADR-0010 §9 がそのまま当てはまる | P3-1 |
| エラーは HTTP ステータス + `Error` スキーマ | `{"error":{"code","message"}}` | P3-1 で `code` の語彙を共通化する(同じ失敗が両経路で同じ `code` になるように) |

### 11. 限界(いまは受け入れる)

- **ブラウザでの実動作は未確認**。Node + `wasm_exec.js` は同じ `engine.wasm` を同じ方法で
  読むが、ブラウザ特有の事情(MIME、`WebAssembly.instantiateStreaming`、キャッシュ、
  Service Worker、メモリ上限)は見ていない。**人間の確認は P4-5**(Web 実装)で行う。
- **配信サイズ 4.63 MiB = 4,851,816 B(gzip -9 で 1,323,675 B = 1.26 MiB)**(§6 の最終実測)。初回ロードは体感できる大きさ。
  改善は tinygo か「オンラインは API、オフラインだけ WASM を遅延ロード」の設計であって、
  リンカフラグではない。P4-5 で遅延ロードにするかを決める。
- **`engine/cmd/wasm` 自体はネイティブでテストできない**。だから中身を最小にし、
  変換・検証を全部 `engine/wasmapi` に寄せた。それでも「登録漏れ」「引数の個数違い」は
  Node 側の一致テストでしか捕まらない。
- **JSON キーは大文字小文字を区別しない**。`encoding/json` はキーを大文字小文字無視でフィールドに
  マッチさせるので、`ATTACKER` や `Attacker` も `attacker` として通る(`DisallowUnknownFields` でも防げない)。
  契約は lowerCamelCase だが、境界は綴りの大文字小文字までは強制しない。
- **重複キーは後勝ちで黙って採用される**。`{"critical":false,"critical":true}` は `true` になる
  (`encoding/json` の仕様で、エラーにはならない)。境界で検出するには自前のトークン走査が要るので、
  誤りが実害になる形(呼び出し側が生成した JSON の重複)は想定せず、いまは受け入れる。
- **ダブル・テラスタル**は engine の対応範囲どおり(`format` は渡すだけ、`teraType` は
  受け取るが engine は未使用)。境界は先に口を開けておく。
- **ベクタは 32 件**。ゴールデンの網羅(1,392 種族)を WASM 側で回してはいない。
  Go と WASM の差は補正の種類ではなく実行環境から出るので、補正を1つずつ通す固定ケースで足りる
  と判断した。全件回したくなったら `-vectors` に別ファイルを渡せる。

### 12. 受け入れ条件(AC-1〜AC-10)と担当テスト

P1-9 の受け入れ条件と、それを守るテスト。AC の番号はテストファイルの節見出し(`wasmapi_test.go` /
`vectors_test.go` / `wiring_test.go`)に書かれているものと同じ。

| AC | 内容 | 担当テスト |
|---|---|---|
| AC-1 | 成功は `{"result":…}` の封筒だけ。`result` のキー集合が契約どおり。末尾に改行を付けない | `TestSuccessEnvelopeShape` |
| AC-2 | 境界は engine の素通し。DTO 出力が `CalcDamage` / `CalcBulk` / `CalcReverse` の直呼びと全フィールドで一致する(独自計算・独自丸めが無い) | `TestCalcMatchesEngineCalcDamage` / `TestCalcBulkMatchesEngineCalcBulk` / `TestCalcReverseMatchesEngineCalcReverse` |
| AC-3 | 失敗は `{"error":{"code","message"}}` の封筒。code は §5 の表どおり(JSON 破損・未知フィールド・末尾データ・列挙・入力検証・sentinel)。どんな入力でも panic を外へ出さず、message に Go のランタイム情報を含めない | `TestErrorEnvelopeCodes` / `TestEmptyEnumMeansDefault` / `TestTrailingDataIsInvalidJSON` / `TestHostileInputNeverPanics` |
| AC-4 | ステートレスで決定的。他の呼び出し・不正入力を挟んでも同じ入力は同じバイト列 | `TestCallsAreStatelessAndDeterministic`(ネイティブ)、`wasm-conformance.mjs` の逆順2周(WASM) |
| AC-5 | 整数は整数のまま(小数点・指数表記にしない)。NaN/Inf を出さない | `TestIntegerFieldsHaveNoFractionOrExponent` |
| AC-6 | マスタを持ち込まない(ID を解釈せず効果定義だけで計算する)。境界パッケージは engine と標準ライブラリにしか依存せず `syscall/js` を持たない | `TestOpaqueMasterIDsArePassedThrough` / `TestBoundaryPackageImportsAreMinimal` |
| AC-7 | **Go/WASM 一致**。同じベクタをネイティブ Go と `engine.wasm`(Node)に通し、レスポンスがバイト一致する(順逆2周)。逆算が 1 秒以内。引数の型違い・壊れた JSON でも文字列の error 封筒を返し、その後も正常に計算できる(Promise が死なない) | `scripts/wasm-conformance.mjs`(`make test-wasm`)。ネイティブ側の `go test` では検証できない(WASM 実行が要る)ので、`make test` には含めず AC-10 の配線テストで存在だけを固定する |
| AC-8 | ベクタが必要な条件(補正の種類・一括・逆算・浮動小数の出る確率)を覆う | `TestVectorsCoverRequiredScenarios` |
| AC-9 | `make wasm`(`scripts/wasm.sh`)が本実装で、`GOOS=js GOARCH=wasm` ビルドと `wasm_exec.js` の GOROOT からの取得を行う | `TestWasmBuildScriptIsImplemented` |
| AC-10 | `make test-wasm` が配線され `make test` に含まれない。一致テストの構成要素が揃い、生成物は `.gitignore` 済み。前提不足はスキップでなく失敗する | `TestWasmConformanceTargetIsWired` / `TestWasmConformanceHarnessExists`(前提不足の失敗は `wasm-conformance.mjs` 自身が `exit 1`) |

AC-7 は仕様上ここに一本化してある。WASM の実行が要る検証は Node 側にしか置けないため、
それを `make test` から外した代わりに、AC-9 / AC-10 が「配線が消えていないこと」を `make test` 側で見張る。

## 却下・保留

- **tinygo を使う**: サイズは大きく減るが、この環境に tinygo が無く、`encoding/json` の
  リフレクションや `map` の挙動で標準 Go と差が出る既知の問題がある。
  「Go と WASM が同じ結果」を保証したい P1-9 で、別のコンパイラを持ち込むのは目的に反する。
  サイズが問題になったら、まず遅延ロードを試す。
- **engine の型に `json` タグを足して DTO を無くす**: 一番短いが、engine のフィールド名が
  そのまま外部契約になり、engine のリファクタが黙って境界を壊す。engine に
  「外向きの表現」という関心事を持ち込むのは絶対ルール2の線を濁す。却下。
- **WASM 内にマスタ(種族・技・持ち物)を持つ**: オフラインで ID だけ渡せて便利だが、
  ドメイン規約「持ち物・技・ポケモンのリストをコードにハードコードしない」に正面から反する。
  マスタは P2 の pokedex-svc が正で、Web がキャッシュして渡す。却下。
- **gob / MessagePack / Protobuf で渡す**: JSON より速く小さいが、ブラウザの devtools で
  中身が読めなくなり、JS 側にデコーダが要る。1回 25 µs の計算に対してエンコードは十分速い。却下。
- **`js.Value` のオブジェクトを直接組み立てて返す**: 文字列パースが消えるが、境界の形が
  `//go:build js && wasm` の下に散らばり、ネイティブ Go でテストできなくなる。却下。
- **期待値 JSON をコミットする**: §7 のとおり却下(陳腐化検知という仕事が増えるだけ)。
- **`make test` に一致テストを入れる**: WASM ビルド(数秒)と Node 起動が毎回入り、
  `make test` < 1分 / 開発ループの速さを損なう。代わりに配線テストで漏れを防ぐ。

## 影響

- `engine/wasmapi/`(境界)、`engine/cmd/wasm/`(WASM エントリ)、`engine/cmd/wasmexpect/`(期待値生成)、
  `scripts/wasm-conformance.mjs`(一致テスト)、`engine/wasmapi/testdata/vectors.json`(ベクタ)を追加する。
- `engine` 本体パッケージ(`damage.go` / `bulk.go` / `reverse.go` / `stats.go` / `ko.go`)は**変更しない**。
  よって `make test-golden` への影響は無い。
- `scripts/wasm.sh` を本実装し、`Makefile` に `test-wasm` を足す。`make lint` の
  `node --check` 対象に `scripts/wasm-conformance.mjs` を追加する。
- `api/openapi.yaml` は変更しない。§10 の差分は P3-1 / P4-5 の作業項目。
- P4-5(Web のオフライン計算)への宿題: マスタ ID → 実体の解決層を Web に1つ置き、
  オンライン(API に ID を送る)とオフライン(WASM に実体を渡す)で同じ型を共有する。
  `openapi-typescript` が生成する型と、この ADR §3 の DTO の対応表を P4-5 で作る。
