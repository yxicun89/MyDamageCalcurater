## 2026-10-03: 【提案】共有 MySQL の NetworkPolicy に wishlist 名前空間からの接続を許可する(Wishlist → データレーン)
Decision(提案・既定案): `deploy/k8s/base/networkpolicy/allow-mysql-ingress.yaml` の `ingress[].from` に、次の 1 項目を足す。
```yaml
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: wishlist
          podSelector:
            matchExpressions:
              - key: app.kubernetes.io/name
                operator: In
                values: [wishlist-api, wishlist-migrate]
```
Reason: wishlist は共有 MySQL に DB `wishlist` を作って使う(仕様 `apps/wishlist/CLAUDE.md` §2)。現在の NetworkPolicy は pokecalc 名前空間の決まった Pod しか 3306 を許していない。
wishlist レーンは共有基盤のマニフェストを変更しない(仕様)ため、持ち主のデータレーンに依頼する。
Impact: 入るまでは、クラスタ上の wishlist-api / migrate Job は DB に接続できない(ローカルの `go test`・docker の MySQL での確認は影響なし)。
DB とユーザーの作成は `apps/wishlist/scripts/bootstrap.sh`(人が 1 回実行。root 権限は Secret pokecalc/mysql-auth から読む)。
