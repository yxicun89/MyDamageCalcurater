# ADR-0321: Web の構築ビルダー — Showdown 形式の取り込み・書き出し UI(P5-5e)

- 状態: 採用(Web レーン、2026-10-03。P5-5e の受け入れ条件とテストの正。実装済み)
- 日付: 2026-10-03
- レーン: Web
- 関連: ADR-0310(変換部 `showdownFormat.ts`。SP を `EVs:` でそのまま読み書き・issue のコード)、ADR-0309(構築画面の骨格・create/update の契約)、
  ADR-0316(メンバー編集)、ADR-0320(メガの持ち物固定 `domain/mega.ts`)、ADR-0304(`capabilities`・`MasterSpeciesSearch`・learnset の解決 A-13)、
  ADR-0308(訪れたタブは mount し続ける)、ADR-0213 §4(Showdown 入出力はクライアント担当)、docs/requirements.md §2「構築ビルダー」

## 背景

変換部(`parseShowdownTeam` / `exportShowdownTeam`)は main にあるが、どの画面にも繋がっていない。要件 §2(必須)は
「Showdown形式のインポート/エクスポート」。構築画面(`web/src/team/`)に UI を足す。

最大の制約は **変換部が名前→ID の全件表(`ShowdownMaster`)を前提にする**こと。実際のマスタは全件を持たない:
オンラインも、キャッシュ済みオフライン(`cachedSources.ts`)も `capabilities.speciesList / moves` が false で、
`MasterData.species / abilities / moves` は空か部分的(持ち物・性格は全件ある)。全件表があるのは架空の例データ(開発・テスト)だけ。
そのため「`master` をそのまま渡す」だけでは実環境ではほぼ全部が `unresolved_name` になる。

## 決定

### 1. 名前引き用のマスタを、必要な種族だけ都度引いて作る(`team/showdownMaster.ts`)

- `candidateSpeciesNames(text)`: 各メンバー(空行区切り)の1行目から種族名の候補(行全体・`@` の前・括弧の中・括弧の前)を重複なしで返す(純粋)。
- `resolveMasterForImport(text, master, masterSearch?)`: `speciesList` が true ならそのまま返す。false なら候補ごとに `searchSpecies` し、
  **`nameJa` が完全一致した種族だけ** `resolveSpecies` して、種族・特性・技を `MasterData` に足した新しいマスタを返す
  (ADR-0304 A-13: 種族を解決すれば learnset の技と特性が付いてくる)。失敗は握りつぶし、その種族は足さない(後段の parse が `unresolved_name` を出す)。
- `resolveMasterForExport(members, master, masterSearch?)`: マスタに無い `speciesKey` だけ `resolveSpecies` して足す。
- **名前は日本語名のみ**(`MasterData` に `nameEn` が無い。英語の Showdown 出力はそのままでは取り込めない)。英語名への対応は別 ADR(データに英語名が載ってから)。
- 探索コスト: 1メンバーあたり候補は最大8、メンバーは6体まで(parse の上限)なので検索は高々 48 回。入力が `input_too_large` 超なら検索しない。

### 2. 取り込みは2段階(確認 → 作成)。作成するのは新しい構築だけ

- 段階を分ける理由: 非同期の名前解決・問題の確認を挟み、「取り込めない項目があるのに黙って作られた」を防ぐ。1段階だと問題を見る前に保存される。
- 領域「Showdown 形式から取り込む」(`role="region"`)に、テキスト欄(`取り込むテキスト`)・構築名欄(`取り込む構築名`)・`[内容を確認]`・`[この内容で作成]`。
- 確認: `resolveMasterForImport` → `parseShowdownTeam` → `planShowdownImport`。結果は `role="status"`(`N体を取り込めます`)と、問題の一覧(`取り込みの問題`)。
  error が1つでもあれば一覧を含む領域を `role="alert"`、warning だけなら `role="status"`。確認中は `名前を確認しています`(`role="status"`)を出し確認ボタンを無効にする。
- **取り込めるメンバーが1体以上なら、その分だけで作れる**(error があるメンバーは取り込まれない旨を一覧の理由で示す)。0体(空入力・全て落ちた)は `取り込めるメンバーがいません`、作成は無効。
- 確認後にテキスト・構築名を編集したら、プレビューを捨てて作成を無効に戻す(古い内容で作らない)。したがって構築名の検査(空・51文字以上)は、確認後に名前を直したあと「内容を確認」し直してから作成を押したときに働く。
- 作成: 構築名の検査は既存の作成と同じ(空・51文字以上は `create` を呼ばず `nameRequiredNotice` / `nameTooLongNotice`)。`teamClient.create({name, members})` を1回だけ呼ぶ(送信中は無効)。
  成功: 一覧の先頭に追加(既存の `addCreatedTeam`)・完了を `role="status"`・入力欄を空に戻す。失敗: `role="alert"`(見出し+サーバーの message)、入力は残す。
- 既存の構築への追記・上書きはしない(取り込みは常に新しい構築。上書きは破壊的で、確認 UI が要るため v1 の外)。

### 3. 取り込み計画(`team/showdownImportPlan.ts`、純粋)とメガの補正

`planShowdownImport(parseResult, master)` → `{ members, issues, notes, canCreate }`。issues は parse のものをそのまま、`canCreate = members.length >= 1`。
メガ種族(`master.species` の `isMega`)は `megaItemLock` で持ち物を `requiredItemId` に直す(別の持ち物・null → `mega_item_fixed` の note、ストーンがマスタに無い → itemId を null にして
`mega_item_unavailable`)。非メガの持ち物は触らない(ADR-0320 と同じ)。**メガ判定には解決後のマスタ(§1)を渡す**(種族が `master.species` に入っているため)。
note の `memberIndex` は plan.members の添字(落ちたメンバーがあると parse の issue の memberIndex〈入力の何体目か〉とずれることを一覧の文言で混同しない:
issue は「入力の N 体目」、note は「取り込まれる N 体目」。ただし v1 は両方「N体目」と出す)。補正は一覧 `取り込み時の補正` に出す。

### 4. 書き出し

- 構築の行に `[「<名前>」を Showdown 形式で書き出す]`。押すと領域 `「<名前>」の Showdown 形式`(`role="region"`。構築ごとに独立、複数同時に開ける)が開き、
  読み取り専用の textarea(`「<名前>」の書き出しテキスト`)に `exportShowdownTeam(members, resolveMasterForExport の結果).text` を出し、textarea にフォーカスする。API は呼ばない。
- `[コピー]`: `navigator.clipboard.writeText`。成功は `role="status"` で `コピーしました`。clipboard が無い・失敗(権限拒否等)のときは textarea を全選択してフォーカスし、
  `role="status"` で手動コピーを案内する(例外は出さない)。
- ExportResult の issues(名前が無く省いた項目・ニックネーム超過など)は一覧 `書き出しの問題` に出す。ID は出力しない(ADR-0310 §8)。
- メンバー0体の構築は書き出しボタンを無効にし、理由 `メンバーがいないので書き出せません` を見える文言+`aria-describedby` で結ぶ。
- `[書き出しを閉じる]` で領域を閉じる(状態を捨てる)。

### 5. 文言・見た目・状態

- 文言は `i18n/team.ts` の `teamShowdownText`(`ja.ts` が再エクスポート)。`issueReason` は `ShowdownIssueCode` の全件(Record。足し忘れをコンパイルで防ぐ)、
  `issueText(issue)` は重大度(エラー/警告)・N体目(1始まり。memberIndex が null なら付けない)・理由・値。コードの生の文字列は画面に出さない。
- デザイントークンのみ(`TeamScreen.css` に足す)。常時動くアニメーションなし。
- 入力・プレビューは `TeamScreen` の `useState`(ADR-0308 により、タブを往復しても残る)。

## テスト

- `team/showdownMaster.test.ts`・`team/showdownImportPlan.test.ts`・`i18n/teamShowdownText.test.ts`・`team/TeamScreen.showdown.test.tsx`(受け入れ条件 M-/P-/I-/E- はファイル冒頭)、
  E2E は `web/e2e/teamShowdown.spec.ts`(取り込み・一部だけ取り込み・書き出し・タブ往復)。

## 帰結・未決

- 英語名の取り込み、既存構築への追記/上書き、ファイル(.txt)の読み込み、ニックネーム編集 UI は範囲外。
- 取り込み時の検索は種族ごとに通信する(オンライン)。多数回になる入力は6体上限と候補数上限で抑える。
- 確認後に `master`/`masterSearch` が変わる(計算モード切替)場合は ADR-0304 A-6 に従いプレビューを捨てる(実装済み: マスタ・masterSearch が変わったらプレビューを捨て、解決中の確認の結果も反映しない)。

## 実装メモ

- 画面は `TeamShowdownImport.tsx`(取り込み。入力・プレビューはこの部品の state)と `TeamShowdownExport.tsx`(構築の行の書き出し。保存でメンバーが変わったら古い書き出しは捨てる)に分け、
  構築名の検査は `teamName.ts`(新規作成・名前変更・取り込みで共通)に寄せた。
- 書き出しの問題の一覧には role を付けない(開く操作の結果として出る情報で、コピーの結果の role=status と取り違えないため)。
- `showdownMaster.ts` の上限(候補8・メンバー6・入力 100,000 文字)は `showdownFormat.ts`(不変)の定数と同じ値を持つ。変換部が export したら参照に置き換える。
