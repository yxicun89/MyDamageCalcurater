# ADR-0015: balance の相性表を P1-13 のデータから読む

- 状態: 採用(2026-09-21。ユーザー決定「balance の相性表は P1-13 でデータ化されたものを使う」を受けた実装方式)
- 日付: 2026-09-21
- 関連: ADR-0012 §7(temporary adapter)、ADR-0013(タイプ相性表はデータ)、ADR-0002(Git に置けるデータ)、ADR-0014

## 背景
TB0 の `TemporaryTypeChart` は相性表をコードに持っていた(ADR-0012 で temporary として許容)。CLAUDE.md は相性表のハードコードを禁じ、
ダメージ計算レーンの P1-13 で相性表は `testdata/golden/typechart.json`(oracle から生成。数値と英語 ID だけでコミット可。ADR-0002)になった。
balance は engine を import しない(claude-review.md 指摘2・ADR-0012)。Docker のビルドコンテキストは `services/balance` だけ。

## 決定
1. `testdata/golden/typechart.json` を `services/balance/internal/master/data/typechart.json` に**バイト複製**し、go:embed で同梱する。
   正は元ファイル。複製がずれたら `TestEmbeddedTypeChartMatchesSharedData` が失敗する(ルートの `make test` に含まれる)。直すのは
   `make balance-sync-typechart`。
2. `master.LoadTypeChart` は元ファイルの schema(schemaVersion 1)を検証して読む: タイプ集合が balance の 18 タイプとちょうど一致、
   コードは 0/1/2/4(×2 した整数)、省略された組は等倍、未知のキー・未知のフィールド・後続 JSON は不正(`ErrInvalidTypeChart`)。
   コードを balance の整数倍率(4 = 等倍)へ変換する。float は使わない。
3. 起動時に読み、不正なら起動失敗。`TemporaryTypeChart` とそのテストは削除する(削除前に 18×18 全件一致を確認した。コミット ea64577)。

## 影響
- oracle の版が変わって typechart.json が再生成されたら、balance の複製も更新が要る(テストが気づかせる)。schema を変える場合は
  DECISIONS.md に書く(balance の loader が追従する)。
- 共通マスタ(P2-2)の相性表が DB や `data/generated/` で配布されるようになったら、`TypeChartProvider` の adapter を差し替える。

## 却下した案
- engine を import して `engine.TypeChart` を使う: モジュールの共有をしない方針(当面)に反する。
- Docker のビルドコンテキストをリポジトリルートにしてファイルをコピー: 他サービスのファイルまでコンテキストに入り、balance の独立性が下がる。
- 環境変数で相性表のパスを渡す: 相性表は架空でない実データとしてコミット可能なので、同梱して常に使える方が単純。
