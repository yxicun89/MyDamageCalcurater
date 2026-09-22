-- 性格のマスタ(ADR-0105 §4。ADR-0100 §3 の「性格はマスタにしない(engine の固定)」を改める)。
-- 取得元は Showdown の natures(補正)と PokeAPI の日本語名(+ override)。値は importer が入れる(ここに INSERT を書かない)。
-- 無補正の性格は plus / minus とも NULL。補正がある性格は plus <> minus で、どちらも HP を指さない。
-- 性格の件数(25)は CHECK にも列挙しない(マスタのデータで、コードに書かない)。

CREATE TABLE natures (
  id VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name_ja VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_ja_0900_as_cs NOT NULL,
  name_ja_source VARCHAR(16) NOT NULL,
  name_en VARCHAR(64) NOT NULL,
  plus VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NULL,
  minus VARCHAR(8) CHARACTER SET ascii COLLATE ascii_bin NULL,
  PRIMARY KEY (id),
  -- 同じ補正の組を持つ性格は1つだけ(calc-svc の NatureID が補正の構造値から ID を引くため。ADR-0200 §2)。
  -- 無補正(NULL, NULL)は UNIQUE の対象外で、複数あってよい。
  UNIQUE KEY uq_natures_plus_minus (plus, minus),
  CONSTRAINT chk_natures_id CHECK (id REGEXP '^[a-z0-9]+$'),
  CONSTRAINT chk_natures_name_ja CHECK (CHAR_LENGTH(name_ja) > 0),
  CONSTRAINT chk_natures_name_ja_source CHECK (name_ja_source IN ('pokeapi', 'override', 'fallback_en')),
  CONSTRAINT chk_natures_plus CHECK (plus IS NULL OR plus IN ('atk', 'def', 'spa', 'spd', 'spe')),
  CONSTRAINT chk_natures_minus CHECK (minus IS NULL OR minus IN ('atk', 'def', 'spa', 'spd', 'spe')),
  CONSTRAINT chk_natures_pair CHECK (
    (plus IS NULL AND minus IS NULL) OR (plus IS NOT NULL AND minus IS NOT NULL AND plus <> minus)
  )
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4;
