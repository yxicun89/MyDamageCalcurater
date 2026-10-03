# ADR-0503: iOS の素早さ比較画面と、契約ごとの API 生成ターゲット

- 状態: 採用(2026-10-03。P6-24。ユーザー決定「iOS に素早さ比較画面を作る」〈DECISIONS.md 2026-10-03〉の具体化。spec-writer の判断)
- 日付: 2026-10-03
- 関連: ADR-0500(iOS の構成・API 生成・モック)、ADR-0501「P6-24 の受け入れ条件」、ADR-0600〜0607(素早さ)、
  ADR-0604(Web の素早さ画面。参照実装)、ADR-0012(サービスローカル契約)、ADR-0802(エラー形)、docs/speed-design.md、
  services/speed/api/openapi.yaml

## 背景

素早さ比較は Web(`web/src/speed/SpeedScreen.tsx`)にだけあった。iOS にも同じ画面を作る。契約は root の `api/openapi.yaml` ではなく
サービスローカルの `services/speed/api/openapi.yaml`(gateway が `/api/speed/*` を speed-svc へ中継する。ADR-0603)。iOS の生成は
これまで root の契約 1 本だけで、判定(P6-25)・タイプバランス(P6-26)も同じ理由で後に続く。

## 決定

### 1. 生成方式: 契約ごとに別ターゲット(案 B)

- `ios/PokeCalcKit` に `PokeCalcSpeedAPI` ターゲットを足す。`services/speed/api/openapi.yaml` の生成物だけを置く(`PokeCalcAPI` と同じ扱い。
  手で編集しない)。root の `PokeCalcAPI` は**不変**(生成物に 1 バイトの差も出ない)。
- `ios/scripts/openapi-gen.sh` は「契約・設定・出力先」の組(スクリプト先頭の `targets` 配列)のループに拡張した。`--check` も同じループ。
  契約を足すときは配列に 1 行と、`ios/tools/openapi-gen/openapi-generator-config.<名前>.yaml` を足すだけで、`make ios-gen` /
  `make ios-gen-check` に乗る。判定(P6-25)・タイプバランス(P6-26)はこの形で足す。
- speed の契約には `tags` が無いので `filter` は掛けない(ヘルスチェック 2 本も生成されるが、使わない)。
- 却下した案:
  - 案 A(root の生成に合流): 契約が別ファイルで、`Client`・`Components`・`Operations`・`_Error`・`Health` などの生成型名が衝突する。
    root の契約は API レーンの持ち物で、iOS から合成できない。
  - 案 C(手書きクライアント): 絶対ルール 1 の趣旨(契約から生成する)に反し、契約の変更に追従できない。
- 注意(名前の衝突): `PokeCalcAPI` と `PokeCalcSpeedAPI` はどちらも `Client` と `Components` を持つ。**1 つのファイルで両方を `import` しない**。
  speed 側は `APISpeedService.swift` だけが `PokeCalcSpeedAPI` を import する。

### 2. 契約との同期

- 生成された enum(`PresetId`・`MinimalPresetId`・`NatureId`・`PositionRequest.ModePayload`・`ErrorCode`)とドメインの enum の値・順序の一致を
  `SpeedContractSyncTests` が固定する。契約が変わって `make ios-gen` したあと、ここが落ちたらドメインと文言を追従する。
- 範囲の数値(SP の最大・ランクの範囲)は専用の定数を作らず `SPLimits.maxPerStat`・`RankLimits` を使う。契約(`PositionRequest.sp` /
  `rank` の minimum・maximum)との一致は `ios/scripts/check-request-limits.sh`(`make ios-test` の一部)が見る(XCTest は契約ファイルを読めないため)。

### 3. サービス境界

- 画面が依存するのは新しいプロトコル `SpeedService`(`pokemon()` / `table(presets:field:)` / `position(_:)`)。`PokeCalcService` には混ぜない
  (`DeviceDataService` と同じ流儀。絶対ルール 5: speed の失敗が計算・構築に影響しない)。
- 実装は `APISpeedService`(`PokeCalcSpeedAPI.Client`)と `MockSpeedService`。失敗は `PokeCalcError`(`code` はサーバーの語彙をそのまま運ぶ)。
- `AppEnvironment.ready` に `speed: any SpeedService` を足す(`deviceData` を足したときと同じ流儀)。API は同じ gateway の `baseURL` と同じ
  `ClientIdentity`(端末 ID・セッション ID は計算と同じ値)。モックは `MockSpeedService(environment:)`。
- 契約に `default` 応答が無いので、生成クライアントは契約外のステータスを `undocumented` で返す。本文が `{code,message}` の JSON なら
  その `code`(gateway の 404/405。ADR-0802)、読めなければ新設の `PokeCalcError.Code.unexpectedStatus`(`client_unexpected_status`)にする。

### 4. ViewModel

- `SpeedViewModel`(`PokeCalcCore`・`@MainActor @Observable`)。入力から要求を作り、応答を表示用に整える。計算ロジックは持たない。
- 位置の要求は入力のたびに `LatestTaskRunner` で先行の要求を cancel し、debounce(`SpeedInput.debounceInterval` =
  `CalcInput.debounceInterval`)のあと送る。最新の世代の応答だけを反映する(サービスが cancel を無視して古い応答を返しても出さない)。
- 表の取り直し(絞り込み・場の状態)は離散の操作なので debounce しない(先行の取り直しは cancel する)。`load()` も debounce しない。
- ポケモン一覧・表・位置は互いに独立(片方の失敗がもう片方の表示を消さない)。`CancellationError` は画面の失敗にしない。
- 表の取り直しの間は `tableState = .loading`(Web と同じ)。位置は入力を変えた時点で `.loading`(古い結果を出し続けない)。
- 実数値(raw)の入力は文字列で持ち、前後の空白を落として 1 以上の整数だけを送る。上限は speed サービスが式から導く値なので画面に複製せず、
  超過は API の 400 を日本語にして出す。SP・ランクはステッパーで操作し、`setSP`/`setRank` は `SPLimits`/`RankLimits` に収める。
- `settle()` は予約済みの表・位置の要求が終わる(または cancel される)まで待つ(テストと View の `.task` 用)。

### 5. ポケモンの選択: speed の一覧を使う簡易ピッカー

- pokedex の検索(`searchSpecies`。先頭 200 件の上限・前方一致)は使わない。speed の `/api/speed/v1/pokemon` を `load()` で 1 回取り、
  シート(`speedPokemonSheet`)に名前の一覧を出して選ぶ。理由: 選べるのは speed のマスタにいるポケモンだけで(`unknown_pokemon`〈422〉を避ける)、
  応答が種族値つきで往復が増えない。
- 件数はレギュレーション次第で数百になり得るので、常に検索欄を付ける(件数による出し分けはしない)。絞り込みはクライアントで、前後の空白を落とし、
  ひらがなをカタカナに寄せた名前の部分一致(Web の `<select>` にはない iOS 固有の補助。`MasterSearch` の検索を再利用しない理由は上と同じ)。
- 絞り込みは表示だけで、選択中のポケモンも位置の要求も変えない。

### 6. 表の境界線: tiers の speed と実数値を直接比べる(Web と同じ例外)

境界線(「ここに自分が入る」)と「自分と同速」の強調は、**表示中の段の speed と応答の自分の実数値を整数の大小で比べて**決める。
応答の `faster`/`slower`/`tie` は常に全 6 行の表が基準(ADR-0602 §3)で、絞り込みで表示行が減ると合わないため(Web の `computeBoundary` と同じ。
ADR-0604・2026-09-22 critic 指摘)。式(素早さの計算)は持たない。トリックルーム中の表は遅い順なので「自分より速い最初の段の前」に引く。
同じ速度の段が表示されていればその段を強調し、境界線は引かない。

### 7. 範囲と画面

- 優先する最小範囲は切らず、Web と同じ入力を最初から持つ(部品が小さく、画面は 1 つ): mode(preset / custom / raw)・調整・スカーフ・自分の追い風・
  まひ・SP・性格補正・ランク・実数値、表の絞り込み(最後の 1 つは外せない)、表の場(相手側の追い風・トリックルーム)。
- 画面は縦の 1 本のスクロール: 上から「自分のポケモン」(入力と結果)、「素早さの表」(絞り込み・場の状態・段)。段は `LazyVStack` ではなく `VStack`
  (全段をアクセシビリティの木に出す)。段は `.accessibilityElement(children: .contain)`(子の識別子を飲み込まない)。
- 入力は選択ボタン(ピル。計算画面の `calcCondition-*` と同じく `isSelected` で状態を出す)にする。`Menu`/メニュー形式の `Picker` は使わない
  (中のボタンに identifier が付かない制約がある)。ポケモンはシート。
- design.md のトークンだけを使い、`lineLimit`・`minimumScaleFactor` は付けない。常時動くアニメーションは入れない。
- 導線: ルート画面に「素早さを比べる」(`openSpeedScreen`)。起動時に開く環境変数は `POKECALC_OPEN_SPEED_SCREEN_AT_LAUNCH=1`
  (既存の `POKECALC_OPEN_*_AT_LAUNCH` と同じ流儀。`ios/scripts/sim-run.sh` の `IOS_SCREEN=speed` にも足す)。
- 文言は `SpeedLabels`(Core)に集約し、Web の `speedScreenText`・`speedPresetText`(`web/src/i18n/ja.ts`)と同じ文言にそろえる。iOS 固有は
  ピッカーの検索欄の案内と 0 件の文言だけ。エラーはサーバーの英語 message を出さず、`code` から日本語にする
  (通信できない・応答が読めないは Web の `speed_unavailable` と同じ「接続できません」)。

### 8. モック

`MockSpeedService`(架空データ。`nameJa` はすべて「テスト」で始まる。環境変数 `POKECALC_MOCK_SPEED` でシナリオを切り替える。
`table-error` / `position-error` / `pokemon-error` / `all-error`、なし・未知は正常)。UI テストと `MockSpeedServiceTests` が頼る固定の事実:

- ポケモンは 4 体(`9001-000`〜`9004-000`)。表(全 6 行)は 4 × 6 = 24 行で、同じ段に 2 行以上ある段(同速)がある。
- 同じポケモンの「最速 + スカーフ」は表の「最速スカーフ」「最速+1」と同じ実数値になる(同速を UI で出す経路)。
- 位置の `faster + tie + slower` は常に 24(全 6 行基準)。実数値 1 は `faster` = 24。
- 相手側の追い風(`tableTailwind`)は表を 2 倍にする。実数値 0 以下は `invalid_request`、未知の `pokemonId` は `unknown_pokemon`。
- モックの式は本物を写したものではない(テスト用の簡易な丸め)。モックは計算の正しさを保証しない。

### 9. 識別子(XCUITest が使う名前)

`SpeedScreenUITests` の冒頭に一覧がある(ルート `openSpeedScreen`、画面 `speedScreen`、自分の入力 `speedMode-*`・`speedPokemonButton`・`speedPreset-*`・
`speedScarf`・`speedSelfTailwind`・`speedParalysis`・`speedNature-*`・`speedSP*`・`speedRank*`・`speedRawValueField`、シート `speedPokemon*`、
結果 `speedResult*`、表 `speedTable`・`speedFilter-*`・`speedTableTailwind`・`speedTrickRoom`・`speedTier-<speed>`・`speedTierTie-<speed>`・
`speedTierSelf-<speed>`・`speedBoundary`、失敗 `speedTableError`・`speedPositionError`・`speedPokemonError`)。

## 結果

- root の API・Web・speed の契約は変えない。iOS が `/api/speed/*` を使い始める(契約を変える側への連絡は DECISIONS.md 2026-10-03)。
- 判定・タイプバランスの生成は同じ形(配列に 1 行 + 設定ファイル + Package.swift の target)で足せる。
- 追加するテスト数・失敗数と実装者への注意は ADR-0501「P6-24 の受け入れ条件」。
