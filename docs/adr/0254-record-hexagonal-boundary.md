# ADR-0254: record-svc をヘキサゴナルの形にそろえる(お気に入り・計算履歴・失効ジョブ)

- 状態: 採用(2026-10-12。実装済み)
- 日付: 2026-10-12
- レーン: API(データレーンが代行。API レーンが止まっていたため ec レーンと合意)
- 関連: ADR-0811(ヘキサゴナルの採用)、ADR-0252(calc の同じ移行。流儀の先行例)、ADR-0209(保存データの保持・削除・端末 ID の境界)、
  ADR-0220(失効ジョブ)、ADR-0227・ADR-0228(お気に入り)、ADR-0230(計算履歴)、docs/plan/architecture-adoption.md §6 S4・§2.3 G6

## 背景

S4(docs/plan/architecture-adoption.md §6)で record-svc を `internal/domain`・`internal/app`・`internal/adapters` にそろえる。
G6(HTTP 層が SP・レベルの上限などのドメインの規則を持つ。`httpapi/favorites.go` が `engine.MaxSPPerStat`・`engine.DefaultLevel` で検証)を解消する。
httpapi のテストは偽の `store.Store`(`fixture_test.go` の fakeStore)で呼ばれたメソッドと端末 ID を記録して越境を確かめているので、
store の呼び出しの列(TouchDevice → 本体)を変えずに手順を app に移す必要がある。失効ジョブ(`internal/expire`)は手順(Run)と
TiDB の SQL が 1 つのパッケージにあり、import の検査(`expire_test.go`)がそのパッケージを見ている。

## 決定

1. **お気に入りの内容の検証・正規化は domain**(`domain.NormalizeFavorite`・`NormalizeIndividual`・`NormalizeCalc`・`EncodeFavorite`)。
   SP・レベルの上限は engine の定数を domain から参照し、ランク・ラベル・内容の大きさ(4096 バイト)の上限も domain に置く(G6)。
   受け口(`*Input`。JSON のキーは契約と同じ)と保存する形(`*Snapshot`。宣言順がキーの順で、既存行のハッシュを決める)も domain の型にする。
   受け口を adapters に別に持つと、同じ形の構造体が 2 組になり写しのコードが増えるため。adapters は本文を `domain.FavoriteInput` へ厳格に読み込む。
2. **列挙が契約に含まれるかの判定と省略時の値(none)は adapters が `domain.Enums` で渡す**(ADR-0252 決定 1 と同じ考え方。列挙の値を domain に複製しない)。
   失敗にする順番と文言は domain のまま。省略時の値が契約の列挙に含まれることはテストで固定する。
3. **失敗の語彙は `domain.Code`(invalid_input・invalid_enum。契約の ErrorCode と同じ文字列)と `domain.Error`**。adapters/httpapi の
   `errFromDomain` が文字列のまま写す。一致はテスト(`TestDomainCodesAreContractErrorCodes`)で固定する。
4. **record DB のポートは app で宣言する**(`app.Store`・型・`ErrUnavailable` ほか。旧 `internal/store/store.go` の中身をそのまま移した)。
   `adapters/store` はその TiDB 実装で、型・定数・失敗を同じ名前の別名として持ち(calc の `adapters/master.Store` と同じ)、既存の呼び名とテストを保つ。
   到達確認は `app.Pinger`(TiDB の実装だけが持つ)。
5. **操作の手順は `app.Service`**: TouchDevice を先に呼び、失敗したら本体を呼ばない(`touchAndRun`)。計算履歴の 1 行多く読む・次の位置・保持期間の下限、
   お気に入りの正規化 → 大きさの確認 → TouchDevice → 作成、readiness の確かめ方。時計は adapters が `time.Now` を渡す。
   ヘッダ・クエリの端末 ID の拒否・厳格デコード・limit の範囲・cursor の形式・お気に入り ID の形式は契約の HTTP の形なので adapters/httpapi に残す
   (どれも手順より前に行う既存の順番なので、境界で済ませても順番は変わらない)。
6. **イベントの購読(`adapters/events`)は `app.Store` を直接使う**。本文の解釈が外側の共有パッケージ `calcevents` に依存するため app に置けず、
   手順は「解釈 → 保存 → Ack/Nak/Term」だけで薄い。
7. **失効ジョブは手順(`app/expire`。Policy・Result・ポート Store・Run)と TiDB 実装(`adapters/expirestore`)に分ける**。
   import の検査(`app/expire/expire_test.go`)は両方のディレクトリを見て、禁止する import(HTTP・NATS・echo・他サービス・
   record の adapters/httpapi・adapters/events)も新しいパスに合わせた。検査の範囲は分ける前と同じ。

## 影響

- `services/record/internal/{httpapi,events,store}` は `internal/adapters/` の下へ、`internal/expire` は `internal/app/expire` と
  `internal/adapters/expirestore` へ移った。cmd とテストの import を追従させた。テストの期待値は変えていない
  (`favorites_calc_test.go` の大きさの境界のテストは domain の関数を直接呼ぶ形に、`adapters/expirestore/tidb_test.go` は共有していた部品の写しを持つ形にした)。
- record は tools/archcheck の未移行の一覧から外れ、向きの検査の対象になった。
- 振る舞い(openapi.yaml・応答・エラーの code とステータスと文言・ログ・失効ジョブ・イベントの購読と冪等性・TiDB のトランザクション)は変えない。
