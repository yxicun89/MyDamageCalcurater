-- species.name_ja_source に 'generated' を足す(issue #515・ADR-0324)。上流に日本語名が無いメガ種族の名前を、
-- 基本種の日本語名とフォーム識別子から importer が生成したことを表す。他の表(types・abilities・items・moves・natures)は
-- 生成しないので変えない。行は importer が書く(migration にデータは書かない。ADR-0100 §2)。
ALTER TABLE species DROP CHECK chk_species_name_ja_source;
ALTER TABLE species
  ADD CONSTRAINT chk_species_name_ja_source CHECK (name_ja_source IN ('pokeapi', 'override', 'fallback_en', 'generated'));
