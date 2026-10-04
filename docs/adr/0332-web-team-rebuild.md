# ADR-0332: 構築の作り直し(構築名の廃止・6枠の編集・Showdown 形式は補助)— F-08 / I-web-7

- 状態: 採用(2026-10-04。実装済み。既定案のまま採用)
- 関連: docs/usability-round2.md F-08(原文12・13)、docs/plan/improvements/web.md I-web-7、
  ADR-0229(TeamInput.name の省略)、ADR-0308(タブ往復で入力保持)、ADR-0309(構築の骨格)、ADR-0310(Showdown 形式)、
  ADR-0316(メンバー編集)、ADR-0320(メガの持ち物固定)、ADR-0321(Showdown の UI)、ADR-0323(画面レジストリ)、
  ADR-0325(画像は任意)、ADR-0331(ビジュアル基盤 F-12)、
  docs/ai-shared/decisions/2026-10-04-api-favorite-calc-team-name.md(API レーンの決定)

## 背景

使用感フィードバック(第2回)
- 原文12: 構築名はいらない。
- 原文13: Showdown 形式に何を入力すればよいか一目で分からず UX がよくない。6匹のポケモンを選べて、それぞれが技を選べる形で
  構築を作れるようにする。使い方が全く分からない。

現状(2026-10-04 に /team を撮影して確認): 上から「新しい構築(構築名+作成)」「Showdown 形式から取り込む(テキスト+構築名)」
「保存した構築(行: 名前・n/6体・最終更新・名前を変更・メンバーを編集・Showdown で書き出す・削除)」が同じ重さで並ぶ。
メンバーを入れるには「名前を付けて作成 → 行の[メンバーを編集] → [メンバーを追加]」の3段が要り、6体の枠は見えない。
Showdown 形式は説明も入力例も無い。

API は完了済み(ADR-0229): `TeamInput.name` は省略可・nullable。省略・null・空・空白だけならサーバーが既定名「名称未設定」を入れる。
応答の `Team.name` は必須の string のまま。

## 決定

### §1 構築名の廃止

- 作成・名前変更の UI を無くす(新規作成の構築名欄・行の[名前を変更]・取り込みの構築名欄)。
- `createTeam`・`updateTeam` は **`name` キーを送らない**(空文字・null・自前の既定名も送らない)。body は `{ members }` だけ。
  組み立ては純粋関数 `teamInputFromMembers(members)`(team/teamSlots.ts)に1か所。
  - 結果: 名前を付けて保存した古い構築も、Web で保存すると「名称未設定」に置き換わる(ADR-0229 §1 の置換の規則)。
    構築名の機能を廃止する以上、名前を保つための読み直し・再送はしない(既定案。利用者の要望「構築名はいらない」と一致)。
- 表示名(純粋関数 `teamDisplayNames(teams)`。team/teamName.ts):
  - 応答の `Team.name` がサーバー既定名「名称未設定」(`SERVER_DEFAULT_TEAM_NAME`。ADR-0229 の契約の値)と完全一致なら
    「構築 N」(`teamScreenText.untitledTeamName(N)`)を出す。それ以外(古い構築に付けた名前)はそのまま出す。
  - N は **既定名の構築だけを作成日時(`createdAt`)の古い順に並べた 1 始まりの番号**。同時刻は `id` の昇順。
    一覧の位置(先頭が新しい)で数えると、作るたびに既存の構築の番号がずれるため、作成順にした(既定案)。
    削除すると後ろの番号は詰まる(サーバーに保存された名前は変えないので、表示だけの番号)。
  - 名前付きの古い構築を Web で保存すると「名称未設定」になるので、作成日時順の番号が割り当たり直し、後ろの既定名の構築の「構築 N」の番号がずれる(表示だけの番号。teamName.test N-5 で固定)。
  - 「名称未設定」の文字は画面に出さない(一覧・編集の見出し・ボタンの名前・削除の確認すべて表示名を使う)。
- team/teamName.ts の扱い: 送信前の検査 `teamNameNotice`・`MAX_TEAM_NAME_LENGTH`・`codePointLength` は使う所が無くなるので
  削除する(他から import されていない。TeamScreen.tsx の `MAX_TEAM_NAME_LENGTH` の再エクスポートも削除)。
  ファイルは「構築名の扱い(表示名の導出)」に役目を変えて残す。i18n の `nameLabel`・`nameRequiredNotice`・`nameTooLongNotice`・
  `createHeading`・`rename*`・`teamShowdownText.importNameLabel` は削除する。
- `teamNameContract.test.ts`(ADR-0229 の型の固定)はそのまま残す。

### §2 6枠が最初から見える編集画面

- 画面は「一覧」と「編集」の2つの表示を切り替える(同じ TeamScreen の中の状態。URL は /team のまま)。
  編集中は一覧・[新しい構築]・取り込みを出さない(狭い幅で編集の邪魔をしないため)。
- [新しい構築](`teamScreenText.createLabel`)を押すと `create({ members: [] })` を1回だけ呼び、成功したら一覧の先頭に足して
  **すぐその構築の編集画面を開く**。送信中はボタンを無効にする。失敗は従来どおり role="alert"(見出し+サーバーの message)。
  一覧が読めなくても[新しい構築]は押せる(ADR-0309 §4 を維持)。
- 一覧のカードの[開く](`teamMemberText.editLabel(表示名)` =「「<表示名>」を開く」)で編集画面へ。API は呼ばない。
- 編集画面(region。名前は `teamMemberText.editorLabel(表示名)`)には **常に6つの枠**(legend「1体目」〜「6体目」の group)が並ぶ。
  - 各枠は ui-card。種族が決まった枠は `.ui-card--typed` と `typeAccentStyle(最初のタイプ)`(`--card-type`)でタイプ色に染める。
  - 空の枠(`speciesKey === null`)は「ポケモン」の選択欄(マスタに種族一覧があれば select、無ければ検索欄)と案内
    `teamMemberText.emptySlotHint` だけを出す。技・持ち物・特性・性格・SP・テラスタイプ・[外す]・入れ替えは出さない。
    空の枠は保存できない理由にならない(「ポケモンを選んでください」の alert を出さない)。
  - 種族を選ぶとその枠が展開し、既存の TeamMemberFields の欄(技4枠〈learnset の全技。変化技も含む〉・持ち物・特性・性格・
    テラスタイプ・SP)が出る。メガ種族の持ち物固定・古い保存データの補正(ADR-0320)・SP の検査(各0〜32・合計66)は維持。
  - 枠の操作: [N体目を上へ]・[N体目を下へ](隣の枠と入れ替え。空の枠とも入れ替わる。1体目の上・6体目の下は無効)、
    [N体目を外す](その枠を空に戻す。**他の枠は動かさない**。フォーカスはその枠の「ポケモン」欄へ)。
  - [メンバーを追加] は無くす(6枠が最初からあるため)。
- 保存は **明示保存**(既定案。自動保存はしない: 入力途中の不正な SP や選びかけの技を送らない・全置換の update を打鍵ごとに
  呼ばない・失敗の伝え方が単純)。
  - [保存](`teamMemberText.saveLabel`)で `update(id, { members })`。members は **種族が決まった枠だけを枠の順に**詰めたもの
    (空の枠は飛ばす)。種族の決まった枠のどれかが不正(SP 範囲外・合計超過・技の重複)なら押せない。送信中も押せない。
  - 成功: 応答の Team で手元の一覧を書き換え、編集画面は開いたまま `savedNotice`(role="status")。
  - 失敗: role="alert" にサーバーの message。下書きは残す。
  - **未保存の印**: 保存済みの members と下書きから作った members が違う(または下書きが不正)ときだけ
    `teamMemberText.unsavedNotice` を保存ボタンの近くに出す(純粋関数 `hasUnsavedChanges`)。開いた直後・保存の成功後は出さない。
    古い保存データの補正(ADR-0320)は開いた時点で下書きを変えるので、印が出る(自動では保存しない、の表示)。
  - [一覧に戻る](`teamMemberText.closeLabel`): 未保存の変更が無ければすぐ一覧へ(API は呼ばない)。あれば2段階で
    `leaveConfirmNotice` と[保存せずに戻る][編集を続ける]を出す(window.confirm は使わない。ADR-0309 §5 と同じ理由)。
- ADR-0308(タブ往復で入力保持): 編集中の下書き・開いている構築・取り込みの入力と折りたたみの開閉は TeamScreen の state に持つ
  (画面は訪れた後 unmount されない)。sessionStorage には書かない。
- reloadToken(この端末のデータを削除)が変わったら、編集画面を閉じて一覧を取り直す(ADR-0318 §6 を編集中にも適用)。

### §3 Showdown 形式は補助の入口

- 一覧の画面の下(一覧の後ろ)に、閉じた状態の `<details>`(summary =`teamShowdownText.importFoldLabel`「Showdown 形式で取り込む」)。
  中に、やさしい説明 `importHelp`、入力例 `importExample`(`<pre>`。見出し `importExampleLabel`)、既存の取り込み領域
  (region「Showdown 形式から取り込む」・テキスト・[内容を確認]→[この内容で作成]の2段階)を置く。
  - 入力例は i18n の例文1つだけ(実在のポケモン名を使ってよいのは画面の例文だけ。名前のリストにはしない。テストは架空データで
    例文の形〈1行目に「@」・Ability:・EVs:・Nature・「- 」で始まる技の行〉と画面に出ることだけを固定し、実在名をテストに書かない)。
  - 取り込みの構築名欄は廃止。`create({ members })`(name キー無し)。常に新しい構築を作る(ADR-0321 §2 を維持)。
    成功したら一覧の先頭に足し、`importCreated(体数)`(role="status")を出す。編集画面には移らない(続けて取り込めるように)。
- 書き出しは構築ごとの操作なので、編集画面の下の閉じた `<details>`(summary =`exportFoldLabel`「Showdown 形式で書き出す」)に移す。
  中身は既存の TeamShowdownExport(ボタン → 読み取り専用の textarea・コピー)。対象は **保存済みの内容**(説明 `exportHelp`)。
  一覧のカードからは書き出しボタンを外す。
- 指示文では「取り込む・書き出す」を1つの折りたたみにする案だったが、取り込みは新しい構築を作り、書き出しは開いた構築が要るため、
  表示ごとに分けた(既定案。どちらも目立たない閉じた折りたたみで、主な流れは6枠の編集)。

### §4 一覧(構築のカード)

- `<ul aria-label="保存した構築">` の各 `<li>` が ui-card。中身: 表示名・6匹のアイコン列・「n/6体」・最終更新・[開く](主ボタン)・
  [削除](危険ボタン。2段階のまま。確定まで remove を呼ばない)。
- アイコン列は `role="group"`(名前 `memberIconsLabel(表示名)`)。メンバー1体につき `role="img"` の要素を1つ
  (名前は種族の nameJa。マスタ〈と検索で解決済みの種族〉で引けなければ `unknownMemberIcon(N)`=「N体目」)。
  中は PokemonImage(画像があれば装飾の `<img alt="">`)、無ければタイプ色のエンブレム(`data-testid="type-emblem"`。
  種族が引けなければブランド色)。一覧のためにオンラインの種族解決 API は呼ばない(構築の数×6回の通信を避ける)。
  入れ子の `<ul>` は使わない(一覧の listitem の数を変えないため)。
- 空の案内(`emptyNotice`)はやさしい言葉で、次にすることを書く。
- 並びはサーバーの応答のまま(ADR-0309 の規則を維持)。

### §5 既存テストへの影響(期待値更新・置換・削除の対象)

仕様変更に直接起因するものだけ。弱めない。i18n の定数を参照しているテストは、定数の文言が変わっても書き換え不要。

**削除(機能の廃止)**
- TeamScreen.test.tsx「AC-3 新規作成(名前だけ・メンバーは空)」の5件(名前の trim・50/51文字・二重押下・失敗時の名前保持)
  → 名前欄の廃止。作成・二重押下・失敗は TeamScreen.rebuild.test.tsx「R-1」に置換。
- TeamScreen.test.tsx「AC-4 名前変更」の6件 → 名前変更の廃止。
- TeamScreen.showdown.test.tsx「I-3 構築名の検査」→ 取り込みの構築名欄の廃止。
- TeamScreen.visual.test.tsx「構築名の欄は ui-field の中にある」→ 欄の廃止。
- TeamScreen.members.test.tsx AC-9「メンバーの保存中は名前変更の保存と[編集を閉じる]を押せない」「名前変更の送信中はメンバーを保存できない」
  → 名前変更の廃止(保存中に[一覧に戻る]を押せないことは置換先で確かめる)。

**置換(同じ性質を新しい UI で確かめる)**
- TeamScreen.test.tsx AC-6「一覧が読めなくても新規作成のフォームは使える」→ R-1「一覧が読めなくても[新しい構築]は押せる」。
- TeamScreen.test.tsx「list() 応答と書き込みの競合」2件 → 名前欄・名前変更を使わず[新しい構築]と[保存]で同じ競合を再現する形に書き換え
  (期待値は同じ: 後から届いた古い list() で上書きしない)。
- TeamScreen.members.test.tsx
  - AC-1「行の[メンバーを編集]で領域が開き…」は定数参照のまま通る見込み。「[編集を閉じる]で領域が消え、未保存の編集は捨てて」→
    R-7(未保存なら確認の2段階)に置換。「領域は同時に1つだけ」→ 編集は1画面なので削除(R-1 の「開くと一覧を隠す」が代わり)。
  - AC-2 の4件(追加・7体目・追加した枠・削除で番号が詰まる・先頭/末尾の無効)→ R-3・R-6(6枠固定・外すと空・入れ替え)に置換。
    「並べ替えの結果が保存する members の順になる」は入れ替えの操作名が同じなので通る見込み。
  - AC-3「何も変えずに保存すると、名前と全メンバーをそのまま送る」→ 期待値を「name キーが無く、全メンバーをそのまま送る」に更新。
    「成功すると応答の Team で一覧の行(メンバー数)が書き換わり、領域は開いたまま」→ 一覧は編集中に隠れるので、[一覧に戻る]の後に
    メンバー数を見る形に更新。「失敗は…一覧は変わらず」も同様。
  - AC-6「新しい枠は種族を選ぶまで保存できない」→ 空の枠は保存の妨げにならない(R-4)に置換。
  - AC-8「キーボードだけで追加できる([メンバーを追加])」→ R-3「キーボードだけで空の枠の種族を選べる」に置換。
  - `addButton`・`memberGroups(editor)` の件数(=メンバー数)を前提にするヘルパー・テストは、6枠のうち種族の決まった枠で数える形に更新。
- TeamScreen.mega.test.tsx: `addLabel` で枠を足していた3件(AC-1 の2件・AC-7「検索欄でメガ種族を選ぶ」)→ 空の枠(2体目など)で
  種族を選ぶ形に更新。AC-4「閉じて開き直すと、下書きは捨てられ…」→ 補正で未保存になるので[一覧に戻る]→[保存せずに戻る]を挟む形に更新。
- TeamScreen.showdown.test.tsx
  - ヘルパー `fillImport` の構築名欄の操作を削除し、取り込みの折りたたみを開く操作を足す(I-1・I-4〜I-12 の全件が対象。期待値は
    名前に関わる部分だけ: I-1 の「一覧に出る名前」は表示名「構築 N」、`importCreated` は体数だけ。I-8 は「テキストの編集」だけ)。
  - E-1〜E-6: 書き出しは編集画面の折りたたみの中へ(開く → 折りたたみを開く → 書き出す)。E-4「メンバー0体は書き出しボタンが無効」は維持。
    E-7「2つの構築を同時に開いても取り違えない」→ 編集は1構築ずつなので「構築を切り替えると書き出しの内容も切り替わる」に置換。
- TeamScreen.visual.test.tsx「一覧は ui-rows。行の「名前を変更」は副ボタン…」→ R-2「各構築は ui-card、[開く]は主ボタン、[削除]は危険ボタン」に置換。
  「新規作成の囲みは ui-card、「作成」は主ボタン」は[新しい構築]が `.team-screen__create.ui-card` の中の主ボタンなら通る(維持)。
- i18n/teamShowdownText.test.ts: `importNameLabel` の固定を削除、`importCreated("A", 3)` → `importCreated(3)` に期待値更新。
- App.tabPersistence.test.tsx「構築名の入力は、計算タブへ行って戻っても残り…」→ App.teamTab.test.tsx(取り込みの折りたたみと
  テキストが往復で残る)に置換。
- e2e/team.spec.ts の4件・e2e/teamShowdown.spec.ts の4件 → e2e/teamRebuild.spec.ts に置換(名前欄・[メンバーを追加]・
  行の[メンバーを編集]・取り込みの構築名欄を使わない)。fake の team-svc は name の省略に既定名を補う(ADR-0229)。

TeamScreen.itemRoles.test.tsx・TeamScreen.reload.test.tsx は定数参照のままで影響しない見込み(実装後に確認)。

### §6 文言(i18n/team.ts。iOS と同じ語)

| キー | 文言 |
|---|---|
| teamScreenText.createLabel | 新しい構築 |
| teamScreenText.emptyNotice | まだ構築がありません。「新しい構築」を押すと、ポケモンを6体まで選んで構築を作れます |
| teamScreenText.untitledTeamName(n) | 構築 n |
| teamScreenText.memberIconsLabel(name) | 「name」のポケモン |
| teamScreenText.unknownMemberIcon(n) | n体目 |
| teamMemberText.editLabel(name) | 「name」を開く |
| teamMemberText.closeLabel | 一覧に戻る |
| teamMemberText.saveLabel | 保存 |
| teamMemberText.unsavedNotice | 保存していない変更があります |
| teamMemberText.leaveConfirmNotice | 保存していない変更があります。保存せずに一覧に戻りますか |
| teamMemberText.leaveDiscardLabel | 保存せずに戻る |
| teamMemberText.leaveCancelLabel | 編集を続ける |
| teamMemberText.emptySlotHint | ポケモンを選ぶと、技・持ち物・特性などを決められます |
| teamMemberText.removeLabel(n) | n体目を外す |
| teamShowdownText.importFoldLabel | Showdown 形式で取り込む |
| teamShowdownText.exportFoldLabel | Showdown 形式で書き出す |
| teamShowdownText.importHelp | Pokémon Showdown などで作った構築のテキストを貼り付けると、新しい構築として取り込めます。ポケモン・持ち物・特性・技は日本語の名前で書き、ポケモンごとに空の行で区切ります |
| teamShowdownText.importExampleLabel | 入力の例(1体分) |
| teamShowdownText.exportHelp | 保存した内容を Showdown 形式のテキストにします。コピーして他のアプリに貼り付けられます |
| teamShowdownText.importCreated(n) | n体の構築を作りました |

削除するキー: teamScreenText の `createHeading`・`nameLabel`・`nameRequiredNotice`・`nameTooLongNotice`・`renameLabel`・
`renameFieldLabel`・`renameSaveLabel`・`renameCancelLabel`・`renameErrorHeading`、teamMemberText の `addLabel`・`addDisabledNotice`、
teamShowdownText の `importNameLabel`。

`importExample`(例文)は「<名前> @ <持ち物>」「Ability: <特性>」「EVs: <SP>」「<性格> Nature」「- <技>」を含む7行以内の1体分で、**日本語名**で書く(取り込みは日本語名だけを引く。ADR-0321。英語名は unresolved_name になる)。入力例の `<pre>` は見出し(`importExampleLabel`)と aria-labelledby で結ぶ。
SP の書き方は ADR-0310(EVs 行に 0〜32 をそのまま書く)。

### §7 a11y・見た目

- 色・余白はデザイントークンと ui-* 部品だけ(ADR-0331)。常時動くアニメーションは入れない。
- [開く]・[削除]などの見える文字は短くてよいが、アクセシブル名は表示名を含む文(見える文字はその一部に含める。WCAG 2.5.3)。
- 空の枠の案内・未保存の印は文字で伝える(色だけに頼らない)。
- フォーカス: 編集画面を開いたら見出し(h2 tabIndex=-1)へ。一覧に戻ったらその構築の[開く](無ければ[新しい構築])へ。
  [編集を続ける]の後は[一覧に戻る]へ。画面を切り替えてもフォーカスを失わない。

## 実装の結果(2026-10-04)

- §1〜§7 の既定案どおり。純粋関数は team/teamSlots.ts・team/teamName.ts、画面は TeamScreen.tsx(一覧・取り込み)と TeamMemberEditor.tsx(6枠・書き出し)。
- 実装で足した決め事: 取り込みのテキストと折りたたみの開閉は一覧↔編集で TeamScreen が持つ(unmount で消さない。ADR-0308)。
  [開く]・[削除]の見える文字は短く、アクセシブル名は表示名を含む文(aria-label)。保存後に応答の名前が既定名になるので、編集画面の見出しは「構築 N」になる。
- §5 の例外として、実装で壊れた次も更新した: App.deviceData.test.tsx(空の案内の文言)・e2e/a11y.spec.ts(構築タブの確認を[新しい構築]に)。

## 結果

- 「新しい構築 → 6枠にポケモンを選ぶ → 技などを選ぶ → 保存」の1本道になり、構築名を考える手間が無くなる。
- 古い構築に付けた名前は表示されるが、Web で保存すると「名称未設定」に戻る(§1。表示は「構築 N」)。
- Showdown 形式は説明と例文つきの補助の入口になる。

## 受け入れ条件

- AC-1 `createTeam`・`updateTeam`(新規作成・保存・取り込み)の body に `name` キーが無い。
- AC-2 一覧で既定名の構築は「構築 N」(作成の古い順)、名前付きの古い構築はその名前で出る。「名称未設定」は画面に出ない。
- AC-3 [新しい構築]で空の構築を作り、すぐ6枠の編集画面が開く。枠は常に6つで、空の枠は種族の欄と案内だけ。
- AC-4 種族を選ぶと技(learnset の全技)・持ち物・特性・性格・SP・テラスタイプが出る。保存は種族の決まった枠だけを枠の順に送る。
- AC-5 SP の検査・メガの持ち物固定・古いデータの補正は従来どおり。不正な枠があると保存できない。
- AC-6 未保存の印が変更の有無に合わせて出る。未保存で[一覧に戻る]は2段階。
- AC-7 一覧の各構築はカードで、メンバーのアイコン列・n/6体・[開く]・[削除](2段階)を持つ。
- AC-8 Showdown の取り込みは閉じた折りたたみの中にあり、説明と入力例が見える。構築名欄は無く、作ると一覧に新しい構築が出る。
  書き出しは編集画面の折りたたみの中。
- AC-9 タブを往復しても編集の下書き・取り込みの入力が残る(ADR-0308)。

## 付録: docs/ai-shared/decisions/ に書く内容の下書き(iOS 向け)

ファイル名案: `2026-10-04-web-team-rebuild.md`(実装の PR で追加する)

```
## 2026-10-04: 構築の作り直し(Web〈web-cf〉から iOS〈ios-6f〉へ)
Decision: Web の構築画面を F-08 に合わせて作り直した(ADR-0332)。
- 構築名を廃止した。Web は createTeam/updateTeam で name を送らない(ADR-0229 の既定名「名称未設定」になる)。
  一覧の表示名は、名前が「名称未設定」なら「構築 N」(N は既定名の構築を作成の古い順に数えた番号)、それ以外は保存された名前。
- 構築を開くと6つの枠(1体目〜6体目)が最初から並ぶ。空の枠は「ポケモン」の欄と「ポケモンを選ぶと、技・持ち物・特性などを
  決められます」の案内だけ。種族を選ぶと技4つ(覚える技すべて)・持ち物・特性・性格・SP・テラスタイプが出る。
  枠ごとに「N体目を上へ/下へ」「N体目を外す」。保存は明示([保存])で、未保存なら「保存していない変更があります」を出す。
  [一覧に戻る]は未保存なら「保存せずに戻る/編集を続ける」の2段階。
- Showdown 形式は補助: 一覧の下の閉じた折りたたみ「Showdown 形式で取り込む」に説明(「Pokémon Showdown などで作った構築の
  テキストを貼り付けると、新しい構築として取り込めます。ポケモンごとに空の行で区切ります」)と1体分の入力例。書き出しは
  編集画面の下の折りたたみ「Showdown 形式で書き出す」。
- 一覧はカード。6体のアイコン(画像が無ければタイプ色のエンブレム)・n/6体・最終更新・[開く]・[削除](2段階)。
Reason: 利用者の要望(原文12「構築名はいらない」・原文13「6匹を選べてそれぞれ技を選べる形に。Showdown 形式は何を入れればよいか分からない」)。
Impact:
- iOS(ios-6f): iOS の構築はローカル保存で team-svc を使っていないので、API の変更で直すコードは無い。画面の文言・流れは
  上の語(新しい構築・N体目・一覧に戻る・保存・保存していない変更があります・N体目を外す・Showdown 形式で取り込む/書き出す・
  構築 N)に揃える(usability-round2 §5「同じ語」)。構築名の入力が iOS にあれば、F-08 の iOS 分で同じく廃止する。
- API: 変更なし(ADR-0229 のまま)。
```
