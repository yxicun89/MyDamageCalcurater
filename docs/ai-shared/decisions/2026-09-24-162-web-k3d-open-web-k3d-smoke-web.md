## 2026-09-24: 動作確認手順の抜け(web-k3d-open)と web-k3d-smoke の案内改善の提案(データレーン → Web レーン)
Lane: データ(docs 作業。専用ブランチ `docs/impl-guide`)→ Web レーンへの提案
Question: `make web-k3d-smoke` が `localhost:5173` に届かず失敗した(ユーザーの動作確認)。原因は `docs/verify-m1.md` §3 に、5173 の port-forward を張る `make web-k3d-open` の手順が無かったこと。`web/scripts/k3d-smoke.sh` の失敗時の案内も追加してよいか。
Default: `docs/verify-m1.md` は本ブランチで修正済み(`web-k3d-open` を別ターミナルで先に実行する手順・8080 で確認する場合の注意を追記)。`web/scripts/k3d-smoke.sh` は Web レーンの範囲なので触らない。Web レーンに、接続失敗時に「`make web-k3d-open` を別ターミナルで実行するか、WEB_URL を指定する」という案内を出すことを提案する。
Status: 既定案で進行・Web レーンの回答待ち(ユーザー承認済みは verify-m1.md の修正のみ)。
Reason: k3d の 5173 は port-forward の宛先で、cluster の公開ポートではない(公開は 8080 → Traefik → gateway。`deploy/k3d.yaml`)。
Impact: `docs/verify-m1.md` §3。`web/scripts/k3d-smoke.sh` は未変更。
