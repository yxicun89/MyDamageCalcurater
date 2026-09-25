# ADR-0115: importer は取得元の不整合を黙って捨てず、止める(issue #269・#310・#311)

- 状態: 採用
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #269・#310・#311、ADR-0101 §5(タイプの節)、ADR-0103 §1・§4(照合報告)、ADR-0104 §3(終了コード)

## 背景

`convertTypes` は calc の相性表(`typeChart`)の攻撃側・防御側の名前が取り込むタイプに無いと `continue` で
捨てていた。除外タイプ(`excludeTypes`)を捨てるための分岐が、未知の名前(表記・大文字小文字の変化)も
同じく捨てていたため、取得元の形が変わると相性表 0 行のまま投入が成功する。engine は「無い組は等倍」
なので、calc-svc は全組み合わせ等倍で計算し続け、ゴールデンテスト(testdata/golden/typechart.json)も
この誤りを検出しない。

## 決定

1. 相性表の検査(#269)
   - 攻撃側・防御側の名前は、除外タイプだけを捨てる。それ以外で取り込むタイプに無い名前は `ErrInvalidData`
     (終了コード 3)。
   - 取り込むタイプはどれも攻撃側のキーを持つこと。無ければ `ErrInvalidData`。calc の相性表は等倍の組を
     省くので、等倍しかないタイプは空オブジェクトとして明示される(tools/importer/fetch-calc.mjs は
     全タイプのキーを書く)。行の有無ではなくキーの有無で判定し、等倍しかないタイプを許す。
   - キーがそろっていても行(等倍でない組)が1件も無ければ `ErrInvalidData`(取得側で effectiveness が
     取れず全タイプが空オブジェクトに落ちた形。全組み合わせ等倍の相性表は実在しない)。
   - 照合報告の `summary.typeChart`(`types`・`rows`)と要約の `typeChart: types=N rows=M` に件数を出す。
2. Blocker(照合の裁定)ではなく `ErrInvalidData` にする。Blocker は「両取得元の値の食い違いを人が裁定する」
   もの(ADR-0103)で、裁定すれば進める。取得元そのものの形の崩れは裁定の対象ではなく、直すまで投入してはならない
   ので、既存の「入力同士の矛盾」(除外名が実在しない等。ADR-0101 §5)と同じ扱いにする。

## 結果

- 固定した calc 0.12.0 の Champions 世代では `types` と `typeChart` のキーが同じ `gen.types` から作られ、
  防御側の名前もすべて `types` に含まれる(2026-09-25 に同版のライブラリで確認)。実データの照合結果
  (blockers: none)は変わらない。要約に `typeChart:` の1行が増える。
- engine の「無い組は等倍」の規則は変えない。

## 追記: 技の値の範囲と DB の制約違反の分類(issue #310)

- 背景: 取得元の技の PP・命中を範囲検査せずに投入していた。`uint8(pp)` の型変換で pp=300 が 44 に化けて
  CHECK(1..64)を通り、命中 150 は CHECK 違反で投入が失敗するが、MySQL のエラーは sentinel でないので
  終了コード 1(再試行で直りうる)になり、CronJob が同じ失敗を再試行していた。
- 決定:
  1. 変換(`convertMoves`)で、取り込む技の pp が 1..64 外、命中が 0(必中)でも 1..100 でもなければ
     `ErrInvalidData`。範囲は migration の CHECK(ADR-0100 §3)を正とし、importer の定数
     (`MovePPMin` 等)が CHECK と一致することをテストで確かめる(二重に持つ値のずれを検出する)。
  2. 投入の CLI は、MySQL の 1062(重複)・1264(型の範囲外)・1406(長すぎる)・1452(外部キー違反)・
     3819(CHECK 違反)を終了コード 3 に分類する。データ自体が制約に合わないので再試行しても同じ結果になり、
     トランザクションは巻き戻るので DB は変わらない。接続断・デッドロック等は従来どおり 1。
- 結果: 取得元の値が変わらない限り、実データの照合結果は変わらない(取り込み済みの値はすでに CHECK を通っている)。

## 追記: 取得元の重複・不正行(issue #311)

- 背景: 変換は取得元の一覧を ID で引く map にするので、同じ ID の行が2つあると後の行が黙って勝っていた
  (calc の技は後の行だけが Showdown と比べられる。PokeAPI は toID(slug) が衝突すると後の名前が採られる)。
  PokeAPI の空白だけの日本語名も採られ(CHECK は CHAR_LENGTH > 0 なので通る)、`fetch-pokeapi.mjs` は
  列数がヘッダと違う CSV の行を黙って捨てていた。
- 決定:
  1. 変換の最初に、calc(toID(名前)。種族・技・持ち物・特性・性格)・Showdown(id。種族・技・持ち物・
     特性・性格)・PokeAPI(toID(slug)。種族・フォーム・技・持ち物・特性・タイプ・性格)の重複を
     `ErrInvalidData` で止める。どちらを採るかは人が決める(Blocker の裁定ではなく取得元の修正・除外が要る)。
  2. 日本語名が空白だけなら採らず、次の言語・英語名へ進む(英語名なら従来どおり name-fallback の警告)。
  3. `tools/importer/pokeapi-csv.mjs`(`fetch-pokeapi.mjs` から切り出した CSV パーサ)は、列数違いの行が
     あれば件数とファイル名を出して失敗する。`make test-tools` で検証する。
- 結果: 2026-09-25 に、固定版の calc 0.12.0 の一覧と、固定 commit の PokeAPI の CSV(抽出対象と同じ絞り込み:
  日本語名を持つ行・既定でないフォーム)に、toID の重複・列数違いの行・空白だけの日本語名が無いことを確かめた。
  PokeAPI の `items.csv` には toID が同じ `roseli-berry` が2行あるが、片方は日本語名を持たず抽出されない。
  Showdown の id は取得元のオブジェクトのキーなので重複しない。実データの照合結果は変わらない見込みで、
  マージ後に `make import-dry-run` の `blockers: none` を確かめる。

## 追記(2026-09-25): Showdown の技の別の版を正規の1件にまとめる

PR #347 のマージ後、実データの `make import-dry-run` が「Showdown の技 id が重複している」で止まった。Showdown の Dex は
同じ技のタイプ違いの版(16 件)を、名前は別のまま基本形と同じ id で返す。以前は ID で引く map で後の行が黙って勝っていた。

- 決定: `Convert` の最初(`foldShowdownMoveVariants`。`services/pokedex/importer/convert_variants.go`)で、同じ id の行のうち
  toID(名前) == id の行がちょうど1件あるときだけ、それを正規として残し、ほかの版は `move-variant-folded` の警告(ID は版の
  toID(名前)、Detail はまとめ先の id)に出して除く。正規の行が無い・複数ある(完全な重複を含む)ときはまとめず、
  #311 の重複検査で止める。
- 確認: 実データで `blockers: none`、技の取り込み件数・照合の verdict は変わらない(`move-variant-folded: 16`)。
- 反省: 取得元の値に新しい検査を足す変更は、マージ前に実データで `make import-dry-run` を実行して `blockers: none` を確かめる
  (架空データと固定版の抜き取りだけでは、Dex が返す形の違いを見逃した)。
