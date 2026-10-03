# ADR-0135: マスタ更新の反映手順と、動いている dataVersion の確認(calc の readyz・起動ログ、版一致スクリプト)

- 状態: 採用
- 日付: 2026-10-03
- レーン: データ(運用 deploy・scripts と連携)
- 関連: issue #281、issue #108、ADR-0128(dataVersion の checksum 化と export の metadata.json)、
  ADR-0204 §3(calc は起動時1回だけ取得。反映は再起動)、ADR-0403・ADR-0603(balance・speed の read model)

## 背景

ADR-0128 で dataVersion は内容が変われば変わる値になり、export には `metadata.json` が入った。残りは次のとおり。

- calc-svc は取得した dataVersion をログにも `/readyz` にも出さず、動いている版を確かめられない。
- `docs/runbooks/data.md` は投入と行数の確認で終わり、calc・balance・speed への反映手順と版の確認が無い。
- 版の一致を機械的に確かめる手段が無く、旧版の consumer が残っても成功に見える。

## 決定

1. calc-svc は、マスタの読み込みに成功した時点で `slog.Info("calc-svc: マスタを読み込んだ", "dataVersion", ...)` を出し、
   マスタ読み込み済みの `GET /readyz` の本文を `{"status":"ok","dataVersion":"<版>"}` にする。読み込み前の 503 は従来どおり。
   dataVersion は公開データの版(取得元の版と checksum 先頭8桁)だけなので、運用エンドポイントに出しても漏えいにならない。
   実装は `services/calc/cmd/calc` の薄いラッパ(`withReadyzDataVersion`)に置く(`internal/master`・`internal/httpapi` は変えない)。
   `/readyz` は openapi に載せない運用エンドポイントなので、`api/openapi.yaml` は変えない。
2. balance・speed の配備スクリプト(`scripts/gitops/k3d-deploy-readmodel.sh`)は、export の `metadata.json` の dataVersion を
   Deployment の注釈 `pokecalc.example/data-version` に残す(pod template ではないので追加の rollout は起きない)。
   balance・speed のサービス本体が dataVersion を読んで表示する変更(loader・schema)は、各レーンの持ち物なので本 ADR では行わない。
   版の確認は配備時に付けた注釈で代える(「配備した read model の版」であり、プロセスが読んだ版の直接の証明ではない)。
3. `scripts/check-master-version.sh`(`make check-master-version`)は、`metadata.json` の dataVersion を期待値にして、
   calc の `/readyz`(API サーバのサービスプロキシ経由)・balance・speed の Deployment 注釈と比べる。クラスタは読み取りだけ。
   1つでも違う・取れないときは `STALE <名前>` を表示して終了コード 1。k3d の context でなければ止まる。
   テストは `scripts/check-master-version_test.sh`(偽の kubectl。`make test-scripts`)。
4. 手順は `docs/runbooks/data.md` §5a に書く: `make pokedex-export` → `make deploy-latest` → `make check-master-version`。
   自動の再取得・hot reload は入れない(ADR-0204 §3 を維持)。importer Pod に Kubernetes API の権限は与えない
   (orchestration は利用者側)。

## 追記: 単一ターゲット `make master-release`(issue #108・#403 D21)

`scripts/master-release.sh`(`make master-release`)が、投入の完了待機から smoke までを上から順に1回で行う。

1. `require-k3d-context.sh` の後、CronJob `pokedex-import` の ownerReference を持つ Job のうち最新のものの完了を待つ。
   Job は**作らない**(無ければ先に投入を流すよう案内して非0。失敗済みの Job も非0)。
2. `make pokedex-export` と、balance・speed の `checkreadmodel` で read model を検証する。DSN は `POKEDEX_DATABASE_DSN` があればそれ、
   無ければ k3d-deploy-latest.sh と同じく mysql へ一時 port-forward して Secret の `pokedex-reader-dsn`(SELECT のみ)の接続先を付け替える。値は表示しない。
3. `check-master-version.sh` の比較を再利用し、export の dataVersion と calc・balance・speed が一致していれば「変化なし」で終了コード0
   (再生成・rollout・smoke をしない)。
4. 違えば balance・speed を `*-k3d-deploy-readmodel` で入れ替え(Argo CD 管理のものは k3d-deploy-latest.sh と同じく飛ばして案内)、
   calc は `rollout restart` → `rollout status` → `/readyz` 待ち(起動時にマスタを取り直す。ADR-0204 §3)。
5. 版一致 → `API_SMOKE_STRICT=1 make api-smoke` → balance・speed の readmodel smoke。
- どの段の失敗も非0で、export 済みなら `STALE <consumer>` を表示する(公開データの版だけ。秘密・第三者データなし)。
- importer Pod に Kubernetes API の書き込み権限は与えない。orchestration は手元の make から行う。
- テストは `scripts/master-release_test.sh`(偽の kubectl・make・go。`make test-scripts`)。段の順序、変化なしで rollout しないこと、
  Job 無し/失敗の案内、途中失敗で非0と旧版表示、DSN が出力に出ないことを確かめる。
- 残り(各レーンの持ち物): balance・speed の loader・schema が dataVersion を保持して運用エンドポイントに出すこと
  (それまでの版の確認は Deployment の注釈。上記「配備した版」の限界は変わらない)。

## 影響・残り

- 版の確認は calc だけが「プロセスが読んだ版」、balance・speed は「配備した版」。#108 の受け入れ条件のうち、
  balance・speed の loader が版を保持して表示することは、各レーンの仕事として残る。投入の完了待機から smoke までの
  1ターゲットと、版が同じなら再生成・rollout をしないことは、上の追記の `make master-release` で満たす。
- `/readyz` の本文が増えるが、probe は status code だけを見るので影響しない。
