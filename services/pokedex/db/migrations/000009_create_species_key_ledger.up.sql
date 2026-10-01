-- 配った種族 key と showdown_id の対応の台帳(issue #277・ADR-0131)。
-- species は投入のたびに全置換されるので、消えた種族の key は species からは分からなくなる。
-- その key が後に別の showdown_id へ付く(team-svc 等が保存した key が別の種族を指す)のを
-- importer が止められるよう、一度配った組を追記だけで覚える。
-- species への外部キーは持たない(全置換で species の行が消えても台帳の行は残す)。
-- 行は importer が書く(migration にデータは書かない。ADR-0100 §2)。

CREATE TABLE species_key_ledger (
  species_key CHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  showdown_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  first_seen_at DATETIME NOT NULL,
  PRIMARY KEY (species_key),
  UNIQUE KEY uq_species_key_ledger_showdown_id (showdown_id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
