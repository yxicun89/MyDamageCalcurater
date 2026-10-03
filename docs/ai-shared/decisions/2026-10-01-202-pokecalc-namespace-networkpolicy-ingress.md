## 2026-10-01: pokecalc namespace に NetworkPolicy(ingress の既定拒否)を入れた(データレーン → 全レーンへ。issue #240・ADR-0132)

- k3d-pokecalc には適用済み(全 smoke・import・Prometheus の scrape が通ることを確認)。許可していない Pod からの ingress は届かない
- 新しい通信(新サービス・新しい呼び出し先)を足すレーンは、`deploy/k8s/base/networkpolicy/` に許可を足し、
  `services/gateway/deploytest/networkpolicy_test.go` の許可表にも足す。許可は宛先 Pod の 8080(Service の 80 ではない)
- API レーンへ: TiDB(record/team)を k3d に上げる前に ADR-0132「確認の結果」の手順を行う(tidb-operator からの許可を足す)。
  M2 で record・team の本体を足すときも許可が要る。PR #416(#284)で gateway → balance/speed/judge は既に許可済み
- 詰まったときの戻し方: `kubectl -n pokecalc delete networkpolicy default-deny-ingress`(許可だけが残る。データは消えない)
