-- 技の機構の中身(ADR-0142 §7)。move_mechanisms(ADR-0121)の機構のうち、取得元のフィールドの値だけで
-- 中身が決まるもの(多段・固定ダメージ・一撃必殺・攻撃/防御に使う能力値)の値を、1技1行で持つ。
-- 行が無い = 中身なし(engine は通常の式で計算して「未対応」の印を付ける。ADR-0123)。
-- 必ず急所・防御ランク無視は機構だけで決まるので中身を持たない。
-- 列はすべて NULL 可(その項目が無い)。中身が1つも無い行は importer と services/internal/master が拒否する。
-- 機構との対応(multi_hit の技にだけ多段の列がある 等)は他の表を見る CHECK が書けないので、同じく
-- importer と services/internal/master が担う。値域は engine.MaxMultiHits(10)と一致させる。

CREATE TABLE move_mechanism_params (
  move_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  multi_hit_min TINYINT UNSIGNED NULL,
  multi_hit_max TINYINT UNSIGNED NULL,
  fixed_damage_level BOOLEAN NULL,
  fixed_damage_value SMALLINT UNSIGNED NULL,
  ohko BOOLEAN NULL,
  ohko_immune_type VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NULL,
  offense_stat VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NULL,
  offense_pokemon VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NULL,
  defense_stat VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (move_id),
  CONSTRAINT fk_move_mechanism_params_move FOREIGN KEY (move_id) REFERENCES moves (id) ON DELETE CASCADE,
  CONSTRAINT chk_move_mechanism_params_multi_hit CHECK (
    (multi_hit_min IS NULL AND multi_hit_max IS NULL)
    OR (multi_hit_min >= 1 AND multi_hit_min <= multi_hit_max AND multi_hit_max >= 2 AND multi_hit_max <= 10)
  ),
  CONSTRAINT chk_move_mechanism_params_fixed_damage CHECK (
    (fixed_damage_level IS NULL AND fixed_damage_value IS NULL)
    OR (fixed_damage_level = TRUE AND fixed_damage_value IS NULL)
    OR (fixed_damage_level IS NULL AND fixed_damage_value > 0)
  ),
  CONSTRAINT chk_move_mechanism_params_ohko CHECK (ohko IS NULL OR ohko = TRUE),
  CONSTRAINT chk_move_mechanism_params_ohko_immune CHECK (ohko_immune_type IS NULL OR ohko = TRUE),
  CONSTRAINT chk_move_mechanism_params_offense_stat CHECK (offense_stat IN ('atk', 'def', 'spa', 'spd', 'spe')),
  CONSTRAINT chk_move_mechanism_params_offense_pokemon CHECK (offense_pokemon IN ('attacker', 'defender')),
  CONSTRAINT chk_move_mechanism_params_defense_stat CHECK (defense_stat IN ('atk', 'def', 'spa', 'spd', 'spe'))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
