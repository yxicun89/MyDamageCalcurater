-- タイプ相性表(ADR-0013・ADR-0015 §3)。types が sort_order の定義順を持ち、
-- type_chart は倍率を ×2 した整数コード(0/1/2/4)で持つ。等倍(2)の組は省略してよい
-- (importer/写像側で「無ければ等倍」として扱う。CHECK はコードの値そのものだけを検査する)。

CREATE TABLE types (
  id VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL,
  name_ja VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_ja_0900_as_cs NOT NULL,
  name_ja_source VARCHAR(16) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_types_sort_order (sort_order),
  CONSTRAINT chk_types_id CHECK (id REGEXP '^[a-z]+$'),
  CONSTRAINT chk_types_name_ja CHECK (CHAR_LENGTH(name_ja) > 0),
  CONSTRAINT chk_types_name_ja_source CHECK (name_ja_source IN ('pokeapi', 'override', 'fallback_en'))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE type_chart (
  attack_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  defense_type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  code TINYINT UNSIGNED NOT NULL,
  PRIMARY KEY (attack_type, defense_type),
  CONSTRAINT fk_type_chart_attack FOREIGN KEY (attack_type) REFERENCES types (id),
  CONSTRAINT fk_type_chart_defense FOREIGN KEY (defense_type) REFERENCES types (id),
  CONSTRAINT chk_type_chart_code CHECK (code IN (0, 1, 2, 4))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
