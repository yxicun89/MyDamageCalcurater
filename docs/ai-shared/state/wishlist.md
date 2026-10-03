## Wishlist
Lane: 欲しいものリスト(別アプリ。`apps/wishlist/` だけを変更する。仕様の正は `apps/wishlist/CLAUDE.md`、設計は `apps/wishlist/docs/design.md`)
Active: Claude Code
Branch: lane/wishlist(作業ディレクトリ ~/MyDamageCalcurater-wishlist)。PR ごとに main へ統合し、次の区切りも同じブランチで続ける
Status: 2026-10-03 レーン開始。PR #572(設計・OpenAPI・migrations・k8s)・#581(フェーズ1 API)マージ済み。
フェーズ1 PWA(`apps/wishlist/web/`。画像グリッド・詳細シート・登録/編集/設定・オフラインキャッシュ・manifest/SW・nginx の Dockerfile)と
`apps/wishlist/docs/shortcut.md`(iOS ショートカット手順)を実装、critic PASS。受け入れ条件は `apps/wishlist/docs/phase1-web-spec.md`。
クラスタへの適用(`make wishlist-k3d-deploy`)は未実施: 共有 MySQL の NetworkPolicy への許可(decisions/2026-10-03-wishlist-mysql-networkpolicy.md。データレーン)待ち。DB 作成は `apps/wishlist/scripts/bootstrap.sh`(人が 1 回)
Next: フェーズ1の完了条件(iPhone で「S.H.Figuarts グリス」を登録し、詳細シートからメルカリと Amazon を開ける)の確認は人の作業:
NetworkPolicy の許可 → `bootstrap.sh` → `make wishlist-k3d-deploy` → `tailscale serve` 経由で `/wishlist/` を開いて設定画面にトークン → 登録。
その後フェーズ2(`apps/wishlist/ios/`。Xcode プロジェクト・swift-openapi-generator・Share Extension)。
