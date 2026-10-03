## Wishlist
Lane: 欲しいものリスト(別アプリ。`apps/wishlist/` だけを変更する。仕様の正は `apps/wishlist/CLAUDE.md`、設計は `apps/wishlist/docs/design.md`)
Active: なし(2026-10-03 22:20 利用枠の上限で停止)
Branch: lane/wishlist(作業ディレクトリ ~/MyDamageCalcurater-wishlist)。PR ごとに main へ統合し、次の区切りも同じブランチで続ける
Status: 2026-10-03 レーン開始。PR #572(設計・OpenAPI・migrations・k8s)マージ済み。
フェーズ1 API(CRUD・画像・from-url・Bearer 認証・SSRF 対策・MySQL/メモリ実装・Dockerfile)は PR #581 でマージ済み。受け入れ条件は `apps/wishlist/docs/phase1-api-spec.md`。
クラスタへの適用(`make wishlist-k3d-deploy`)は未実施: 共有 MySQL の NetworkPolicy への許可(decisions/2026-10-03-wishlist-mysql-networkpolicy.md。データレーン)待ち。DB 作成は `apps/wishlist/scripts/bootstrap.sh`(人が 1 回)
Next: PWA の実装を続ける(WIP コミット済み。spec-writer のテストは揃い、implementer が利用枠の上限で途中停止)。
受け入れ条件 `apps/wishlist/docs/phase1-web-spec.md`。`cd apps/wishlist/web && npm test` を通す(テストは弱めない)。
注意: `public/apple-touch-icon.png` は公開前検査(テキスト以外のファイルは追跡不可)に当たるため、Git に置かず、
node の zlib で生成するスクリプトをビルド・テスト前に実行する方式にする(テストが PNG の存在を見るなら、生成スクリプトを先に走らせる)。
残り: manifest・CSS(safe-area)・Dockerfile・nginx.conf・`wishlist-web-docker-build`→ critic → PR。その後 `docs/shortcut.md`(iOS ショートカット手順)
