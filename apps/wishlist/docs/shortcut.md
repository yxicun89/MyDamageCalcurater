# iOS ショートカットで登録する(ネイティブ版ができるまでのつなぎ)

Safari などで商品ページを開き、共有シートから「欲しいものに追加」を選ぶと、
`POST /api/items/from-url` で下書き(名前・画像)を取り、ジャンルを選んで `POST /api/items` で確定する。
仕様は [../CLAUDE.md](../CLAUDE.md) §9、API は [../api/openapi.yaml](../api/openapi.yaml)。

## 前提

- iPhone に Tailscale アプリを入れ、Mac と同じ tailnet に入っている
- Mac 側で wishlist がクラスタに入っており、`https://<Mac の tailnet 名>/wishlist/` が開ける
- API トークンを持っている(`apps/wishlist/scripts/bootstrap.sh` の最後に表示される。再表示は同スクリプトの案内どおり)

以下、`<BASE>` は `https://<Mac の tailnet 名>/wishlist`(末尾の `/` なし)、`<TOKEN>` は API トークン。

## 作り方

ショートカット App → 「+」で新規作成し、名前を「欲しいものに追加」にする。上から順にアクションを足す。

1. **ショートカットの詳細**(下部の ⓘ)で「共有シートに表示」をオンにし、受け付ける入力を「URL」だけにする
2. **テキスト**:`<TOKEN>` を入れ、変数名を `token` にする(以降の「ヘッダ」で使う)
3. **URL の内容を取得**(ジャンル一覧)
   - URL:`<BASE>/api/genres`、方法:GET
   - ヘッダ:`Authorization` = `Bearer ` + `token`(「Bearer」と半角空白のあとに変数を挿入)
4. **辞書の値を取得**:キー `genres`(3 の結果から)
5. **リストから選択**:4 の結果。プロンプト「ジャンル」。項目の表示は各要素の `name`
   (選んだ要素から **辞書の値を取得** でキー `id` を取り、変数 `genre_id` にする)
6. **URL の内容を取得**(下書き)
   - URL:`<BASE>/api/items/from-url`、方法:POST、本文を要求:JSON
   - ヘッダ:`Authorization` = `Bearer ` + `token`
   - JSON:`url` = ショートカットの入力、`genre_id` = `genre_id`(数値)
7. **辞書の値を取得**:キー `name`(6 の結果から)。**入力を要求**(テキスト、初期値にこの値)で名前を確認・修正し、変数 `name` にする
8. **辞書の値を取得**:キー `image_url`(6 の結果から)。変数 `image_url`
   - 画像が取れないページ(`image_url` が空)は、**もしも** で「画像が見つかりません」と通知して終了する
     (写真で登録するときはアプリ側の「+」から)
9. **URL の内容を取得**(確定)
   - URL:`<BASE>/api/items`、方法:POST、本文を要求:JSON
   - ヘッダ:`Authorization` = `Bearer ` + `token`
   - JSON:`genre_id` = `genre_id`(数値)、`name` = `name`、`image_url` = `image_url`、`source_url` = ショートカットの入力
10. **通知を表示**:「登録しました」

## 動作確認

1. Safari で商品ページ(S.H.Figuarts の公式ページなど)を開き、共有 → 「欲しいものに追加」
2. ジャンルを選び、名前を確認して完了 → 「登録しました」
3. PWA(`<BASE>/`)のホームに画像が出る。タップして詳細シートのメルカリ・Amazon の行から検索結果が開く

## うまくいかないとき

| 症状 | 確かめること |
|---|---|
| 401 | ヘッダが `Bearer <TOKEN>`(Bearer の後ろに半角空白)になっているか |
| 422 | from-url:URL が http(s) か。確定:`genre_id` が数値か、名前が空でないか |
| 502 | 商品ページ・画像の取得に失敗した(ページ側の制限など)。アプリの「+」から写真で登録する |
| つながらない | iPhone の Tailscale がオンか、Mac のクラスタが動いているか |

トークンはショートカットの中に平文で入る。ショートカットを他人に共有しない(共有するときはテキストのアクションを空にしてから)。
