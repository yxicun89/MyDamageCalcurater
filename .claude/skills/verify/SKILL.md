---
name: verify
description: 全テストとローカル環境の起動確認を行い、結果と人間向けの動作確認手順を報告する。
---
# /verify

手順の正は `docs/verify-m1.md`。ここではその §2(自動テスト)と §5(k3d の自動の動作確認)を、同じコマンドで流す。

1. `make doctor`
2. 自動テスト(verify-m1 §2。クラスタ不要): `make test` / `make lint` / `make build` / `make test-golden` / `make test-all-species` /
   `make test-wasm` / `make web-test-wasm` / `make e2e`(Playwright 3件。k3d-pokecalc があれば k3d のスモーク・Playwright も)
3. k3d(verify-m1 §5): `kubectl config current-context` が `k3d-pokecalc` のときだけ `make web-k3d-smoke` / `make api-smoke` / `make web-k3d-e2e`。
   クラスタが無いときは `make up` を勝手に実行せず、この3件を「未実施(クラスタ無し)」として表に書く
4. iOS がある場合は `make ios-test`
5. 結果を表でまとめる(成功/失敗/未実施/所要時間)。終了コードが 0 以外のものは失敗。未実装のターゲット(`make assets` など。終了コード 2)は
   「未実施」で、成功に数えない
6. 人間が触って確かめる手順(URL、試す操作、期待する結果)を5〜10項目で書く。verify-m1 §6 を元にする

失敗があっても修正はしない。報告のみ。修正は `/phase` か `/improve` で行う。
