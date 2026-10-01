# ADR-0132: pokecalc namespace に ingress の default-deny と許可リストを置く(egress は絞らない)

- 状態: 採用(spec-writer の提案。実装はこの ADR の受け入れ条件=テストが正)
- 日付: 2026-10-01
- レーン: データ(ADR 帯 `0100〜`。運用の D25)
- 関連: issue #240、issue #403(パッケージ D25)、#148、ADR-0204(`/internal` は gateway が 404)、ADR-0211(TiDB)、ADR-0406(ServiceMonitor)

## 背景

`pokecalc` namespace に NetworkPolicy が無く、`/internal/pokedex/master`・各サービスの `/metrics`・MySQL 3306 が
クラスタ内のどの Pod からも届く。`/internal` は gateway が 404 にするだけで、pokedex 自身は誰にでも返す。

## 決定

1. base に `deploy/k8s/base/networkpolicy/`(kustomization.yaml と各 NetworkPolicy)を置き、`base/kustomization.yaml` が読み込む。
   balance・speed・judge は各レーンの Kustomize(`services/*/deploy`)で同じ `pokecalc` に入るため、base のポリシーが効く。
   各レーンの manifest は変えず、既存ラベル `app.kubernetes.io/name` で選ぶ。
2. **ingress のみ** default-deny(`podSelector: {}`・`policyTypes: [Ingress]`)を1本置く。egress は絞らない
   (kube-dns・外部への取得〈importer〉・k8s API の許可を一括で間違えると全体が止まるため、別タスクにする)。
3. 許可は「宛先 Pod ごと」または「宛先+送信元の組」ごとに1本ずつ。`ipBlock` は使わない。他 namespace の送信元は
   `namespaceSelector`(`kubernetes.io/metadata.name`。Kubernetes が自動で付けるラベルだけに頼る)と `podSelector` を**同じ peer に両方**書く
   (別々の peer にすると「その namespace の全 Pod」または「同 namespace の同名ラベル」が通ってしまう)。
4. ポートは **Pod 側の containerPort**(HTTP は 8080)。Service の port 80 ではない(NetworkPolicy は DNAT 後に評価される)。

### 許可する通信(これ以外は拒否)

| 宛先(:ポート) | 送信元 | 根拠 |
|---|---|---|
| gateway :8080 | kube-system の Traefik(`app.kubernetes.io/name=traefik`) | gateway の Ingress |
| balance・speed・judge :8080 | 同上 + gateway | 各 Ingress。gateway からは issue #284 の配線(`GATEWAY_*_URL`)が入ると必要 |
| calc :8080 | gateway、judge | `GATEWAY_CALC_URL`、`JUDGE_CALC_BASE_URL` |
| pokedex :8080 | gateway、calc、judge | `GATEWAY_POKEDEX_URL`、`CALC_MASTER_URL`(内部 API)、`JUDGE_POKEDEX_BASE_URL` |
| web :8080 | gateway | `GATEWAY_WEB_URL` |
| mysql :3306 | pokedex、pokedex-migrate、pokedex-import | DSN・migrate Job・importer CronJob |
| nats :4222 | calc | `CALC_NATS_URL` |
| TiDB(operator ラベル `instance=pokecalc-tidb`)tidb :4000 | record-migrate、team-migrate | `pokecalc-tidb-tidb:4000` |
| TiDB の pd・tikv・tidb 相互 | 同じ `instance=pokecalc-tidb` の Pod | TiDB クラスタ内通信(下記の注意) |
| gateway・calc・pokedex・balance・speed・judge :8080 | observability の Prometheus(`app.kubernetes.io/name=prometheus`) | ServiceMonitor(`/metrics`) |

拒否の例(テストが検査): web → pokedex・calc・mysql・nats、calc・gateway・judge → mysql、importer → pokedex、
default namespace の一時 Pod → pokedex・calc・mysql・gateway、kube-dns → pokedex、observability の Grafana → calc、
別 namespace に同じラベルを付けた偽 Prometheus / pokecalc 内の偽 Traefik、Service の port 80 での到達。

### 限界

- L3/L4 のポリシーなので**パスは区別できない**。pokedex :8080 を許された calc・judge・Prometheus は `/internal` も `/metrics` も叩ける。
  gateway が `/internal` を 404 にする(ADR-0204)層は残す。多層の1層目であって、パス単位の制御ではない。
- TiDB はオペレーターが作る Pod で、本 ADR の時点でクラスタに TiDB が起動していない。ラベル(`app.kubernetes.io/instance`・`component`・
  `managed-by=tidb-operator`)は標準値を想定したので、実装者は TiDB 起動後に `kubectl -n pokecalc get pod --show-labels` で確かめる。
  tidb-operator(namespace `tidb-admin`)が TiDB の status ポートへ入る経路が必要なら、実クラスタで確かめて許可を足す
  (足したらテストの許可表にも足す)。
- 外向き(egress)は未制限。k3d の CNI が NetworkPolicy を強制すること(k3s 既定の kube-router)が前提。

## 受け入れ条件

AC-N1〜N5 は `services/gateway/deploytest/networkpolicy_test.go`(`kubectl kustomize` の描画に対する静的検査)が正。
実クラスタでの確認は実装者が行う(下記)。

## 実クラスタでの確認手順(実装者向け)

```
cd ~/MyDamageCalcurater
kubectl -n pokecalc apply --dry-run=server -k deploy/k8s/base/networkpolicy   # 先に dry-run
kubectl -n pokecalc apply -k deploy/k8s/base/networkpolicy        # NetworkPolicy だけを適用(他は触らない)
kubectl -n pokecalc get networkpolicy
make api-smoke web-k3d-smoke balance-smoke speed-smoke judge-smoke    # 全部通る
make import-k8s                                                       # importer → mysql
# 拒否の確認(web の Pod は wget を持たないので busybox の一時 Pod を使う)
kubectl -n pokecalc run np-probe --rm -it --restart=Never --image=busybox:1.37 --labels=app.kubernetes.io/name=np-probe -- \
  sh -c 'wget -qO- -T 3 http://pokedex/internal/pokedex/master | head -c 100; echo "exit=$?"'   # タイムアウト(届かない)
kubectl -n pokecalc run np-probe --rm -it --restart=Never --image=busybox:1.37 -- nc -zv -w 3 mysql 3306   # タイムアウト
kubectl -n default run np-probe --rm -it --restart=Never --image=busybox:1.37 -- wget -qO- -T 3 http://calc.pokecalc/metrics   # タイムアウト
# 許可の確認: gateway の Pod から calc・pokedex が届く(smoke が通れば足りる)
# Prometheus: Grafana/Prometheus の Targets で pokecalc の6件が UP のまま
```

止まった場合は `kubectl -n pokecalc delete networkpolicy default-deny-ingress` で直ちに戻せる(許可だけが残るので安全)。
DB・namespace は消さない。
