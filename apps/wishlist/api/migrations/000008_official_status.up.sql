-- フェーズ4-3 公式サイトの販売状況の監視(docs/phase4-spec.md)。
-- items.watch_official は監視の ON/OFF(既定 OFF)。official_status は 1 商品 1 行。
-- status・evidence・checked_at は最後に判定できた状態とそれを確かめた時刻(failed・blocked で上書きしない)、
-- last_result・last_attempt_at は最後の試行の結果と時刻。changed_at・previous_status は判定済み同士の変化だけ。
ALTER TABLE items ADD COLUMN watch_official BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS official_status (
  item_id BIGINT NOT NULL,
  status ENUM('available','preorder','soldout','ended','unknown','ambiguous','blocked','failed') NOT NULL,
  evidence JSON NOT NULL,
  checked_at DATETIME NOT NULL,
  changed_at DATETIME NULL,
  previous_status ENUM('available','preorder','soldout','ended','unknown','ambiguous','blocked','failed') NULL,
  last_result ENUM('available','preorder','soldout','ended','unknown','ambiguous','blocked','failed') NOT NULL,
  last_attempt_at DATETIME NOT NULL,
  PRIMARY KEY (item_id),
  CONSTRAINT fk_official_status_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
