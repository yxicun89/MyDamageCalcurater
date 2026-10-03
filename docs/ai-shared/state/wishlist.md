## Wishlist
Lane: 欲しいものリスト(別アプリ。`apps/wishlist/` だけを変更する。仕様の正は `apps/wishlist/CLAUDE.md`、設計は `apps/wishlist/docs/design.md`)
Active: Claude Code
Branch: lane/wishlist(作業ディレクトリ ~/MyDamageCalcurater-wishlist)。PR ごとに main へ統合し、次の区切りも同じブランチで続ける
Status: 2026-10-03 レーン開始。PR #572(設計・OpenAPI・migrations・k8s)マージ済み。
フェーズ1 API(CRUD・画像・from-url・Bearer 認証・SSRF 対策・MySQL/メモリ実装・Dockerfile)を実装、critic PASS。受け入れ条件は `apps/wishlist/docs/phase1-api-spec.md`。
クラスタへの適用(`make wishlist-k3d-deploy`)は未実施: 共有 MySQL の NetworkPolicy への許可(decisions/2026-10-03-wishlist-mysql-networkpolicy.md。データレーン)待ち。DB 作成は `apps/wishlist/scripts/bootstrap.sh`(人が 1 回)
Next: PWA(`apps/wishlist/web/`。Vite+React+TS、openapi-typescript、`base: /wishlist/`、画像グリッド・詳細シート・ディープリンク〈testdata/query-cases.json を TS でも検査〉・設定・オフラインキャッシュ・manifest/SW)→ wishlist-web の Dockerfile → `docs/shortcut.md`(iOS ショートカット手順)
