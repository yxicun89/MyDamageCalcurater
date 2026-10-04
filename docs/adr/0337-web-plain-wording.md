# ADR-0337: 文言のやさしい言い換えと用語集 — F-13 / I-web-11

- 状態: 採用(2026-10-04。実装・既存テストの期待値更新まで完了)
- 関連: docs/usability-round2.md F-13(原文20)、docs/plan/improvements/web.md I-web-11、docs/glossary.md(新規。画面で使う言葉の正)、
  docs/design.md(ポップ・カラフル、やさしい言葉)、ADR-0323(i18n のレーン別ファイル)、ADR-0326(メガストーンの表示)、
  ADR-0320(メガシンカの持ち物固定)、ADR-0301(計算モード)、ADR-0319・0331(調整)、ADR-0318(端末のデータ)、ADR-0336(F-12 PR-2)

## 背景

画面の文言に、アプリの内部の仕組みや開発者向けの語がそのまま出ている(「観測」「観測1の単位」「指数と 16n を見る」
「オフライン(WASM)」「ダメージ計算の実行場所」「〜がマスタにありません」「メガシンカ: メガストーンを持ちます」など)。
利用者の決定(F-13): 見た目はポケモンらしいポップ・カラフルで、言葉はやさしく、使っていて楽しいこと。
ec レーンからも、調整の「目標を満たす最小の振り方」「指数」などの言い換えを依頼されている(ec は今は調整を触らない)。

## 決定

### §1 言い換えの基準

- **ポケモン対戦でふつうに使う語・ゲーム内の語は変えない**: 確定数(確定3発)・乱数(乱数4発(40.0%))・特化・無振り・準速・最速・
  性格・特性・持ち物・テラスタイプ・能力ポイント(SP)・実数値・逆算・仮想敵・メガストーン・メガシンカ・ばつぐん・いまひとつ・急所。
- **アプリの内部の概念・開発者向けの語だけを言い換える**: 観測・プリセット・指数・16n・API・WASM・マスタ・実行場所・識別子・
  識別情報・リクエスト・ID・種族、と機械的な書き方(「メガシンカ: 〜」の見出し+コロン、「〇〇のメガストーン」)。
- 文言資源の**キー名は変えず、値(文言)だけ**を変える(画面のコード・テストが参照するキーを壊さない)。
- 構造・DOM・role・常時のアニメーションは変えない(文言だけの変更)。

### §2 用語集 `docs/glossary.md` を画面の言葉の正にする

- 表は3つ: 「変えない語」「言い換える語(禁止語)」(禁止語・やさしい言い換え・使う画面・理由)「例外」(ファイル・キー・禁止語・理由)。
- Web の i18n と iOS の Text は用語集の語を使う。新しい画面・文言は用語集を参照し、新しい語が要るときは先に用語集に足す。

### §3 用語集と i18n の一致を保つ検査(`web/src/i18n/glossary.test.ts`)

- 用語集の禁止語の表を読み、`web/src/i18n/` のテスト以外の `.ts` の文字列(TypeScript の構文木から集める。コメント・import 先・型・
  オブジェクトのキー・`===`/`case` の比較相手は除く)に禁止語が含まれないことを検査する。
- 例外は用語集の「例外」の表だけ。使われなくなった例外も失敗にする(古い例外を残さない)。
- 「変えない語」がどれかの文言に残っていることも検査する(言い換えすぎの防止)。
- 例外(初版): `judge.ts` 全体(判定は非表示〈F-07〉で判定レーンの担当。表示を戻すときにそろえる)、
  `aboutText.dataSources.detail` の `API`(出典の固有名詞 PokeAPI)、`apiEngineText.invalidPreset`(画面の操作では起きない開発者向けのエラー)。

### §4 言い換えの対応表(旧 → 新。既定案)

代表の文言は `web/src/i18n/plainWording.test.ts` で固定する。表に無いキーは変えない。

| ファイル | キー | 旧 | 新 |
|---|---|---|---|
| ja.ts | `reverseScreenText.sideGroupLabel` | 観測したダメージ | どちらのダメージ |
| ja.ts | `reverseScreenText.observationLabel(n)` | 観測n | ダメージn |
| ja.ts | `reverseScreenText.observationUnitGroupLabel(n)` | 観測nの単位 | ダメージnの単位 |
| ja.ts | `reverseScreenText.removeObservationLabel(n)` | 観測nを削除 | ダメージnを削除 |
| ja.ts | `reverseScreenText.addObservationLabel` | 観測を追加 | ダメージを追加 |
| ja.ts | `reverseScreenText.resultsListLabel` | 推定結果 | 考えられる振り方 |
| ja.ts | `reverseResultText.closeCandidateLabel` | 近い候補 | ほぼ合う候補 |
| ja.ts | `reverseResultText.noExactCandidateNotice` | 入力した観測を説明できる調整がありません(技・持ち物・入力値を確認) | 入力したダメージにぴったり合う振り方が見つかりません(技・持ち物・入力した値を確かめてください) |
| ja.ts | `requestLimitText.observationLimitReached(max)` | 観測は{max}件までです。追加するには、どれかの行を削除してください | ダメージは{max}件まで入力できます。追加するには、どれかの行を削除してください |
| ja.ts | `appText.calcModeGroupLabel` | ダメージ計算の実行場所 | 計算する場所 |
| ja.ts | `appText.calcModeOfflineLabel` | オフライン(WASM) | この端末(オフライン) |
| ja.ts | `appText.calcModeOnlineLabel` | オンライン(API) | サーバー(オンライン) |
| ja.ts | `appText.masterLoadError` | マスタデータの読み込みに失敗しました | ポケモンのデータを読み込めませんでした |
| ja.ts | `appText.onlineMasterLoadError` | オンラインのマスタを読み込めませんでした。接続を確かめて、もう一度お試しください | サーバーからポケモンのデータを読み込めませんでした。接続を確かめて、もう一度お試しください |
| ja.ts | `appText.masterCacheEmptyError` | オフラインで使うには、一度オンラインで開いてマスタを取得してください | オフラインで使うには、一度オンラインで開いてポケモンのデータを取り込んでください |
| ja.ts | `calcScreenText.anyAbilityOption` | おまかせ(種族の全特性) | おまかせ(すべての特性で計算) |
| ja.ts | `attackerStatText.natureUnresolved` | この性格補正の組み合わせに当たる性格がマスタにありません | この性格補正の組み合わせに当たる性格が、データにありません |
| ja.ts | `megaItemText.lockedReason` | メガシンカ: メガストーンを持ちます | メガシンカするので、持ち物はメガストーンに決まっています |
| ja.ts | `megaItemText.missingReason` | メガシンカ: メガストーンがマスタに見つかりません | メガシンカに使うメガストーンが、データに見つかりません |
| ja.ts | `megaItemText.compareDisabledReason` | メガシンカ: 防御側の持ち物はメガストーンに固定されるため、候補は比較しません | 防御側はメガシンカするので持ち物がメガストーンに決まっています。持ち物の候補は比べません |
| ja.ts | `megaItemText.clearedNotice` | メガシンカのメガストーンがマスタに無いため、持ち物を空にしました。保存すると反映されます | メガシンカに使うメガストーンがデータに無いため、持ち物を空にしました。保存すると反映されます |
| ja.ts | `recordClientText.unavailable` | 記録の API に接続できません | 記録のサーバーに接続できません |
| ja.ts | `apiEngineText.unavailable` | API に接続できません | サーバーに接続できません |
| ja.ts | `apiEngineText.unknownNature` | この性格に対応するマスタの性格が見つかりません | この性格がデータに見つかりません |
| items.ts | `itemRoleText.megaStoneOf(name)` | {name}のメガストーン | {name}専用のメガストーン |
| adjust.ts | `adjustClientText.unavailable` / `adjustErrorText.adjust_unavailable` | 調整の API に接続できません | 調整のサーバーに接続できません |
| adjust.ts | `adjustErrorText.unknown_species` / `unknown_move` / `unknown_nature` / `unknown_item` / `unknown_ability` | この{ポケモン/技/性格/持ち物/特性}はマスタにありません | この{…}はデータにありません |
| adjust.ts | `adjustErrorText.missing_header` / `invalid_header` | 端末の識別子を送れませんでした。ページを読み込み直してください | 端末の情報を送れませんでした。ページを読み込み直してください |
| adjust.ts | `adjustErrorText.master_unavailable` | マスタの準備ができていません。しばらくしてからお試しください | ポケモンのデータの準備ができていません。しばらくしてからお試しください |
| adjust.ts | `adjustScreenText.modeLabel.indices` | 指数と 16n を見る | 今の耐久・火力と HP を見る |
| adjust.ts | `adjustScreenText.modeLabel.minKo` | 倒せる最小の振り方 | 倒せるいちばん少ない振り方 |
| adjust.ts | `adjustScreenText.modeLabel.minSurvive` | 耐えられる最小の振り方 | 耐えられるいちばん少ない振り方 |
| adjust.ts | `adjustScreenText.natureNotFoundMessage` | 相手の調整に合う性格がマスタにありません | 相手の調整に合う性格がデータにありません |
| adjust.ts | `adjustScreenText.goalNatureNotFoundMessage(n)` | 目標 n: 相手の振り方に合う性格がマスタにありません | 目標 n: 相手の振り方に合う性格がデータにありません |
| adjust.ts | `adjustScreenText.indicesHeading` | 今の振り方の指数 | 今の振り方の強さ(目安) |
| adjust.ts | `adjustScreenText.firepowerIndexLabel` | 火力指数 | 火力の目安 |
| adjust.ts | `adjustScreenText.physicalBulkLabel` | 物理耐久指数 | 物理耐久の目安 |
| adjust.ts | `adjustScreenText.specialBulkLabel` | 特殊耐久指数 | 特殊耐久の目安 |
| adjust.ts | `adjustScreenText.indexNote` | 火力指数の補正はタイプ一致だけを含めます(…) | 火力の目安には、タイプ一致だけを含めます(持ち物・特性・テラスタルは含めません) |
| adjust.ts | `adjustScreenText.hpLineHeading` | HP の 16n | HP と 16 の倍数 |
| adjust.ts | `adjustScreenText.hpLineKindLabel` | 16n でも 16n-1 でもない / 16n / 16n-1 | 16の倍数でも、16の倍数-1でもない / 16の倍数 / 16の倍数-1 |
| adjust.ts | `adjustScreenText.next16nLabel` / `prev16nLabel` / `next16nMinus1Label` / `prev16nMinus1Label` | 次の 16n / 前の 16n / 次の 16n-1 / 前の 16n-1 | 次の16の倍数 / 前の16の倍数 / 次の16の倍数-1 / 前の16の倍数-1 |
| adjust.ts | `adjustScreenText.maxIndexHeading` | 指数が最大になる振り方 | いちばん強くなる振り方 |
| adjust.ts | `adjustScreenText.minSpHeading` | 目標を満たす最小の振り方 | 目標に届くいちばん少ない振り方 |
| adjust.ts | `adjustScreenText.minSpNotRequested` | 目標を指定すると、目標を満たす最小の振り方も出します | 目標を指定すると、目標に届くいちばん少ない振り方も出します |
| balance.ts | `balanceClientText.unavailable` / `balanceErrorText.balance_unavailable` | タイプバランスの API に接続できません | タイプバランスのサーバーに接続できません |
| balance.ts | `balanceErrorText.invalid_request` | リクエストが正しくありません。入力を見直してください | 入力の内容が正しくありません。見直してください |
| balance.ts | `balanceErrorText.unknown_pokemon` / `unknown_move` / `unknown_ability` | 選んだ{…}がサーバーのマスタにありません。選び直してください | 選んだ{…}がサーバーのデータにありません。選び直してください |
| balance.ts | `balanceErrorText.master_unavailable` | サーバーのマスタを読み込めません。… | サーバーのデータを読み込めません。しばらくしてからもう一度お試しください |
| speed.ts | `speedClientText.unavailable` / `speedScreenText.errorByCode.speed_unavailable` | 素早さの API に接続できません | 素早さのサーバーに接続できません |
| speed.ts | `speedScreenText.modeLabel.preset` | プリセット | 定番の振り方 |
| speed.ts | `speedScreenText.errorByCode.missing_header` | 端末の識別情報が送られていません | 端末の情報が送られていません。ページを開き直してください |
| speed.ts | `speedScreenText.errorByCode.invalid_header` | 端末の識別情報の形が正しくありません | 端末の情報が正しくありません。ページを開き直してください |
| speed.ts | `speedScreenText.errorByCode.unknown_pokemon` | このポケモンはマスタにありません | このポケモンはデータにありません |
| speed.ts | `speedScreenText.errorByCode.master_unavailable` | ポケモンのマスタを読み込めません | ポケモンのデータを読み込めません |
| team.ts | `teamClientText.unavailable` | 構築の API に接続できません | 構築のサーバーに接続できません |
| team.ts | `issueReason.unresolved_name` | 名前がマスタに見つかりません | 名前がデータに見つかりません |
| team.ts | `issueReason.missing_name` | 名前がマスタに無く、書き出せませんでした | 名前がデータに無く、書き出せませんでした |
| team.ts | `teamShowdownText.megaNoteText`(2つ目の分岐) | …: メガストーンがマスタに無いため、持ち物を空にしました | …: メガストーンがデータに無いため、持ち物を空にしました |
| favorites.ts | `favoritesScreenText.offlineNotice` | お気に入りはオンラインモードで使えます。ヘッダーの計算モードをオンラインにしてください。 | お気に入りはオンラインで使えます。画面上の「計算する場所」を「サーバー(オンライン)」にしてください。 |
| about.ts | `deviceDataText.explanation[0]` | …この端末に割り当てた ID でサーバーに… | …この端末に割り当てた番号でサーバーに… |
| about.ts | `deviceDataText.explanation[1]` | ID が変わると(…) | 番号が変わると(…) |

`favoritesScreenText.offlineNotice` は `appText.calcModeGroupLabel`・`calcModeOnlineLabel` と同じ語を含むこと(テストで固定)。
favorites.ts は ja.ts を import しない(循環。ADR-0323)ので、語は文字列で同じにそろえる。

### §5 既存テストの期待値更新の対象(仕様変更に直接起因する文字列の置換だけ。検査の強さは変えない)

実装者が、下の行の旧い文字列を §4 の新しい文字列に置き換える(できれば i18n の定数を参照する形に。強さを弱める書き換えはしない)。
「任意」はテスト自身が作る偽の応答の message で、置き換えなくても通るが、実物とそろえるため置き換えを勧める。

単体テスト(vitest、32 ファイル):

| ファイル | 行 | 旧い文字列 |
|---|---|---|
| src/App.about.test.tsx | 271, 282, 289, 293 | 観測1 |
| src/App.tabPersistence.test.tsx | 131, 139, 172, 183 / 292 / 323, 361, 370, 410 | 観測1 / ダメージ計算の実行場所 / オンライン(API)・オフライン(WASM) |
| src/App.test.tsx | 162, 448 / 291 / 295, 296 | 観測1 / ダメージ計算の実行場所 / オフライン(WASM)・オンライン(API) |
| src/App.masterSources.test.tsx | 116 / 118, 119 | 実行場所 / モードの radio |
| src/App.masterSearch.test.tsx | 33 / 35, 36 | 実行場所 / モードの radio |
| src/App.masterCache.test.tsx | 80, 99, 103 | モードの radio |
| src/App.frequentOpponents.test.tsx | 68, 93 | オンライン(API) |
| src/App.onlineMasterScreens.test.tsx | 95 / 106 | オンラインのマスタを読み込めませんでした / 実行場所 |
| src/AboutScreen.deviceData.test.tsx | 27, 28 | 割り当てた ID / ID が変わると |
| src/i18n/deviceDataText.test.ts | 13, 14 | 同上 |
| src/i18n/itemText.test.ts | 8 | ルカリオのメガストーン |
| src/i18n/megaItemText.test.ts | 8 | メガシンカ: メガストーンを持ちます |
| src/domain/itemRoles.test.ts | 212, 224 | {基本種名}のメガストーン |
| src/domain/megaStoneName.test.ts | 14〜19, 52 | {基本種名}のメガストーン |
| src/adjust/AdjustScreen.test.tsx | 1159(1206 は任意) | 調整の API に接続できません |
| src/speed/SpeedScreen.test.tsx | 740(732 は任意) | 素早さの API に接続できません |
| src/screens/BalanceScreen.test.tsx | 576, 1183(572, 1176 は任意) | タイプバランスの API に接続できません |
| src/screens/CalcScreen.attackerStats.test.tsx | 497 | …性格がマスタにありません |
| src/screens/CalcScreen.stoneName.test.tsx | 63, 64, 82, 104 | テスト〇〇のメガストーン |
| src/screens/ReverseScreen.test.tsx | 84, 89, 90, 91, 139, 154, 156, 289, 488, 521, 522, 597, 608, 704, 727, 831, 915, 928, 936, 949, 955, 956 | 観測… / 推定結果 / 近い候補 |
| src/screens/ReverseScreen.abilities.test.tsx | 46, 85, 243 | 観測したダメージ / 観測1 / 推定結果 |
| src/screens/ReverseScreen.debounce.test.tsx | 62, 63, 64, 249, 347(319, 364, 367 は任意) | 観測… / 推定結果 |
| src/screens/ReverseScreen.visual.test.tsx | 78, 107, 119, 122, 141, 158, 160 | 観測… / 推定結果 |
| src/screens/ReverseScreen.online.test.tsx | 55, 119, 213 | 観測1 / 推定結果 / 観測したダメージ |
| src/screens/ReverseScreen.motion.test.tsx | 41, 42, 43, 119 | 観測… / 推定結果 |
| src/screens/ReverseScreen.mega.test.tsx | 65, 71 | 観測したダメージ / 観測n |
| src/screens/ReverseScreen.itemRoles.test.tsx | 61, 69 | 観測したダメージ / 観測n |
| src/screens/ReverseScreen.damagingMoves.test.tsx | 55 | 観測したダメージ |
| src/screens/ReverseScreen.stoneName.test.tsx | 48, 50, 68, 77 | テスト〇〇のメガストーン / 観測1 |
| src/screens/ReverseScreen.unsupportedNames.test.tsx | 68 | 観測1 |
| src/screens/visibleLabels.test.tsx | 164, 178, 186, 193 | 観測1 / 観測1の単位 / 観測したダメージ |
| src/team/TeamScreen.mega.test.tsx | 402 | テストほのおのメガストーン |
| (任意)src/team/TeamScreen.test.tsx | 215 | 構築の API に接続できません |

E2E(Playwright、9 ファイル。アクセシブル名が変わるので `getByRole` の name・`chooseRadio` の引数を直す。axe の検査は名前に依らない):

| ファイル | 行 | 旧い文字列 |
|---|---|---|
| e2e/support/calcPage.ts | 24 / 53 / 130, 176, 179 | lockedReason / 推定結果 / 実行場所・モードの radio |
| e2e/reverse.spec.ts | 52, 63, 70, 74, 78, 79, 92, 100 | 観測… / 近い候補 / 推定結果 |
| e2e/mobile.spec.ts | 77, 78, 79, 80, 113, 119, 120, 121, 123, 124 | 観測… |
| e2e/online.spec.ts | 34, 35, 47, 83, 109, 120, 137, 163, 188 | 実行場所・モードの radio |
| e2e/offline.spec.ts | 56, 61, 89, 128, 167, 179, 226 | 実行場所・モードの radio |
| e2e/layoutOverflow.spec.ts | 177, 178 | 倒せる/耐えられる最小の振り方 |
| e2e/about.spec.ts | 49 | 観測1 |
| e2e/routing.spec.ts | 113 | 観測1 |
| e2e-k3d/k3d.spec.ts | 76, 92 | 実行場所・モードの radio |

テストの題名(`test("観測を追加すると…")` など)に旧い語が残るのは検査の対象外(直してもよい)。行番号は 2026-10-04 時点(fab8e2a0)。

## 受け入れ条件

1. `docs/glossary.md` があり、「変えない語」「言い換える語(禁止語)」「例外」の3表を持つ。禁止語に 観測・プリセット・指数・16n・API・WASM・マスタ を含み、
   どの禁止語にもやさしい言い換えが書いてあり、言い換えの語は禁止語を含まない(glossary.test.ts)。
2. `web/src/i18n/` のテスト以外の文言に、用語集の禁止語が例外を除いて1つも無い。用語集の例外はどれも実際の文言に当たる(glossary.test.ts)。
3. 用語集の「変えない語」(確定・乱数・特化・無振り・準速・最速・性格・特性・持ち物・テラスタイプ・能力ポイント・実数値・逆算・仮想敵・
   メガストーン・メガシンカ・ばつぐん・いまひとつ・急所)が、どれかの文言に残っている(glossary.test.ts)。
4. §4 の代表の文言が新しい語になっている。キー名は変わっていない(plainWording.test.ts。キーを参照してコンパイルが通る)。
5. 対戦の標準の用語(確定・乱数・ばつぐん・無振り・特化・HB特化・準速・最速)と画面の名前(逆算・調整)は変わっていない(plainWording.test.ts)。
6. §5 の既存テストが新しい語で通り、検査の強さは変わっていない。`make test`・`npm run lint`(eslint + prettier)・Playwright の e2e が成功する。
7. 構造・DOM・role・CSS・常時のアニメーションは変えていない(文言資源の値だけの差分。web/src/judge/・engine・api・services は触らない)。
8. iOS 向けの変更語の一覧を `docs/ai-shared/decisions/` に新規ファイルで置く(下の付録 A を写す)。

## 却下した案

- **キー名も新しい語に合わせて変える**(`observationLabel` → `damageLabel` など): 参照するコードとテストが大きく動き、文言の変更と
  リファクタリングが混ざる。キーは内部の名前なので変えない。
- **禁止語の一覧をテストのコードに持つ**: 用語集と二重になり、片方だけ直してずれる。用語集を正にしてテストが読む。
- **「仮想敵」も言い換える**(「気になる相手」など): 対戦で広く使う語で、タイプバランスの利用者には伝わる。下の「未決」で人間の確認を待つ。
- **お気に入りの復元の問題に出す ID(`(attacker-xyz)` など)を消す**: 何を戻せなかったかを特定するのに要る。文言の固定部分には「ID」の
  語を出さないので禁止語の検査には当たらない。表示を名前に替えるのは別タスク。

## 未決(人間の確認。既定案で進める)

- 「仮想敵」を残すか(既定: 残す)。
- 固定のメガストーンの表示「{基本種名}専用のメガストーン」(既定)でよいか。代案「メガストーン({基本種名}用)」。
- 計算する場所の語「この端末(オフライン)/サーバー(オンライン)」(既定)でよいか。

## 結果

- i18n の値だけを §4 の表どおりに直した(ja.ts・items.ts・adjust.ts・balance.ts・speed.ts・team.ts・favorites.ts・about.ts。キー・DOM・role・CSS は不変。judge.ts は未変更)。
- 既存テストは §5 の対象を新しい語に更新した(検査の中身は変えていない。多くは文字列の置換)。glossary.test.ts・plainWording.test.ts は変更なし。
- 未決の3点(仮想敵・専用のメガストーン・計算する場所の語)は既定案のまま。人間の確認で変わる場合は i18n の値と用語集を直す。

## 影響

- Web: `web/src/i18n/` の値だけ(ja.ts・items.ts・adjust.ts・balance.ts・speed.ts・team.ts・favorites.ts・about.ts)。judge.ts は触らない。
- iOS: 付録 A の語に揃える(I-ios で別に行う)。揃うまでは Web の語が正。`deviceDataText` は iOS と一字一句同じだったが、iOS が揃えるまで一時的に違う。
- docs/design.md・docs/requirements.md の本文に旧い語が出る箇所は、画面の文言としてではなく設計の説明として残してよい(用語集が画面の言葉の正)。

## 付録 A: `docs/ai-shared/decisions/2026-10-04-web-plain-wording.md` の下書き(iOS へ)

```markdown
## 2026-10-04: 画面の文言のやさしい言い換え(Web〈web-plain-wording-f13〉から iOS へ)
Decision: 画面で使う言葉の正を docs/glossary.md(用語集)にした(ADR-0337)。iOS の Text も用語集の語に揃える。
- 変えない語: 確定・乱数・特化・無振り・準速・最速・性格・特性・持ち物・テラスタイプ・能力ポイント(SP)・実数値・逆算・仮想敵・
  メガストーン・メガシンカ・ばつぐん・いまひとつ・急所。
- 使わない語 → 言い換え: 観測 → ダメージ(「ダメージ1」「ダメージ1の単位」「ダメージを追加」「どちらのダメージ」)/
  推定結果 → 考えられる振り方 / 近い候補 → ほぼ合う候補 / プリセット → 定番の振り方 / 指数 → 〜の目安(火力の目安・物理耐久の目安・
  特殊耐久の目安)・いちばん強くなる振り方 / 16n → 16の倍数(16の倍数-1・次の16の倍数)/ 最小の振り方 → いちばん少ない振り方 /
  API → サーバー / WASM → この端末 / 実行場所・計算モード → 計算する場所 / マスタ → データ / 識別子・識別情報 → 端末の情報 /
  リクエスト → 入力の内容 / ID → 番号 / 種族の全特性 → すべての特性で計算 / 「メガシンカ: 〜」→ ふつうの文 /
  「{基本種名}のメガストーン」→「{基本種名}専用のメガストーン」。
- 文単位の対応は ADR-0337 §4 の表(キーは Web の i18n。iOS は同じ意味の Text を同じ文にする)。
- 端末のデータの説明(DeviceDataText)は「この端末に割り当てた番号」「番号が変わると」に変わった(以前は Web と一字一句同じだった)。
Reason: 利用者の要望(F-13・原文20)。内部の仕組みや開発者向けの語を出さず、対戦の標準の用語はそのまま使う。
Impact: iOS の該当する Text・アクセシビリティラベルと、それを引く XCTest / UI テストの期待値を直す。engine・API の契約は変わらない。
```
