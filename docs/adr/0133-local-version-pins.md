# ADR-0133: ローカル環境の版を最新安定版で完全固定する(k3s・Node・Go・registry)と、ローカルの待ち受けを 127.0.0.1 に限定する

- 状態: 採用(2026-10-02。k3d.yaml の変更は既存クラスタに反映されない。作り直しは人間の確認が要る)
- レーン: データ(ADR 帯 `0100〜`。運用の D26)
- 関連: issue #242、#302(k3d.yaml 部分)、#332(Dockerfile 部分)、#403、DECISIONS.md 2026-09-22(最新安定版を固定する方針)

## 決定

1. `deploy/k3d.yaml` に `image: rancher/k3s:<版>@sha256:<digest>` を置く。
2. `deploy/k3d.yaml` の Ingress を `127.0.0.1:8080:80`、k8s API を `kubeAPI.host/hostIP: 127.0.0.1` に限定する(同じ LAN から届かせない。#302)。
3. `scripts/doctor.sh` は go・k3d・kubectl・helm が固定版より古ければ NG、Node は `web/.node-version` と違えば警告にする。
4. Dockerfile の Go は全サービスで `golang:1.27.1-alpine`(patch まで)+ digest。pokedex の Node は web と同じ 26.10.0。balance の registry は 3.1.2。

## 固定した版と根拠(確認日 2026-10-02)

| 対象 | 版 | digest | 取得 |
|---|---|---|---|
| k3s | v1.37.1-k3s1(2026-09-30、非 prerelease。以前の既定は v1.35.5) | `sha256:ca7f37d9…` | `gh api repos/k3s-io/k3s/releases`、`docker buildx imagetools inspect rancher/k3s:v1.37.1-k3s1` |
| Node(pokedex importer) | 26.10.0-alpine(2026-09-21) | `sha256:0b36e8c1…` | `curl https://nodejs.org/dist/index.json`、`docker buildx imagetools inspect node:26.10.0-alpine` |
| Go | 1.27.1-alpine(変更なし。balance・speed・judge の表記だけ `1.27` から patch 付きへ) | `sha256:8a5910f3…` | `docker buildx imagetools inspect golang:1.27.1-alpine`、go.dev/dl |
| registry | 3.1.2 | `sha256:ddf75434…` | `docker buildx imagetools inspect registry:3.1.2` |
| k3d | v5.9.0(最新と一致。doctor の下限) | — | `gh api repos/k3d-io/k3d/releases/latest` |

Traefik は k3s 同梱で k3s の版に従う(1.37 系で 3.7 系になる想定)。作り直したクラスタで Ingress の挙動(`make api-smoke`)を確認する。

## 結果・注意

- k3d.yaml の `image` の digest 付き指定と `kubeAPI` の 127.0.0.1 指定は、**実クラスタでの作成は未確認**(クラスタの作成・削除は人間の確認のため行っていない)。作り直すときに `make up` が通るか確かめ、通らなければ digest を外してタグ固定にする。
- 版を上げるときは k3d.yaml・各 Dockerfile・web/.node-version・`web/package.json` の engines・doctor の下限を同時に更新する。
