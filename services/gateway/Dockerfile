# ビルドコンテキストはリポジトリ直下(services が engine を ../engine の replace で参照するため。
# services/pokedex/Dockerfile と同じ)。
#   docker build -f services/gateway/Dockerfile -t pokecalc/gateway:local .
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /src
COPY engine ./engine
COPY services ./services
WORKDIR /src/services
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gateway ./gateway/cmd/gateway

# 最終段は scratch(シェル・パッケージマネージャを持たない)。非 root の数値 UID で動く
# (runAsNonRoot は数値 UID でないと検証できない。deploy/k8s/base/gateway/deployment.yaml)。
FROM scratch
COPY --from=build /out/gateway /gateway
USER 65532:65532
ENTRYPOINT ["/gateway"]
