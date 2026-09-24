CREATE TABLE devices (
  device_id      VARCHAR(36) NOT NULL,
  last_seen_at   DATETIME(6) NOT NULL,
  purged_at      DATETIME(6) NULL,
  orphaned_since DATETIME(6) NULL,
  PRIMARY KEY (device_id),
  KEY idx_devices_last_seen_at (last_seen_at),
  KEY idx_devices_orphaned_since (orphaned_since)
);
