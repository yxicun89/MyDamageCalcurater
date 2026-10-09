-- 列を落とす前に、その列を参照する CHECK を先に落とす(MySQL は CHECK が参照する列を DROP COLUMN できない)。
ALTER TABLE species DROP CHECK chk_species_weight_hg;
ALTER TABLE species DROP COLUMN weight_hg;
