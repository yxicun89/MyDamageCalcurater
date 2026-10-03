## 2026-10-02: record-svc・team-svc を k3d に配線し、失効ジョブを CronJob にした(API レーン。他レーンへの連絡。ADR-0220)
Decision: `deploy/k8s/base/{record,team}` を追加し、base の kustomization と gateway(`GATEWAY_RECORD_URL=http://record`・`GATEWAY_TEAM_URL=http://team`)に配線した。
失効(ADR-0209 §4)は同じバイナリのサブコマンド `record expire` / `team expire` を日次 CronJob(`record-expire`・`team-expire`。日本時間 12:00)で起動する。
Reason: 設定の解釈を serve と共有でき、イメージを増やさない(ADR-0220 §3)。
Impact(他レーンへ):
- Web・iOS: `make up` 後、k3d で `/api/record/*`・`/api/team/*` が gateway 経由で届く(TiDB の導入に成功し record/team の Pod が Ready のとき。無ければ従来どおり 503 `upstream_unavailable`)。
- 運用: 失効ジョブの CronJob が増えた(失敗は kube_job_status_failed で見る。削除件数はログの `expire done` 1行)。cloud overlay では TiDB・Secret が無いので suspend 済み。
  NetworkPolicy は record/team の gateway 上流・TiDB・NATS(購読)・Prometheus を許可し、record ↔ team・calc → record/team・record/team → mysql を拒否する。ServiceMonitor は8サービスになった。
  `make deploy-latest` に record/team を加えるかは運用レーンの判断(今回は触っていない)。up.sh は record/team の Ready を待たない。
- 失効ジョブは local・cloud とも `suspend: true`(実データへ初めて向ける承認 ADR-0209・ADR-0220 未決事項 0 が済むまで。既定案。plan.md ブロッカー)。手動実行は `kubectl -n pokecalc create job --from=cronjob/record-expire record-expire-manual-...`。承認後に local の suspend patch を外す。
- 保持日数は ConfigMap `record-retention`・`team-retention` の1か所(ADR-0211 §7 の既定値。変えるときは ADR も更新)。
