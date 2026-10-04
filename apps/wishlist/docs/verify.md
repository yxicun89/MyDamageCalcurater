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
→ 初回は `DB 'wishlist' と Secret 'wishlist/wishlist-api' を作りました。` の後に API トークンが 1 行出る(控えなくてよい。後で取り出せる)。
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
TOKEN=$(kubectl -n wishlist get secret wishlist-api -o jsonpath='{.data.api-token}' | base64 -d)
curl -s -o /dev/null -w "web %{http_code}\n" http://localhost:8080/wishlist/
curl -s -o /dev/null -w "noauth %{http_code}\n" http://localhost:8080/wishlist/api/genres
curl -s -o /dev/null -w "genres %{http_code}\n" -H "Authorization: Bearer $TOKEN" http://localhost:8080/wishlist/api/genres
curl -s -o /dev/null -w "calc %{http_code}\n" http://localhost:8080/
```
→ `web 200`、`noauth 401`、`genres 200`、`calc 200`(最後はダメ計の画面。同居しても動いている)。

夜間の更新(CronJob)を 1 回だけ手で動かす。商品が登録されていると、実サイトへ取得に行く(1 サイトにつき数回)。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n wishlist create job --from=cronjob/wishlist-refresher wishlist-refresher-manual
kubectl -n wishlist wait --for=condition=complete job/wishlist-refresher-manual --timeout=600s
kubectl -n wishlist logs job/wishlist-refresher-manual | tail -1
kubectl -n wishlist delete job wishlist-refresher-manual
```
→ `refresher: items=<商品数> failed=0 official=<監視数> official_failed=0`(商品が 0 件なら `items=0 failed=0 official=0 official_failed=0`)。

## 6. ブラウザで確認する(Chrome と Safari)

`http://localhost:8080/wishlist/` を開く。開発者ツールの Network タブを開いておく(赤い行 = 失敗した通信)。

トークンを表示する(画面に入れるため)。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n wishlist get secret wishlist-api -o jsonpath='{.data.api-token}' | base64 -d; echo
```
→ 16 進の文字列が 1 行出る。

1. 初回は設定画面が開く。「APIのURL」は空のまま、「トークン」に上の値を入れて「保存」→「戻る」 → ホームに戻る(画像のグリッドは空、上にジャンルのチップ)。
2. 右下の「+」 → 「写真」で手元の画像を選び、「名前」に `グリス`、「ジャンル」に `S.H.Figuarts` を選んで「登録」 → グリッドに画像が 1 枚出る(文字は出ない)。
3. 画像をタップ → 下から詳細シートが出る。検索ワードが `S.H.Figuarts グリス`。
4. 「メルカリ」の行をタップ → 新しいタブでメルカリの検索結果(`S.H.Figuarts グリス`)が開く。「Amazon」の行も同じ。
5. 詳細シートのサマリが数秒以内に「だいたい ¥A〜¥B で買えそう(M/D 時点)」か「出品ないかも」になる。サイト行に、取得したサイトの目安・件数・在庫が出る(取得しないサイトはリンクだけ)。
6. 「参考外 N件」が出ていれば開く → 商品名が一致しない・安すぎる出品と、その理由が並ぶ。
7. 「価格の推移」を開く → 初日は「推移はまだありません」。
8. 検索ワードをタップして変え、閉じる → 開き直すと元の検索ワードに戻っている(「保存」を押したときだけ保存される)。
9. 画像を長押し → 「編集」「削除」が出る。「編集」で「公式ページを監視する」は、元ページの URL が無い商品では選べない。
10. 「設定」ボタン → ジャンルの「S.H.Figuarts を編集」 → 「別名グループ」に `S.H.Figuarts, SHフィギュアーツ` の行がある。
11. Network に赤い行(4xx・5xx)が無い。

URL からの登録:

1. 「+」 → 「URL」に商品ページの URL を入れて「取得」 → 「名前」に商品ページのタイトルが入る。
2. 「ジャンル」を選んで「登録」 → グリッドに画像が増える。

オフライン:

1. 開発者ツールの Network で Offline にしてページを再読み込みする → グリッドと画像が出る。
2. 画像をタップ → サマリが「オフライン」。サイト行のリンクは出ている。
3. Offline を戻す。

確認用に登録した商品は、長押し → 「削除」 → 「削除する」で消す。

## 7. iPhone で PWA を確認する

Mac と iPhone に Tailscale を入れ、同じ tailnet にログインしておく。

```sh
cd "$(git rev-parse --show-toplevel)"
tailscale serve https / http://localhost:8080
tailscale serve status
```
→ Tailscale が発行した HTTPS の URL が `http://127.0.0.1:8080` へ転送されていると出る。この URL を控える(tailnet 固有の値なので、この手順書には書かない)。

トークンを表示する(iPhone に入れるため)。

```sh
cd "$(git rev-parse --show-toplevel)"
kubectl -n wishlist get secret wishlist-api -o jsonpath='{.data.api-token}' | base64 -d; echo
```
→ 16 進の文字列が 1 行出る。

1. iPhone の Safari で `<控えた URL>/wishlist/` を開く → 設定画面が出る。
2. 「トークン」に上の値を入れて「保存」→「戻る」 → ホームが出る。
3. 共有 → 「ホーム画面に追加」 → ホーム画面に「欲しいもの」が出る。そこから開くとアドレスバーが無い。
4. 「+」 → 「写真」で画像を選び、「名前」に `グリス`、「ジャンル」に `S.H.Figuarts` を選んで「登録」 → グリッドに画像が出る。
5. 画像をタップ → 「メルカリ」の行をタップ → メルカリの検索結果が開く(アプリが入っていればアプリ側)。「Amazon」の行も同じ。
6. 詳細シートのサマリが数秒以内に「だいたい ¥A〜¥B で買えそう(M/D 時点)」か「出品ないかも」になる。
7. 機内モードにしてホーム画面から開く → 一覧と画像が出る。サイト行のリンクは出ている(遷移先はオンラインのときだけ)。
8. 機内モードを戻し、確認用の商品を長押し → 「削除」 → 「削除する」で消す。

公開をやめるとき:

```sh
cd "$(git rev-parse --show-toplevel)"
tailscale serve off
```
→ `tailscale serve status` に公開の表示が無い。

## 8. iOS アプリを確認する(実機)

iPhone に Tailscale の iOS アプリを入れ、Mac と同じ tailnet にログインしておく。

```sh
cd "$(git rev-parse --show-toplevel)"
tailscale serve https / http://localhost:8080
tailscale serve status
open apps/wishlist/ios/Wishlist.xcodeproj
```
→ HTTPS の URL が `http://127.0.0.1:8080` へ転送されていると出る(この URL を控える)。Xcode が開く。

1. target `Wishlist` と `WishlistShare` の Signing & Capabilities で、Team に Personal Team を選ぶ(Bundle ID が衝突したら末尾を変える)。
2. iPhone をつないで Run → アプリが起動する。初回は iPhone の 設定 → 一般 → VPN とデバイス管理 で開発元を信頼する。
3. アプリの設定画面に、API のベース URL `<控えた URL>/wishlist/` と、トークン(Mac で `kubectl -n wishlist get secret wishlist-api -o jsonpath='{.data.api-token}' | base64 -d` の出力)を入れて保存 → ホームに一覧が出る。
4. Safari で商品の公式ページを開き、共有 → 「欲しいもの」 → 拡張の設定画面で同じ URL とトークンを入れて保存 → プレビュー(名前と画像)が出る。ジャンルを選んで保存 → アプリのホームに画像が増える。
5. 画像をタップ → 詳細シートが出る。メルカリの行をタップ → メルカリ(アプリかブラウザ)の検索結果が開く。
6. 機内モードにしてアプリを開き直す → 一覧と画像が出る。詳細シートのサマリが「オフライン」。
7. 機内モードを戻し、確認用の商品を長押し → 「削除」 → 「削除する」で消す。

署名は 7 日で切れる。切れたら Xcode から Run し直す。
