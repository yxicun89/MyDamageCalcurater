-- 技の機構(多段・固定ダメージ・威力変動・参照する能力値の差し替え 等)のマスタ(ADR-0121)。
-- 1つの技が複数の機構を持つことがある(例: 多段かつ威力が変わる)ので、learnsets と同じく
-- (技, 機構) の組を1行にする。行が無い = 通常の技(威力・分類・タイプから通常の式で計算できる)。
-- 値の一覧は services/internal/master の AllMoveMechanisms と一致させる(layout テストで確かめる)。
-- 変化技は機構を持たない(他の表の列を見る CHECK は書けないので importer と services/internal/master が担う)。

CREATE TABLE move_mechanisms (
  move_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  mechanism VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (move_id, mechanism),
  CONSTRAINT fk_move_mechanisms_move FOREIGN KEY (move_id) REFERENCES moves (id) ON DELETE CASCADE,
  CONSTRAINT chk_move_mechanisms_mechanism CHECK (mechanism IN (
    'alt_defense_stat', 'alt_offense_stat', 'always_crit', 'effectiveness_change', 'field_specific',
    'fixed_damage', 'ignore_defense_ranks', 'move_specific', 'multi_hit', 'ohko', 'priority_change',
    'type_change', 'variable_power'
  ))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
