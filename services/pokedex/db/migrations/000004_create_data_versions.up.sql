-- 取得元ごとの最新の適用済み版(ADR-0100 §3)。importer がデータの置き換えと同じ
-- トランザクションで更新する。取得元の名前の一覧は形式だけ検査し、CHECK に列挙しない。

CREATE TABLE data_versions (
  source VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version VARCHAR(128) NOT NULL,
  checksum CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  imported_at DATETIME(6) NOT NULL,
  PRIMARY KEY (source),
  CONSTRAINT chk_data_versions_source CHECK (source REGEXP '^[a-z0-9]+(-[a-z0-9]+)*$'),
  CONSTRAINT chk_data_versions_version CHECK (CHAR_LENGTH(version) > 0),
  CONSTRAINT chk_data_versions_checksum CHECK (checksum REGEXP '^[0-9a-f]{64}$')
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
