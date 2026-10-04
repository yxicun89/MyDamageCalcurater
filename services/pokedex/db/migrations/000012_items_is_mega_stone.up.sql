-- 取得元(Showdown の持ち物データ)から導いたメガストーンの判定(ADR-0140・issue #607)。行は importer が書く。
ALTER TABLE items ADD COLUMN is_mega_stone TINYINT(1) NOT NULL DEFAULT 0;
