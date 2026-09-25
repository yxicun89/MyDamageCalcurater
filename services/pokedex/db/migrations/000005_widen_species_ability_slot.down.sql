-- slot = 4 の行が残っていると CHECK(1..3)を付け直せず down が dirty で止まるため、先に消す
-- (issue #278・ADR-0124)。down は migrate-down(CONFIRM_DESTROY 必須)からだけ流す全削除の途中で、
-- species_abilities は 000002 の down で表ごと消えるので、ここで消してもデータの損失は増えない。
DELETE FROM species_abilities WHERE slot = 4;
ALTER TABLE species_abilities
  DROP CHECK chk_species_abilities_slot,
  ADD CONSTRAINT chk_species_abilities_slot CHECK (slot IN (1, 2, 3));
