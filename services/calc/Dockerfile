# ビルドコンテキストはリポジトリ直下(services が engine を ../engine の replace で参照するため。
# services/pokedex/Dockerfile と同じ)。
#   docker build -f services/calc/Dockerfile -t pokecalc/calc:local .
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /src
COPY engine ./engine
COPY services ./services
WORKDIR /src/services
RUN go mod download
# ビルドの版(git の短縮 SHA など)。version.Version に埋め込み、起動ログと /healthz に出す(issue #217)。
ARG VERSION=0.0.0-dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X example.com/pokecalc/services/internal/version.Version=${VERSION}" -o /out/calc ./calc/cmd/calc

# 最終段は scratch(シェル・パッケージマネージャを持たない)。非 root の数値 UID で動く
# (runAsNonRoot は数値 UID でないと検証できない。deploy/k8s/base/calc/deployment.yaml)。
FROM scratch
COPY --from=build /out/calc /calc
USER 65532:65532
ENTRYPOINT ["/calc"]
