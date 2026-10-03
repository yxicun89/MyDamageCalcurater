# Current State

レーンごとの状態(Lane / Active / Branch / Status / Next)は、レーン別のファイルにある(ADR-0170。レーンの PR が同じファイルで衝突しないため)。
自分のレーンのファイルだけを編集する。全レーン共通の事項だけをこのファイルに置く。

| レーン | 見出し | ファイル |
|---|---|---|
| データ | Damage Calculator | [state/data.md](state/data.md) |
| API | API | [state/api.md](state/api.md) |
| Web | Web | [state/web.md](state/web.md) |
| iOS | iOS | [state/ios.md](state/ios.md) |
| タイプバランス | Type Balance Checker | [state/tb.md](state/tb.md) |
| 素早さ | Speed | [state/speed.md](state/speed.md) |
| 判定 | Judge | [state/judge.md](state/judge.md) |
| 運用 | Ops | [state/ops.md](state/ops.md) |

## Shared Interfaces
- Pokemon ID: pokedex-svc の `{図鑑番号4桁}-{フォルム3桁}` 形式に準拠
- Type: 18タイプの英語小文字ID(fire, water, ...)。表示名・色は docs/design.md のトークンに準拠
- Type multiplier: 分数ではなく整数表現(claude-review.md 参照)
- サービス境界: damage-calc と balance は兄弟。相互の実行時 API へ直接依存しない
- 共通マスタ: 正本は1つ。`feat/claude-p1-engine` の ADR-0002 にあるコミット済みスナップショット案を候補とし、人間の確認待ち
- balance の type chart: P1-13 の `testdata/golden/typechart.json` をバイト複製して同梱(ADR-0015)。正式マスタ確定後に provider を差し替える
- 共有状態の正本: `origin/main` の `docs/ai-shared/`。未マージの続きは各レーン欄の `Branch` の最新コミット(COORDINATION.md)
- 開発の正本: private の `origin` の `main`(PR でのみ更新。Argo CD が参照)。作業ディレクトリはレーンごとに `~/MyDamageCalcurater`(ダメージ計算)と `~/MyDamageCalcurater-tb`(タイプバランス)
