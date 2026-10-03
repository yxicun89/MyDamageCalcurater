## 2026-10-03: ポケモン画像(P8-1)は MinIO を入れず、ローカル変換+gateway `/images/*` で配信する(タイプバランスレーン → 全レーン。ADR-0807)
Decision: 画像は手元の画像フォルダ(`data/generated/images/src`。Git に載らない)を `make assets`(Node・sharp)で WebP 128/512px+`manifest.json` に変換し、
gateway が `GATEWAY_IMAGES_DIR`(任意)から `/images/*` を配信する。画像が無ければ全員エンブレム。MinIO・`/assets` 転送の変更はしない。
Reason: 個人利用・学習目的でローカル完結・費用ゼロ。MinIO の常駐 Pod・認証・アップロード手順は本題ではない。
Impact:
- **API レーン**: gateway の `/images/*` と `images` の予約セグメント(`images_test.go` の AC-I1〜I6 を通す)。`GATEWAY_IMAGES_DIR` を `main.go` に足し、`main_test.go` の `TestEnvNames` の一覧に追記する(openapi は変えない)。
- **データ/運用レーン**: `tools/assets`(convert.mjs・README・package.json)と `make assets`、`scripts/make-targets_test.sh` の「make assets は非0」を「画像なしで 0」に置き換える。k3d への配線は後続。
- **Web レーン・iOS レーン(依頼)**: manifest.json(`/images/manifest.json`。404・不正はエンブレム)にキーがあれば遅延読み込みで画像、無ければ既存のタイプ色エンブレム。manifest の形は ADR-0807 §3。画像が無い状態でテストが通ること。
