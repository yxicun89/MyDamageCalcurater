-- 持ち物のなげつけるの威力(Showdown の fling.basePower。ADR-0144 §5)。なげつける型の技の威力に使う。
-- NULL を許し、既定値を付けない: 運用中の DB には既に持ち物の行があり、NOT NULL では migrate が失敗する。
-- NULL は「投げられない、またはまだ importer が入れていない」(engine は 0 として未対応の印を残す)。
-- 行は importer が書く(migration にデータは書かない。ADR-0100 §2)。
-- 番号はマージ前に origin/main の最新を確かめ、先に別の 000017 が入っていたら繰り下げる(layout テストの定数も合わせる)。

ALTER TABLE items
  ADD COLUMN fling_power SMALLINT UNSIGNED NULL,
  ADD CONSTRAINT chk_items_fling_power CHECK (fling_power IS NULL OR fling_power > 0);
