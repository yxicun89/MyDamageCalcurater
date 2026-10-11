# ヘキサゴナル(ポートとアダプタ)への適合の確認

- 日付: 2026-10-11(基準: `origin/main`。フェーズ5のマージ後)
- 規則の正: [../../../docs/plan/architecture-adoption.md](../../../docs/plan/architecture-adoption.md)(ユーザー決定 2026-10-11)。
  Go は `internal/domain`(純粋な核)/ `internal/app`(ユースケース)/ `internal/adapters`(httpapi・store ほか)の形にそろえ、
  **依存は adapters → app → domain の一方向**。Web は `app` → `screens` → `features` → `shared` の向き。iOS は View → ViewModel → Service プロトコル(ポート)← `API*`(アダプタ)で、生成 API を import するのは `API*.swift` だけ。
- 上の計画は §9 で `apps/wishlist` を対象外にしている。ユーザーの意向(2026-10-11「このプロジェクト全てで」)に合わせて、同じ規則で確かめた。
- 確かめ方: `GOWORK=off go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...` の内部 import と、各パッケージの中身を読んだ。

## 結論

| 対象 | 判定 | 要点 |
|---|---|---|
| iOS | 合っている | 生成 API(`WishlistAPI`)を import するのは `WishlistCore/APIWishlistService.swift` だけ。View → ViewModel → `WishlistService`(プロトコル)← `APIWishlistService`・`FakeWishlistService` |
| Web | おおむね合っている | `App.tsx`(app)→ `ui/`(screens)→ `lib/`(shared)の一方向。`lib/` から `ui/`・`App` への逆向きの import は無い。フォルダ名は計画の 4 段と違い、向きの lint も無い |
| API(Go) | **形が違い、向きの崩れが 4 か所ある** | 考え方(リポジトリのインターフェースと Memory/MySQL の 2 実装、純粋な `query`・`deeplink`・`estimate`)はポートとアダプタに近いが、domain / app / adapters のフォルダが無く、1 パッケージに層が混ざっている |

## API の現状の割り当て

| 今のパッケージ | 中身 | 本来の置き場所 |
|---|---|---|
| `query`・`deeplink`・`estimate` | 純粋な計算(I/O なし) | domain |
| `item`(`item.go`・`summary.go`・`guess.go` ほか) | 商品・ジャンル・サイトの型と規則 | domain |
| `item/service.go` | 登録・更新などの手順 | app |
| `item/mysql*.go`・`store`(sqlc 生成) | MySQL の実装 | adapters/store |
| `item/memory*.go` | テスト用のメモリ実装 | adapters(または app のテスト用) |
| `refresh` | 目安価格の更新の手順 | app |
| `fetcher`・`chromium`・`official`・`ogp`・`storage`・`netguard` | 外部サイト・ファイル・SSRF ガード | adapters |
| `httpapi`・`api`(oapi-codegen 生成) | HTTP | adapters/httpapi |

## 向きの崩れ

1. **domain が adapter を import している**: `item` パッケージに MySQL 実装(`mysql*.go`)が同居し、`item` が `store`(sqlc 生成)を import する。domain の型を使うだけのコードからも MySQL の部品が見える
2. **app が adapter のパッケージのポートに依存している**: `item.Service` は `storage.Storage` を受け取る。ポート(インターフェース)が adapter のパッケージ(`storage`)に置かれている
3. **app が adapter を具体的に使っている**: `refresh` が `fetcher.Registry`・`fetcher.Listing`・`fetcher.MaxListings`・`fetcher.NightlyOnlyHosts` と `official.Result` を直接使う。取得件数の上限や夜間だけのサイトといった規則が adapter に置かれている
4. **ユースケースの手順が HTTP ハンドラにある**: `httpapi/handlers.go` の from-url が、URL の検査(`netguard`)・静的な下書き(`ogp.StaticDraft`)・名前の整形(`ogp.CleanTitle`)・Safari から来た名前と画像の採用を、ハンドラの中で組み立てている(計画の G3 と同じ型)

adapter 同士の import(`official` → `fetcher.HostGate`、`fetcher` → `item` の型)は規則の違反ではないが、サイトごとのアクセス間隔は「節度」の規則なので domain か app に置く方が読みやすい。

## 直し方の案(未着手)

計画 §6 の「1 サービス = 1 PR」に合わせ、このレーンの PR 1〜2 本で行う。動作は変えず、テストはそのまま通す。

1. `internal/domain`: `query`・`deeplink`・`estimate` と、`item` の型・規則(推定・要約)。`Listing`・取得件数の上限・夜間だけのサイト・アクセス間隔もここへ
2. `internal/app`: `item.Service`・`refresh.Service`・from-url の手順。ポート(`Repository`・`ImageStore`・`Fetcher`・`OfficialChecker`・`PageReader`)をここで宣言する
3. `internal/adapters`: `httpapi`・`store`(MySQL。今の `item/mysql*.go` と sqlc)・`memory`・`fetcher`・`chromium`・`official`・`ogp`・`storage`・`netguard`
4. 向きの検査は、ダメ計の `tools/archcheck`(S0 で作る予定)ができたら対象に入れる。それまではこのモジュールの Go テストで import の向きを確かめる

ダメ計側の S0(ADR-0811・archcheck)の決定より先に進めるかはユーザーの判断待ち。
