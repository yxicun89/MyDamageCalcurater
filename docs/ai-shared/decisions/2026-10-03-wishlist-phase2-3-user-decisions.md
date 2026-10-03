## 2026-10-03: Wishlist のフェーズ2・3 の進め方(ユーザー決定)
Decision:
- 実サイト(カードラッシュ・ドラゴンスター・あみあみ・駿河屋・メルカリ・Yahoo!フリマ・Yahoo!ショッピング・プレバン・魂ウェブ・ポケモンセンターオンライン)の検索 URL と HTML 構造は、AI が手動確認の範囲(1 サイト数回)で実際に開いて確かめてよい。
  テスト用 fixture は実ページを丸ごと保存せず、構造だけを写した架空の内容で作る。確かめた結果は `apps/wishlist/docs/sites.md` に記録する。
- クラスタへの適用は AI が行ってよい: 共有 MySQL の NetworkPolicy に wishlist 名前空間からの許可を 1 項目追加(PR で入れる)、`bootstrap.sh` で DB `wishlist`・ユーザー・Secret を作成、`make wishlist-k3d-deploy` と疎通確認。いずれもローカル k3d で課金なし。
- headless(chromedp + Chromium)は refresher 専用イメージにだけ載せ、CronJob(毎日 03:00 JST)でのみ取得する。api は軽いまま。
  仕様 §6「headless のサイトは詳細シートを開いたときと深夜の CronJob でのみ取得」のうち、詳細シートを開いたときの取得は当面行わない(仕様からの逸脱。必要になったら api から一時 Job を起こす案を検討)。
- Yahoo! の appid は当面なし(api 型の取得は appid 未設定で無効のまま)。Xcode はユーザーが入れる予定(入ったら iOS のビルド・シミュレータテストを AI が確認。署名チームと実機インストールは人)。
Reason: ユーザー回答(2026-10-03 23:0x)。
Impact: decisions/2026-10-03-wishlist-mysql-networkpolicy.md の提案は、このユーザー決定によりデータレーンを待たずに wishlist レーンが入れる。
