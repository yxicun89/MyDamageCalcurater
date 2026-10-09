-- 技の処理の定義(威力の式・条件つきの威力・タイプ・相性・優先度・壁。ADR-0143 §5)。
-- item_effects / move_effects とまったく同じ形にする: 行が無い = 定義なし。
-- 空オブジェクト {} は「定義なし」と区別できないため作らない。
-- JSON の中身の妥当性(厳格デコード)は services/internal/master の DecodeMoveRule が担う。

CREATE TABLE move_rules (
  move_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  rule JSON NOT NULL,
  PRIMARY KEY (move_id),
  CONSTRAINT fk_move_rules_move FOREIGN KEY (move_id) REFERENCES moves (id) ON DELETE CASCADE,
  CONSTRAINT chk_move_rules_rule CHECK (JSON_TYPE(rule) = 'OBJECT')
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
