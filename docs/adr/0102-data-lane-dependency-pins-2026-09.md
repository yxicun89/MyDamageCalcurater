# ADR-0102: データレーンの依存の最新安定版固定(2026-09)。MySQL は LTS 系列(9.7)を既定にする

- 状態: 採用
- 日付: 2026-09-22
- 関連: docs/ai-shared/DECISIONS.md(2026-09-21「ミドルウェア・ライブラリ・ツールは導入時点の最新の安定版にする」)、
  ADR-0100(pokedex のスキーマと migrate。MySQL 8.4 を前提としていた箇所を本 ADR が更新する)

## 背景

ユーザー決定により、言語・ミドルウェア・ライブラリは導入/更新時点の最新の安定版にし、`latest` 等の自動追従では
なく正確な番号(タグ+digest、go.mod の版)で固定する。Go のツールチェーンは全モジュールで揃え、データレーンが
先に上げる。データレーンの範囲(Go ツールチェーン、engine/tools/services の一部モジュール、pokedex の
コンテナイメージ、MySQL イメージ)を対象に確認した。

## 決定

### 1. Go ツールチェーン

`go.dev/dl` の配布情報(`https://go.dev/VERSION?m=text` と `https://go.dev/dl/?mode=json`)で確認した最新の
安定版は `go1.27.1`(2026-08-28 リリース、`stable: true`)。ローカル環境の `go version` も既に `go1.27.1` で
一致している。`go.work` と `engine/go.mod` / `services/go.mod` / `tools/go.mod` / `services/balance/go.mod`
(balance は go/toolchain 行のみ)の `go` 行を `go 1.27.1` に統一する(`toolchain` 行は追加不要。`go` 行の
パッチ版指定がそのまま最小要求バージョンになる)。

### 2. Go モジュール(データレーンが使うもの)

`go list -m -u all`(engine / services / tools / services/balance の全モジュール)で確認した結果、
`golang-migrate/migrate/v4`(v4.20.1)・`go-sql-driver/mysql`(v1.10.1)・`gopkg.in/yaml.v3`(v3.0.1)・
`sqlc-dev/sqlc`(v1.31.1、tools の tool ディレクティブ)は、いずれも `go mod tidy` 後も版が変わらず、
proxy.golang.org の `@latest` と一致した。**既に最新の安定版のため変更なし**。

`services/go.mod` の `oapi-codegen/oapi-codegen/v2`(tool ディレクティブ)は、生成先が `services/internal/api`
(API レーンの生成物)であるため、版を上げない。データレーンが版を上げると `make gen-go` の出力が API レーンの
管理下のファイルを意図せず変えるおそれがあるため、対象から明示的に除外する(API レーンの担当とする)。

### 3. pokedex のビルドイメージ

`services/pokedex/Dockerfile` の `golang:1.27-alpine` を `golang:1.27.1-alpine`(digest
`sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414`)に固定する。これは
`services/balance/Dockerfile` が既に使っている digest と同じで、ビルド環境が両レーンで揃う。

### 4. MySQL は最新の LTS 系列(9.7)を既定にする

Docker 公式イメージ(`docker-library/mysql`)のタグ一覧を確認したところ、MySQL の隔月/半年ごとの
Innovation 系列は年ベースの番号(`26.7.0`、`innovation`/`latest` タグ)に移行しており、**現在の LTS 系列は
`9.x`(最新 `9.7.2`、`lts` タグ)**である。旧 LTS だった `8.4` は `lts` タグが外れており(`8.4.11`、`8`、`8.4`
のみ)、後継の LTS ではなくなっている。

Innovation ではなく LTS を既定にする理由: 個人開発でアップデートの手間を減らすことが目的であり、Innovation は
数か月単位で次の版に置き換わるため追従コストが高い。LTS は長期間同じメジャー系列で安定した保守を受けられる。

対象: `deploy/k8s/overlays/local/mysql/statefulset.yaml`、`deploy/k8s/base/pokedex/job-migrate.yaml`
(`wait-for-mysql` initContainer)、`scripts/db-local-up.sh`。いずれも
`mysql:9.7.2@sha256:29abb0a179982e4a8928138bfc7f918af9eda64e7eeb1b1d084c1720a20159e6`(マルチアーキ index
digest。amd64/arm64 を含む)に固定した。`mysql` ユーザーの uid/gid(999)、`utf8mb4_0900_ai_ci` 照合順序は
9.7.2 でも変わらないことを確認済み。

ADR-0100 の「MySQL は 8.4(LTS)を前提とする」等の記述は、本 ADR が版の部分を更新する(ADR-0100 本文は
決定当時の記録として書き換えない)。CHECK 制約・`REGEXP_LIKE`・JSON・生成列など ADR-0100 が前提とする機能は
9.7 でも利用可能(後方互換)。

#### 既存データのアップグレード(戻せない)

- 既存の k3d の PVC や `make db-local-up` の docker ボリュームは 8.4 のデータのまま 9.7 で開かれ、インプレースで上がる。
  LTS から次の LTS への直接のアップグレードは MySQL が公式にサポートする経路。
- **一度上げたデータは 8.4 に戻せない**。作り直す(PVC・ボリュームを消して migrate から流す)のは DB データの削除に当たるので、
  人間の確認が要る(CLAUDE.md「人間の確認が必要なこと」)。
- `scripts/db-local-up.sh` は同名のコンテナがあれば `docker start` で既存のまま起動するため、既存の環境は 8.4 のまま残りうる
  (スクリプトは停止・削除をしない方針)。新しいイメージで動いているかは `SELECT VERSION()` で確かめる。
- 系列の出典: Docker 公式イメージ(https://hub.docker.com/_/mysql 、`lts` / `innovation` タグ)と MySQL のリリースモデル
  (https://dev.mysql.com/doc/refman/9.7/en/mysql-releases.html)。確認日 2026-09-22。

### 5. Node / @smogon/calc

`tools/golden/package.json` は `@smogon/calc` のみに依存する。`npm view @smogon/calc versions` で確認したところ、
公開されている最新は現状の固定版と同じ `0.12.0` で、より新しい版は無い。上げる対象が無いため今回は変更なし
(0.12.0 より新しい版が出た場合のみ、ゴールデンの期待値との diff を確認する別タスクにする)。`tools/importer/`
はこの時点で `.gitkeep` のみで package.json が存在しないため対象外(別レーンの未マージ作業)。

### 6. `make deps-outdated`

Go は各モジュールで `go list -m -u all`、Node は `tools/golden` で `npm outdated` を実行し一覧を表示する
ターゲットをルート `Makefile` に追加した。ネットワークを使うため `make test` には含めない。失敗しても
一覧を出す形にする(`|| true` 相当)。

## 影響

- `go.work` / `engine/go.mod` / `services/go.mod` / `tools/go.mod` / `services/balance/go.mod` の `go` 行
- `services/pokedex/Dockerfile` の base image
- `deploy/k8s/overlays/local/mysql/statefulset.yaml`、`deploy/k8s/base/pokedex/job-migrate.yaml`、
  `scripts/db-local-up.sh`、`.env.example` の MySQL イメージ表記
- ルート `Makefile` に `deps-outdated` を追加
- `services/DEPENDENCIES.md` / `tools/DEPENDENCIES.md` に確認日と版を記録
- `services/balance/` の依存(go.mod の require、Dockerfile)は変更しない(タイプバランスレーンの範囲)
