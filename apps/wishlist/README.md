# 欲しいものリスト(wishlist)

自分用の欲しいものリスト。欲しいものを画像だけで並べ、タップすると「だいたいいくらで買えそうか」と、
各サイトを検索した状態で開くボタンを出す。価格は厳密な最安値ではなく目安。

ポケモンのダメージ計算とは別のアプリで、機能もデータも共有しない。同じ k3d クラスタ(namespace `wishlist`)と
MySQL のインスタンス(DB `wishlist`)だけを共用する。

## 何が動くか

- API(Go・Echo): 商品・ジャンル・サイトの登録、URL からの下書き(画像と名前)、目安価格の算出、画像の配信
- 夜間の価格取得(CronJob。毎日 03:00): サイトごとに間隔を空けて上位 20 件を取り、あやしい出品(名前が違う・安すぎる)を分ける
- Web(PWA。Vite + React): ホーム・詳細・登録・設定。オフラインでも一覧と検索リンクが使える
- iOS(SwiftUI): 同じ画面と、Safari の共有シートからの登録
- 入口: `http://localhost:8080/wishlist/`(iPhone からは Tailscale の URL + `/wishlist/`)

## 起動する

k3d クラスタ(ダメ計の `make up`)が動いている前提。

```sh
cd "$(git rev-parse --show-toplevel)"
apps/wishlist/scripts/bootstrap.sh
make wishlist-k3d-deploy
```

`http://localhost:8080/wishlist/` を開く。続きと確認点は [docs/verify.md](docs/verify.md)。

## 文書

| 文書 | 内容 |
|---|---|
| [CLAUDE.md](CLAUDE.md) | 仕様の正(画面・検索ワード・目安価格・データモデル・API・フェーズ) |
| [docs/verify.md](docs/verify.md) | 動作確認の手順 |
| [docs/design.md](docs/design.md) | 設計判断(W-01〜) |
| [docs/sites.md](docs/sites.md)・[docs/sites-headless.md](docs/sites-headless.md) | 各サイトの検索 URL・取得方法を実サイトで確かめた記録 |
| [docs/shortcut.md](docs/shortcut.md) | iOS ショートカットからの登録 |
| `docs/phase*-spec.md` | フェーズごとの受け入れ条件 |

## 構成

| パス | 役割 |
|---|---|
| `api/` | Go の API と CronJob(`cmd/api`・`cmd/refresher`)、`openapi.yaml`(API の正)、migrations |
| `web/` | PWA |
| `ios/` | SwiftUI アプリ(`Wishlist.xcodeproj`。ダメ計とは別プロジェクト・別 Bundle ID) |
| `deploy/k8s/` | namespace `wishlist` の Kustomize |
| `scripts/` | DB と Secret の初期化(`bootstrap.sh`) |
| `Makefile` | `make wishlist-*`(ルートの Makefile から include) |
