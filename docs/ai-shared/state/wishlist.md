## Wishlist
Lane: 欲しいものリスト(別アプリ。`apps/wishlist/` だけを変更する。仕様の正は `apps/wishlist/CLAUDE.md`、設計は `apps/wishlist/docs/design.md`)
Active: Claude Code
Branch: lane/wishlist(作業ディレクトリ ~/MyDamageCalcurater-wishlist)。PR ごとに main へ統合し、次の区切りも同じブランチで続ける
Status: 2026-10-03 レーン開始。フェーズ1の土台(設計・`api/openapi.yaml`・migrations・k8s マニフェスト・`make wishlist-*`)を作成中。
共有基盤への依頼: MySQL の NetworkPolicy に wishlist 名前空間からの許可(decisions/2026-10-03-wishlist-mysql-networkpolicy.md)
Next: フェーズ1の API 実装(item・genre・site の CRUD、画像保存、from-url の OGP)→ PWA → iOS ショートカット手順書
