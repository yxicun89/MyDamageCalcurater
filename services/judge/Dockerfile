# ビルドコンテキストはリポジトリのルート(engine を含むため。ADR-0700 §1。speed の Dockerfile と同じ形):
#   docker build -f services/judge/Dockerfile .
FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /src
# 依存の取得に要るのは go.mod だけ。replace ../../engine が解決できるよう engine の go.mod を
# 同じ配置でコピーし、engine のコードの変更で依存のキャッシュが無効にならないようにする。
COPY engine/go.mod ./engine/
COPY services/judge/go.mod services/judge/go.sum ./services/judge/
WORKDIR /src/services/judge
RUN GOWORK=off go mod download

WORKDIR /src
COPY engine ./engine
COPY services/judge ./services/judge
WORKDIR /src/services/judge
RUN CGO_ENABLED=0 GOWORK=off go build -trimpath -ldflags="-s -w" -o /out/judge-api ./cmd/api

FROM scratch
COPY --from=build /out/judge-api /judge-api
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/judge-api"]
