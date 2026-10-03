# wishlist 設計(フェーズ1の土台)

仕様の正は [../CLAUDE.md](../CLAUDE.md)。このファイルは、仕様に書かれていない部分をどう決めたかと、
API 契約・DB・k8s の形をまとめる。決定は末尾の「設計判断」に 1 件ずつ書く(W-番号)。

## 1. 全体

```
iPhone(PWA / 将来 iOS アプリ)
  └─ Tailscale ─ Mac(k3d の Traefik :8080)
                   ├─ /wishlist/api/*, /wishlist/images/*  → wishlist-api(Go + Echo)─ MySQL(pokecalc 名前空間の共有インスタンス。DB `wishlist`)
                   └─ /wishlist/*                           → wishlist-web(nginx。PWA の静的配信)
```

- 名前空間は `wishlist`。ダメ計の gateway(`/` で全部を受ける)とは**パスの前置 `/wishlist`** で分ける(W-03)
- ディープリンクと検索ワードの組み立ては**フロントだけ**で行う。API が落ちていても、キャッシュ済みの一覧からサイトを開ける

## 2. ディレクトリ

| パス | 中身 |
|---|---|
| `api/openapi.yaml` | API 契約の唯一の正。Go のサーバー型(oapi-codegen)と Web の型(openapi-typescript)を生成する |
| `api/go.mod` | Go モジュール `example.com/pokecalc/apps/wishlist/api`。ルートの `go.work` には入れず `GOWORK=off` で扱う(W-01) |
| `api/migrations/` | golang-migrate 形式(`NNNNNN_name.up.sql` / `.down.sql`) |
| `api/internal/{item,query,deeplink,fetcher,estimate,ogp,storage}` | 仕様 §2 のとおり。フェーズ1では `item`・`query`・`ogp`・`storage` と HTTP 層 |
| `testdata/query-cases.json` | 検索ワード生成・ディープリンクの**共通テストベクタ**。Go と TS の両方のテストが読む(実装のずれを防ぐ) |
| `web/` | Vite + React + TS の PWA |
| `deploy/k8s/` | Kustomize(`base` / `overlays/local`) |
| `Makefile` | ルートの Makefile から include。リポジトリ直下で `make wishlist-test` 等(`make test`・`lint`・`build` にも含まれる。W-01) |

## 3. API(要点。詳細は `api/openapi.yaml`)

- 認証: `/api/*` は `Authorization: Bearer <token>`。トークンは Secret `wishlist-api` の `api-token`。比較は定数時間
- `/images/{name}` は**認証なし**。`<img>` はヘッダーを付けられないため。ファイル名はランダム(UUID v4 + 拡張子)で推測できない。
  公開範囲は Tailscale 内だけ(W-04)
- `/healthz` は認証なし
- エラーは `{"code": "...", "message": "..."}` に統一
- 仕様 §8 からの追加・具体化:
  - `PUT /api/items/{id}/image`(multipart):画像の差し替え。PATCH は JSON だけにして単純にする
  - `GET /api/items/{id}` を足す(編集画面用)
  - `GET /api/genres` は各ジャンルの `site_ids`(表示順)を含む。PATCH で `site_ids` を渡すと並びごと置き換える
  - `POST /api/items` は `multipart/form-data`(`image` ファイル)と `application/json`(`image_url`)の両方を受ける。
    `image_url` の場合はサーバーが取得して保存する(外部の画像に依存しない)
  - `POST /api/items/from-url` は**保存しない**。OGP から `{name, image_url, source_url, genre_id}` の下書きを返す
- フェーズ3の範囲(`/estimates`・`/listings`)はフェーズ1で契約だけ先に置き、フェーズ3で実装した(W-06。詳細は docs/phase3-api-spec.md)
- 外部 URL の取得(OGP・`image_url`)は SSRF 対策をする:http/https のみ、プライベート・ループバック・リンクローカル宛ては拒否、
  タイムアウト 10 秒、HTML は 2 MiB・画像は 10 MiB まで(W-05)

## 4. DB(`api/migrations/`)

仕様 §7 のテーブルをそのまま作り、次を足す。

- 文字コードは `utf8mb4` / `utf8mb4_0900_ai_ci`(既存 pokedex に合わせる)
- 外部キー:`genre_sites`・`items`・`item_site_overrides`・`listings`・`estimates` から親へ。
  商品の削除で子(上書き・出品・目安)は `ON DELETE CASCADE`。ジャンルは商品が使っていれば消せない(`items.genre_id` は `RESTRICT`)。
  サイトの削除は、そのサイトへの紐づけ(`genre_sites`・`item_site_overrides`)と取得結果(`listings`・`estimates`)を `CASCADE` で消す
  (サイトは設定であり、紐づけ・取得結果はサイトなしでは意味を持たないため。API にサイトの削除はまだ無い)
- `genres.name`・`sites.name` は UNIQUE(設定画面で同名を重複登録しない。重複は 422)
- `items.updated_at` を足す(一覧キャッシュの更新判定に使う)
- 初期データ(`000002_seed`):ジャンル 4 件と、**確認済みの 2 サイト(メルカリ・Amazon)だけ**。
  他サイトの URL は推測で入れない(仕様 §5。人が確認して設定画面から登録する)

## 5. k8s(`deploy/k8s/`)

| リソース | 内容 |
|---|---|
| `Namespace wishlist` | |
| `Deployment wishlist-api`(1 レプリカ)+ `Service` | 画像は PVC を `/data/images` にマウント。`Recreate`(RWO の PVC を2つの Pod で取り合わない) |
| `Deployment wishlist-web` + `Service` | nginx の静的配信 |
| `PersistentVolumeClaim wishlist-images` | 1Gi、RWO(k3d の local-path) |
| `Job wishlist-migrate` | api イメージの `migrate up`。Job の template は変えられないため、2 回目以降は `kubectl delete job` してから apply(デプロイ用ターゲットは API 実装と一緒に足す) |
| `Ingress wishlist` + Traefik `Middleware wishlist-strip` | `/wishlist` を外して api / web へ |
| `NetworkPolicy` | wishlist 名前空間で default-deny ingress、Traefik → api/web だけ許可 |
| `Secret wishlist-api` | **Git に置かない**。`scripts/bootstrap.sh` が無いときだけ作る(`database-dsn`・`api-token`。`yahoo-appid` は環境変数 `WISHLIST_YAHOO_APPID` があるときだけ入れる任意のキー) |
| `CronJob wishlist-refresher` | 毎日 03:00 JST。Chromium 入りの専用イメージ(`wishlist/refresher`。`api/Dockerfile.refresher`)の `/wishlist-refresher` を起動する。api のイメージには Chromium を載せない |

### 共有基盤の変更

- **MySQL の NetworkPolicy**(`deploy/k8s/base/networkpolicy/allow-mysql-ingress.yaml`)に、wishlist 名前空間の
  `app.kubernetes.io/name: wishlist-api` / `wishlist-migrate` / `wishlist-refresher` から 3306 への許可を足した
  (ユーザー決定 2026-10-03 により wishlist レーンが PR #587 で入れた。`docs/ai-shared/decisions/2026-10-03-wishlist-phase2-3-user-decisions.md`)
- Pod の起動直後は許可の反映が数秒遅れ、接続が拒否されることがある。api・migrate・refresher は起動時に DB への接続を再試行する(`internal/dbwait`)
- DB `wishlist` と専用ユーザーの作成は root 権限が要る。`scripts/bootstrap.sh`(1 回だけ実行する。2026-10-03 に実行済み。共有の mysql Pod に `kubectl exec`。秘密は stdin・ファイル経由で渡し argv に載せない)

## 6. Web(PWA)

- Vite + React + TS。型は openapi-typescript で生成(ダメ計の Web と同じ方式)。HTTP は `fetch` の薄いラッパ
- `base: '/wishlist/'`。manifest(名前「欲しいもの」、`display: standalone`)と Service Worker を自前で書く(依存を増やさない)
- オフライン:一覧(items・genres・sites)は最後に取れた JSON を localStorage にも保存、画像は SW の Cache Storage
- API の URL とトークンは設定画面で入力して端末に保存

## 7. 設計判断

- **W-01 ルートの go.work には入れず、Makefile は include 1 行だけ**:仕様で `apps/wishlist/` 外を変えないため。`GOWORK=off` で単独のモジュールにし、
  コード生成は既存の `services`(oapi-codegen)・`tools`(sqlc・staticcheck)モジュールの `go tool` を**変更せずに**呼ぶ。
  CI で回すため、ルート Makefile に `include apps/wishlist/Makefile` の 1 行だけ足した(ユーザー決定 2026-10-03)。
  staticcheck はルートの `staticcheck` ターゲットを変えず、`wishlist-lint` から呼ぶ
- **W-02 生成コードはコミットする**:ダメ計と同じ(差分検査ができる)
- **W-03 パスの前置で分ける**:gateway の Ingress がホスト指定なしの `/` を持つため、ホスト名で分けるには Tailscale 側の設定変更が要る。
  `/wishlist` 前置 + StripPrefix なら共有基盤を一切変えずに同じ入口(`tailscale serve` → :8080)に同居できる
- **W-04 画像は認証なし**:上記の理由。推測できないファイル名で代替する
- **W-05 外部取得の SSRF 対策**:自分専用でも、クラスタ内(MySQL 等)へサーバーから届く経路を作らない
- **W-06 フェーズ3の API は契約だけ先に置く**:クライアントの生成をやり直さずに済むため。フェーズ1〜2 の間は未実装を 501 で明示した(成功を装わない)。フェーズ3で実装済み
- **W-07 検索ワード・ディープリンクの共通テストベクタ**:Go(フェーズ3の取得で使う)と TS(フロントのリンク)の二重実装になるため、
  同じ JSON で両方を検査する
- **W-08 PATCH の「省略」と「null」を区別する**:oapi-codegen の `output-options.nullable-type: true`(`nullable.Nullable[T]`)で生成する。
  既定の `*T` では null(消す)と省略(変えない)が同じになり、仕様の部分更新ができないため
- **W-09 migrate だけが multiStatements を使う**:API サーバーの DSN には付けない(1 回の Query で複数文を通さない)。
  migrate は DSN に `multiStatements=true` を足してから接続する(services/internal/dbmigrate と同じ考え方。wishlist は共有パッケージを使わず自前で持つ)

### 未決事項(フェーズ3で決めた)

- サイト行の「状態(在庫あり等)」(仕様 §3)は `SiteEstimate.in_stock_count`(参考外を除いた在庫ありの件数)で表す。
  フェーズ3の決めたことは [phase3-api-spec.md](phase3-api-spec.md)
