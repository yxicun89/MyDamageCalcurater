-- サイト行の「状態(在庫あり等)」(仕様 §3)を表すため、参考外を除いた在庫ありの件数を持つ(docs/phase3-api-spec.md)。
ALTER TABLE estimates
  ADD COLUMN in_stock_count INT NOT NULL DEFAULT 0 AFTER suspicious_count;
