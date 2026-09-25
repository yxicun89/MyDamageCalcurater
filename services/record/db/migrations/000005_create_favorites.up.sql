CREATE TABLE favorites (
  id           BIGINT NOT NULL AUTO_INCREMENT,
  device_id    VARCHAR(36) NOT NULL,
  species_key  VARCHAR(16) NOT NULL,
  snapshot     JSON NOT NULL,
  created_at   DATETIME(6) NOT NULL,
  updated_at   DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_favorites_device_id (device_id, updated_at)
);
