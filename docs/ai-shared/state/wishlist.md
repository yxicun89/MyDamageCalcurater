## Wishlist
Lane: 欲しいものリスト(別アプリ。`apps/wishlist/` だけを変更する。仕様の正は `apps/wishlist/CLAUDE.md`、設計は `apps/wishlist/docs/design.md`)
Active: Claude Code
Branch: lane/wishlist(作業ディレクトリ ~/MyDamageCalcurater-wishlist)。PR ごとに main へ統合し、次の区切りも同じブランチで続ける
Status: PR #572・#581・#582(フェーズ1)、#587(NetworkPolicy)、#588(フェーズ3 Go)マージ済み。ローカル k3d にフェーズ1をデプロイ済み(2026-10-03)。
サイト別の取得(カードラッシュ・あみあみ・Yahoo!フリマ・駿河屋〈30 秒間隔・夜間のみ〉)・確認済みサイトの seed(000004)・PWA の目安価格表示(該当なしは「出品ないかも」)を実装、critic PASS。
フェーズ2 iOS はブランチ feat/wishlist-ios(worktree ~/MyDamageCalcurater-wishlist-ios)で実装中。
Next: この PR のマージ → k3d へ再デプロイ(migrate 000003・000004、CronJob)→ iOS の critic・PR。
未着手で残すもの: メルカリ・ドラゴンスターの headless 取得と Chromium 入りイメージ(構造が未確認。ドラゴンスターは Cloudflare で 403)、Yahoo! の appid(ユーザー未取得)
