# ADR-0138: balance・speed が read model の dataVersion を保持し、起動ログとヘルスに出す(issue #108 の残り)

- 状態: 採用
- 日付: 2026-10-03
- レーン: タイプバランス(balance・speed のサービス本体)
- 関連: ADR-0128(metadata.json)、ADR-0135(版の確認・`make master-release`)、ADR-0403・ADR-0603(balance・speed の read model)

## 背景

ADR-0135 は、balance・speed の「動いている版」を Deployment の注釈(配備時に付けた値)で代えていた。
注釈は「配備した版」であり、プロセスが読んだ版の証明ではない。#108 の残りは、サービス自身が版を持って示すこと。

## 決定

1. 各サービスの `internal/master.LoadDataVersionNextTo(readModelPath)` が、read model(balance は `BALANCE_POKEMON_TYPES_PATH`、
   speed は `SPEED_POKEMON_PATH`)と**同じディレクトリ**の `metadata.json` の `dataVersion` を読む。他のファイルの読み方は変えない。
2. 欠落と不正を分ける。
   - `metadata.json` が無い(古い export・read model 未設定): 版不明(空文字)。起動は続け、`Warn` を出す。read model 本体が
     任意(未設定でも起動して 503)という既存の方針に揃える。
   - あるのに JSON として読めない・`dataVersion` が空: 起動エラー(非 0 で終了)。read model 本体が不正なときと同じ fail 方針。
3. 起動ログに `slog.Info(..., "dataVersion", ...)` を出す。
4. 両サービスの openapi の `Health` に**任意の** `dataVersion`(string)を足し、`/healthz` と `/api/<svc>/healthz` で返す。
   版不明のときはキーごと省く。必須の `status` は変えないので既存クライアントは壊れない(additive。balance 0.9.0・speed 0.6.0)。
   dataVersion は公開データの版(取得元の版と checksum 先頭8桁)だけで、ADR-0135 §1 と同じく漏えいにならない。
   生成物は `make balance-gen speed-gen`・`make gen-ts`(`web/src/api/balance.gen.ts`)・`make ios-gen`。
5. k3d の手動配備(`scripts/gitops/k3d-deploy-readmodel.sh`)は、`metadata.json` があれば ConfigMap にも入れて同じディレクトリに置く
   (GitOps overlay は initContainer の export が全ファイルを書くので既に置かれる)。Deployment の注釈は残す(配備時の版)。

## 影響・残り

- 版の確認は、注釈(配備した版)と `/healthz` の `dataVersion`(プロセスが読んだ版)の両方で突き合わせられる。
  `check-master-version.sh` を healthz 参照に切り替えるのは、データレーンの持ち物なので本 ADR では行わない。
- 既存の export が `metadata.json` を持たない場合は版不明(警告)で動く。
