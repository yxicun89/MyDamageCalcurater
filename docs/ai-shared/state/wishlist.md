## Wishlist
Lane: 欲しいものリスト(別アプリ。`apps/wishlist/` だけを変更する。仕様の正は `apps/wishlist/CLAUDE.md`、設計は `apps/wishlist/docs/design.md`)
Active: Claude Code
Branch: lane/wishlist(作業ディレクトリ ~/MyDamageCalcurater-wishlist)。PR ごとに main へ統合し、次の区切りも同じブランチで続ける
Status: PR #572・#581・#582(フェーズ1)、#587(共有 MySQL の NetworkPolicy)マージ済み。ローカル k3d にデプロイ済み(2026-10-03。DB・Secret 作成、登録→画像→削除まで確認)。
フェーズ3 Go(目安価格の算出・参考外の判定・Yahoo!ショッピング API・5 秒間隔・更新 API・CronJob 03:00 JST・DB 接続の再試行)を実装、critic PASS。受け入れ条件 `apps/wishlist/docs/phase3-api-spec.md`。
フェーズ2 iOS はブランチ feat/wishlist-ios(worktree ~/MyDamageCalcurater-wishlist-ios)で実装中。
Next: PWA の目安価格表示(サマリ・サイト別・参考外)→ サイト別の取得処理(カードラッシュ・あみあみ・Yahoo!フリマ。確認結果は apps/wishlist/docs/sites.md に入れる)と確認済みサイトの seed →
iOS(feat/wishlist-ios)の critic・PR。未着手で残すもの: メルカリ・ドラゴンスターの headless 取得と Chromium 入りイメージ(構造が未確認)、駿河屋(robots.txt の扱いが要判断)
