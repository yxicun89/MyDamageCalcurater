# wishlist の Web(PWA)。ビルドコンテキストは apps/wishlist/web:
#   docker build -t wishlist/web:local apps/wishlist/web
# 型検査とテストはホストの make wishlist-build / wishlist-test が受け持つ(テストが ../testdata を読むため、
# このコンテキストには含まれない)。ここでは vite build だけを行う。

# Node の版は package.json の engines と揃える(ルートの web/Dockerfile と同じダイジェスト)。
FROM node:26.10.0-alpine@sha256:0b36e8c136b94cd4fcf02188228e76c31ad5872eef3fec8cbd2eee500cfd9e80 AS build

WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
# apple-touch-icon.png は Git に置かず、scripts/gen-icons.mjs で生成する。
RUN node scripts/gen-icons.mjs && npx vite build

# 非 root の nginx で静的配信する(ルートの web/Dockerfile と同じ基底イメージ)。
FROM nginxinc/nginx-unprivileged:1.31.6-alpine@sha256:e75f89810bf5bfbcf58a1cfb32a1a11de55b7623d732e67735d513b720d7436a

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html

USER 101
EXPOSE 8080
