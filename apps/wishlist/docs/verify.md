# 欲しいものリスト(wishlist)の動作確認

上から順に実行する。各コマンドの下の「→」が成功の見え方。

入口は1つ: **ブラウザ・iOS とも `http://localhost:8080/wishlist/`**(iPhone からは Tailscale の HTTPS の URL + `/wishlist/`)。

## 1. 準備(初回だけ)

```sh
cd "$(git rev-parse --show-toplevel)"
make doctor
make wishlist-web-deps
```
→ doctor が不足ツールを報告しない。`wishlist-web-deps` がエラーなく終わる。

## 2. 自動テスト

```sh
cd "$(git rev-parse --show-toplevel)"
make wishlist-test && make wishlist-lint && make wishlist-build
make wishlist-test-mysql
```
→ どちらも最後まで成功する。`wishlist-test-mysql` の最後の行が `ok  	example.com/pokecalc/apps/wishlist/api/migrations`。

iOS のテスト(Xcode が要る):

```sh
cd "$(git rev-parse --show-toplevel)"
xcode-select -p
```
→ `/Applications/Xcode.app/Contents/Developer` なら次へ。`/Library/Developer/CommandLineTools` なら、このターミナルで `export DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer` を実行してから次へ。

```sh
cd "$(git rev-parse --show-toplevel)"
make wishlist-ios-gen-check
make wishlist-ios-sync-check
make wishlist-ios-test
```
→ `生成物は ... と一致`、`テストリソースは元と一致`、`Executed <件数> tests, with 0 failures`。

シミュレータのテストは、他のアプリと共有しない専用のシミュレータで実行する(共有すると UI テストが不安定になる)。

```sh
cd "$(git rev-parse --show-toplevel)"
xcrun simctl shutdown "iPhone 18 Pro" 2>/dev/null || true
xcrun simctl clone "iPhone 18 Pro" wishlist-verify
WISHLIST_IOS_SIMULATOR=wishlist-verify make wishlist-ios-xcode-test
xcrun simctl delete wishlist-verify
```
→ `wishlist-ios-xcode-test(kit): 全 N 件 / 成功 N / 失敗 0` と `wishlist-ios-xcode-test(ui): 全 N 件 / 成功 N / 失敗 0` が出る。

## 3. k3d を用意する(初回だけ)

ダメ計の k3d クラスタに同居する。クラスタが無ければ作る。

```sh
cd "$(git rev-parse --show-toplevel)"
k3d cluster list
```
→ `pokecalc` の行があり、SERVERS が `1/1`。行が無ければ `make up`、`0/1` なら `k3d cluster start pokecalc` を実行してから次へ。

DB `wishlist` と Secret `wishlist-api` を作る(2 回目以降は何もしない)。

```sh
cd "$(git rev-parse --show-toplevel)"
apps/wishlist/scripts/bootstrap.sh
```
→ 初回は `DB 'wishlist' と Secret 'wishlist/wishlist-api' を作りました。`。
2 回目以降は `Secret 'wishlist/wishlist-api' は既に存在します。何もしません`。

## 4. 最新のコードを k3d に入れる(動作確認の前に毎回)

```sh
cd "$(git rev-parse --show-toplevel)"
git switch main && git pull --ff-only
make wishlist-k3d-deploy
kubectl -n wishlist wait --for=condition=complete job/wishlist-migrate --timeout=180s
kubectl -n wishlist logs job/wishlist-migrate | tail -1
kubectl -n wishlist rollout status deploy/wishlist-api --timeout=180s
kubectl -n wishlist rollout status deploy/wishlist-web --timeout=120s
```
→ `job.batch/wishlist-migrate condition met`、`migrate up: ok`、`deployment "wishlist-api" successfully rolled out`、`deployment "wishlist-web" successfully rolled out`。

## 5. 疎通を確認する

```sh
cd "$(git rev-parse --show-toplevel)"
curl -s -o /dev/null -w "web %{http_code}\n" http://localhost:8080/wishlist/
curl -s -o /dev/null -w "genres %{http_code}\n" http://localhost:8080/wishlist/api/genres
curl -s -o /dev/null -w "calc %{http_code}\n" http://localhost:8080/
```
→ `web 200`、`genres 200`、`calc 200`(最後はダメ計の画面。同居しても動いている)。

夜間の更新(CronJob)を 1 回だけ手で動かす。商品が登録されていると、実サイトへ取得に行く(1 サイトにつき数回)。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n wishlist create job --from=cronjob/wishlist-refresher wishlist-refresher-manual
kubectl -n wishlist wait --for=condition=complete job/wishlist-refresher-manual --timeout=600s
kubectl -n wishlist logs job/wishlist-refresher-manual | tail -1
kubectl -n wishlist delete job wishlist-refresher-manual
```
→ `refresher: items=<商品数> failed=0 official=<監視数> official_failed=0`(商品が 0 件なら `items=0 failed=0 official=0 official_failed=0`)。

ジャンルの色と絵柄を確認する。

```sh
curl -s http://localhost:8080/wishlist/api/genres
```
→ 各ジャンルに `color`(`#` と 6 桁の小文字)と `icon`、`match_hosts`、`match_keywords` がある。

URL からの下書きを確認する(取得先の実サイトへ 1 回アクセスする)。

```sh
curl -s -X POST -H "Content-Type: application/json" -d '{"url":"https://dm.takaratomy.co.jp/card/detail?id=dm26rp3-077"}' http://localhost:8080/wishlist/api/items/from-url
curl -s http://localhost:8080/wishlist/api/genres
```
→ 1 つ目は `"genre_guessed":true`、`name` にサイト名と型番が付かない。`genre_id` が、2 つ目の「デュエル・マスターズ」の `id` と同じ。

Yahoo!ショッピングの行を確認する(商品を登録済みで、上の手動 Job のあと)。

```sh
curl -s "http://localhost:8080/wishlist/api/items/<商品の id>/estimates"
```
→ Yahoo!ショッピングの `site_id` の行が、`low` に価格を持つか `status` が `no_result`(`failed` ではない)。

## 6. ブラウザで確認する(Chrome と Safari)

`http://localhost:8080/wishlist/` を開く。開発者ツールの Network タブを開いておく(赤い行 = 失敗した通信)。

1. ページを開く → 設定画面を挟まずホームが出る。上に「欲しいもの」と件数、右に設定ボタン。ジャンルの絵柄のボタンが並び、先頭の「すべて」が青で塗られている。
2. 右下の「+」 → 下からシートが出て、「コピーした URL を追加」「URL を入力」「写真」が並ぶ。
3. 「写真」で手元の画像を選ぶ → 確認の画面が出る。「名前」に `グリス` を入れ、ジャンルの絵柄で `S.H.Figuarts` を選ぶ。「追加」が青くなる → 「追加」で、グリッドに画像が 1 枚出る(枠はジャンルの色。文字は出ない)。
4. 画像をタップ → 下から詳細シートが出る。ジャンル色の点と名前、検索ワード `S.H.Figuarts グリス` が出る。
5. 「メルカリ」のタイルをタップ → 新しいタブでメルカリの検索結果(`S.H.Figuarts グリス`)が開く。「Amazon」のタイルも同じ。
6. 数秒待つ → 価格の帯(`¥A` と `〜 ¥B`、サイトごとの点)か「出品ないかも」が出る。最安のタイルが青い縁で、出品なしのサイトは薄く「なし」と出る。取得しないサイトはマークだけ。
7. 「あやしい出品 N」が出ていれば押す → 小さな画像・価格・理由のタグ(名前が違う・安すぎ・下限より下)・サイトが並ぶ。「戻る」で詳細に戻る。
8. 「価格の推移」を開く → 初日は「推移はまだありません」。
9. 検索ワードをタップして変え、閉じる → 開き直すと元の検索ワードに戻っている(「保存」を押したときだけ保存される)。
10. 画像を長押し → 背景がぼけて、画像と「編集」「削除」が出る。元のページの URL がある商品には「元のページを開く」も出る。Esc か背景のタップで閉じる。
11. 長押し → 「編集」 → ジャンルの絵柄を押して替え、「保存」 → 枠の色が替わる。「公式ページを監視する」は、元ページの URL が無い商品では選べない。
12. 右上の設定ボタン → 「接続」の欄(トークン・APIのURL)が無い。「ジャンル」の各行に絵柄・色・商品の件数が出る。
13. 「S.H.Figuarts を編集」 → 色の見本(12 色)から別の色を押し、「ホームでの見え方」の枠が同じ色になる → 「保存」して「戻る」 → ホームのタイルの枠の色が替わる。
14. もう一度「S.H.Figuarts を編集」 → 「絵柄」「URL の手がかり」「別名グループ」に `S.H.Figuarts, SHフィギュアーツ` の行がある。
15. Network に赤い行(4xx・5xx)が無い。

URL からの登録:

1. 商品ページの URL をコピーしておく。「+」 → 「コピーした URL を追加」 → ブラウザが貼り付けの許可を聞いたら許可する。画像・名前・推定されたジャンルの絵柄(押された状態)が出る。
2. ジャンルが違っていれば絵柄を押して替え、「追加」 → グリッドに画像が増える。
3. 「+」 → 「URL を入力」 → URL を貼って「取得」 → 同じ確認の画面が出る。

オフライン:

1. 開発者ツールの Network で Offline にしてページを再読み込みする → 上に「オフライン」の札。グリッドと画像が出る(価格の札は出ない)。
2. 画像をタップ → 状態が「オフライン」。サイトのタイルは押せる。
3. Offline を戻す → 「オフライン」の札が消える。

確認用に登録した商品は、長押し → 「削除」 → 「削除する」で消す。

## 7. iPhone で PWA を確認する

Mac と iPhone に Tailscale を入れ、同じ tailnet にログインしておく。

```sh
cd "$(git rev-parse --show-toplevel)"
tailscale serve https / http://localhost:8080
tailscale serve status
```
→ Tailscale が発行した HTTPS の URL が `http://127.0.0.1:8080` へ転送されていると出る。この URL を控える(tailnet 固有の値なので、この手順書には書かない)。

1. iPhone の Safari で `<控えた URL>/wishlist/` を開く → ホームが出る(設定画面は挟まない)。
2. 共有 → 「ホーム画面に追加」 → ホーム画面に「欲しいもの」が出る。そこから開くとアドレスバーが無い。上と下の端が切れていない。
3. 「+」 → 「写真」で画像を選び、「名前」に `グリス`、ジャンルの絵柄で `S.H.Figuarts` を選んで「追加」 → グリッドに画像が出る。
4. 商品ページの URL をコピーして「+」 → 「コピーした URL を追加」 → 貼り付けの許可を聞かれたら許可 → 画像・名前・ジャンルが入った確認の画面が出る → 「追加」。
5. 画像をタップ → 「メルカリ」のタイルをタップ → メルカリの検索結果が開く(アプリが入っていればアプリ側)。「Amazon」のタイルも同じ。
6. 詳細シートに、数秒以内に価格の帯か「出品ないかも」が出る。
7. 機内モードにしてホーム画面から開く → 「オフライン」の札、一覧と画像が出る。サイトのタイルは押せる(遷移先はオンラインのときだけ)。
8. 機内モードを戻し、確認用の商品を長押し → 「削除」 → 「削除する」で消す。

公開をやめるとき:

```sh
cd "$(git rev-parse --show-toplevel)"
tailscale serve off
```
→ `tailscale serve status` に公開の表示が無い。

## 8. iOS アプリを確認する(実機)

iPhone に Tailscale の iOS アプリを入れ、Mac と同じ tailnet にログインしておく。接続先(API の URL)はビルド設定で渡し、アプリにトークンや URL の入力欄は無い(認証なし。フェーズ5-3)。

```sh
cd "$(git rev-parse --show-toplevel)"
tailscale serve https / http://localhost:8080
tailscale serve status
cp apps/wishlist/ios/Config/Local.xcconfig.example apps/wishlist/ios/Config/Local.xcconfig
open apps/wishlist/ios/Config/Local.xcconfig apps/wishlist/ios/Wishlist.xcodeproj
```
→ HTTPS の URL が `http://127.0.0.1:8080` へ転送されていると出る(この URL を控える)。`Local.xcconfig`(git に入らない)と Xcode が開く。

1. `Local.xcconfig` の `<ホスト>` を、控えた URL のホスト名(`https://` を除いた部分)に書き換え、末尾は `/wishlist/` のままにして保存する。
2. target `Wishlist` と `WishlistShare` の Signing & Capabilities で、Team に Personal Team を選ぶ(Bundle ID が衝突したら末尾を変える)。
3. iPhone をつないで Run → アプリが起動し、設定画面を挟まずホームに一覧が出る(上に「オフライン」の札は出ない)。初回は iPhone の 設定 → 一般 → VPN とデバイス管理 で開発元を信頼する。
4. ホームの設定(右上の歯車)を開く → 「接続」の欄が無く、ジャンルの行に絵柄・名前・件数が出る。ジャンルの行をタップ → 色(12 色)と絵柄を替えられる。
5. Safari で商品の公式ページを開き、共有 → 「欲しいもの」 → ページの画像と名前のプレビューが出て、推定したジャンルの絵柄が選ばれている。「追加」を押す → アプリのホームに画像が増える。
6. ホームの「+」→ 「コピーした URL を追加」(先に Safari で URL をコピーしておく)→ 確認の画面で「追加」。
7. 画像をタップ → 詳細シートが出る。価格の帯とサイトのタイルが並び、メルカリのタイルをタップ → メルカリ(アプリかブラウザ)の検索結果が開く。
8. 機内モードにしてアプリを開き直す → 一覧と画像が出て、上に「オフライン」の札が出る(価格の札は消える)。
9. 機内モードを戻し、確認用の商品を長押し → 背景がぼけてメニューが出る → 「削除」 → 「削除する」で消す。

署名は 7 日で切れる。切れたら Xcode から Run し直す。接続先を替えたら `Local.xcconfig` を直してビルドし直す。
