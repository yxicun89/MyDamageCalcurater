CREATE TABLE calc_events (
  event_id             VARCHAR(64) NOT NULL,
  device_id            VARCHAR(36) NOT NULL,
  session_id           VARCHAR(36) NOT NULL,
  operation            VARCHAR(32) NOT NULL,
  occurred_at          DATETIME(6) NOT NULL,
  defender_species_key VARCHAR(16) NOT NULL DEFAULT '',
  payload              JSON NOT NULL,
  created_at           DATETIME(6) NOT NULL,
  PRIMARY KEY (event_id),
  KEY idx_calc_events_device_id (device_id, occurred_at)
);
