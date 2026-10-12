# ADR-0255: team-svc をヘキサゴナルの形にそろえる(構築・失効ジョブ)

- 状態: 採用(2026-10-12。実装済み)
- 日付: 2026-10-12
- レーン: API(データレーンが代行。API レーンが止まっていたため ec レーンと合意)
- 関連: ADR-0811(ヘキサゴナルの採用)、ADR-0254(record の同じ移行。流儀の先行例)、ADR-0252(calc)、
  ADR-0209(保存データの保持・削除・端末 ID の境界)、ADR-0213(構築)、ADR-0220(失効ジョブ)、
  docs/plan/architecture-adoption.md §6 S6

## 背景

S6(docs/plan/architecture-adoption.md §6)で team-svc を `internal/domain`・`internal/app`・`internal/adapters` にそろえる。
「record と同じ形(app は薄い)」。team は record と違い、検証が生成型(`api.TeamInput`)を直接受けて
`store.Member` を作っていたので、検証を domain に移すには構築の型と受け口の置き場所を決める必要がある。

## 決定

1. **構築の形(`Team`・`Member`・`StatBlock`)と形の上限は domain に置く**。検証(`domain.ValidateTeamInput`)が
   通した値をそのまま保存するので、検証の出力の型を domain が持つ(domain は app を import できない)。
   app のポート(`app/ports.go`)はこれを同じ名前の別名で持ち、`adapters/store` はさらに app の別名を持つ
   (既存の呼び名 `store.Team` などとテストを保つ。ADR-0254 決定 4 と同じ)。
2. **受け口は domain の別の型(`domain.TeamInput`・`MemberInput`)にし、本文は従来どおり生成型 `api.TeamInput` へ
   厳格に読み込んでから adapters/httpapi が写す**。record(ADR-0254 決定 1)は JSON のキーつきの domain 型へ直接読むが、
   team は生成型へのデコードの失敗の文言(型が合わないフィールド名など)をそのまま保つため、写しを 1 段持つ。
   写しは値をそのまま渡すだけ(省略の nil もそのまま)で、正規化は domain が行う。
3. **失敗の語彙は `domain.Code`(invalid_input。契約の ErrorCode と同じ文字列)と `domain.Error`**。
   adapters/httpapi の `errFromDomain` が文字列のまま写し、一致はテスト(`TestDomainCodesAreContractErrorCodes`)で固定する。
4. **操作の手順は `app.Service`**: TouchDevice を先に呼び、失敗したら本体を呼ばない。作成・置換は検証 → TouchDevice → 保存。
   readiness の確かめ方(`app.Pinger` があれば Ping、無ければ ListTeams)。時計は adapters が `time.Now` を渡す。
   app は store の失敗を包まずに返し、adapters/httpapi の `errFromApp` が ErrNotFound → 404、ErrTeamLimitReached → 400、
   それ以外 → `errFromStore`(503 / 500 とログ)に写す。
5. **イベントの購読(`adapters/events`)は `app.Store` を直接使う**(ADR-0254 決定 6 と同じ)。
6. **失効ジョブは手順(`app/expire`)と TiDB 実装(`adapters/expirestore`)に分ける**。import の検査(`app/expire/expire_test.go`)は
   両方のディレクトリを見て、禁止する import(team の adapters/httpapi・adapters/events ほか)も新しいパスに合わせた。検査の範囲は分ける前と同じ。

## 影響

- `services/team/internal/{httpapi,events,store}` は `internal/adapters/` の下へ、`internal/expire` は `internal/app/expire` と
  `internal/adapters/expirestore` へ、`store/store.go` の契約は `internal/app/ports.go` へ移った。cmd とテストの import を追従させた。
  テストの期待値は変えていない(`teams_test.go` は上限の定数を domain から引く形に、`adapters/expirestore/tidb_test.go` は
  共有していた部品〈day・defaultPolicy・discardLogger〉の写しを持つ形にした)。app の手順のテスト(`app/service_test.go`)を足した。
- team は tools/archcheck の未移行の一覧から外れ、向きの検査の対象になった。
- 振る舞い(openapi.yaml・応答・エラーの code とステータスと文言・ログ・端末 ID・保存する値・失効ジョブ・イベントの購読)は変えない。
