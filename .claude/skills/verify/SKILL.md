---
name: verify
description: 全テストとローカル環境の起動確認を行い、結果と人間向けの動作確認手順を報告する。
---
# /verify

1. `make doctor`
2. `make test` / `make test-golden` / `make test-all-species`
3. `make up` 後に `make e2e`
4. iOS がある場合は `make ios-test`
5. 結果を表でまとめる(成功/失敗/所要時間)
6. 人間が触って確かめる手順(URL、試す操作、期待する結果)を5〜10項目で書く

失敗があっても修正はしない。報告のみ。修正は `/phase` か `/improve` で行う。
