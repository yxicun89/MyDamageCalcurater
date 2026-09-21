# ビルドコンテキストはリポジトリ直下(services が engine を ../engine の replace で参照するため)。
#   docker build -f services/pokedex/Dockerfile --target migrate -t <image> .
FROM golang:1.27-alpine@sha256:4cb7ac979db5fcc41cae44b2227ba5ab8a51e8807f40d9ba4dee20a0ad960b5b AS build

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
