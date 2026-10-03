-- 技の対象(単体・全体 等。ADR-0136・issue #288)。値は Showdown の技データの target の文字列のまま
-- (@smogon/calc 0.12.0 の MoveTarget と同じ 15 種)。一覧は services/internal/master の AllMoveTargets と一致させる
-- (layout テストで確かめる)。
-- NULL を許し、既定値を付けない: 運用中の DB には既に技の行があり、NOT NULL + CHECK では migrate が失敗する。
-- 既定値で埋めると取り込み前の行が誤った対象を黙って持つ。NULL は「まだ importer が入れていない」。
-- 行は importer が書く(migration にデータは書かない。ADR-0100 §2)。

ALTER TABLE moves
  ADD COLUMN target VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NULL,
  ADD CONSTRAINT chk_moves_target CHECK (target IN (
    'adjacentAlly', 'adjacentAllyOrSelf', 'adjacentFoe', 'all', 'allAdjacent', 'allAdjacentFoes',
    'allies', 'allySide', 'allyTeam', 'any', 'foeSide', 'normal', 'randomNormal', 'scripted', 'self'
  ));
