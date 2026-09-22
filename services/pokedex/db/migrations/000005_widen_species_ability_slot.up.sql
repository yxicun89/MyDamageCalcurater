-- species_abilities.slot を 1..4 の4枠に広げる(実データで Showdown の "S"(特殊な特性)が
-- "H"(隠れ特性。slot 3)と同じ種族に共存することが判明したため、slot 4 に分けて置く。
-- ADR-0100(スキーマ)・ADR-0101 §5・ADR-0103 §12 を参照。000002 の CHECK(1..3)を置き換える
-- (000002 は main に取り込み済みのため書き換えない)。

ALTER TABLE species_abilities
  DROP CHECK chk_species_abilities_slot,
  ADD CONSTRAINT chk_species_abilities_slot CHECK (slot IN (1, 2, 3, 4));
