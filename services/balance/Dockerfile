FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# API 契約・SQL の生成物は Git に置かず、ホストで make gen したものをコンテキストから受け取る(ADR-0171)。
RUN test -f internal/api/openapi.gen.go || { echo "生成物が無い: リポジトリのルートで make gen を実行してから docker build する(ADR-0171)" >&2; exit 1; }
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/balance-api ./cmd/api

FROM scratch
COPY --from=build /out/balance-api /balance-api
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/balance-api"]
