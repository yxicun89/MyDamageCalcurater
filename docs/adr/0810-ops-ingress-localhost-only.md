# ADR-0810: k3d の Ingress(8080)を 127.0.0.1 だけで待ち受ける(#302 の残り)

- 状態: 採用
- 日付: 2026-10-11
- レーン: 運用(提案は Wishlist レーン)
- 関連: ADR-0133(k8s API の 127.0.0.1 限定)、ADR-0210(private の境界)、docs/ai-shared/decisions/2026-10-11-wishlist-ingress-bind.md

## 背景

`deploy/k3d.yaml` の `8080:80` はホストの全インターフェースで待ち受けていた。同じ Wi-Fi にいる他人の端末から、ダメ計の API と
欲しいものリストに届く。欲しいものリストはこのために固定トークンを要求していたが、ユーザーから「トークンは何それになる」と感想があった。
ダメ計の実機の手順はすでに Tailscale 経由だけになっており(docs/runbooks/ios-device-install.md)、LAN の IP でつなぐ手順は残っていない。

## 決定

1. `deploy/k3d.yaml` を `127.0.0.1:8080:80` にする。iPhone からは `tailscale serve`(Mac の 127.0.0.1:8080 へ転送)を通す(ユーザー決定 2026-10-11)。
2. 作成済みのクラスタは作り直さず、ロードバランサの待ち受けだけを替える(DB の中身は消えない):

   ```sh
   k3d cluster edit pokecalc --port-delete 8080:80@loadbalancer
   k3d cluster edit pokecalc --port-add 127.0.0.1:8080:80@loadbalancer
   ```

## 影響

- 同じ Wi-Fi の別の端末から `http://<Mac の IP>:8080` では開けなくなる。開くときは Tailscale の URL を使う。
- 欲しいものリストは、この前提でトークンと接続の画面を無くせる(apps/wishlist で別に扱う)。
- gateway の `/metrics` が Ingress 経由で読める点(#216)は変わらない。LAN から届かなくなるだけ。
