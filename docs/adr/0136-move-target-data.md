# ADR-0136: 技の対象(単体・全体 等)をマスタに持たせる(issue #288 のデータレーン分)

- 状態: 採用
- 日付: 2026-10-03
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #288(ダブルの壁・全体技の補正)、`docs/ai-shared/DECISIONS.md` 2026-10-03「技の対象(単体/全体)を
  マスタに持たせてほしい(判定レーン → データレーン)」、ADR-0005(ダブル固有補正は未適用)、ADR-0100(スキーマ)、
  ADR-0101 §3(取得元の表現のまま出し、検証・変換は Go 側)、ADR-0115(黙って落とさない)、
  ADR-0121(技の機構のマスタ化。同じ流れの前例)、ADR-0122(変換結果の版)、ADR-0124(適用済み migration を書き換えない)、
  ADR-0125(表ごとの権限)、ADR-0128(read model の形)

## 背景

ダブルでは、相手2体に当たる技(全体技)のダメージに ×3072/4096 がかかる。engine はまだこの補正を持たない
(`engine/modifiers.go`。ADR-0005)。補正を入れるには、技ごとに「誰に当たるか」がデータとして要るが、マスタに無い。
判定レーンから、`MasterMove` とスキーマに技の対象を足し、pokedex-svc から出すよう依頼があった。
本 ADR はデータ側だけを決める。engine の補正・ゴールデン(ダブルのベクタ)・判定画面は判定レーンの仕事。

## 調査(実データ。技の ID は書かない)

- Showdown(ピン留めした commit・champions mod)は**全技**が `target` を持つ。値は Showdown の `MoveTarget` 型の 15 種で、
  @smogon/calc 0.12.0 の `data/interface.d.ts` の `MoveTarget` と同じ集合:
  `adjacentAlly`・`adjacentAllyOrSelf`・`adjacentFoe`・`all`・`allAdjacent`・`allAdjacentFoes`・`allies`・`allySide`・
  `allyTeam`・`any`・`foeSide`・`normal`・`randomNormal`・`scripted`・`self`。
  - mod 全体(1014 技)では 15 種すべてが現れる。使用可(isNonstandard が null)の 515 技では `adjacentFoe` 以外の 14 種。
  - 使用可の攻撃技 335 の内訳: normal 276・allAdjacentFoes 20・any 15・allAdjacent 14・randomNormal 6・scripted 4。
    変化技は self 62・normal 73・all 18・allySide 8・adjacentAlly 4・foeSide 4・allAdjacentFoes 3・allAdjacent 2・
    allies 2・adjacentAllyOrSelf 2・allyTeam 1・any 1。
- @smogon/calc 0.12.0(Champions 世代)の技データは、全体技(`allAdjacent` 16・`allAdjacentFoes` 23。うち攻撃技 34)に
  だけ `target` を持ち、他の 487 技は省略する。calc が `target` を持つ 39 技はすべて Showdown と同じ値で、calc が
  省略した技に Showdown の全体技は無い。calc の全体技補正の条件は `['allAdjacent', 'allAdjacentFoes'].includes(move.target)`
  (`mechanics/gen789.js`)。
- 取得側(`tools/importer/fetch-showdown.mjs`・`fetch-calc.mjs`)はどちらも `target` をまだ出していない。

## 決定

### 1. スキーマ: `moves.target`(migration 000010)

- 1つの技の対象は1つなので、`move_mechanisms` のような子表でなく `moves` の列にする:
  `ALTER TABLE moves ADD COLUMN target VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NULL` と
  `CONSTRAINT chk_moves_target CHECK (target IN (<15 種>))`。down は CHECK と列を落とす。
- **NULL を許し、既定値を付けない**。運用中の DB には既に技の行があり、NOT NULL + CHECK では migrate が失敗する。
  既定値(例: `normal`)で埋めると、取り込み前の行が誤った対象を黙って持つ。NULL は「まだ取り込んでいない」。
- importer は投入で必ず値を入れる(`-tags mysql` のテストで「投入後に NULL の行が無い」ことを確かめる)。
  `MoveRow.Target` が変換結果に入るので、変換結果の版(ADR-0122)が変わり、migrate 後の最初の取り込みで
  取得元の版が同じでも全行が入れ直される。
- 表は増えないので、ADR-0125 の表ごとの権限・ADR-0131 の台帳は変わらない。migration に行は書かない
  (`TestMigrationsHaveNoData`)。
- 全環境で取り込みが済んだ後に NOT NULL へ締めるかは、必要になったら別の migration と ADR で決める。

### 2. 語彙: Showdown の文字列をそのまま持つ

- 値は Showdown(= @smogon/calc の `MoveTarget`)の 15 種の文字列のまま(camelCase)。正規化した列挙
  (`all_adjacent_foes` 等)にしない。理由: 取得元と oracle の型が同じ閉じた集合で、1対1の読み替え表を足しても
  情報が増えず、判定レーンが oracle と突き合わせるときに読み替えが要るだけになる。
- 値の一覧の正は `services/internal/master` の `AllMoveTargets`(昇順)。migration の CHECK と一致することを
  `services/pokedex/db` の layout テストで確かめる。engine はまだ対象を読まないので engine には置かない
  (判定レーンが `engine.Move` に載せるとき、機構(ADR-0121)と同じく正を engine へ移し、master は別名にしてよい)。
- `MoveTarget.IsSpread()` を置く: `allAdjacent`・`allAdjacentFoes` なら true(calc の全体技補正の条件と同じ集合)。
  importer の照合(§3)が使い、判定レーンの補正も同じ定義を使う。

### 3. 取得・照合・投入

- `fetch-showdown.mjs` は技ごとに `target`(取得元の文字列)を出す。`fetch-calc.mjs` は技ごとに `target` を出し、
  calc が省略した技は `""` にする。
- `services/pokedex/importer` は、Showdown・calc とも `target` を**必須**としてデコードする(キーが無い古い取得物は
  `ErrInvalidInput`。`make import-fetch` で取り直す)。ADR-0121 の `mechanism` と同じ扱い。
- 変換: moves 表に採る技の対象は Showdown の値(calc は全体技以外を省略するので、Showdown が正)。
  未知の値・空・無い(nil)は `ErrInvalidData` で止める(黙って既定の対象にしない。ADR-0115)。
- 照合(calc と Showdown の両方にある技): calc が `target` を持つなら Showdown と同じ値、calc が省略したなら
  Showdown は全体技でないこと。食い違いは `move-value-mismatch`(Detail `target`)で、**攻撃技は Blocker**
  (ダブルのダメージに効く)、変化技は警告(タイプの食い違いと同じ扱い。ADR-0002 追記 P2-1c 規則3)。
- 投入: `InsertMove` が `target` を入れる。`ListMoves` は `target` を返す(`moves` の全列を選ぶので、
  sqlc の行型は `store.Move` のまま)。

### 4. 出口(この ADR の範囲)

- `services/internal/master` の `MoveRow.Target` を `master.Move` で検証する(未知の値は `ErrInvalidRow`、
  空は「不明」として通す)。空を通すのは、内部 API がまだ対象を運ばない calc-svc の経路と、migrate 直後の
  NULL の行のため。`engine.Move` には載せない(ダメージ計算の挙動を変えない。ゴールデン不変)。
  よって engine の example マスタ・WASM の DTO・Go/WASM 一致テストは変わらない。
- 内部 API `/internal/pokedex/master` の `MasterMove`、公開 API の技の応答(`GET /api/pokedex/moves/{key}`・
  `/moves/batch`・検索・learnset 等)への追加は `api/openapi.yaml` の変更で、API レーンの持ち物(COORDINATION.md)。
  API レーンへ次を依頼する:
  - `MasterMove.target`: `type: string, nullable: true`、必須キー。値は §2 の 15 種(enum は付けない。ADR-0121 の
    `mechanisms` と同じ理由で、検証は `services/internal/master.MoveTargetOf` が持つ)。null は「まだ取り込んでいない」。
    pokedex-svc の `master.go` は `store.Move.Target`(NULL 可)をそのまま写し、calc-svc の `buildMoves` は
    `MoveRow.Target` に渡す(null は `""`)。
  - 公開 API の技の応答に出すか・その名前と語彙(画面で「単体 / 相手全体 / 場全体」の表示に使うか)は
    API レーンと判定レーンで決める。
  それまで対象は calc-svc にも利用者にも届かない。
- balance/speed の read model(`moves.json`)には**足さない**。balance の読み込みは未知のキーを拒否する
  (ADR-0128・ADR-0107 決定7)。既存の番人 `TestMovesReadModelEntryKeysAreFixed` がこれを守る。
  read model に要るようになったら、タイプバランス・素早さレーンと合意してスキーマの版を上げる。

## 結果

- 良い点: 判定レーンがダブルの全体技補正を実装する材料がそろう。値は oracle の型と同じ語彙で、
  calc との照合で全体技の判定がずれないことを取り込みのたびに確かめる。
- 注意: このマージ後は既存の取得物(`data/generated/showdown/<commit>/snapshot.json` と
  `data/generated/calc/<version>/snapshot.json`)に `target` が無いので、`tools/importer` で両方を取り直す
  (同じ版。Showdown はキャッシュ済みのソースを使う)。migrate(000010)→ 取り込み の順で行い、
  取り込みが済むまで既存の行の対象は NULL。
- 契約の変更が API レーンで入るまで、pokedex-svc の外には出ない(§4)。
