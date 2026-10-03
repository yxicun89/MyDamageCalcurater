## Wishlist
Lane: 欲しいものリスト(別アプリ。`apps/wishlist/` だけを変更する。仕様の正は `apps/wishlist/CLAUDE.md`、設計は `apps/wishlist/docs/design.md`)
Active: なし(2026-10-04 フェーズ1〜3 の実装完了)
Branch: lane/wishlist(作業ディレクトリ ~/MyDamageCalcurater-wishlist)。PR ごとに main へ統合し、次の区切りも同じブランチで続ける
Status: フェーズ1(PR #572・#581・#582)・NetworkPolicy(#587)・フェーズ3(#588・#591)マージ済み。ローカル k3d にデプロイ済み(2026-10-04。migrate 000004、CronJob 03:00 JST。
実サイト〈あみあみ・Yahoo!フリマ〉から目安価格が取れることを確認)。フェーズ2 iOS と iOS の目安価格表示を実装、critic PASS(このブランチ feat/wishlist-ios の PR)。
Next: 人の確認(apps/wishlist/ios/README.md・docs/shortcut.md): iPhone で PWA と iOS アプリ(署名・実機インストール・共有シート・機内モード・Liquid Glass の見た目)。
未着手で残すもの: メルカリ・ドラゴンスターの headless 取得と Chromium 入りイメージ(構造が未確認。ドラゴンスターは Cloudflare で 403)、Yahoo! の appid(未取得のため Yahoo!ショッピングの取得は無効)、
プレバン・魂ウェブ・ポケセンのリンク(検索結果まで未確認。docs/sites.md の候補を人が確かめて設定画面から登録)
