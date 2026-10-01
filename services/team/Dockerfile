# ビルドコンテキストはリポジトリ直下(services が engine を ../engine の replace で参照するため)。
#   docker build -f services/team/Dockerfile --target migrate -t <image> .
#   docker build -f services/team/Dockerfile --target server -t <image> .
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /src
COPY engine ./engine
COPY services ./services
WORKDIR /src/services
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/team-migrate ./team/cmd/migrate
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/team ./team/cmd/team

# migrate: team-migrate Job(deploy/k8s/base/team)が使うイメージ。up だけを実行する
# (down はここに含めない。CLAUDE.md: DB のデータ削除は人間の確認が必要)。
FROM scratch AS migrate
COPY --from=build /out/team-migrate /team-migrate
USER 65532:65532
ENTRYPOINT ["/team-migrate"]
CMD ["up"]

# server: team-svc 本体(`team serve`: HTTP + NATS JetStream の購読)と失効ジョブ(`team expire`)の
# 共通イメージ。Deployment・CronJob・Service は deploy/k8s/base/team(ADR-0220)。
FROM scratch AS server
COPY --from=build /out/team /team
USER 65532:65532
ENTRYPOINT ["/team"]
