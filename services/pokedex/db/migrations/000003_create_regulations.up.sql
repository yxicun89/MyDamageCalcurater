-- レギュレーション(v1 は M-C)と使用可能集合(ADR-0100 §3 判断: 使用可否は行の
-- available 列でなく集合テーブルで持つ)。id の値はデータであり、コードに書かない。
-- 既定のレギュレーションは default_marker(生成列)への UNIQUE 制約で高々1件に制限する。
-- regulations は人が管理する定義(名前は override 前提)なので name_ja_source は持たない。

CREATE TABLE regulations (
  id VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name_ja VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_ja_0900_as_cs NOT NULL,
  is_default TINYINT(1) NOT NULL,
  default_marker TINYINT GENERATED ALWAYS AS (IF(is_default = 1, 1, NULL)) STORED,
  starts_on DATE NULL,
  ends_on DATE NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_regulations_default_marker (default_marker),
  CONSTRAINT chk_regulations_id CHECK (id REGEXP '^[a-z0-9]+(-[a-z0-9]+)*$'),
  CONSTRAINT chk_regulations_name_ja CHECK (CHAR_LENGTH(name_ja) > 0),
  CONSTRAINT chk_regulations_is_default CHECK (is_default IN (0, 1)),
  CONSTRAINT chk_regulations_dates CHECK (ends_on IS NULL OR starts_on IS NULL OR starts_on <= ends_on)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE regulation_species (
  regulation_id VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  species_key CHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (regulation_id, species_key),
  CONSTRAINT fk_regulation_species_regulation FOREIGN KEY (regulation_id) REFERENCES regulations (id) ON DELETE CASCADE,
  CONSTRAINT fk_regulation_species_species FOREIGN KEY (species_key) REFERENCES species (`key`) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE regulation_moves (
  regulation_id VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  move_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (regulation_id, move_id),
  CONSTRAINT fk_regulation_moves_regulation FOREIGN KEY (regulation_id) REFERENCES regulations (id) ON DELETE CASCADE,
  CONSTRAINT fk_regulation_moves_move FOREIGN KEY (move_id) REFERENCES moves (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

-- 逆算(調整推定)の持ち物候補は regulation_items ∩ item_effects から導出する(専用テーブルは持たない)。
CREATE TABLE regulation_items (
  regulation_id VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  item_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (regulation_id, item_id),
  CONSTRAINT fk_regulation_items_regulation FOREIGN KEY (regulation_id) REFERENCES regulations (id) ON DELETE CASCADE,
  CONSTRAINT fk_regulation_items_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE regulation_abilities (
  regulation_id VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  ability_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (regulation_id, ability_id),
  CONSTRAINT fk_regulation_abilities_regulation FOREIGN KEY (regulation_id) REFERENCES regulations (id) ON DELETE CASCADE,
  CONSTRAINT fk_regulation_abilities_ability FOREIGN KEY (ability_id) REFERENCES abilities (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
