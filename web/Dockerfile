# P4-11: Web(nginx の静的配信。engine.wasm を含む)のコンテナ。
# ビルドコンテキストはリポジトリ直下(engine の WASM ビルドと testdata/golden/typechart.json を使うため。
# services/calc/Dockerfile と同じ考え方)。
#   docker build -f web/Dockerfile -t pokecalc/web:local .

# --- 1) engine.wasm と wasm_exec.js(ADR-0011 §6) ---
FROM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS wasm

WORKDIR /src
COPY engine ./engine
RUN cd engine && GOOS=js GOARCH=wasm go build -trimpath -o /out/engine.wasm ./cmd/wasm
RUN cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" /out/wasm_exec.js

# --- 2) Web 本体のビルド(vite build) ---
# Node の版は web/package.json の engines と揃える。
FROM node:26.9.0-alpine@sha256:dbaa92e5758cbbcf85d65d5403fdb530fe3442cbe8c6dbfb7ef23365450d5070 AS build

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund

WORKDIR /src
# vite.config.ts が ../testdata/golden/typechart.json(web/ の隣)を正として読む(ADR-0300 §3)。
COPY testdata/golden/typechart.json ./testdata/golden/typechart.json
COPY web ./web
COPY --from=wasm /out/engine.wasm ./web/public/engine.wasm
COPY --from=wasm /out/wasm_exec.js ./web/public/wasm_exec.js

WORKDIR /src/web
# npm run build = 型検査 + vite build + JS の配信サイズ予算(ホストの make build と同じ関門をイメージにも掛ける)。
RUN npm run build

# --- 3) 配信(非 root の nginx) ---
FROM nginxinc/nginx-unprivileged:1.31.6-alpine@sha256:e75f89810bf5bfbcf58a1cfb32a1a11de55b7623d732e67735d513b720d7436a

COPY web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/web/dist /usr/share/nginx/html

USER 101
EXPOSE 8080
