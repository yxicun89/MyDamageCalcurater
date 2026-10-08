-- 技のフラグ(接触・音・パンチ・かみつき・切る・波動・弾・反動・追加効果あり)のマスタ(ADR-0178)。
-- 1つの技が複数のフラグを持つので、move_mechanisms(ADR-0121)と同じく (技, フラグ) の組を1行にする。
-- 行が無い = その技はフラグなし。表全体が空 = まだ取り込んでいない(pokedex-svc はフラグを「不明」として返す。ADR-0178 §3)。
-- 値の一覧は engine の AllMoveFlags(services/internal/master の AllMoveFlags)と一致させる(layout テストで確かめる)。
-- 行は importer が書く(migration にデータは書かない。ADR-0100 §2)。

CREATE TABLE move_flags (
  move_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  flag VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  PRIMARY KEY (move_id, flag),
  CONSTRAINT fk_move_flags_move FOREIGN KEY (move_id) REFERENCES moves (id) ON DELETE CASCADE,
  CONSTRAINT chk_move_flags_flag CHECK (flag IN (
    'bite', 'bullet', 'contact', 'pulse', 'punch', 'recoil', 'secondary', 'slicing', 'sound'
  ))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
