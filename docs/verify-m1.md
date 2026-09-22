# M1 の動作確認(ドラフト)

上から順に実行する。各コマンドの下の「→」が成功の見え方。

## 1. 準備(初回だけ)

```sh
cd "$(git rev-parse --show-toplevel)"
make doctor
make web-install
make web-e2e-install
```
→ doctor が不足ツールを報告しない。

## 2. 自動テスト

```sh
cd "$(git rev-parse --show-toplevel)"
make test && make lint && make build
make test-golden
make test-wasm
make web-test-wasm
make web-e2e
make web-e2e-container
make web-e2e-online
make web-e2e-balance
```
→ すべて最後まで成功する(失敗したらそこで止めて plan.md に記録)。

## 3. k3d(コンテナ)で起動

```sh
cd "$(git rev-parse --show-toplevel)"
make up
make api-k3d-deploy
make web-k3d-deploy
kubectl -n pokecalc get pods
```
→ `calc`・`gateway`・`web` の Pod が `Running`。

```sh
cd "$(git rev-parse --show-toplevel)"
make web-k3d-smoke
make api-smoke
```
→ どちらも `すべて成功`(`api-smoke` の出力に `web=200` が出る。gateway が `/` 等を Web に転送する。ADR-0205)。

## 4. 画面の確認(Chrome と Safari の両方で)

`http://localhost:8080` を開き、開発者ツールの Network タブを開いておく(gateway 経由。API と画面がこの1つの URL で揃う)。

1. ポケモンを攻撃側・防御側とも選ぶ → 結果が5行出る。
2. 自分の調整を「A特化」にする → 各行の%が上がる。
3. 「持ち物の候補も比較」をオンにする → 行が増え、各行に持ち物の名前が出る。
4. 「攻守入れ替え」を押す → 左右が入れ替わり、結果が変わる。
5. 「逆算」タブを押す → URL が `/reverse` になる。
6. 自分・相手を選び、観測1に計算タブの「H振り」の行の範囲の整数%を入れる → 「H32 を仮定」と候補が出る。
7. 「観測を追加」で同じ値を入れる → 「近い候補」でない候補の数が増えない。
8. 「タイプバランス」タブを押す → URL が `/balance` になる。メンバーを2体選ぶ → 防御相性の表(18タイプ)とチームの集計が出る。
9. メンバーに技を選ぶ → 攻撃範囲の表が出る。
10. ブラウザの戻るを押す → 前のタブに戻る。
11. `http://localhost:8080/reverse` を直接開く → 逆算タブが選ばれている。
12. タブで ← → キーを押す → タブが切り替わる。

## 5. 結果の記録

確かめた日付・ブラウザ・うまくいかなかった番号を、docs/plan.md の P4-5 の行に書く(または AI に伝える)。

## 未完了(完成版で手順に加える)

- 実マスタでの計算・オンライン(API)での画面確認: pokedex-svc(P2-3)と Web の MasterSource 待ち
