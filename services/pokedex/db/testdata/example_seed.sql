-- 架空データの example(ADR-0100 §7)。実在のポケモン・技・持ち物・特性の名前や値は入れない。
-- 図鑑番号は 9001 以降、ID は test で始まる英小文字、日本語名は「テスト」で始める。
-- タイプ ID は一般名を使うが、相性のコードは架空の小さな表。
-- `make test-db` のテスト(services/pokedex/db/mysql_test.go)が migrate 直後の空の DB に流し込む。

INSERT INTO types (id, sort_order, name_ja, name_ja_source) VALUES
  ('fire', 1, 'テストほのお', 'override'),
  ('water', 2, 'テストみず', 'override'),
  ('grass', 3, 'テストくさ', 'override'),
  ('normal', 4, 'テストふつう', 'override');

INSERT INTO type_chart (attack_type, defense_type, code) VALUES
  ('fire', 'fire', 1), ('fire', 'water', 1), ('fire', 'grass', 4), ('fire', 'normal', 2),
  ('water', 'fire', 4), ('water', 'water', 1), ('water', 'grass', 1), ('water', 'normal', 2),
  ('grass', 'fire', 1), ('grass', 'water', 4), ('grass', 'grass', 1), ('grass', 'normal', 2),
  ('normal', 'fire', 2), ('normal', 'water', 2), ('normal', 'grass', 2), ('normal', 'normal', 0);

INSERT INTO abilities (id, name_ja, name_ja_source, name_en) VALUES
  ('testability', 'テストとくせい', 'override', 'Test Ability'),
  ('testguard', 'テストガード', 'override', 'Test Guard'),
  ('testhidden', 'テストかくれ', 'override', 'Test Hidden');

INSERT INTO items (id, name_ja, name_ja_source, name_en) VALUES
  ('teststone', 'テストストーン', 'override', 'Test Stone'),
  ('testorb', 'テストオーブ', 'override', 'Test Orb'),
  ('testplain', 'テストただのもの', 'override', 'Test Plain');

INSERT INTO item_effects (item_id, effect) VALUES
  ('testorb', '{"DamageMod":5324}');

INSERT INTO ability_effects (ability_id, effect) VALUES
  ('testability', '{"StabMod":8192}'),
  ('testguard', '{"DefResistType":{"fire":2048,"water":2048}}');

INSERT INTO moves (id, name_ja, name_ja_source, name_en, type, category, power, accuracy, pp, priority) VALUES
  ('testflame', 'テストフレイム', 'override', 'Test Flame', 'fire', 'special', 90, 100, 15, 0),
  ('testsplash', 'テストスプラッシュ', 'override', 'Test Splash', 'water', 'physical', 80, 95, 10, 0),
  ('testshield', 'テストシールド', 'override', 'Test Shield', 'normal', 'status', 0, NULL, 10, 4);

INSERT INTO species (`key`, dex_no, form, showdown_id, name_ja, name_ja_source, name_en, type1, type2,
                     base_hp, base_atk, base_def, base_spa, base_spd, base_spe,
                     is_mega, base_species_key, required_item_id) VALUES
  ('9001-000', 9001, 0, 'testmon', 'テストモン', 'override', 'Testmon', 'fire', NULL,
   80, 90, 70, 100, 75, 85, 0, NULL, NULL),
  ('9001-001', 9001, 1, 'testmonmega', 'テストモン(メガ)', 'override', 'Testmon-Mega', 'fire', 'water',
   80, 110, 90, 130, 95, 95, 1, '9001-000', 'teststone'),
  ('9002-000', 9002, 0, 'testleaf', 'テストリーフ', 'override', 'Testleaf', 'grass', 'normal',
   95, 70, 110, 60, 105, 40, 0, NULL, NULL);

INSERT INTO species_abilities (species_key, slot, ability_id) VALUES
  ('9001-000', 1, 'testability'),
  ('9001-000', 3, 'testhidden'),
  ('9001-001', 1, 'testguard'),
  ('9002-000', 1, 'testguard'),
  ('9002-000', 2, 'testability');

INSERT INTO learnsets (species_key, move_id) VALUES
  ('9001-000', 'testflame'),
  ('9001-000', 'testshield'),
  ('9001-001', 'testflame'),
  ('9001-001', 'testsplash'),
  ('9002-000', 'testshield');

INSERT INTO regulations (id, name_ja, is_default, starts_on, ends_on) VALUES
  ('test-a', 'テストレギュレーションA', 1, NULL, NULL),
  ('test-b', 'テストレギュレーションB', 0, NULL, NULL);

INSERT INTO regulation_species (regulation_id, species_key) VALUES
  ('test-a', '9001-000'), ('test-a', '9001-001'), ('test-a', '9002-000'),
  ('test-b', '9002-000');

INSERT INTO regulation_moves (regulation_id, move_id) VALUES
  ('test-a', 'testflame'), ('test-a', 'testsplash'), ('test-a', 'testshield'),
  ('test-b', 'testshield');

INSERT INTO regulation_items (regulation_id, item_id) VALUES
  ('test-a', 'teststone'), ('test-a', 'testorb'), ('test-a', 'testplain'),
  ('test-b', 'testplain');

INSERT INTO regulation_abilities (regulation_id, ability_id) VALUES
  ('test-a', 'testability'), ('test-a', 'testguard'), ('test-a', 'testhidden'),
  ('test-b', 'testguard');

INSERT INTO data_versions (source, version, checksum, imported_at) VALUES
  ('example', 'example-1', '0000000000000000000000000000000000000000000000000000000000000000', '2000-01-01 00:00:00.000000');
