## 2026-09-22: 提案(データレーン・整備レーンへ): `make up` 後の calc・gateway のイメージ(API レーン。既定案)
Decision: P3-3 で共有の local overlay に `components: [api]` を足したため、`make up`(scripts/up.sh)も calc・gateway の Deployment を作るようになる。
up.sh は `pokecalc/calc:local` / `pokecalc/gateway:local` をビルド・import しないので、`make api-k3d-deploy` を流すまで ImagePullBackOff のまま残る。
既定案: 手順として `make up && make api-k3d-deploy` を README(services/gateway/README.md)に明記する(API レーンで実施済み)。
up.sh の最後で `make api-docker-build` と `k3d image import` を呼ぶ形にするかは、up.sh の持ち主(データレーン・整備レーン)の判断に任せる。API レーンは scripts/up.sh を変えない。
Reason: critic の推奨。共有スクリプトは他レーンの範囲のため。
Impact: `api-k3d-deploy` は他レーンのリソースに触れないよう、常に API 専用の overlay(deploy/k8s/overlays/local-api)だけを適用する(ADR-0203)。
