# ADR-0403: pokedex export の read model を k3d の balance に読ませる

- 状態: 採用(2026-09-22。データレーンの依頼。タイプバランスレーンの判断)
- 日付: 2026-09-22
- 関連: ADR-0012(実行時のサービス間依存を足さない)、ADR-0014 §2・ADR-0401 §5(read model)、ADR-0402(JSON Schema)、
  ADR-0105(pokedex export。データレーン)、ADR-0002(実データを Git に置かない)

## 決定
1. pokedex export の出力(`data/generated/readmodel/` の `pokemon-types.json`・`moves.json`・`abilities.json`。Git 管理外)を、
   ConfigMap `balance-readmodel` にして balance にマウントする。ConfigMap は Git に置かず、`make balance-k3d-deploy-readmodel` が作る
   (Kustomize の load restrictor で overlay の外のファイルを参照できず、実データは Git に入れられないため)。
2. overlay `deploy/k8s/overlays/local-readmodel` は ConfigMap をマウントし、3つの `BALANCE_*_PATH` を設定する。
   Pod テンプレートに内容の hash の annotation を入れ、中身が変わったら Pod を作り直す。
3. デプロイの前に `cmd/checkreadmodel` が、サービスと同じ loader で3つのファイルを検証する(不正ならデプロイしない)。
   ConfigMap の上限(1 MiB)を超える大きさなら止める。
4. `make balance-smoke-readmodel` は、read model の先頭のポケモンで analyze と recommendations が 200 になることを確かめる。
5. 特性の候補(`abilityIds`)の上限は 4(ADR-0401 §5 を変更)。実データに特性スロットが4つの種族があるため。

## 影響
- 架空データの local overlay(`make balance-k3d-deploy`)と GitOps の overlay はそのまま。どれを最後に適用したかで k3d の balance の中身が決まる。
- クラウドでの実データの配布(Secret / ボリューム / イメージへの同梱など)は、デプロイ先が決まったときに別に決める。
- 無効・吸収の特性(例: 特定のタイプを受けない特性)は、今の export に出ない(データレーンの効果定義に無い)。当面は倍率を変える特性だけが反映される。

## 却下した案
- balance が起動時に pokedex-svc の API から取る: ADR-0012 に反する(実行時の依存)。
- 実データを overlay のディレクトリに置く: Git 管理下になりうる(ADR-0002)。
