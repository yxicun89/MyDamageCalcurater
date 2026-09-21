-- 共通マスタ(特性・持ち物・技・種族)と、その効果定義・習得技(ADR-0100 §3)。
-- 効果定義(item_effects / ability_effects)は正規化せず JSON 列に持つ(§3 判断)。
-- 行が無い = 補正なし。空オブジェクト {} は「補正なし」と区別できないため
-- JSON の中身の妥当性(厳格デコード)は services/internal/master が担う。

CREATE TABLE abilities (
  id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name_ja VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_ja_0900_as_cs NOT NULL,
  name_ja_source VARCHAR(16) NOT NULL,
  name_en VARCHAR(64) NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT chk_abilities_id CHECK (id REGEXP '^[a-z0-9]+$'),
  CONSTRAINT chk_abilities_name_ja CHECK (CHAR_LENGTH(name_ja) > 0),
  CONSTRAINT chk_abilities_name_ja_source CHECK (name_ja_source IN ('pokeapi', 'override', 'fallback_en'))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE items (
  id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name_ja VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_ja_0900_as_cs NOT NULL,
  name_ja_source VARCHAR(16) NOT NULL,
  name_en VARCHAR(64) NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT chk_items_id CHECK (id REGEXP '^[a-z0-9]+$'),
  CONSTRAINT chk_items_name_ja CHECK (CHAR_LENGTH(name_ja) > 0),
  CONSTRAINT chk_items_name_ja_source CHECK (name_ja_source IN ('pokeapi', 'override', 'fallback_en'))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE moves (
  id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name_ja VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_ja_0900_as_cs NOT NULL,
  name_ja_source VARCHAR(16) NOT NULL,
  name_en VARCHAR(64) NOT NULL,
  type VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  category VARCHAR(16) NOT NULL,
  power SMALLINT UNSIGNED NOT NULL,
  accuracy TINYINT UNSIGNED NULL,
  pp TINYINT UNSIGNED NOT NULL,
  priority TINYINT NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_moves_type FOREIGN KEY (type) REFERENCES types (id),
  CONSTRAINT chk_moves_id CHECK (id REGEXP '^[a-z0-9]+$'),
  CONSTRAINT chk_moves_name_ja CHECK (CHAR_LENGTH(name_ja) > 0),
  CONSTRAINT chk_moves_name_ja_source CHECK (name_ja_source IN ('pokeapi', 'override', 'fallback_en')),
  CONSTRAINT chk_moves_category CHECK (category IN ('physical', 'special', 'status')),
  CONSTRAINT chk_moves_power CHECK (power BETWEEN 0 AND 999),
  CONSTRAINT chk_moves_power_status CHECK (category <> 'status' OR power = 0),
  CONSTRAINT chk_moves_accuracy CHECK (accuracy IS NULL OR accuracy BETWEEN 1 AND 100),
  CONSTRAINT chk_moves_pp CHECK (pp BETWEEN 1 AND 64),
  CONSTRAINT chk_moves_priority CHECK (priority BETWEEN -7 AND 5)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

-- `key` は MySQL の予約語なのでバッククォートで囲む。メガの3列(is_mega・base_species_key・
-- required_item_id)は「性能が同じ見た目違いフォームは1件、性能が違うフォーム/メガは別行」
-- (DECISIONS 2026-09-21)の採番を支える。base_species_key / required_item_id の外部キーは
-- 既定(RESTRICT)にする: MySQL は CHECK 制約が参照する列に ON DELETE/UPDATE CASCADE|SET NULL を
-- 付けられないため(ADR-0100 §2)。
CREATE TABLE species (
  `key` CHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  dex_no SMALLINT UNSIGNED NOT NULL,
  form SMALLINT UNSIGNED NOT NULL,
  showdown_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name_ja VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_ja_0900_as_cs NOT NULL,
  name_ja_source VARCHAR(16) NOT NULL,
  name_en VARCHAR(64) NOT NULL,
  type1 VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  type2 VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NULL,
  base_hp SMALLINT UNSIGNED NOT NULL,
  base_atk SMALLINT UNSIGNED NOT NULL,
  base_def SMALLINT UNSIGNED NOT NULL,
  base_spa SMALLINT UNSIGNED NOT NULL,
  base_spd SMALLINT UNSIGNED NOT NULL,
  base_spe SMALLINT UNSIGNED NOT NULL,
  is_mega TINYINT(1) NOT NULL,
  base_species_key CHAR(8) CHARACTER SET ascii COLLATE ascii_bin NULL,
  required_item_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (`key`),
  UNIQUE KEY uq_species_showdown_id (showdown_id),
  UNIQUE KEY uq_species_dex_form (dex_no, form),
  CONSTRAINT fk_species_type1 FOREIGN KEY (type1) REFERENCES types (id),
  CONSTRAINT fk_species_type2 FOREIGN KEY (type2) REFERENCES types (id),
  CONSTRAINT fk_species_base_species FOREIGN KEY (base_species_key) REFERENCES species (`key`),
  CONSTRAINT fk_species_required_item FOREIGN KEY (required_item_id) REFERENCES items (id),
  CONSTRAINT chk_species_key CHECK (`key` = CONCAT(LPAD(dex_no, 4, '0'), '-', LPAD(form, 3, '0'))),
  CONSTRAINT chk_species_dex_no CHECK (dex_no BETWEEN 1 AND 9999),
  CONSTRAINT chk_species_form CHECK (form BETWEEN 0 AND 999),
  CONSTRAINT chk_species_name_ja CHECK (CHAR_LENGTH(name_ja) > 0),
  CONSTRAINT chk_species_name_ja_source CHECK (name_ja_source IN ('pokeapi', 'override', 'fallback_en')),
  CONSTRAINT chk_species_type2_differs CHECK (type2 IS NULL OR type2 <> type1),
  CONSTRAINT chk_species_base_hp CHECK (base_hp BETWEEN 1 AND 255),
  CONSTRAINT chk_species_base_atk CHECK (base_atk BETWEEN 1 AND 255),
  CONSTRAINT chk_species_base_def CHECK (base_def BETWEEN 1 AND 255),
  CONSTRAINT chk_species_base_spa CHECK (base_spa BETWEEN 1 AND 255),
  CONSTRAINT chk_species_base_spd CHECK (base_spd BETWEEN 1 AND 255),
  CONSTRAINT chk_species_base_spe CHECK (base_spe BETWEEN 1 AND 255),
  CONSTRAINT chk_species_is_mega CHECK (is_mega IN (0, 1)),
  CONSTRAINT chk_species_mega CHECK (
    (is_mega = 1 AND base_species_key IS NOT NULL AND required_item_id IS NOT NULL)
    OR (is_mega = 0 AND base_species_key IS NULL AND required_item_id IS NULL)
  ),
  CONSTRAINT chk_species_base_species_not_self CHECK (base_species_key IS NULL OR base_species_key <> `key`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE species_abilities (
  species_key CHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  slot TINYINT UNSIGNED NOT NULL,
  ability_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (species_key, slot),
  UNIQUE KEY uq_species_abilities_species_ability (species_key, ability_id),
  CONSTRAINT fk_species_abilities_species FOREIGN KEY (species_key) REFERENCES species (`key`) ON DELETE CASCADE,
  CONSTRAINT fk_species_abilities_ability FOREIGN KEY (ability_id) REFERENCES abilities (id),
  CONSTRAINT chk_species_abilities_slot CHECK (slot IN (1, 2, 3))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE item_effects (
  item_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect JSON NOT NULL,
  PRIMARY KEY (item_id),
  CONSTRAINT fk_item_effects_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE CASCADE,
  CONSTRAINT chk_item_effects_effect CHECK (JSON_TYPE(effect) = 'OBJECT')
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

CREATE TABLE ability_effects (
  ability_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect JSON NOT NULL,
  PRIMARY KEY (ability_id),
  CONSTRAINT fk_ability_effects_ability FOREIGN KEY (ability_id) REFERENCES abilities (id) ON DELETE CASCADE,
  CONSTRAINT chk_ability_effects_effect CHECK (JSON_TYPE(effect) = 'OBJECT')
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;

-- 使える技 = learnsets ∩ regulation_moves(DECISIONS: 断定できない技は使用可として残す。P2-1c)。
-- レギュレーションには依存させない(§3 判断)。
CREATE TABLE learnsets (
  species_key CHAR(8) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  move_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (species_key, move_id),
  CONSTRAINT fk_learnsets_species FOREIGN KEY (species_key) REFERENCES species (`key`) ON DELETE CASCADE,
  CONSTRAINT fk_learnsets_move FOREIGN KEY (move_id) REFERENCES moves (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
