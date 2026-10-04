## Wishlist
Lane: 欲しいものリスト(別アプリ。`apps/wishlist/` だけを変更する。仕様の正は `apps/wishlist/CLAUDE.md`、設計は `apps/wishlist/docs/design.md`)
Active: なし(2026-10-04 全フェーズ実装完了)
Branch: lane/wishlist(作業ディレクトリ ~/MyDamageCalcurater-wishlist)。PR ごとに main へ統合し、次の区切りも同じブランチで続ける
Status: 2026-10-04 全フェーズの実装完了。フェーズ1(#572・#581・#582)・NetworkPolicy(#587)・フェーズ3(#588・#591・#601〈メルカリ headless・Chromium 入り refresher〉)・
フェーズ2 iOS(#596)・フェーズ4-1 辞書(#599)・4-2 価格推移(#605)・4-3 公式の販売状況と iOS の辞書編集(この PR)。ローカル k3d にデプロイ済み。
フェーズ4-4 通知は作らない(ユーザー指示)。
Next: 人の確認(iPhone で PWA と iOS アプリ。apps/wishlist/ios/README.md・docs/shortcut.md)。Yahoo! の appid を取ったら Secret に入れる(bootstrap.sh の WISHLIST_YAHOO_APPID)。
取得しないもの: ドラゴンスター(Cloudflare で取得不可)。リンクのみ: Amazon・プレバン・魂ウェブ・ポケセン。
既定案で進行・ユーザー未確認: decisions/2026-10-04-wishlist-phase4-plan.md(辞書・推移・公式の販売状況の作り方)
