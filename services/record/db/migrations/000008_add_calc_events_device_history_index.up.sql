-- 計算履歴の一覧(GET /api/record/calc-history。ADR-0230 §7)用の索引。
-- 端末・operation の絞り込みと occurred_at, event_id の並びを索引だけで解く。000003 の索引は失効ジョブ・全削除が使うので残す。
ALTER TABLE calc_events ADD KEY idx_calc_events_device_history (device_id, operation, occurred_at, event_id);
