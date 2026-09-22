# ビルドコンテキストはリポジトリ直下(services が engine を ../engine の replace で参照するため)。
#   docker build -f services/pokedex/Dockerfile --target migrate -t <image> .
#   docker build -f services/pokedex/Dockerfile --target importer -t <image> .
#   docker build -f services/pokedex/Dockerfile --target server -t <image> .
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /src
COPY engine ./engine
COPY services ./services
WORKDIR /src/services
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/pokedex-migrate ./pokedex/cmd/migrate
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/pokedex-import ./pokedex/cmd/import
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/pokedex ./pokedex/cmd/pokedex

# migrate: pokedex-migrate Job(deploy/k8s/base/pokedex)が使うイメージ。up だけを実行する
# (down はここに含めない。CLAUDE.md: DB のデータ削除は人間の確認が必要)。
FROM scratch AS migrate
COPY --from=build /out/pokedex-migrate /pokedex-migrate
USER 65532:65532
ENTRYPOINT ["/pokedex-migrate"]
CMD ["up"]

# server: pokedex(検索 API・内部 API)の Deployment(deploy/k8s/base/pokedex)が使うイメージ。
# 既定は serve(HTTP)。read model の export はローカルの make pokedex-export / タイプバランスレーンの
# 手順から別に流す(ADR-0105 §5・§6)。
FROM scratch AS server
COPY --from=build /out/pokedex /pokedex
USER 65532:65532
ENTRYPOINT ["/pokedex"]
CMD ["serve"]

# importer-deps: tools/importer の依存(package-lock.json の固定版)だけを npm ci する段。
# package.json・package-lock.json だけを COPY してキャッシュを効かせる(ADR-0104 §6)。
# Node の版は web/package.json の engines.node(26.9.0)と同じ系列(導入時点の最新の安定版。
# 確認した digest は本ファイルの ADR-0104 の追記を参照)。
FROM node:26.9.0-alpine@sha256:dbaa92e5758cbbcf85d65d5403fdb530fe3442cbe8c6dbfb7ef23365450d5070 AS importer-deps
WORKDIR /app/tools/importer
COPY tools/importer/package.json tools/importer/package-lock.json ./
RUN npm ci --omit=dev

# importer: pokedex-import(CronJob)が使うイメージ。取得(Node)・上流の検出(Node)・
# 照合と投入(Go)を1つのイメージにする(ADR-0104 §2・§6)。data/importer(config・effects・
# regulations)は Git の固定版なのでイメージに焼く。data/generated・data/local(実データ)は
# 焼かない(PVC・ConfigMap で渡す)。
FROM node:26.9.0-alpine@sha256:dbaa92e5758cbbcf85d65d5403fdb530fe3442cbe8c6dbfb7ef23365450d5070 AS importer
WORKDIR /app
COPY --from=importer-deps /app/tools/importer/node_modules ./tools/importer/node_modules
COPY tools/importer/*.mjs ./tools/importer/
COPY tools/importer/package.json ./tools/importer/package.json
COPY tools/importer/cronjob.sh ./tools/importer/cronjob.sh
COPY data/importer/config.json data/importer/effects.json data/importer/regulations.json ./data/importer/
COPY --from=build /out/pokedex-import /app/pokedex-import
RUN chmod +x /app/tools/importer/cronjob.sh
USER node
ENTRYPOINT ["/app/tools/importer/cronjob.sh"]
