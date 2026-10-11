# ADR-0252: calc-svc をヘキサゴナルの形にそろえる(計算・一括・逆算)

- 状態: 採用
- 日付: 2026-10-12
- レーン: API
- 関連: ADR-0811(ヘキサゴナルの採用)、ADR-0200(calc-svc の契約・失敗の順番)、ADR-0212(計算イベント)、ADR-0801(締め切り)、docs/plan/architecture-adoption.md §6 S3

## 背景

S3(docs/plan/architecture-adoption.md §6)で calc-svc を `internal/domain`・`internal/app`・`internal/adapters` にそろえる。
計算・一括・逆算の手順はハンドラ(`httpapi`)にあり、列挙の検証(生成型の `Valid()`)・ID の解決・入力の検証が
決まった順で交互に並んでいる(例: 攻撃側の列挙 → 攻撃側の ID → 防御側の列挙 → … → 技の ID → 場の列挙)。
同じ入力に同じ code を返すこと(ADR-0200)が契約なので、この順番を変えずに手順を app に移す必要がある。
一方、app・domain は生成型(`services/internal/api`)を import できない(ADR-0811 の向きの規則)。

## 決定

1. **列挙が契約に含まれるかの判定は adapters、失敗にする順番と文言は domain・app**。
   adapters/httpapi は生成型の `Valid()` の結果を `domain.Enum{Value, Known}` に入れて渡し、domain の `ParseFormat`・`ParseStatus`・
   `ParseTeraType`・`ParseField` が手順の中の同じ位置で invalid_enum にする。列挙の値の一覧を domain に複製しない(契約が正)。
2. **失敗の語彙は `domain.Code`(契約の ErrorCode と同じ文字列)と `domain.Error`**。HTTP のステータスへの写しは adapters/httpapi の
   `statusForCode` のまま。engine の sentinel → code の表は `domain.FromEngine` に移す。想定外の失敗の元は `Error.Cause` に残し、ログは adapters が出す。
   一致はテスト(`TestDomainCodesAreContractErrorCodes`)で固定する。
3. **ポートは app で宣言する**(G10): `app.Master`(マスタの参照。`adapters/master.Store` はその別名にして既存の呼び名を保つ)、
   `app.EventPublisher`(イベント)、`app.Expired`(締め切りの確認。実装は `httpguard.Expired`)。締め切り超過は `app.ErrDeadlineExceeded` で返し、
   adapters が今までどおり 503 upstream_unavailable + Retry-After にする。
4. **計算イベントの中身(生成型を持つ `calcevents.CalcDetail`)は adapters が作る**(G9 の calc 側)。app のイベントはリクエスト(app の型)と
   engine の結果を持ち、adapters/httpapi が生成型へ写し戻す。省略の有無(nil)も含めて元のリクエストと一致することをテストで固定する。
   `services/internal/calcevents` 自体の型は record・team も使うので変えない。
5. **ヘッダ・厳格デコード・件数上限(ADR-0208)は adapters に残す**。どれも列挙の検証より前に行う既存の順番なので、境界で済ませても順番は変わらない。
6. **調整(`adjust.go`)は次の PR**。それまでは adapters/httpapi の薄い関数(`resolveIndividual`・`parseFormat` など)が app・domain に委ねる。

## 影響

- `services/calc/internal/{httpapi,master,events}` は `internal/adapters/` の下に移った。`make test` の NATS のテストの対象パスと、
  Web の `exportSnapshot.contract.test.ts` が読む `export.go` のパスを追従させた。
- calc は tools/archcheck の未移行の一覧から外れ、向きの検査の対象になった。
- 振る舞い(openapi.yaml・エラーの code とステータス・失敗の順番・イベントの中身)は変えない。
