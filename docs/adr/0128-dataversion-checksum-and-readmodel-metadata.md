# ADR-0128: dataVersion に checksum を含め、pokedex export に版(metadata.json)と相性表(type-chart.json)を足す

- 状態: 採用
- 日付: 2026-10-01
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #281(データ部分)、issue #108-a、issue #259-a、issue #403(パッケージ D20)、ADR-0105 §2・§5、
  ADR-0127(1つの読み取り専用トランザクション)、ADR-0013(タイプ相性表はマスタ)、ADR-0015(balance の埋め込み)

## 背景

- 内部 API `GET /internal/pokedex/master` の `dataVersion` は `data_versions` の `source=version` を連結するだけだった。
  effects・regulations・importer-config・name-overrides の version は常に `"local"` なので、effects.json を直しても
  `dataVersion` が変わらず、中身の違うマスタを区別できない(#281)。
- `pokedex export` の4ファイルには元 DB の版が無く、balance・speed がどの版を読んでいるかを照合できない(#108)。
- タイプ相性表は balance が golden の複製を埋め込んでおり、DB の `type_chart` と突き合わせる手段が無い(#259)。
- balance の loader(`services/balance/internal/master`)・speed の loader(`services/speed/internal/master`)は
  `DisallowUnknownFields` で、balance の schema(`services/balance/schema/*.schema.json`)は `additionalProperties: false`。
  既存の4ファイルに欄を足すと、今の loader と `checkreadmodel` が拒否する。

## 検討した案

版の置き場所:

- (A) 既存の4ファイルに省略可の `dataVersion` を足す。loader・schema が未知のフィールドを拒否するので、
  タイプバランス・素早さレーンの loader・schema の変更が先に要る(レーンをまたいだ同時変更)。不採用。
- (B) 別ファイル `metadata.json` に出す。既存の4ファイルはバイト単位で今のまま。今の loader と `checkreadmodel` は
  知らないファイルを読まないので影響しない。**採用**。consumer が版を持つようになったら、#108 の照合は
  metadata.json の `dataVersion` と各 consumer の表示を比べる。

dataVersion の形: (1) `source=version@checksum先頭8桁`(#281 の既定案。**採用**)、(2) checksum 全体(64桁×7件で長い)、
(3) 全 checksum の合成ハッシュ(どの取得元が変わったかが読めない)。

## 決定

1. `dataVersion` は `data_versions` の各行を `source=version@checksum先頭8桁` にし、source 昇順に `,` で連結する
   (例 `calc=test-calc-1@22222222,pokeapi=…@33333333`)。checksum は既存の `data_versions.checksum`
   (CHECK で 64 桁の小文字16進。取得元ファイルの SHA-256)を使う。内部 API と export は同じ関数で作る(同じ値になる)。
   応答の形(`MasterExport.dataVersion` は1文字以上の文字列)は変わらないので、`api/openapi.yaml` と calc-svc は変えない
   (calc-svc は空でないことだけを見る)。契約の説明文に形を書く更新は API レーンが行う(#281 の残り)。
2. export は既存の4ファイルに加えて2ファイルを書く。既存の4ファイルの形は変えない。
   - `metadata.json`: `{"schemaVersion":1,"dataVersion":"<1 と同じ値>"}` だけ。
   - `type-chart.json`: `services/balance/schema/type-chart.schema.json` の形。`source` は `"pokedex"`、
     `version` は 1 と同じ dataVersion、`generation` は `0`(対象の Champions。golden と同じ番号)、`note` はコードの意味、
     `excludedTypes` は `[]`(除外タイプは importer が DB に入れない)、`types` は DB の `types` の ID 昇順、
     `effectiveness` は types × types の全組(DB に行が無い組は等倍 `2`。importer は等倍の組を省く)。
     使用可能集合では絞らない(相性表はレギュレーションに依らない)。タイプ数が 18 かどうかは export では検査しない
     (表はマスタのデータ。balance の loader と schema が 18 を要求する)。
3. dataVersion は export の他の SELECT と同じ読み取り専用トランザクションの中で `ListDataVersions` から取る(ADR-0127)。
   `data_versions` が空なら `ErrInvalidExport`(版の無い read model を出さない)。
4. `WriteDir` は `metadata.json` を最後に書く。各ファイルは従来どおり一時ファイル + rename。metadata.json が
   読めた時点で他の5ファイルは同じ export の中身になっている。
5. balance が `type-chart.json` を `BALANCE_TYPE_CHART_PATH` で読むこと、balance・speed・calc が版を表示すること、
   runbook と orchestration(#108 の残り・#281 の残り)は各レーンの持ち物で、本 ADR の対象外。

## 影響

- dataVersion の値が変わる(形は同じ)。calc-svc は記録とログに使うだけで、計算結果は変わらない。
- checksum の先頭8桁(32 ビット)なので、同じ取得元で先頭8桁が偶然一致する別内容は区別できない。版の照合
  (取り違えの検出)の目的には十分とし、改ざん検出には使わない。
- テスト: `services/pokedex/internal/httpapi/master_dataversion_test.go`、
  `services/pokedex/internal/readmodel/readmodel_metadata_test.go`・`readmodel_typechart_test.go`
  (golden の `testdata/golden/typechart.json` を偽の DB に入れて export の表が一致することを確かめる)、
  実 MySQL は `services/pokedex/importer/dataversion_mysql_test.go`(`make test-db`)。
