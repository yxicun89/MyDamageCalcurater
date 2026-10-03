## 2026-09-25: issue #237(GitOps overlayのread model欠如)を既定案で決定
Decision: 既定案(a)のinitContainer方式(起動時にpokedex exportを実行)で進める。これに伴い、pokedex-svcのserver
イメージをbalance-registryへdigest固定でpushする新しい依頼をデータレーンへ送った(急ぎではない)。
Reason: ユーザー回答(AskUserQuestion、2026-09-25)。素早さレーンが指摘した新しい依存関係(pokedex-svcイメージの
push作業がデータレーンに発生すること)を判断材料に含めた上での決定。
Impact: タイプバランスレーンが主担当として#263と合わせて実装する。素早さレーン側のoverlay・scriptsは素早さレーンが対応。
