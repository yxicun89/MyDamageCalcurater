-- 列を落とす前に、その列を参照する CHECK を先に落とす(MySQL は CHECK が参照する列を DROP COLUMN できない)。
ALTER TABLE items DROP CHECK chk_items_fling_power;
ALTER TABLE items DROP COLUMN fling_power;
