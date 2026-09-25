# ADR-0122: 変換結果の内容ハッシュを版として記録し、取得元が同じでも変換結果が変われば投入する(issue #379)

- 状態: 採用
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #379、ADR-0101 §9(版と冪等な投入)、ADR-0104 §1(CronJob は版に変化が無ければ取り込まない)、
  ADR-0105 §2(MasterExport の `dataVersion`)、ADR-0121(move_mechanisms。この問題が見つかった変更)

## 背景

`pokedex-import` は、DB の `data_versions` と取得元(calc・Showdown・PokeAPI・設定ファイル)の版・checksum が
すべて同じなら投入をスキップする(`NeedsImport`)。この判定は「取得元のデータ」しか見ていないため、
importer の変換ロジックや DB スキーマだけが変わったとき(例: PR #376 の move_mechanisms 表の追加)は、
CronJob も `make import-k8s` も「スキップ」で終わり、新しい表は空のまま・直した変換も DB に入らなかった
(手で `-force` を付けて投入するまで)。

## 検討した案

- (A) migration の版(`schema_migrations`)と、変換ロジックの版の定数を記録して比べる。
  変換を変える PR が定数を上げ忘れると同じ問題が起きる。migration を足さない変換の修正は定数だけが頼り。
- (B) 変換結果(`Output`)の内容ハッシュを記録して比べる。変換結果が変われば必ず投入され、上げ忘れが起きない。
  スキーマの変更も、新しい表を埋める変換(= `Output` の変化)を伴うので同じ判定で拾える。**採用**。

## 決定

### 1. 変換結果の版は `data_versions` の1行(source = `importer-output`)

- 新しい列・表は足さない(migration なし)。`data_versions` は「取り込んだデータを識別する版の集合」であり、
  変換結果の版もその1つとして同じ行の形(source・version・checksum・imported_at)で持つ。
  `version` はハッシュの先頭12桁(表示用)、`checksum` は sha256 の全64桁(判定に使う)。
- `RunStore` が、取得元の版の末尾に変換結果の版を足してから `NeedsImport` と `Apply` に渡す。
  CLI・CronJob・`make import-k8s` はすべて `RunStore` を通るので、**追加の設定なしで効く**。
- 取得元の設定に `importer-output` という名前があれば、重複として `ErrInvalidInput`(exit 3)にする。
- この変更より前に投入した DB には `importer-output` の行が無いので、変更後の最初の取り込みで1回だけ投入し直す
  (全置換で中身は同じか、新しい変換の結果になる)。
- `-force` は従来どおり、版が同じでも投入する。

### 2. ハッシュの直列化は決定的にする

- `Output` のフィールド(表)ごとに、行を `encoding/json` で直列化し、**文字列順に並べてから**
  「表の名前・行数・各行」を sha256 に流す。先頭に直列化の形式の版(`importer-output/v1`)を入れる。
- 行の並び順は DB の中身を変えないので、ハッシュにも効かせない(Convert の中の map の反復順などで揺れない)。
  JSON は map のキーを整列し、行の中に改行を含まないので、同じ内容は同じバイト列になり、行の境界もあいまいにならない。
- フィールドは reflect で列挙する。`Output` に表を足せば自動でハッシュに入る(テストで全フィールドを確かめる)。
- 直列化の形式を変えたら形式の版を上げる(全件が1回再投入になるだけで、害は無い)。

### 3. 表示

- CLI は照合の後に `import: 変換結果の版 importer-output=<先頭12桁>` を出す(dry-run でも)。
  kubectl logs と実データの dry-run で、次の取り込みが投入になるかを比べられる。
- スキップ時の表示は「取得元の版と変換結果に変化が無いのでスキップ」にする。
- pokedex の MasterExport の `dataVersion`(`source=version` の連結。ADR-0105 §2)にも
  `importer-output=<先頭12桁>` が入る。変換結果が違えば `dataVersion` も違うので、記録の識別子としてはむしろ正確になる
  (calc-svc は記録とログに使うだけで、計算には影響しない)。

## 影響

- 変換ロジック・スキーマの変更は、取得元の版を上げなくても次の CronJob / `make import-k8s` で DB に入る。
- 変換結果が同じならスキップ(従来どおり)。1回の取り込みで sha256 を1回計算するだけで、所要時間はほぼ変わらない。
- テスト: `importer/output_version_test.go`(決定性・並び順の無視・全表の網羅・RunStore の判定)、
  `cmd/import/main_test.go`(取得元が同じで変換結果が違えば投入・版の表示)、
  `importer/mysql_test.go` の `TestRunReimportsWhenOutputChanges`(`-tags mysql`。変更前の DB からの再投入とその後のスキップ)。

## 追記(2026-09-25): ハッシュに入る列の条件

ハッシュは行の型を `encoding/json` で直列化して作るので、`json:"-"` の列や非公開(小文字始まり)の列は入らない。
`Output` の行の型にはタグを付けず、すべて公開フィールドにする(列を足すときもこの条件を守る。PR #380 の critic 軽微指摘)。
