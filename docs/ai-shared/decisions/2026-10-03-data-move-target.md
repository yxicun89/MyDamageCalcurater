## 2026-10-03: 技の対象(target)をマスタに持たせた。内部 API・公開 API への露出を API レーンへ依頼(データレーン → API・判定レーン。issue #288・ADR-0136)
Decision: 技の対象(Showdown の `target`。15種: adjacentAlly・adjacentAllyOrSelf・adjacentFoe・all・allAdjacent・allAdjacentFoes・allies・
allySide・allyTeam・any・foeSide・normal・randomNormal・scripted・self)を、pokedex の `moves.target`(migration 000010。NULL 可・CHECK)・
importer(Showdown と calc の両方で必須。calc は全体技だけ持つので照合する)・`services/internal/master`(`MoveTarget`・`IsSpread` 等)に持たせた。
値は Showdown の文字列をそのまま持つ(@smogon/calc 0.12.0 の `MoveTarget` と同じ語彙)。全体技(`allAdjacent`・`allAdjacentFoes`)は使用可の技で39。
Reason: 判定レーンの依頼(2026-10-03。ダブルの壁 ×2732/4096・全体技 ×3072/4096 の補正には技の対象が要る)。
Impact:
- **API レーンへの依頼**: 内部 API `/internal/pokedex/master` の `MasterMove` は api/openapi.yaml で定義されている(データレーンは変えられない)。
  既定案: `MasterMove.target` を `type: string, nullable: true` の必須キーとして追加(null = まだ取り込んでいない。enum は付けない。値の検証は
  `master.MoveTargetOf` が持つ。ADR-0121 の mechanisms と同じ扱い)。追加後に、pokedex の `services/pokedex/internal/httpapi/master.go` で
  `store.Move.Target` を写し、calc の `services/calc/internal/master/export.go` の `buildMoves` で `MoveRow.Target` に渡す(null は "")。
  公開 API(`GET /api/pokedex/moves/{key}`・`/moves/batch`・検索・learnset の技の応答)に出すか・名前・表示の語彙は API レーンと判定レーンで決める
- **判定レーンへ**: 上の契約が入ると calc-svc の `master.Move` に target が届く(`IsSpread()` で全体技を判定できる)。engine の補正・ダブルのゴールデンは
  判定レーンの作業(engine の `Move` には今回は載せていない。ゴールデン不変)
- 既存 DB は migration 000010 の後の最初の取り込みで target が入る(変換結果の版が target で変わるため全行を入れ直す)。取得物(snapshot)は
  fetch のたびに作り直すので、古い取得物で止まらない
