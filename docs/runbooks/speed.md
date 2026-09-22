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

## 3. pokedex export の read model で確かめる(export があるときだけ)

データレーンの pokedex export(`make pokedex-export`。`POKEDEX_DATABASE_DSN` が必要)で
`data/generated/readmodel/speed-pokemon.json` ができてから実行する。DB を用意する手順は [`data.md`](data.md) を見る。

```sh
cd "$(git rev-parse --show-toplevel)"
make speed-k3d-deploy-readmodel
SPEED_URL=http://localhost:8080 make speed-smoke-readmodel
```
確認: 最後の行が `speed readmodel smoke: pokemon=<ID> list=200 table=200`。
ファイルが無い・不正なときは `missing ...` や `read model check failed` で止まり、デプロイされない(ADR-0603 §3)。

架空データに戻すときは 2 をもう一度実行する(あとから適用したほうが k3d の speed の中身になる)。

GitOps(Argo CD)は、イメージの digest が決まる段階で別に作る(ADR-0603 影響)。
