# ADR-0509: iOS の持ち物を役割で絞り、メガ種族の持ち物をメガストーンに固定して日本語で見せる

- 状態: 採用(実装済み。ブランチ feat/ios-item-roles)
- 日付: 2026-10-03
- レーン: iOS
- 関連: ADR-0175 §4(クライアントの使い方。正)、ADR-0326(Web の持ち物の役割・メガストーン表示。そろえる相手。構築は役割で絞らない §2)、ADR-0320(Web のメガ持ち物固定)、
  docs/mega-evolution-spec.md §4-3、ADR-0200 §4(メガ種族に別の持ち物を持たせた計算は 400。持ち物なしは受け付ける)、
  ADR-0500 §1・§3(ロジックは PokeCalcCore の ViewModel、View は描くだけ)、ADR-0501「P6-19」(防御側・相手の
  `species(key:)` を入力のたびに読まない約束)、ADR-0507(機能レジストリ。今回は画面を足さないので触らない)
- 番号: iOS 帯 0500〜 の空き。0505 は欠番のまま、0508 は open PR(#580 画像)が使うので 0509

## 背景(2026-10-03 のユーザー報告)

1. メガストーンが英語表記で出る。2. 攻撃側にオボンのみ・ルカリオにボーマンダナイトのような意味の無い持ち物を選べる。
3. 持ち物はすべて日本語にしたい。4. メガ種族を選ぶと持ち物は選べない(メガストーンに固定)。

PR #579(ADR-0175)で `Item.roles`(`attacker`/`defender`。メガストーンは空)・`Item.isMegaStone`・
`SpeciesDetail.baseSpeciesKey`/`baseSpeciesNameJa` が公開 API に入った(`isMega`/`requiredItemId` は #515 で入り済み)。
iOS はこれらをまだ写しておらず、**メガの固定も未実装**(`git log origin/main --grep=メガ -- ios` は API 生成物の更新だけ)。
本 ADR で固定まで実装する範囲に含める。

## 決定

### 1. ドメイン型(`DomainTypes.swift`)

- `ItemRole`(`attacker`/`defender`。raw value は契約の値)。
- `Item` に `roles: [ItemRole]?`(nil = 不明。古いサーバー・古いモック)と `isMegaStone: Bool?`(nil = 不明)を足す。
  init の既定値は nil(既存の呼び出し・テストを変えない)。
- `SpeciesDetail` に `isMega: Bool`(既定 false)・`requiredItemId`・`baseSpeciesKey`・`baseSpeciesNameJa`(既定 nil)を足す。
- `APIPokeCalcService` は応答をそのまま写す(省略は nil / false。未知の役割の値は生成物が decode で落とすので考えない)。
  クライアントで効果から役割を再導出しない(ADR-0175 §4)。

### 2. 役割で絞る純粋関数は1か所(`ItemRoles.swift` の `ItemRoleFilter.options(_:for:keeping:)`)

`ItemRoleRequirement` は Web の `ItemRole` と同じ4種: `.attacker` / `.defender` / `.either`(どちらかの役割を持つ)/ `.any`(役割を見ず、メガストーンだけ外す)。規則:

| 持ち物 | `.attacker` | `.defender` | `.either` | `.any` |
|---|---|---|---|---|
| `roles` に attacker を含む | 出す | (defender も含めば)出す | 出す | 出す |
| `roles` が空(効果なし) | 出さない | 出さない | 出さない | **出す** |
| `roles == nil`(不明) | **出す**(絞らない) | 出す | 出す | 出す |
| `isMegaStone == true` | **常に出さない**(`roles == nil` でも) | 同 | 同 | 同 |

- 並びは入力(マスタ)の順のまま。
- `keeping`: いま選ばれている ID(nil 可)。上の規則で外れても、`items` にあればその1件だけ残す(選択済みを黙って消さない。§5)。
- 「持ち物なし」は関数の外(各画面が先頭に必ず置く。固定中は置かない)。

### 3. 画面ごとの役割

| 画面・欄 | 役割 | 理由 |
|---|---|---|
| 計算: 攻撃側の持ち物 | `.attacker` | 攻撃側の欄。攻守入れ替えは種族だけを入れ替える(規則6)ので欄の役割は変わらない |
| 計算: 「持ち物の候補も比較」(防御側) | `.defender` | `itemVariants` は防御側の持ち物 |
| 逆算 与えたダメージ(`side == .defender`): 自分 / 相手の候補 | `.attacker` / `.defender` | 自分が攻撃側 |
| 逆算 受けたダメージ(`side == .attacker`): 自分 / 相手の候補 | `.defender` / `.attacker` | 側の切り替えで両方を外すのは従来どおり |
| 構築の編集: メンバー | `.any` | 役割で絞らずメガストーンだけ外す(ADR-0326 §2。回復のきのみ・きあいのタスキなど計算に効かないが実戦で持たせる持ち物を記録でき、Showdown の取り込みと食い違わない) |
| 判定: 自分・各候補 | `.either` | 判定は自分の技で候補を撃ち、候補の技で撃ち返されるので、どの個体も攻守の両方をする |
| 調整: 自分 | `.either` | モード(指数・耐久・火力・最小)を選んだ後に変えられる。選び直しで値が消えないよう両方 |

各 ViewModel は既存の `itemOptions`(絞り込む前の全件。名前の引き当て・固定の検索に使う)を残し、画面の選択肢として
次を足す: 計算 `attackerItemOptions`・`defenderCompareItemOptions`、逆算 `myItemOptions`・`opponentItemCandidateOptions`、
構築 `itemOptions(forMember:)`、判定 `selectableItemOptions`、調整 `ownItemOptions`。View はこれだけを Picker に出す。
選択の関数(`selectAttackerItem` など)は、固定中と選択肢に無い ID(`keeping` で残った現在値を除く)を無視する。

### 4. メガ種族の固定(`MegaItemLock`)

- `MegaItemLock.make(for: SpeciesDetail?, allItems: [Item])`:
  - 非メガ・詳細が無い → `.none`
  - メガで `requiredItemId` が**絞り込む前の全件**にある → `.locked(itemId:displayName:)`
  - メガだがストーンを引けない(`requiredItemId` が nil・全件に無い)→ `.missing`(Web の missing と同じ。持ち物は空、欄は操作不可)
- `displayName` は `MegaItemText.stoneName(baseSpeciesNameJa:)`:「{基本種名}のメガストーン」、null なら「メガストーン」だけ
  (名前を推測しない。ストーンの `nameJa` は使わない)。
- 種族を変えたときの持ち物は `MegaItemLock.itemIdAfterSpeciesChange(previous:next:currentItemId:)`:
  次が locked → ストーンの ID / 次が missing → nil / 前が locked・missing で次が none → nil(メガストーンを残さない)/
  none → none → 現在の持ち物のまま。
- 固定中の要求: 自分の側(計算の攻撃側・逆算の自分・構築・判定・調整)は `itemId = ストーン`(missing は nil)。
  相手の側の候補(計算の `itemVariants`・逆算の `itemCandidates`)は locked なら `[ストーン]`(null も混ぜない。Web と同じ)、
  missing なら `[]`。比較・候補のトグルは保持するが要求に使わず、固定中の ON 操作は無視する。
- 詳細(`species(key:)`)の読み方: VM は読んだ詳細のメガ情報を種族キーごとに覚える(`MegaSpeciesInfo`)。
  **ADR-0501「P6-19」の約束(計算の防御側・逆算の相手・受けたダメージの自分の変更で `species(key:)` を読まない)は保つ**。
  そのため、それらの側は:
  - L1: 種族を変えた時点でその側に持ち物(または比較・候補のトグル)があり、新しい種族のメガ情報をまだ知らないときだけ、
    要求を組む前に `species(key:)` を読む(送れない入力を出さないため)。
  - L2: それ以外は View がその側の種族が変わるたびに読む(`loadDefenderAbilityOptions` / `loadOpponentAbilityOptions` /
    新設の `ReverseViewModel.loadMySpeciesDetail()`。`.task(id:)`)。読んだ結果で固定が変わったら、持ち物を直して**1回だけ**
    計算し直す(計算できる状態のときだけ。固定が変わらなければ計算しない = 既存テストの「読み込みでは計算しない」を保つ)。
- 構築の保存データの補正(Web ADR-0320 PR-B と同じ方針): `MegaItemLock.correction(currentItemId:lock:)`
  → `.unchanged` / `.fixed(itemId:displayName:)` / `.cleared`。構築を開いた時点(`load()`)・取り込み(`importMembers`)で
  メンバーごとに1回適用し、通知を `itemNotice(forMember:)` に出す(`MegaItemText.correctedNotice` / `clearedNotice`)。
  補正は下書き(未保存)。非メガにメガストーンを持たせた保存データは直さない(API が非メガを検査しないので挙動を変えない)。
  メンバーが種族を変えたら通知は消える。

### 5. 選択済みが絞り込みで消えるとき

構築から呼び出した個体・保存データ・古いサーバーの値が、その欄の役割を持たない持ち物を持つことがある(例: 計算の攻撃側に
防御側専用の持ち物)。**値は保ち、その1件を選択肢に残す**(`keeping`)。黙って外すと、計算結果が利用者の知らないうちに変わるため。
別の持ち物を選ぶと、その1件は選択肢から消える。逆算の側の切り替えは従来どおり両方を外す(既存の規則7)。

### 6. 表示名(「持ち物はすべて日本語」)

- `ItemDisplayName.text(itemId:items:megaStoneNames:)` を唯一の表示名の関数にする:
  nil →「持ち物なし」/ `megaStoneNames` にある(VM が知っているメガ種族のストーン)→「{基本種名}のメガストーン」/
  `isMegaStone == true` →「メガストーン」/ それ以外 → `nameJa`(マスタに無い ID は ID のまま。既存の規則)。
- `BulkRowDisplay.itemLabel(itemId:items:)` はこれに委ねる(メガストーンの `nameJa` は画面に出さない)。
  各 VM の `itemLabel(for:)` は自分が知るメガ情報を渡す。計算の行・逆算の候補の持ち物名もこの関数を通す。

### 7. 文言(`MegaItemText`。Web `megaItemText` と同じ語)

| キー | 文言 |
|---|---|
| `stoneName(baseSpeciesNameJa:)` | 「{基本種名}のメガストーン」/ null は「メガストーン」 |
| `lockedReason` | 「メガシンカ: メガストーンを持ちます」 |
| `missingReason` | 「メガシンカ: メガストーンがマスタに見つかりません」 |
| `compareDisabledReason` | 「メガシンカ: 防御側の持ち物はメガストーンに固定されるため、候補は比較しません」 |
| `fixedItemName(_:)` | 「持ち物: {名前}」(持ち物欄の無い相手のカード) |
| `correctedNotice(_:)` | 「メガシンカのため持ち物を{名前}に直しました。保存すると反映されます」 |
| `clearedNotice` | 「メガシンカのメガストーンがマスタに無いため、持ち物を空にしました。保存すると反映されます」 |

### 8. a11y・レイアウト

- 固定中の持ち物欄は `.disabled(true)` で、ラベルを固定のストーン名にし、`accessibilityHint` に `lockedReason`(missing は
  `missingReason`)を入れる。見える理由の文(`*ItemLockReason` の identifier)も出す(spec §4-3: 見える文言と hint の両方)。
- identifier: 計算 `attackerItemPicker`(既存)・`attackerItemLockReason`・`defenderItemLock`(防御側カードの「持ち物: …」)・
  `defenderItemCompareLockReason`。逆算・構築・判定・調整も同じ形(`<既存の持ち物欄の id>` + `LockReason`)。
- Dynamic Type AX5 で理由の文が折り返し、横にはみ出さない(`LargeTextLayoutUITests` と同じ検査をメガ固定の状態で行う)。

### 9. モック(`Resources/items.json`・`species.json`)

- 既存の `test-item-berry`・`test-item-unsupported` は `roles: [attacker, defender]`(既存の XCUITest が両側で使う)。
- 足す: 攻撃側専用 `test-item-attack-only`・防御側専用 `test-item-defense-only`・役割なし `test-item-no-role`・
  メガストーン `test-item-mega-stone`(`roles: []`・`isMegaStone: true`。`nameJa` は画面に出てはならない値)。
- 足す種族: `9001-001`「テストメガモンいち」(`isMega`・`requiredItemId: test-item-mega-stone`・`baseSpeciesKey: 9001-000`・
  `baseSpeciesNameJa: テストモンいち`)。末尾に置く(既定の攻撃側・防御側 = 先頭2件を変えない)。
- `MockFixtures` の decode・`MockPokeCalcService` の写像に `roles`・`isMegaStone`・メガの項目を足す(JSON に無ければ nil)。

## 検討した代替

- **`isMegaStone` だけでストーンを外し、役割は見ない**: 報告 2(意味の無い持ち物)が直らない。
- **役割を iOS で効果から再導出する**: ADR-0175 が1か所(pokedex)に決めた規則が分かれる。
- **防御側・相手の種族を変えるたびに `species(key:)` を読む**: 固定は簡単になるが ADR-0501「P6-19」の約束と既存テスト
  (呼び出し回数)を壊す。L1・L2 で、送れない入力を出さずに約束を保つ。
- **調整の持ち物をモードごとの役割で絞る**: モードを変えるたびに選択が消えたり残ったりして分かりにくい。`.either` にした。
- **固定中の表示にストーンの `nameJa` を使う**: 上流に日本語名の無いストーンが英語のまま出る(報告 1)。

## 検証(spec-writer が書いた失敗するテスト)

- `PokeCalcCoreTests/ItemRoleFilterTests.swift`(表駆動)・`MegaItemLockTests.swift`(固定・解除・補正・文言・表示名)
- `APIPokeCalcServiceItemRolesTests.swift`(写像。項目あり・省略)・`MockPokeCalcServiceItemRolesTests.swift`
- `CalcViewModelItemRolesTests.swift`・`ReverseViewModelItemRolesTests.swift`・`TeamEditViewModelItemRolesTests.swift`・
  `JudgeViewModelItemRolesTests.swift`・`AdjustViewModelItemRolesTests.swift`
- XCUITest `PokeCalcUITests/MegaItemLockUITests.swift`(モック強制。計算画面の固定・解除・選択肢・AX5)

## 実装メモ(implementer)

- 表示名: `ItemDisplayName.displayItems(_:megaStoneNames:)` で、メガストーンの `nameJa` を「{基本種名}のメガストーン」に置いた
  一覧(置いたものは `isMegaStone = false`)を作り、結果の行・逆算の候補・未対応の印の注記にはこれを渡す。
  `BulkRowDisplay.itemLabel` / `ReverseCandidateDisplay` は `ItemDisplayName.text` に任せる(循環しない: 後者は前者を呼ばない)。
- 各 VM は `megaInfo: [種族キー: MegaSpeciesInfo]` を持ち、`species(key:)` を読んだ場所(攻撃側の learnset 読み込み・
  `loadDefenderAbilityOptions` / `loadOpponentAbilityOptions` / `loadMySpeciesDetail`)で更新する。固定は `megaInfo` から都度導く。
- 逆算の相手がメガのときの理由の文は、攻撃側にも防御側にもなり得るので `MegaItemText.lockedReason` を使う
  (計算の防御側の比較欄だけ `compareDisabledReason`)。
- 判定の View は `selectableItemOptions(for:)`(そのスロットのいまの持ち物を残す)を使う。`selectableItemOptions` は全体の選択肢(テスト用)。
- 構築の `addMember` でメガ種族を足したときも、持ち物はストーンに固定する(通知は出さない)。

## Web との違い(意図したもの)

語・絞り込みの規則・固定の表示は Web(ADR-0326・0320)と同じ。次の違いだけある。

- **調整のメガ固定**: Web の調整は固定を持たず、ストーンを外して持ち物なしで計算する。iOS の調整は固定し、ストーンを要求に送る
  (メガ種族の計算は持ち物がストーンでないと API が 400 にするため、固定のほうが利用者の意図に近い)。
- **役割から外れた選択済みの持ち物**: Web は外して `role="status"` で通知する。iOS は `keeping` で選択肢に1件だけ残し、通知しない。
  iOS の計算は攻守の入れ替えで種族だけを動かし、逆算は側の切り替えで両方を外すので、役割から外れた値が残るのは主に構築から呼び出した個体。
- **詳細の読み込み(P6-19)**: 防御側・自分(受けたダメージ)の詳細は View が種族の変更ごとに読む(`DefenderCardView`・`ReverseMyCardView` の `.task(id:)`。
  「詳細」セクション側の重複した `.task` は外した)。L1(要求の前に読む)と同時に走ると、まれに同じ種族を2回読み得るが、
  結果は同じで計算は余計に走らない(既知の制約)。
