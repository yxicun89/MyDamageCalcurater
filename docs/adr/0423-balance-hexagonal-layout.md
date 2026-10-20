# ADR-0423: balance-svc をヘキサゴナルの形にそろえる(S8)

- 状態: 採用(2026-10-21。実装済み)
- 日付: 2026-10-21
- レーン: タイプバランス(データレーンが代行。タイプバランスレーンが止まっていたため)
- 関連: ADR-0811(ヘキサゴナルの採用)、ADR-0252(calc)・ADR-0254(record)・ADR-0255(team)の同じ移行、
  ADR-0015(相性表の埋め込み)、ADR-0145・ADR-0107(read model の厳格な読み込み)、ADR-0422(おすすめの skew)、
  docs/plan/architecture-adoption.md §6 S8

## 背景

S8 で balance-svc を `internal/domain`・`internal/app`・`internal/adapters` にそろえる
(`internal/balance` → `domain`、`master`・`httpapi`・`httpguard`・`httpmetrics` → `adapters`、app は薄い)。
`internal/balance` の外部テストパッケージ(`balance_test`)の 6 ファイル(`ability_chart_test.go`・`analysis_chart_test.go`・
`coverage_test.go`・`moverange_test.go`・`recommend_test.go`・`threat_test.go`)は、同梱の相性表を
`master.EmbeddedTypeChart()` で読んで domain を検査している。domain の下に置いたままだと
domain → adapters の import になり、`tools/archcheck`(テストファイルも検査する)に違反する。

## 決定

1. **パッケージ名は `domain`**(他サービスと同じ)。参照側は `balance.X` を `domain.X` に書き換えただけ。
2. **同梱の相性表を使う上の 6 ファイルは `internal/adapters/master` へ移し、パッケージ名を `master_test` にする**。
   中身(テストの関数・補助関数・期待値。`recommend_test.go` の独立に skew を計算する oracle を含む)は変えない。
   domain の純粋な単体テスト(架空の相性表を使う内部テスト)は domain に残る。
   - 採らなかった案: 相性表の埋め込み(`type_chart_data.go`)を domain へ移す(S8 の「master → adapters」と食い違い、
     read model の loader が 2 か所に分かれる)。archcheck でテストファイルを除外する(全サービスの検査を弱める)。
3. **app は doc.go だけ**。各操作は read model の Provider を渡して domain の関数を 1 回呼ぶだけで、束ねる手順が無い
   (gateway の S7 と同じ扱い)。
4. 振る舞い(応答・エラー・ログ・メトリクス・環境変数・read model の `DisallowUnknownFields` による厳格な読み込み)、
   gitops overlay・digest・Makefile の balance ターゲットの挙動は変えない。テストの相対パスは 1 段深くなった分だけ直した。

## 結果

- `tools/archcheck` の未移行の一覧から balance を外した(残りは speed)。
- `go test ./services/balance/...` のテスト関数の数は移行前と同じ(304)。
