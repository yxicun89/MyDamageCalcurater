# ADR-0603: pokedex export の read model を k3d の speed に読ませる

- 状態: 採用(2026-09-22。データレーンの依頼。素早さレーンの判断)
- 日付: 2026-09-22
- 関連: ADR-0012(実行時のサービス間依存を足さない)、ADR-0600 §4(read model の形)、
  ADR-0105 §120(pokedex export の `speed-pokemon.json`。データレーン)、ADR-0002(実データを Git に置かない)、
  ADR-0403(balance の同種の配線。同じ形にそろえる)

## 決定
1. pokedex export の出力(`data/generated/readmodel/speed-pokemon.json`。Git 管理外)を、ConfigMap `speed-readmodel` にして
   speed にマウントする。ConfigMap は Git に置かず、`make speed-k3d-deploy-readmodel` が作る(Kustomize の load restrictor で
   overlay の外のファイルを参照できず、実データは Git に入れられないため。balance の ADR-0403 §1 と同じ理由)。
2. overlay `deploy/k8s/overlays/local-readmodel` は ConfigMap をマウントし、`SPEED_POKEMON_PATH` を設定する。
   Pod テンプレートに内容の hash の annotation を入れ、中身が変わったら Pod を作り直す(balance と同じ形)。
3. デプロイの前に `cmd/checkreadmodel` が、サービスと同じ loader(`master.LoadPokemonFile`)でファイルを検証する
   (不正ならデプロイしない)。ConfigMap の上限(1 MiB)を超える大きさなら止める。`kubectl apply` は `--server-side` を使う
   (client-side apply の `last-applied-configuration` annotation の 256 KiB 上限を避けるため)。
4. `make speed-smoke-readmodel` は、read model の先頭のポケモンで `/api/speed/v1/pokemon`(該当ポケモンが含まれる)と
   `/api/speed/v1/table`(200)が通ることを確かめる。
5. read model の形は ADR-0600 §4 のまま変更しない(pokedex export はこの形に合わせて出力される。ADR-0105 §120)。
   コアの `PokemonProvider` インターフェースは変更不要(adapter の差し替えは無い。同じ `master.LoadPokemonFile` を使う)。

## 影響
- 架空データの local overlay(`make speed-k3d-deploy`)はそのまま残す(pokedex-svc の実データ投入が要らない開発ループ用)。
  どちらを最後に適用したかで k3d の speed の中身が決まる(balance と同じ運用)。
- GitOps の overlay と Argo CD Application は、この ADR の対象外(ADR-0600 §2 のとおり、イメージの digest が決まる段階で別途作る)。
- クラウドでの実データの配布は、デプロイ先が決まったときに別に決める。
- 空の roster(0 件)の扱いは、pokedex export が「1 件以上」を返す前提(ADR-0105)であればこの ADR の対象外(SP1 critic の軽微。
  実データで 0 件になる状況が起きたら別途決める)。

## 却下した案
- speed が起動時に pokedex-svc の API から取る: ADR-0012 に反する(実行時の依存)。
- 実データを overlay のディレクトリに置く: Git 管理下になりうる(ADR-0002)。
