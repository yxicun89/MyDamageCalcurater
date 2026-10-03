# 動作確認の通し手順(M1〜M4)

マイルストーンの順に、各手順書を上から実行する。手順の中身はここに書かない(各手順書が正)。

## k3d の前提(初回と、コードを更新したとき)

初回は [verify-m1.md](verify-m1.md) §3(`make up` → マスタ投入 → `make pokedex-export-k3d`)を実行し、そのあと次を実行する。

```sh
cd "$(git rev-parse --show-toplevel)"
make deploy-latest
make pokedex-export-k3d
```
→ `deploy-latest: 全サービスを <コミット> の内容で入れ替えた` と、サービスの READY が 1。`export: ../data/generated/readmodel に書いた`。

## 順序と完了判定

| 順 | 手順書 | 緑の条件(これが揃えば OK) |
|---|---|---|
| 1 | [verify-m1.md](verify-m1.md)(ブラウザで計算) | §2 の自動テストが全部成功。§5 の `web-k3d-smoke`・`api-smoke` が成功。§6 の目視に赤い通信が無い。`make web-k3d-e2e` は record 未デプロイの間 `503 /api/record/frequent-opponents` で 2 件失敗する(既知) |
| 2 | [verify-m2.md](verify-m2.md)(保存・構築) | §1 の自動テストが成功。§2 は record・team・TiDB・NATS が k3d に入ってから(入るまでは未実施。API レーンの P5-3b) |
| 3 | [verify-m3.md](verify-m3.md)(iOS) | `swift test` 失敗 0・`make ios-gen-check` 成功・`xcodebuild build` が `BUILD SUCCEEDED`。シミュレータ・実機の目視と `make ios-test` は人間の作業 |
| 4 | [verify-m4.md](verify-m4.md)(運用) | `make test-scripts` 成功・`promtool check rules` が calc・balance とも SUCCESS。実クラスタの ServiceMonitor が UP・ダッシュボード表示・GitOps は人間の作業を含む |

## 他レーンの手順書

[runbooks/balance.md](runbooks/balance.md)・[runbooks/speed.md](runbooks/speed.md)・[runbooks/observability.md](runbooks/observability.md)・[runbooks/ios-device-install.md](runbooks/ios-device-install.md)。
項目と実装の対応は [impl/verify-mapping.md](impl/verify-mapping.md)。
