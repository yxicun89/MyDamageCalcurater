# ビルドコンテキストはリポジトリ直下(services が engine を ../engine の replace で参照するため)。
#   docker build -f services/record/Dockerfile --target migrate -t <image> .
#   docker build -f services/record/Dockerfile --target server -t <image> .
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /src
COPY engine ./engine
COPY services ./services
WORKDIR /src/services
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/record-migrate ./record/cmd/migrate
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/record ./record/cmd/record

# migrate: record-migrate Job(deploy/k8s/base/record)が使うイメージ。up だけを実行する
# (down はここに含めない。CLAUDE.md: DB のデータ削除は人間の確認が必要)。
FROM scratch AS migrate
COPY --from=build /out/record-migrate /record-migrate
USER 65532:65532
ENTRYPOINT ["/record-migrate"]
CMD ["up"]

# server: record-svc 本体(HTTP + NATS JetStream の購読)のイメージ。
# 対応する Deployment・Service・gateway への配線(GATEWAY_RECORD_URL)・scripts/up.sh のビルド対象への
# 追加は、まだ deploy/k8s/base/record に存在しない(critic レビュー R-4 で判明。P5-3b で行う。
# docs/plan.md・ADR-0209「影響」参照)。このイメージ自体はビルドできるが、k3d クラスタには
# まだ配線されていないため、現状は `/api/record/*` へのアクセスは gateway 側で
# upstream_unavailable(503)のままになる。
FROM scratch AS server
COPY --from=build /out/record /record
USER 65532:65532
ENTRYPOINT ["/record"]
