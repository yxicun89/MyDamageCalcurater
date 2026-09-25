# ADR-0118: 相性表・効果定義・calc の版の「正」を決め、もう片方との一致を検査する(issue #280)

- 状態: 採用
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #280、#259(タイプバランス レーン。balance の埋め込みコピーと DB の突き合わせ)、ADR-0013(相性表はデータ)、
  ADR-0015(balance の相性表の複製)、ADR-0101(importer)、ADR-0103(照合と Blocker)、ADR-0005(効果定義の形)

## 背景

同じ中身のファイルが2か所にあり、どちらを直しても `make test`・`make test-golden` が通ってしまっていた。

| 中身 | 本番(DB)が使うもの | ゴールデン・balance・Web が使うもの |
|---|---|---|
| 相性表 | importer が calc スナップショット(`data/generated/calc/<版>/snapshot.json`)から作る `types`・`type_chart` | `tools/golden` が生成する `testdata/golden/typechart.json` |
| 持ち物・特性の効果定義 | `data/importer/effects.json`(キーは ID) | `testdata/golden/effects.json`(キーは @smogon/calc の英語名) |
| @smogon/calc の版 | `data/importer/config.json` の `sources.calc`、`tools/importer/fetch-calc.mjs` の `EXPECTED_VERSION`、`tools/importer` の依存 | `tools/golden` の依存、`testdata/golden/metadata.json`・`typechart.json` の `version` |

ゴールデンテストが保証するのはゴールデン側の定義での計算で、本番の DB に入る定義での計算ではない。

## 決定

1. **効果定義の正は `data/importer/effects.json`**(ADR-0101 のとおり本番の正)。`testdata/golden/effects.json` はその写しで、
   `services/pokedex/importer/source_of_truth_test.go` の `TestGoldenEffectsMatchImporterEffects` が、キーを toID で正規化して
   両者の items・abilities が JSON として等しいことを確かめる。片方だけを編集すると `go test ./pokedex/importer/...`
   (`make test`)が失敗する。直すときは正(data/importer)を先に直し、ゴールデン側を同じ値にして `make golden-generate` で再生成する。
   - 生成物にしなかった理由(issue の既定案からの変更): ゴールデン側のキーは生成器(`tools/golden/generate.mjs`)が
     @smogon/calc に渡す英語名で、ID から英語名を戻すには calc の表を引く変更が生成器に要る。生成器の変更は
     ゴールデンの出力(sha256 を engine のテストが固定している)を動かしうるので、本 issue の「一致の仕組みを作るだけ・
     データの内容は変えない」の範囲を超える。一致検査なら中身を1バイトも変えずに同じ保証(片方だけの変更は通らない)が得られる。
2. **相性表の正は importer が取り込む calc スナップショット**(DB と calc-svc が使う実運用の表)。`testdata/golden/typechart.json` は
   同じ版の calc から別経路で生成した参照で、import の照合(Reconcile)で比べる。
   - `pokedex-import` は `-typechart <path>`(既定は `<data>/../testdata/golden/typechart.json`)で参照を読む。読めない・形式が
     違うときは ErrInvalidInput(終了コード 3)で止める。照合を黙って飛ばさない。
   - 比べるのは、参照の `version` と `sources.calc`、タイプ ID の集合、両方にあるタイプの全組の倍率コード(importer は等倍の組を
     省くので、無い組は等倍 2 として比べる)。食い違いは Blocker `type-chart-reference-mismatch`(ID は `version`・`types`・
     `<攻撃>><防御>`)。ADR-0103 と同じく報告を書き、DB は変えない。
   - 実データはテストに置けないので、比較器は架空データ(`services/pokedex/importer/testdata/fictional-reference/typechart.json`)で
     単体テストする。本番の表どうしの一致は import のたびに確かめる(導入時の実データの dry-run で 18 タイプ・324 組の差 0 を確認)。
   - importer のイメージ(`services/pokedex/Dockerfile` の `importer` ステージ)に `testdata/golden/typechart.json` を
     `/app/testdata/golden/typechart.json` として焼く(CronJob も既定の場所で読める)。
   - 参照側の `excludedTypes`(`???`・`stellar`)と importer の `excludeTypes`(`???`)は違うが、比べるのは取り込んだタイプの集合
     なので、Champions 世代に stellar が無い今は一致する。calc が stellar を返すようになれば `types` の Blocker で人が気づく。
3. **calc の版は `data/importer/config.json` の `sources.calc` を正**とし、`TestCalcVersionPinnedConsistently` が
   `fetch-calc.mjs` の `EXPECTED_VERSION`、`tools/importer`・`tools/golden` の `package.json` と `package-lock.json`、
   `testdata/golden/metadata.json`・`typechart.json` の `version` がすべて同じことを確かめる。1か所から読む形に寄せなかったのは、
   Node の取得スクリプト・npm の依存・生成物がそれぞれ別の仕組みで版を持つため(npm の依存は package.json に書くしかない)。
   版を上げるときは全部を同じ PR で揃える。

## 対象外(他レーンへの依頼)

- balance の埋め込みコピー(`services/balance/internal/master/data/typechart.json`)と `testdata/golden/typechart.json` の一致は、
  既存の `TestEmbeddedTypeChartMatchesSharedData`(`make balance-sync-typechart`)に任せる。本 ADR の照合で DB ⇔ golden が
  つながるので、balance ⇔ golden ⇔ DB が推移的に一致する。export に相性表を足す案(#259)はタイプバランス レーンの判断。
- Web の `@typechart`(`web/vite.config.ts`)は golden を直接読むので、golden ⇔ DB の一致で足りる。変更しない。

## 影響

- 相性表または効果定義を片方だけ変える変更は、テスト(効果定義・版)か import の照合(相性表)で止まる。
- calc の版を上げると、ゴールデンを再生成するまで import が `type-chart-reference-mismatch`(`version`)で止まる。
  これは意図どおり(ゴールデンで検証していない版を本番に入れない)。
