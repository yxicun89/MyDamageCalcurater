# wishlist(欲しいものリスト)

自分専用の欲しいものリスト。欲しいものを**画像だけ**で並べて、タップすると
「だいたいいくらで買えそうか」と、各サイトを**検索した状態で開くボタン**を出す。

### ルートの CLAUDE.md との関係
- リポジトリ直下の CLAUDE.md は、主にダメージ計算アプリ向けのもの。そのうち**ダメ計アプリ固有の仕様・画面・データ・用語はこのアプリには適用しない**
- **共通の運用ルールには従う**:`docs/ai-shared/COORDINATION.md`、DECISIONS.md による提案、PR 経由でのマージ、commit・push の運用、spec-writer → implementer → critic の流れ
- 両者が矛盾する場合、このアプリの仕様についてはこのファイルを優先する
- ブランチは `lane/wishlist`。初回は `git push -u origin lane/wishlist` で push する

このディレクトリ(`apps/wishlist/`)の外のコードは変更しないこと。
共通基盤(Ingress / MySQL / 監視)のマニフェストを変える必要が出た場合は、変更せずに提案だけすること。

---

## 1. 前提とスコープ

- 利用者は自分1人。クライアントは2つ:**iOSネイティブアプリ(SwiftUI、メイン)** と **PWA(ブラウザ版)**。どちらも同じAPIを使う
- PC対応は不要(PWAが崩れなければ良い)
- 対象ジャンル:デュエル・マスターズ、S.H.Figuarts、ガンプラ、ポケモングッズ(ジャンルは後から追加できる)
- 価格は**厳密な最安値ではなく目安**。最終判断は人間がサイトを見て行う
- 認証は簡易でよい(Tailscale内のみ公開+固定トークン)

### やらないこと
- 厳密な最安値の保証、購入代行、価格の履歴グラフ(後回し)
- 複数ユーザー対応
- Android版

---

## 2. 構成

```
apps/wishlist/
├── CLAUDE.md
├── api/            # Go + Echo
│   ├── cmd/api/        # HTTPサーバ
│   ├── cmd/refresher/  # CronJob用 目安価格の一括更新
│   ├── internal/
│   │   ├── item/       # 商品CRUD
│   │   ├── query/      # 検索ワード生成・正規化
│   │   ├── deeplink/   # 検索URL生成
│   │   ├── fetcher/    # サイト別の価格取得(Fetcherインターフェース)
│   │   ├── estimate/   # 目安価格の算出・参考外判定
│   │   ├── ogp/        # URLからOGP画像・タイトル取得
│   │   └── storage/    # 画像保存(インターフェース。初期実装はPVC上のファイル)
│   └── migrations/
├── api/openapi.yaml # APIの正。web と ios のクライアントはここから生成する
├── web/            # React + Vite + TypeScript(PWA)
├── ios/            # SwiftUI。Wishlist.xcodeproj(ダメ計アプリとは別プロジェクト・別Bundle ID)
│   ├── Wishlist/           # アプリ本体
│   └── WishlistShare/      # Share Extension(共有シートから登録)
└── deploy/k8s/     # namespace: wishlist(Deployment, Service, CronJob, PVC, Ingressルール)
```

- 既存のリポジトリ構成(ダメ計アプリ、タイプバランスチェッカー)と命名規則が違う場合は、既存側に合わせる
- k8sは既存のMac上クラスタに namespace `wishlist` で同居させる
- MySQLは既存インスタンスを共有し、データベース `wishlist` を新しく作る
- 外出先からは Tailscale 経由でアクセスする

---

## 3. 画面

### ホーム
- 画像のみのグリッド(2〜3列)。文字は出さない
- 上部にジャンルのチップ(すべて / 各ジャンル)。スクロールで隠れる
- 長押しで編集・削除
- 右下に「+」(写真アップロード or URL貼り付けで登録)

### 詳細シート(画像タップで下から出る)
1. 画像(大)
2. 検索ワード(タップでその場編集。編集はこのシート内だけの一時的なもの、保存は「保存」ボタンを押したときのみ)
3. サマリ:`だいたい ¥3,000〜¥4,500 で買えそう(10/3 時点)`
4. サイト行(ジャンルに紐づくサイトを表示順に並べる)
   - サイト名 / 目安価格 / 件数 / 状態(在庫あり等)
   - **行タップで、そのサイトを検索ワードで検索した状態の画面を新しいタブで開く**(`target="_blank"`, `rel="noopener"`)
5. 「参考外 N件」(折りたたみ):参考外と判定した出品の画像・タイトル・価格・理由・リンク

### 設定
- ジャンル:名前、検索ワードのテンプレート、表示するサイトと順序
- サイト:名前、検索URLのテンプレート、取得方式

---

## 4. 検索ワード

ジャンルごとにテンプレートを持ち、商品の `name` と `option` を埋め込む。

| ジャンル | テンプレート例 | name | option | 生成結果 |
|---|---|---|---|---|
| S.H.Figuarts | `S.H.Figuarts {name}` | グリス | | S.H.Figuarts グリス |
| デュエマ | `{name} {option}` | ボルシャック | 銀トレジャー | ボルシャック 銀トレジャー |
| ガンプラ | `{name}` | HG ガンダムエアリアル | | HG ガンダムエアリアル |

- 空の `{option}` を埋めたあとの余分な空白は詰める
- `items.query_override` があればテンプレートより優先する
- `item_site_overrides.query` があれば、そのサイトだけさらに優先する

### 正規化(タイトル照合用)
- NFKC正規化 → 小文字化 → 空白・記号(`.` `-` `・` など)を除去して比較する
- 例:`S.H.Figuarts` と `SHフィギュアーツ` の表記揺れはジャンルごとの別名辞書で吸収する(フェーズ2)

---

## 5. ディープリンク

`sites.search_url_template` の `{q}` を `encodeURIComponent(query)` で置き換える。空白は `%20` になる。

初期データ(確認済み):
- メルカリ:`https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc`
- Amazon:`https://www.amazon.co.jp/s?k={q}&s=price-asc-rank`

**以下は実際のサイトで検索して確認してから登録すること。推測でURLを作らない:**
- Yahoo!フリマ、カードラッシュ、ドラゴンスター、あみあみ、駿河屋、Yahoo!ショッピング、プレバン、魂ウェブ、ポケモンセンターオンライン

ディープリンクはフロントエンド側だけで組み立てる(バックエンドが落ちていても動くようにする)。

---

## 6. 目安価格

### 取得方式(`sites.fetch_type`)
| 値 | 対象 | 実装 |
|---|---|---|
| `api` | Yahoo!ショッピング | 公式API(appidは Secret) |
| `scrape` | カードラッシュ、ドラゴンスター、あみあみ、駿河屋 | HTTP取得 + goquery |
| `headless` | メルカリ、Yahoo!フリマ | chromedp |
| `link_only` | Amazon、プレバン、魂ウェブ、ポケセン | 価格は取得せず、リンクのみ |

```go
type Fetcher interface {
    Fetch(ctx context.Context, site Site, query string) ([]Listing, error)
}
type Listing struct {
    Title    string
    Price    int    // 円。送料は含めない
    URL      string
    ImageURL string
    InStock  bool
}
```

### アクセスの節度
- 1サイトあたり上位20件まで取得する
- 同一サイトへのリクエストは最低5秒あける。並列で叩かない
- `headless` のサイトは**詳細シートを開いたとき**と、深夜のCronJobでのみ取得する。キャッシュの有効期限は24時間
- 取得に失敗してもエラー画面にはしない。前回値を「最終取得: ○日前」と表示し、リンクは常に出す

### 参考外の判定(キーワードを仕込んだ別商品対策)
各出品について、以下のどれかに当てはまれば `suspicious` とし、理由を記録する:
1. `title_mismatch`:正規化したタイトルに、`name` の全トークンが含まれていない
2. `too_cheap`:基準価格 × `cheap_ratio`(初期値 0.3)未満
3. `below_min`:`items.min_price` 未満(設定時のみ)

**基準価格** = 同じ商品の `scrape` / `api` サイトの、参考外を除いた価格の中央値。
基準価格が取れない場合(ショップ側にない場合など)は、2の判定をスキップする。

### 目安の算出
- 参考外を除いた出品の価格について、**下位25パーセンタイル〜中央値**をそのサイトの目安とする
- 件数が3件未満なら、最小値だけを表示する
- サマリは全サイトの目安の中から、最も低い下限と、その中央値付近をとって表示する
- 判定結果は捨てない。参考外の出品も `listings` に保存し、UIで確認できるようにする

### 例
カードラッシュ ¥5,000 / ドラゴンスター ¥4,800 / メルカリ ¥300(タイトルに商品名なし)
→ 基準価格 ¥4,900。メルカリの ¥300 は `title_mismatch` と `too_cheap` で参考外になり、サマリには含めない。
「参考外 1件」に画像付きで表示される。

---

## 7. データモデル(MySQL)

```sql
CREATE TABLE genres (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  query_template VARCHAR(255) NOT NULL DEFAULT '{name} {option}',
  sort_order INT NOT NULL DEFAULT 0
);

CREATE TABLE sites (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  search_url_template VARCHAR(1024) NOT NULL,
  fetch_type ENUM('api','scrape','headless','link_only') NOT NULL DEFAULT 'link_only',
  is_reference BOOLEAN NOT NULL DEFAULT FALSE  -- 基準価格の算出に使うか(ショップ系)
);

CREATE TABLE genre_sites (
  genre_id BIGINT NOT NULL,
  site_id BIGINT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  PRIMARY KEY (genre_id, site_id)
);

CREATE TABLE items (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  genre_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  option_text VARCHAR(255) NULL,
  query_override VARCHAR(255) NULL,
  image_path VARCHAR(512) NOT NULL,
  source_url VARCHAR(1024) NULL,
  min_price INT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE item_site_overrides (
  item_id BIGINT NOT NULL,
  site_id BIGINT NOT NULL,
  query VARCHAR(255) NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  PRIMARY KEY (item_id, site_id)
);

CREATE TABLE listings (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  item_id BIGINT NOT NULL,
  site_id BIGINT NOT NULL,
  title VARCHAR(512) NOT NULL,
  price INT NOT NULL,
  url VARCHAR(1024) NOT NULL,
  image_url VARCHAR(1024) NULL,
  in_stock BOOLEAN NOT NULL DEFAULT TRUE,
  suspicious_reasons JSON NULL,           -- ["title_mismatch","too_cheap"]
  fetched_at DATETIME NOT NULL,
  INDEX idx_item_site_fetched (item_id, site_id, fetched_at)
);

CREATE TABLE estimates (
  item_id BIGINT NOT NULL,
  site_id BIGINT NOT NULL,
  low INT NULL,
  mid INT NULL,
  count INT NOT NULL DEFAULT 0,
  suspicious_count INT NOT NULL DEFAULT 0,
  status ENUM('ok','failed','no_result') NOT NULL,
  fetched_at DATETIME NOT NULL,
  PRIMARY KEY (item_id, site_id)
);
```

`listings` は、商品×サイトごとに最新の取得分だけ残す(古いものは取得のたびに削除)。

---

## 8. API

全エンドポイントで `Authorization: Bearer <token>` を要求する(トークンは Secret)。

| メソッド | パス | 内容 |
|---|---|---|
| GET | `/api/items?genre_id=` | 一覧 |
| POST | `/api/items` | 登録(multipartで画像、または `image_url`) |
| POST | `/api/items/from-url` | `{url, genre_id?}` からOGP画像とタイトルを取得し、下書きを返す |
| PATCH | `/api/items/:id` | 更新 |
| DELETE | `/api/items/:id` | 削除 |
| GET | `/api/items/:id/estimates` | キャッシュを即返す。24時間より古ければ裏で更新を起動し、`refreshing: true` を付ける |
| POST | `/api/items/:id/estimates/refresh` | 手動で更新 |
| GET | `/api/items/:id/listings?site_id=` | 出品一覧(参考外も含む) |
| GET/POST/PATCH | `/api/genres`, `/api/sites` | 設定 |
| GET | `/images/:path` | 画像配信 |

OpenAPIで定義し、フロントの型はそこから生成する(ダメ計アプリと同じ方式があればそれに合わせる)。

---

## 9. 登録の動線

1. **ネイティブ版のShare Extension(メイン)**:共有シートから「欲しいもの」を選ぶ → 拡張内でOGPのプレビューを表示し、ジャンルを選んで保存
2. **iOSショートカット(ネイティブ版ができるまでのつなぎ)**:URLを `/api/items/from-url` にPOST → 下書きを確定する
3. **アプリ内(ネイティブ / PWA)**:写真(カードのスクショなど)を選ぶか、URLを貼り付ける

ショートカットの作り方は `docs/shortcut.md` に手順を書く(フェーズ1の成果物に含める)。

---

## 9.5 ネイティブ版(iOS)

- SwiftUI。iOSの最新版を対象にする。見た目はダメ計アプリと同じくLiquid Glass系の方向に合わせる
- APIクライアントは `swift-openapi-generator` で `api/openapi.yaml` から生成する。手書きしない
- サイトを開くときは `UIApplication.shared.open(url)` を使う。メルカリやAmazonのアプリが入っていれば、ユニバーサルリンクでアプリ側の検索結果が開く
- 一覧データと画像はローカルにキャッシュし(SwiftData + ファイルキャッシュ)、**Macが落ちていても一覧とディープリンクは使える**ようにする
- Share Extension は本体アプリとデータ共有せず、**拡張から直接APIへPOSTする**(無料の署名でも動く構成にするため)。APIのURLとトークンは拡張側の設定にも持たせる
- iPhoneから接続するために Tailscale のiOSアプリを使う

### 署名と配布
- 初期は無料の Personal Team で実機にインストールする
  - 署名は7日で切れるので、Xcodeから再インストールが必要
  - プッシュ通知は使えない(フェーズ4の通知は、有料アカウントにした場合のみ)
  - 同時にインストールできるアプリ数に上限があるため、ダメ計アプリと合わせて枠に注意する
- 有料の Apple Developer Program に切り替えると、署名期間が1年になり、TestFlightとプッシュ通知が使える

---

## 10. PWA

- `manifest.webmanifest`:アプリ名「欲しいもの」、専用アイコン、`display: standalone`
- Service Worker:一覧データと画像をキャッシュし、**Macが落ちていてもホーム画面とディープリンクは使える**ようにする
- オフライン時は目安価格の欄に「オフライン」と表示する
- iPhoneのセーフエリアに対応する

---

## 11. k8s

- `Deployment` api(1レプリカ)、`Deployment` web(静的配信)
- `CronJob` refresher:毎日 03:00 JST。全商品を順番に処理し、サイトごとのアクセス間隔を守る
- `PersistentVolumeClaim`:画像保存用
- `Secret`:DB接続情報、APIトークン、Yahoo appid
- chromedp用のヘッドレスChromeは、refresher と api のイメージに含めるか、サイドカーにする(リソース消費を見て決める)

---

## 12. フェーズと完了条件

### フェーズ1
- [ ] ジャンル・サイト・商品のCRUD、画像アップロード
- [ ] 画像グリッドと詳細シート
- [ ] 検索ワード生成とディープリンク(全サイト)
- [ ] PWA化とオフラインキャッシュ
- [ ] iOSショートカット経由の登録と手順書

**完了条件**:iPhoneで「S.H.Figuarts グリス」を登録し、詳細シートからメルカリとAmazonを検索した状態で開ける。

### フェーズ2(ネイティブ版)
- [ ] Xcodeプロジェクト作成(`ios/`)、OpenAPIからクライアント生成
- [ ] 画像グリッドと詳細シート(PWAと同じ挙動)
- [ ] Share Extension での登録
- [ ] ローカルキャッシュとオフライン表示

**完了条件**:SafariでS.H.Figuartsの公式ページを開き、共有シートから登録できる。機内モードでも一覧が表示され、検索ボタンが動く(遷移先の表示はオンライン時のみ)。

### フェーズ3(目安価格)
- [ ] Fetcher実装(Yahoo!ショッピングAPI → カードラッシュ → ドラゴンスター → メルカリ → Yahoo!フリマ → あみあみ → 駿河屋)
- [ ] 参考外の判定と目安価格の算出
- [ ] CronJob

**完了条件**:デュエマの商品で、ショップ価格とかけ離れたメルカリの出品が「参考外」に入る。

### フェーズ4(任意)
- 表記揺れの辞書、価格の推移、公式サイトの販売状況監視、プッシュ通知

---

## 13. 開発ルール

- テストを先に書く。特に `query`、`deeplink`、`estimate` はテーブル駆動テストで網羅する
- Fetcherのテストは、実サイトを叩かず、保存したHTMLのfixtureで行う
- 実サイトへのアクセスは、手動確認とCronJobからのみ行う
- サイトのURLやHTML構造を推測で書かない。分からない場合はTODOを残して先に進む