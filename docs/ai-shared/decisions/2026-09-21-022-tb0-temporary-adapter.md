## 2026-09-21: TB0 のタイプ相性表は差し替え可能な temporary adapter とする
Decision: 共通マスタの恒久正本が未確定の間、現行18タイプ相性を balance 内の temporary/static adapter として
利用してよい。ただし純粋コアは provider interface に依存し、正式な共通スナップショット確定後に差し替える。
Reason: Claude 側 P2-1 の ADR-0002 は共通マスタのコミット済みスナップショットを提案しているが人間確認待ち。
一方、タイプ相性コア・HTTP・Kubernetes 基盤はその確定を待たずに検証できる。
Impact: 前エントリ「TB0 タイプ相性データの取得元・契約を確認待ち」の停止条件は解除する。
temporary データを balance 独自の恒久正本として扱わず、API や新サービスを先行追加しない。
