-- フェーズ4-2 価格の推移(docs/phase4-spec.md)。1 商品×1 サイト×1 日(JST)1 行。
-- 点を作るのは取得が ok で low がある日だけ。mid は件数 3 未満の日は NULL。
CREATE TABLE IF NOT EXISTS price_history (
  item_id BIGINT NOT NULL,
  site_id BIGINT NOT NULL,
  day DATE NOT NULL,
  low INT NOT NULL,
  mid INT NULL,
  count INT NOT NULL,
  recorded_at DATETIME NOT NULL,
  PRIMARY KEY (item_id, site_id, day),
  KEY idx_price_history_day (day),
  CONSTRAINT fk_price_history_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE CASCADE,
  CONSTRAINT fk_price_history_site FOREIGN KEY (site_id) REFERENCES sites (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
