# speed の手順書(ローカル k3d)

前提: k3d の `pokecalc` クラスタが起動している(`make up`)。API は `services/speed/api/openapi.yaml`、設計は ADR-0600・ADR-0601。

## 1. テストと静的検査を通す

```sh
cd "$(git rev-parse --show-toplevel)"
make test lint build check-publishable
```
確認: 最後の行が `check-publishable: 0 件` で、エラーで止まらない。

## 2. local overlay(架空データの read model)で k3d にデプロイして疎通を確かめる

```sh
cd "$(git rev-parse --show-toplevel)"
make speed-k3d-deploy
SPEED_URL=http://localhost:8080 make speed-smoke
```
確認: 最後の行が `speed smoke: health=200 pokemon=200 (count=8) missing_headers=400 table=200 (tiers=7, tie 219) invalid_presets=400 position=200 (146, 29/2/17) invalid_position=400 unknown_pokemon=422`
(1回目がロールアウト直後で失敗したら、`SPEED_URL=http://localhost:8080 make speed-smoke` をもう一度)。

GitOps(Argo CD)は SP4 で足す(ADR-0600 §2)。それまでの動作確認は 1〜2 で完結する。
