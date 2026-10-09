-- 種族の重さ(hg。0.1kg 単位の整数。ADR-0143 §5)。重さで威力が決まる技の計算に使う。
-- NULL を許し、既定値を付けない: 運用中の DB には既に種族の行があり、NOT NULL では migrate が失敗する。
-- 既定値で埋めると取り込み前の行が誤った重さを黙って持つ。NULL は「まだ importer が入れていない」。
-- 行は importer が書く(migration にデータは書かない。ADR-0100 §2)。

ALTER TABLE species
  ADD COLUMN weight_hg SMALLINT UNSIGNED NULL,
  ADD CONSTRAINT chk_species_weight_hg CHECK (weight_hg IS NULL OR weight_hg > 0);
