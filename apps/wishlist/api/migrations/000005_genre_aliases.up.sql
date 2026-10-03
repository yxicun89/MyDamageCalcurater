-- フェーズ4-1 表記揺れの辞書(docs/phase4-spec.md)。ジャンルごとの別名グループ。
-- group_no はジャンル内のグループの順(0 始まり)。グループ内の語の順は id 昇順。
-- normalized は query.Normalize(alias)(NFKC → 小文字 → 空白・記号を除去)。照合は Go 側で行うので SQL では正規化しない。
-- utf8mb4_bin にするのは、既定の ai_ci だと濁点(ガ/カ)・ひらがなとカタカナを同一視し、正規化後に違う語を重複として弾くため。
CREATE TABLE IF NOT EXISTS genre_aliases (
  id BIGINT NOT NULL AUTO_INCREMENT,
  genre_id BIGINT NOT NULL,
  group_no INT NOT NULL,
  alias VARCHAR(64) NOT NULL,
  normalized VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_genre_aliases_normalized (genre_id, normalized),
  CONSTRAINT fk_genre_aliases_genre FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

-- 初期データ。ジャンルは名前で引き(id を書かない)、無いジャンルは飛ばす。同じ (genre_id, normalized) があれば足さない。
-- ジャンル・サイト・紐づけは足さない・変えない。
-- NOT EXISTS は語の単位で判定する。golang-migrate は 1 回しか流さない前提(もう一度流しても、足りない語だけが入り壊れない)。
-- 「フィギュアーツ」単独は入れない(部分一致で「フィギュアーツZERO」等の別シリーズまで一致扱いになるため)。
INSERT INTO genre_aliases (genre_id, group_no, alias, normalized)
SELECT g.id, v.group_no, v.alias, v.normalized
FROM (
  SELECT 'S.H.Figuarts' AS genre_name, 0 AS group_no, 0 AS ord, 'S.H.Figuarts' AS alias, 'shfiguarts' AS normalized
  UNION ALL SELECT 'S.H.Figuarts', 0, 1, 'SHフィギュアーツ', 'shフィギュアーツ'
  UNION ALL SELECT 'ガンプラ', 0, 0, 'HG', 'hg'
  UNION ALL SELECT 'ガンプラ', 0, 1, 'ハイグレード', 'ハイグレード'
  UNION ALL SELECT 'ガンプラ', 1, 0, 'MG', 'mg'
  UNION ALL SELECT 'ガンプラ', 1, 1, 'マスターグレード', 'マスターグレード'
  UNION ALL SELECT 'ガンプラ', 2, 0, 'RG', 'rg'
  UNION ALL SELECT 'ガンプラ', 2, 1, 'リアルグレード', 'リアルグレード'
) AS v
JOIN genres g ON g.name = v.genre_name
WHERE NOT EXISTS (
  SELECT 1 FROM genre_aliases a WHERE a.genre_id = g.id AND a.normalized = v.normalized COLLATE utf8mb4_bin
)
ORDER BY g.id, v.group_no, v.ord;
