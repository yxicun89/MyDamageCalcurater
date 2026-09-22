# ビルドコンテキストはリポジトリ直下(services が engine を ../engine の replace で参照するため)。
#   docker build -f services/pokedex/Dockerfile --target migrate -t <image> .
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /src
COPY engine ./engine
COPY services ./services
WORKDIR /src/services
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/pokedex-migrate ./pokedex/cmd/migrate

# migrate: pokedex-migrate Job(deploy/k8s/base/pokedex)が使うイメージ。up だけを実行する
# (down はここに含めない。CLAUDE.md: DB のデータ削除は人間の確認が必要)。
FROM scratch AS migrate
COPY --from=build /out/pokedex-migrate /pokedex-migrate
USER 65532:65532
ENTRYPOINT ["/pokedex-migrate"]
CMD ["up"]
