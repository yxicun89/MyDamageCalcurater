# ビルドコンテキストはリポジトリ直下(services が engine を ../engine の replace で参照するため。
# services/pokedex/Dockerfile と同じ)。
#   docker build -f services/gateway/Dockerfile -t pokecalc/gateway:local .
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /src
COPY engine ./engine
COPY services ./services
WORKDIR /src/services
RUN go mod download
# API 契約・SQL の生成物は Git に置かず、ホストで make gen したものをコンテキストから受け取る(ADR-0806)。
RUN test -f internal/api/openapi.gen.go || { echo "生成物が無い: リポジトリのルートで make gen を実行してから docker build する(ADR-0806)" >&2; exit 1; }
# ビルドの版(git の短縮 SHA など)。version.Version に埋め込み、起動ログと /healthz に出す(issue #217)。
ARG VERSION=0.0.0-dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X example.com/pokecalc/services/internal/version.Version=${VERSION}" -o /out/gateway ./gateway/cmd/gateway

# 最終段は scratch(シェル・パッケージマネージャを持たない)。非 root の数値 UID で動く
# (runAsNonRoot は数値 UID でないと検証できない。deploy/k8s/base/gateway/deployment.yaml)。
FROM scratch
COPY --from=build /out/gateway /gateway
USER 65532:65532
ENTRYPOINT ["/gateway"]
