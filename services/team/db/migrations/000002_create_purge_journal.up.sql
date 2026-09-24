CREATE TABLE purge_journal (
  id           BIGINT NOT NULL AUTO_INCREMENT,
  device_id    VARCHAR(36) NOT NULL,
  requested_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_purge_journal_device_id (device_id),
  KEY idx_purge_journal_requested_at (requested_at)
);
