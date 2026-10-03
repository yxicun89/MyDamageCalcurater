## 2026-10-03: balance・speed が read model の dataVersion を読んで起動ログとヘルスに出す(#108 の残り。ADR-0138)
Decision: read model と同じディレクトリの `metadata.json` を起動時に読む。欠落は版不明(警告・起動は続ける)、不正は起動エラー。`/healthz`・`/api/<svc>/healthz` の応答に任意の `dataVersion` を追加(additive)。
Reason: ADR-0135 の注釈は「配備した版」で、プロセスが読んだ版の証明ではなかった。read model 本体の既存 fail 方針(未設定は任意・設定したのに不正は終了)に揃えた。
Impact: balance 0.9.0・speed 0.6.0(契約は後方互換)。web の balance.gen.ts・iOS の Balance/Speed 生成物を更新。k3d-deploy-readmodel.sh は metadata.json も ConfigMap に入れる。`check-master-version.sh` の healthz 参照への切替はデータレーンの持ち物(未着手)。
