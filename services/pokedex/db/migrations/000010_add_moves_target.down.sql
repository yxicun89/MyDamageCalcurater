-- 列を落とす前に、その列を参照する CHECK を先に落とす(MySQL は CHECK が参照する列を DROP COLUMN できない)。
ALTER TABLE moves DROP CHECK chk_moves_target;
ALTER TABLE moves DROP COLUMN target;
