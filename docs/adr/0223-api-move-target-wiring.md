# ADR-0223: 技の対象を内部 API・calc-svc・公開 API に通す(issue 288 の API レーン分)

- 状態: 採用(critic PASS。2026-10-03)
- 日付: 2026-10-03
- 関連: ADR-0136(データレーン。moves.target と `services/internal/master` の MoveTarget)、ADR-0222(engine のダブルの
  壁・全体技と `move_target_unknown` の印)、ADR-0121(MasterMove.mechanisms。内部 API で enum を付けない理由)、
  ADR-0204(calc-svc のマスタは内部 API から)、ADR-0218(公開 API の効果。検証できない値は 503)、ADR-0304(Web のオンライン取り込み)

## 背景

データレーン(ADR-0136)が技の対象(Showdown の `target`。15種の文字列)を pokedex の `moves.target` と共通マスタに持たせ、
判定レーン(ADR-0222)が engine に `Move.Target`(`""` | `single` | `spread`)とダブルの全体技補正・`move_target_unknown` の印を入れた。
両者をつなぐ経路(pokedex-svc → 内部 API → calc-svc → engine)と、クライアント(Web のオフライン計算)へ出す経路が無いため、
calc-svc のダブルの攻撃技はすべて「対象が不明」のままだった。

## 決定

### 1. 内部 API(`MasterMove.target`)

- `type: string, nullable: true` の**必須キー**。値は DB の Showdown の文字列そのまま(分類しない)。null はまだ取り込んでいない行。
- enum は付けない(ADR-0121 の mechanisms と同じ。値の検証は受け取った calc-svc が `master.MoveTargetOf` で行う)。
- pokedex-svc は `store.Move.Target`(sql.NullString)を写す。NULL は JSON の null(キーは省かない)。

### 2. calc-svc と共通マスタ(engine.Move.Target への写像)

- `services/calc/internal/master/export.go` の `buildMoves` は `MoveRow.Target` に渡す(null は `""`)。
- `services/internal/master.Move` は `engine.Move.Target` に写す: 全体技(`IsSpread`: allAdjacent・allAdjacentFoes)→ `spread`、
  その他の既知の 13 種 → `single`、空 → `""`(不明)。未知の値は従来どおり `ErrInvalidRow`(calc-svc では `ErrInvalidMaster`)。
  ADR-0136 §4 の「engine.Move には載せない」を本 ADR で置き換える。
- 自分・味方・場の技も `single`: @smogon/calc は全体技の補正を spread の2種にだけかけるので、それ以外は単体として計算するのが oracle と同じ。
- シングルの計算は Target を読まない(ADR-0222)ので、ゴールデンは変わらない。

### 3. 入れ替えの順序に依存しない

calc-svc の `DecodeExport` は `target` キーの無い本文(target を運ばない古い pokedex-svc)も受け付け、不明として扱う
(オブジェクト内の必須キーは今も検査していない。ADR-0204 §2)。pokedex-svc と calc-svc のどちらを先に入れ替えても
calc-svc はマスタを読み込める。古い組み合わせの間はダブルの攻撃技に `move_target_unknown` の印が付く(黙って誤らない)。

### 4. 公開 API(`Move.target`)— 既定案として出す

- 公開の `Move`(getMove・getMovesByIds・searchMoves)に**省略可**の `target` を足す。値は engine と同じ分類 `single` | `spread`
  (enum。Go の定数名は Format の `single` と衝突しないよう `x-enum-varnames` で `MoveTargetSingle` / `MoveTargetSpread`)。
  対象が NULL の技は**キーごと省く**。DB に未知の値があれば分類できないので 503 `master_unavailable`(ADR-0218 の効果と同じ扱い)。
- 分類を返す理由: Web のオフライン計算(WASM の技 DTO の `target` は `""` | `single` | `spread`。ADR-0222)にそのまま渡せる。
  Showdown の 15 種を返すと、全体技の集合(IsSpread)をクライアントに書き写すことになる(ハードコードしない。coding-rules)。
- learnset(getSpecies.learnset)は技の ID の配列なので変えない。Web は getMovesByIds で実体を引く(ADR-0304 A-13)ので target が届く。
- 影響範囲: pokedex の SQL(GetMove・GetMovesByIDs・SearchMoves に `target` を足す。`make gen`)と storetest の偽の Querier、
  search.go の3か所。iOS・Web の生成物は省略可のキーが増えるだけで、既存のコードは壊れない(iOS の `Components.Schemas.Move.init` は既定値 nil)。
- 判定レーンと名前・語彙を決め直す場合は本節だけを差し替える(内部 API・calc-svc の §1〜§3 は独立)。

### 5. Web(別レーンへ連絡)

- `web/src/master/exportSnapshot.ts`(calc-svc 用のマスタ書き出し)は `target: null` を書く(必須キー。例データは対象を持たない)。
  calc-svc の起動に関わる契約テスト(exportSnapshot.contract.test.ts)が openapi.yaml の required から追従を求めるため、本 PR で直す。
- `web/src/master/onlineSource.ts` の `mapMove` が `target` を Web の技に写し、オフラインの WASM 計算に渡すのは Web レーンの追従
  (docs/ai-shared/decisions/ の新規ファイルで連絡する)。

## 結果

- ダブルの calc-svc 計算で、対象を取り込んだ技は `move_target_unknown` の印が付かず、全体技は ×3072/4096 になる。
  対象が NULL の技(取り込み前)だけに印が付く。
- 既存 DB は ADR-0136 のとおり次の取り込みで target が入る。それまでは内部 API が null を返し、印が付く。

## 未決事項

- 公開 API の `target` の名前・語彙(§4)は判定レーンの確認待ち。既定案のまま実装する。
- iOS がダブルの計算で target を使うかは iOS レーンの判断(iOS はオンライン計算なので calc-svc 経由で対象が効く)。
