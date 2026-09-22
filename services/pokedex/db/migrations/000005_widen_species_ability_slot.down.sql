-- 注意: slot = 4 の行が残っていると CHECK の付け直しに失敗する。down は migrate-down(CONFIRM_DESTROY 必須)からだけ流す。
ALTER TABLE species_abilities
  DROP CHECK chk_species_abilities_slot,
  ADD CONSTRAINT chk_species_abilities_slot CHECK (slot IN (1, 2, 3));
