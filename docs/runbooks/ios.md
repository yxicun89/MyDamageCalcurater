# iOS の手順書(シミュレータ・モック)

前提: Xcode 27 と iOS 27 シミュレータ(iPhone 18 Pro)が入っている。ここでは常にモックで起動する(API 接続の確認は P6-4)。実機へのインストールは P6-4 の手順書で行う。構成の説明は ios/README.md。

## 1. テストを通す

```sh
cd "$(git rev-parse --show-toplevel)"
make ios-test | grep '^ios-'
```
確認: 出力が次の5行(`ios-gen-check: PokeCalcAPI/Generated は api/openapi.yaml と一致` / `ios-gen-check: PokeCalcSpeedAPI/Generated は services/speed/api/openapi.yaml と一致` / `ios-test-unit: 全 N 件 / 成功 N / 失敗 0 / スキップ 0 / 想定内の失敗 0` / `ios-test-ui: 全 N 件 / 成功 N / 失敗 0 / スキップ 0 / 想定内の失敗 0` / `ios-check-infoplist: … が入っている`)。

## 2. ルート画面を開く

```sh
cd "$(git rev-parse --show-toplevel)"
make ios-sim-run IOS_SCREEN=root
```
確認: 開いたスクリーンショット(`ios/build/screenshots/root-light-large.png`)に「モックデータで動作中」と「計算する」「逆算する」「構築」「素早さを比べる」のボタンがある。

## 3. 計算画面を開く

```sh
cd "$(git rev-parse --show-toplevel)"
make ios-sim-run IOS_SCREEN=calc
```
確認: 左右に「テストモンいち」「テストモンに」のカード、「A特化 / A振り / 無振り」の3つのボタン、技の行に「威力37 / 物理 / 等倍」、結果が「無振り」から順に並ぶ(上から4行が見え、5行目はスクロールで見える)。

## 4. 逆算画面を開く

```sh
cd "$(git rev-parse --show-toplevel)"
make ios-sim-run IOS_SCREEN=reverse
```
確認: 「与えたダメージ / 受けたダメージ」の切り替え、カード2枚、空の観測の行(単位 %)と「観測を追加」がある。

## 5. 構築画面を開く

```sh
cd "$(git rev-parse --show-toplevel)"
make ios-sim-run IOS_SCREEN=team
```
確認: 「まだ構築がありません。「新規作成」から始めましょう。」と「新規作成」がある(初回はチームが無いので一覧は空)。

## 6. 素早さ比較画面を開く

```sh
cd "$(git rev-parse --show-toplevel)"
make ios-sim-run IOS_SCREEN=speed
```
確認: 「自分のポケモン」(入力の方法・ポケモン「未選択」・調整)の下に「素早さの表」があり、「表の絞り込み」の6つのボタンと、「テスト」で始まる名前の段が速い順に並ぶ。
モックの挙動は `POKECALC_MOCK_SPEED=table-error|position-error|pokemon-error|all-error` で切り替える(ADR-0503 §8)。

## 7. ダークモードと大きい文字で崩れないことを見る

```sh
cd "$(git rev-parse --show-toplevel)"
make ios-sim-run IOS_SCREEN=calc IOS_APPEARANCE=dark IOS_CONTENT_SIZE=extra-extra-large
make ios-sim-run IOS_SCREEN=reverse IOS_APPEARANCE=dark IOS_CONTENT_SIZE=accessibility-large
make ios-sim-run IOS_SCREEN=team IOS_APPEARANCE=dark IOS_CONTENT_SIZE=extra-extra-large
make ios-sim-run IOS_SCREEN=speed IOS_APPEARANCE=dark IOS_CONTENT_SIZE=accessibility-large
```
確認: 背景が黒に近く文字が白い。文字が1字ずつ縦に折り返したり「…」で切れたりしていない(accessibility-large ではカードが縦に並ぶ)。

## 8. シミュレータを標準の表示に戻す

```sh
cd "$(git rev-parse --show-toplevel)"
make ios-sim-run IOS_SCREEN=root IOS_APPEARANCE=light IOS_CONTENT_SIZE=large
```
確認: スクリーンショットが白い背景・標準の文字サイズに戻っている。
