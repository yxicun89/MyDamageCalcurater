## 2026-10-21: k3d の共有クラスタを変更するスクリプトは mkdir ロックで同時に 1 つにする(API レーン / ADR-0805)
Decision: `scripts/lib/k3d-lock.sh` の `k3d_lock_run` を、k3d-deploy-latest・k3d-m2-deploy・up・k3d-deploy-tagged・argocd-bootstrap・observability-bootstrap・tidb-operator-bootstrap の先頭で呼ぶ。ロックを取ったうえで元のスクリプトを子として流し直し、終了で解放する。ロックを持つ子孫は `K3D_LOCK_HELD` で素通りする(deploy-latest → m2-deploy の入れ子で自己待ちしない)。
- 方式は ios/scripts/xcode-test-lock.sh と同じ mkdir ロック(macOS に flock が無い)。置き場はリポジトリ外の `/tmp/pokecalc-k3d-<CLUSTER>.lock`(クラスタ単位。`K3D_SCRIPT_LOCK_DIR` で上書き)。待ちは既定 1800 秒、保持者の pid が死んでいれば回収、`K3D_SCRIPT_LOCK_DISABLE=1` で無効化。
- 流し直し方式にした理由: 各スクリプトが独自の EXIT trap を持つ・解除する(`trap - EXIT`)ため、同じスクリプト内で trap による解放をすると衝突する。
Reason: 複数セッションが同時にイメージ取り込み(k3d の image import)をすると k3d-tools コンテナ名が衝突して失敗した。
Impact: 上記スクリプトの冒頭 3 行と scripts/lib/k3d-lock.sh、scripts/k3d-lock_test.sh(make test-scripts)。既存スクリプトのテストは `K3D_SCRIPT_LOCK_DISABLE=1` で実行。API・engine は不変。
