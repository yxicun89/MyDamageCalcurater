## Wishlist
Lane: 欲しいものリスト(別アプリ。`apps/wishlist/` だけを変更する。仕様の正は `apps/wishlist/CLAUDE.md`、設計は `apps/wishlist/docs/design.md`)
Active: なし(2026-10-11 フェーズ5〈使用感の作り直し〉の 1 周目を実装。ユーザーの感想待ち)
Branch: lane/wishlist(作業ディレクトリ ~/MyDamageCalcurater-wishlist)。PR ごとに main へ統合し、次の区切りも同じブランチで続ける
Status: 2026-10-04 全フェーズの実装完了。フェーズ1(#572・#581・#582)・NetworkPolicy(#587)・フェーズ3(#588・#591・#601〈メルカリ headless・Chromium 入り refresher〉)・
フェーズ2 iOS(#596)・フェーズ4-1 辞書(#599)・4-2 価格推移(#605)・4-3 公式の販売状況と iOS の辞書編集(この PR)。ローカル k3d にデプロイ済み。
フェーズ4-4 通知は作らない(ユーザー指示)。
2026-10-11 フェーズ5(使用感の改善ループ 1 周目。apps/wishlist/docs/ux-loop.md・phase5-spec.md): API(#718)・Web と取得の不具合(#722)・iOS(この PR)。
トークンと Yahoo の appid を廃止(8080 は 127.0.0.1 だけ。ADR-0810・#712)。iOS は #723。
2026-10-12 API をヘキサゴナル(domain・app・adapters)に再配置(動作は不変。apps/wishlist/docs/arch-spec.md。向きは internal/archtest で検査)。
Next: ユーザーに作り直した画面を触ってもらい、感想から 2 周目(ux-loop.md に追記)。iPhone は Tailscale を入れてから(apps/wishlist/docs/verify.md §7・§8)。
取得しないもの: ドラゴンスター(Cloudflare で取得不可)。リンクのみ: Amazon・プレバン・魂ウェブ・ポケセン。
既定案で進行・ユーザー未確認: decisions/2026-10-04-wishlist-phase4-plan.md(辞書・推移・公式の販売状況の作り方)
