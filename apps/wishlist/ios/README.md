# wishlist iOS(SwiftUI)

「欲しいものリスト」のネイティブ版(仕様: [../CLAUDE.md](../CLAUDE.md) §9.5・§12 フェーズ2、受け入れ条件: [../docs/phase2-ios-spec.md](../docs/phase2-ios-spec.md))。
ダメ計の `ios/` と同じ構成で、別プロジェクト・別 Bundle ID。API クライアントは `api/openapi.yaml` から swift-openapi-generator で生成し、手で書かない。

| パス | 役割 |
|---|---|
| `WishlistKit/` | Swift Package。View 以外のすべて(ドメイン・検索ワードとディープリンク・`WishlistService`・キャッシュ・設定・ViewModel)。`swift test` で検証する |
| `WishlistKit/Sources/WishlistAPI/Generated/` | `api/openapi.yaml` の生成物(コミットする・手で編集しない) |
| `WishlistKit/Tests/WishlistCoreTests/Resources/query-cases.json` | `testdata/query-cases.json` のコピー(`scripts/sync-testdata.sh` で同期。シンボリックリンクにしない) |
| `Wishlist.xcodeproj` / `Wishlist/` | アプリ(SwiftUI の View だけ)。フォルダ同期の手書きプロジェクト |
| `WishlistShare/` | Share Extension(共有シートから登録)。本体とデータを共有せず、API へ直接 POST する |
| `WishlistUITests/` | XCUITest(`WISHLIST_USE_FAKE=1|offline` で通信なしの起動) |
| `tools/openapi-gen/` | 生成器の版(1.13.1)を固定する生成専用パッケージと設定 |
| `scripts/` | 生成・テストデータの同期・xcodebuild の合否判定 |

依存の版は完全固定(swift-openapi-generator 1.13.1・swift-openapi-runtime 1.12.1・swift-openapi-urlsession 1.3.1・swift-http-types 1.8.0。いずれも Apache-2.0)。
Bundle ID は `com.example.wishlist`(本体)・`com.example.wishlist.share`(拡張)。署名チーム(`DEVELOPMENT_TEAM`)は空。

## コマンド(リポジトリ直下で)

```sh
cd "$(git rev-parse --show-toplevel)"
make wishlist-ios-gen          # api/openapi.yaml を変えたら(生成物をコミットする)
make wishlist-ios-gen-check    # コミット済みの生成物が openapi.yaml と一致するか
make wishlist-ios-test         # WishlistKit のテストを macOS で(シミュレータ不要)
make wishlist-ios-xcode-test   # xcodebuild: Kit のテスト + アプリの XCUITest(Xcode 27 とシミュレータが要る。無ければ終了コード 2)
```

`xcode-select` が CommandLineTools のままでも、スクリプトが `DEVELOPER_DIR` を `/Applications/Xcode.app` に向ける。
CI は Linux なので `make wishlist-test` には iOS を含めない(構文チェックの `bash -n` だけ `make wishlist-lint` に入る)。

## 人が Xcode で確認すること(Personal Team の署名・実機)

AI は署名とシミュレータの目視をしない。実装後に人が次を確認する。

1. `open apps/wishlist/ios/Wishlist.xcodeproj` → target `Wishlist` と `WishlistShare` の Signing で Team に Personal Team を選ぶ(Bundle ID が衝突したら末尾を変える)。
2. iPhone を接続して Run。初回は 設定 → 一般 → VPN とデバイス管理 で開発元を信頼する。署名は 7 日で切れるので、切れたら Xcode から入れ直す(同時に入れられるアプリ数に上限があり、ダメ計と枠を分け合う)。
3. 設定画面で API のベース URL(例 `https://<Tailscale のホスト名>/wishlist/`)とトークンを入れる。iPhone には Tailscale の iOS アプリが要る。
4. 「S.H.Figuarts グリス」を登録し、詳細シートからメルカリと Amazon を開く(メルカリ・Amazon のアプリが入っていればアプリ側の検索結果が開く)。
5. Safari で S.H.Figuarts の公式ページを開き、共有シートの「欲しいもの」から登録する(拡張にも接続設定が要る。初回は拡張の設定画面で入力)。
6. 機内モードにして起動し、一覧と画像が出て、サイト行が動く(遷移先はオンラインのときだけ)。サマリは「オフライン」。

詳しい項目は [../docs/phase2-ios-spec.md](../docs/phase2-ios-spec.md) の「Xcode が要る未検証項目」。
