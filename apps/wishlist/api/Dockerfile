# ビルドコンテキストは apps/wishlist/api(他のモジュールに依存しない単独モジュール。docs/design.md W-01):
#   docker build -t wishlist/api:local apps/wishlist/api
# 引数は k8s の args(serve / migrate up)。
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN GOWORK=off go mod download

COPY . .
RUN CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags="-s -w" -o /out/wishlist-api ./cmd/api

# 外部の HTTPS(商品ページの OGP・画像)を取りに行くため CA 証明書が要る。
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/wishlist-api /wishlist-api
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/wishlist-api"]
