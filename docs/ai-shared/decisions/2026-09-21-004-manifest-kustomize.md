## 2026-09-21: manifest は Kustomize に統一
Decision: plain YAML / Helm ではなく Kustomize(pokecalc と同じ)。
Reason: 個人開発の規模でHelmのテンプレート化は過剰。pokecalcと構成を揃える。
Impact: services/balance/deploy/k8s も base/overlays 構成にする。
