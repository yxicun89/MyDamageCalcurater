FROM golang:1.27-alpine@sha256:4cb7ac979db5fcc41cae44b2227ba5ab8a51e8807f40d9ba4dee20a0ad960b5b AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/balance-api ./cmd/api

FROM scratch
COPY --from=build /out/balance-api /balance-api
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/balance-api"]
