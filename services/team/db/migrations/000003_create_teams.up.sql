CREATE TABLE teams (
  id         VARCHAR(36) NOT NULL,
  device_id  VARCHAR(36) NOT NULL,
  name       VARCHAR(50) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_teams_device_id (device_id, updated_at)
);
