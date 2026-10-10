# ADR-0522: iOS の構築の作り直し(構築名の廃止・6 枠の編集・Showdown 形式は補助)— F-08 / I-ios-7

- 状態: 採用(2026-10-10)
- 日付: 2026-10-10
- 関連: docs/usability-round2.md F-08(原文 12・13)、ADR-0332(Web の作り直し。語はこれと同じ)、
  docs/ai-shared/decisions/2026-10-04-web-team-rebuild.md、ADR-0521(F-12 の部品)、ADR-0506(Showdown 風テキスト)、
  ADR-0502、ADR-0509(メガの持ち物固定)、ADR-0501 末尾「F-08 iOS 構築の作り直し」

## 背景

iOS の構築は端末内保存(`LocalTeamStore`。team-svc は使わない)。作成時に構築名を入力させ、編集画面は「メンバーを追加」で 1 体ずつ足し、
保存すると一覧へ戻る形だった。Showdown 形式のテキストは編集画面の入口から開くシートで、何を入れればよいかの説明も例も無かった。
Web は ADR-0332 で作り直し済み。iOS も同じ語・同じ流れにそろえる。

## 決定

1. **構築名を廃止**。名前の入力 UI(新規作成のアラート・編集画面の名前欄)を無くす。保存する名前は既定名 `TeamNaming.defaultName`(「名称未設定」。
   Web/team-svc の既定名と同じ値)。表示名は純粋関数 `TeamNaming.displayNames(for:)`: 既定名の構築は「構築 N」(N は既定名の構築だけを
   `TeamStore.list()` の追加順=作成の古い順に数えた 1 始まりの番号)、それ以外(旧データで名前を持つもの)は保存された名前。「名称未設定」の文字は画面に出さない。
   - **旧データの互換**: 保存済みの `[Team]` は名前を持つままそのまま読める。**Web と違い、旧データの名前は保存で消さない**(Web は API の全置換で
     既定名に戻る。iOS はローカルで、名前を保つのに追加の処理が要らず、データを失わせないほうが安全なため)。名前を直す UI は無いので、旧名は残り続ける。
   - `Team.updatedAt: Date?`(最終更新)を足した。**Optional** にして、無い旧データも `Codable` の合成でそのまま読める(`TeamCodableCompatTests`)。
     保存のたびに `TeamEditViewModel.save()` / 作成時の `TeamListViewModel` が刻む(時刻は `now` クロージャで注入してテストする。Core の `Date()` は既定値のみ)。
   - 「構築から選ぶ」(計算・逆算・判定)の構築名も表示名にした(`TeamIndividualOptions.groups`)。
2. **6 つの枠が最初から並ぶ**。`TeamEditViewModel.slotIDs`(長さ 6。メンバー id か nil)を枠の真実とし、`team.members` は枠の順に詰めた並びを保つ。
   - 空の枠は「ポケモン」の欄(検索シート)と案内「ポケモンを選ぶと、技・持ち物・特性などを決められます」だけ。種族を選ぶ(`selectSpecies(slot:speciesKey:)`)と
     その枠が既存の `MemberCardView`(技 4 つ〈覚える技すべて〉・持ち物・特性・性格・SP)になる。テラスタイプは iOS の対象外(既存方針)で、新しい操作は足さない。
     既存の個体の `teraType` はそのまま保つ(`TeamMember` に残り、書き出し・呼び出しにも使われる)。
   - 枠ごとに「N体目を上へ」「N体目を下へ」(隣の枠と入れ替え。空の枠とも。1 体目の上・6 体目の下は無効)・「N体目を外す」(その枠だけ空に戻す。他は動かさない)。
     見える文字は「上へ」「下へ」「外す」、読み上げ名が「N体目を…」(label-in-name)。
   - **空き枠の位置は保存しない**(Web と同じ): 保存は種族の決まった枠を枠の順に詰める。開き直すと先頭から詰まって並ぶ。
   - 既存の `addMember(speciesKey:)`・`importMembers` は、最初の空の枠から埋める形に直した(`rebuildMembersFromSlots`)。`importMembers` は画面からは使わなくなった
     (取り込みは新しい構築を作る)が、テスト済みの API として残している。
3. **明示保存**。**従来は[保存]で保存して一覧へ戻った**。今回は保存しても編集画面に残り、「保存しました」を出す(Web と同じ)。未保存(`hasUnsavedChanges`:
   保存済みのメンバーと枠の並びが違う)のあいだ「保存していない変更があります」を保存ボタンの上に出す。保存の帯は `safeAreaInset(.bottom)` で常に見える(6 枠は縦に長いため)。
   [一覧に戻る]は未保存なら 2 段階(「保存していない変更があります。保存せずに一覧に戻りますか」+[保存せずに戻る]/[編集を続ける]。alert ではなくカード。P6-7 と同じ理由)。
   システムの戻るボタンは隠し、戻る手段を[一覧に戻る]に一本化する(確認を必ず通すため)。古い保存データのメガ補正(ADR-0509)は開いた時点で下書きを変えるので未保存の印が出る(自動では保存しない)。
4. **Showdown 形式は補助**。取り込みは一覧の下の閉じた折りたたみ「Showdown 形式で取り込む」(説明文は決定ファイルの文言。日本語名で書いた 1 体分の入力例つき)。
   常に**新しい構築**を作り(`TeamListViewModel.createTeam(members:)`)、編集画面へは移らず「N体の構築を作りました」を出す。書き出しは編集画面の下の閉じた折りたたみ
   「Showdown 形式で書き出す」(**いまの内容**を書き出す。Web は保存済みの内容。iOS は未保存の下書きも書き出せるほうが使いやすいため)。既存の `TeamTextTransferViewModel`
   (ADR-0506)を再利用し、シートをやめて折りたたみの中に置いた(`TeamTextSections.swift`)。
   - **入力例**: 実在のポケモン名・技名を Git に置かない(ADR-0002)ため、書式のひな形(「ポケモンの名前 @ 持ち物の名前」「Ability: 特性の名前」「SP: 32 Atk / 32 Spe」…)にした。
     書式はこのアプリのパーサに合わせる(能力ポイントは `SP:`。Web の例は `EVs:`。ADR-0502)。`TeamLabelsTests` で「アプリのパーサが拒否なしで 1 体に解釈できる」ことを固定。
5. **一覧はカード**(`popCard`): 表示名・6 体までのアイコン(`SpeciesImageView`。画像が無ければタイプ色のエンブレム。種族は先頭ページの 1 回だけ引き、引けなければ無彩色)・
   「n/6体」・最終更新・[開く]・[削除]。[削除]は 2 段階(確認のカード。確定まで `store.delete` を呼ばない)。
6. **既存の規則を保つ**: `Menu` は新しい UI に使わない(既存のカード内の持ち物・特性・性格・テラスのピッカーは従来どおり)、`lineLimit` を新規に使わない、タップ 36pt 以上、
   AX5 は `isAccessibilitySize` で縦積み、色はトークンと F-12 の部品のみ、常時アニメーション無し。ラベル文字は `TeamLabels`(Core)。

## 識別子の変更(XCUITest も同時に更新)

| 前 | 後 | 理由 |
|---|---|---|
| `teamNameField`・`teamNameError` | 廃止 | 構築名の廃止 |
| `addMemberButton` | `slotSpeciesPicker-N`(空の枠の「ポケモン」欄) | 6 枠が最初からあるため追加ボタンが無い |
| `memberDelete-<id>` | `slotRemove-N` | 枠の操作に移した |
| `exportMemberTextButton-<id>`・`teamTextTransferButton`・`teamTextSheet` | `teamExportFoldToggle`/`teamExportFoldContent`、`teamImportFoldToggle`/`teamImportFoldContent` | シートをやめて折りたたみに |
| `teamRow-<id>` | `teamCard-<id>`(+ `teamName-`・`teamCount-`・`teamUpdated-`・`teamIcons-`・`teamOpen-`・`teamDelete-<id>`・`teamDeleteConfirm-`/`teamDeleteCancel-`) | カード化 |
| (なし) | `backToListButton`・`teamUnsavedNotice`・`teamSavedNotice`・`teamLeave*`・`teamSlot-N`・`slotEmptyHint-N`・`slotMoveUp-N`/`slotMoveDown-N` | 新規 |

`saveTeamButton`・`createTeamButton`・`memberCard-<id>` 以下(`memberSpeciesPicker-` など)・`importTextEditor`・`analyzeImportTextButton` 等の取り込み/書き出しの中身の識別子は不変。

## 結果・申し送り

- 構築の API(team-svc)は使わないので契約変更なし。`LocalTeamStore` の保存形式は `Team` に任意項目 1 つ(`updatedAt`)を足しただけで、旧データを読める。
- 後続: 一覧のアイコンは先頭ページの種族しか引かない(先頭ページの外の種族はエンブレムが無彩色)。必要なら `species(key:)` の個別解決を足す。
