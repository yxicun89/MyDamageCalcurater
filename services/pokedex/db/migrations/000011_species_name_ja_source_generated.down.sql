-- 'generated' の行が残っていると CHECK を戻せないので、先に 'fallback_en' へ寄せる(名前そのものは変えない)。
UPDATE species SET name_ja_source = 'fallback_en' WHERE name_ja_source = 'generated';
ALTER TABLE species DROP CHECK chk_species_name_ja_source;
ALTER TABLE species
  ADD CONSTRAINT chk_species_name_ja_source CHECK (name_ja_source IN ('pokeapi', 'override', 'fallback_en'));
