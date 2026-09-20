このリポジトリで、タイプバランスチェッカー(services/balance/)の TB0 から実装を進めてください。

進め方:
1. AGENTS.md を読み、担当範囲を厳守する(services/balance/ 以外は変更しない)
2. docs/ai-shared/CURRENT_STATE.md と DECISIONS.md を読む
3. docs/type-balance-design.md(設計書)を読む。これが実装の唯一の起点
4. docs/ai-shared/claude-review.md の指摘(マスタはAPI経由、Goモジュール共有はまだしない、
   TB0でEffect構造を先に決める)を設計に反映する
5. feat/codex-tb0-foundation ブランチを main から新規に切って作業する(AGENTS.md「Git ブランチ運用」参照)
6. TB0(ディレクトリ構成・型定義・タイプ相性表・Dockerfile・Kustomize base/overlays・
   Argo CD Application・k3d上での最小疎通・単体テスト基盤)を実装する
7. pokedex-svc のタイプ相性データは、そのAPI(または既存の MySQL 取り込みデータのエクスポート)
   から取得する形にする。取得方法が不明な場合は DECISIONS.md に確認事項として書いて止まる
8. 完了したら docs/ai-shared/CODEX_LOG.md と CURRENT_STATE.md の Type Balance Checker 欄を更新する
9. 判断に迷う設計変更(既存インターフェースへの影響があるもの)は実装せず、
   DECISIONS.md に提案として書いて報告する

pokecalc(ダメージ計算アプリ)側のコードは読んでよいが変更しないでください。
