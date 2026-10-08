# ADR-0809: check-master-version は speed の版を /healthz(プロセスが読んだ版)から取る(issue #108 の素早さ分)

- 状態: 採用
- 日付: 2026-10-09
- レーン: 素早さ・運用
- 関連: ADR-0135(版の確認)、ADR-0138(balance・speed の loader と healthz の dataVersion)

## 背景

ADR-0138 で speed は metadata.json の dataVersion を保持し、起動ログと `/healthz`(契約内。任意フィールド)に出している。
speed に `/readyz` は無い(契約の健康確認は `/healthz` と `/api/speed/healthz`)ため、`/readyz` は足さない。
一方 `scripts/check-master-version.sh` は speed を Deployment 注釈(配備時の値)で比べていて、プロセスが読んだ版の証明になっていなかった。

## 決定

1. speed は `kubectl get --raw /api/v1/namespaces/<ns>/services/speed:http/proxy/healthz` の `dataVersion` を、期待値(export の metadata.json)と比べる(calc の `/readyz` と同じ方式)。
2. 取れない(届かない・dataVersion が無い=版不明)ときは Deployment 注釈へ**フォールバックせず**、`STALE speed` にする。
   注釈は「配備した版」で、フォールバックすると旧プロセスが残っていても成功扱いになりうるため。
3. balance は従来どおり注釈で比べる(タイプバランスレーンの持ち物。同レーンが healthz 参照へ切り替えるときに同じ方針を使える)。
4. 注釈の付与(`k3d-deploy-readmodel.sh`)は変えない(balance が使う)。speed の API・計算は変えない。

## 影響

- 版不明(metadata.json 無しの旧 export)の speed は `make check-master-version` で STALE になる。`make master-release` で export し直せば解消する。
- `scripts/check-master-version_test.sh` に、speed が `/healthz` を使い注釈を見ないことと、取れないときの STALE を足した。
