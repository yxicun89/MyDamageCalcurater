## 2026-10-03: M2(TiDB・NATS・record・team)が k3d で動く。クラスタ全体の操作は API レーンだけが行う(API レーン → 全レーンへ)

- 状況: M2 のバックエンドを共有 k3d に実適用した(ADR-0226)。TidbCluster Ready・TidbInitializer Completed・NATS・record・team が Running。
  `make web-k3d-e2e`(`/api/record/frequent-opponents` の 503 で失敗していた分)は成功。`make api-smoke`・`make web-k3d-smoke` も成功。
- 決めたこと:
  1. **k3d へのクラスタ全体の操作(`make deploy-latest`・image import・TiDB 等の apply・NetworkPolicy の apply)は API レーンだけが行う。** 他レーンは自分のサービスの
     `*-k3d-deploy`(overlays/local-api・local-web 等)だけを使い、共有の `deploy/k8s/overlays/local` や `deploy/k8s/base/networkpolicy` を丸ごと apply しない
     (実測中に NetworkPolicy が古い内容に戻っていた。operator→PD の許可が消えると TiDB が壊れる)。
  2. **`make deploy-latest` は NATS・TiDB・record・team まで入れるようになった**(`scripts/k3d-m2-deploy.sh`。`make up` も同じ)。M2 の準備に失敗しても他のサービスの
     入れ替えは続き、理由を表示して最後に非ゼロで終わる。docs/verify-m2.md の「deploy-latest は record・team を入れない」は解消した(記述を直した)。
  3. Web レーンへ: `docs/verify-m2.md` §2 を実機で確認できる。`make web-k3d-e2e` は緑。
  4. 注意: TiKV は常駐メモリが約 2.2GiB(limit 3Gi)で、Docker Desktop の VM(7.75GiB)の余裕は小さい。k3d が重いときはまず `kubectl top nodes` を見る。
  5. 失効 CronJob(record-expire・team-expire)は承認まで `suspend: true` のまま(変更なし)。
- 影響: データレーン・Web・iOS は、M2 の実機確認を k3d で行える。P5-3c(お気に入り API)は別タスク。
