# ビルドコンテキストはリポジトリ直下(services が engine を ../engine の replace で参照するため)。
#   docker build -f services/record/Dockerfile --target migrate -t <image> .
#   docker build -f services/record/Dockerfile --target server -t <image> .
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /src
COPY engine ./engine
COPY services ./services
WORKDIR /src/services
RUN go mod download
# API 契約・SQL の生成物は Git に置かず、ホストで make gen したものをコンテキストから受け取る(ADR-0171)。
RUN test -f internal/api/openapi.gen.go || { echo "生成物が無い: リポジトリのルートで make gen を実行してから docker build する(ADR-0171)" >&2; exit 1; }
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/record-migrate ./record/cmd/migrate
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/record ./record/cmd/record

# migrate: record-migrate Job(deploy/k8s/base/record)が使うイメージ。up だけを実行する
# (down はここに含めない。CLAUDE.md: DB のデータ削除は人間の確認が必要)。
FROM scratch AS migrate
COPY --from=build /out/record-migrate /record-migrate
USER 65532:65532
ENTRYPOINT ["/record-migrate"]
CMD ["up"]

# server: record-svc 本体(`record serve`: HTTP + NATS JetStream の購読)と失効ジョブ(`record expire`)の
# 共通イメージ。Deployment・CronJob・Service は deploy/k8s/base/record(ADR-0220)。
FROM scratch AS server
COPY --from=build /out/record /record
USER 65532:65532
ENTRYPOINT ["/record"]
