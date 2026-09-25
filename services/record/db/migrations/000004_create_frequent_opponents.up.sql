CREATE TABLE frequent_opponents (
  device_id          VARCHAR(36) NOT NULL,
  species_key        VARCHAR(16) NOT NULL,
  score              DOUBLE NOT NULL,
  count              INT NOT NULL,
  last_calculated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (device_id, species_key)
);
