-- 技の追加効果(命中時のランク変化)のマスタ(ADR-0107 決定5)。
-- item_effects / ability_effects とまったく同じ形にする: 行が無い = 追加効果なし。
-- 空オブジェクト {} は「追加効果なし」と区別できないため作らない。
-- JSON の中身の妥当性(厳格デコード)は services/internal/master が担う。

CREATE TABLE move_effects (
  move_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  effect JSON NOT NULL,
  PRIMARY KEY (move_id),
  CONSTRAINT fk_move_effects_move FOREIGN KEY (move_id) REFERENCES moves (id) ON DELETE CASCADE,
  CONSTRAINT chk_move_effects_effect CHECK (JSON_TYPE(effect) = 'OBJECT')
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
