-- sqlc のクエリ(ADR-0015 §1)。services/internal/master(DB行→engine型の写像)が
-- 受け取る素朴な行の型(TypeRow・SpeciesRow 等)にそのまま詰め替えられる列の並びにする。

-- name: ListTypes :many
SELECT id, sort_order, name_ja, name_ja_source
FROM types
ORDER BY sort_order;

-- name: ListTypeChart :many
SELECT attack_type, defense_type, code
FROM type_chart;

-- name: GetSpeciesByKey :one
SELECT `key`, dex_no, form, showdown_id, name_ja, name_ja_source, name_en, type1, type2,
       base_hp, base_atk, base_def, base_spa, base_spd, base_spe,
       is_mega, base_species_key, required_item_id
FROM species
WHERE `key` = ?;

-- name: ListSpeciesAbilities :many
SELECT species_key, slot, ability_id
FROM species_abilities
WHERE species_key = ?
ORDER BY slot;

-- name: ListMoves :many
SELECT id, name_ja, name_ja_source, name_en, type, category, power, accuracy, pp, priority
FROM moves
ORDER BY id;

-- name: GetItem :one
SELECT id, name_ja, name_ja_source, name_en
FROM items
WHERE id = ?;

-- name: GetItemEffect :one
SELECT item_id, effect
FROM item_effects
WHERE item_id = ?;

-- name: GetAbility :one
SELECT id, name_ja, name_ja_source, name_en
FROM abilities
WHERE id = ?;

-- name: GetAbilityEffect :one
SELECT ability_id, effect
FROM ability_effects
WHERE ability_id = ?;

-- name: ListRegulations :many
SELECT id, name_ja, is_default, starts_on, ends_on
FROM regulations
ORDER BY id;

-- name: GetDataVersion :one
SELECT source, version, checksum, imported_at
FROM data_versions
WHERE source = ?;

-- name: ListDataVersions :many
SELECT source, version, checksum, imported_at
FROM data_versions
ORDER BY source;

-- 冪等な投入(importer.Apply。ADR-0101 §9)。全置き換えを1トランザクションで行う。
-- 自己参照の外部キー(species.base_species_key)があるので、削除はメガを先・挿入はメガを後にする。

-- name: ListSpeciesKeys :many
SELECT `key`, showdown_id
FROM species;

-- name: DeleteRegulationSpecies :exec
DELETE FROM regulation_species;

-- name: DeleteRegulationMoves :exec
DELETE FROM regulation_moves;

-- name: DeleteRegulationItems :exec
DELETE FROM regulation_items;

-- name: DeleteRegulationAbilities :exec
DELETE FROM regulation_abilities;

-- name: DeleteRegulations :exec
DELETE FROM regulations;

-- name: DeleteLearnsets :exec
DELETE FROM learnsets;

-- name: DeleteSpeciesAbilities :exec
DELETE FROM species_abilities;

-- name: DeleteItemEffects :exec
DELETE FROM item_effects;

-- name: DeleteAbilityEffects :exec
DELETE FROM ability_effects;

-- name: DeleteMegaSpecies :exec
DELETE FROM species WHERE is_mega = 1;

-- name: DeleteRemainingSpecies :exec
DELETE FROM species;

-- name: DeleteMoves :exec
DELETE FROM moves;

-- name: DeleteItems :exec
DELETE FROM items;

-- name: DeleteAbilities :exec
DELETE FROM abilities;

-- name: DeleteTypeChart :exec
DELETE FROM type_chart;

-- name: DeleteTypes :exec
DELETE FROM types;

-- name: DeleteDataVersions :exec
DELETE FROM data_versions;

-- name: InsertType :exec
INSERT INTO types (id, sort_order, name_ja, name_ja_source)
VALUES (?, ?, ?, ?);

-- name: InsertTypeChart :exec
INSERT INTO type_chart (attack_type, defense_type, code)
VALUES (?, ?, ?);

-- name: InsertAbility :exec
INSERT INTO abilities (id, name_ja, name_ja_source, name_en)
VALUES (?, ?, ?, ?);

-- name: InsertItem :exec
INSERT INTO items (id, name_ja, name_ja_source, name_en)
VALUES (?, ?, ?, ?);

-- name: InsertMove :exec
INSERT INTO moves (id, name_ja, name_ja_source, name_en, type, category, power, accuracy, pp, priority)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertSpecies :exec
INSERT INTO species (`key`, dex_no, form, showdown_id, name_ja, name_ja_source, name_en, type1, type2,
                      base_hp, base_atk, base_def, base_spa, base_spd, base_spe, is_mega, base_species_key, required_item_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertSpeciesAbility :exec
INSERT INTO species_abilities (species_key, slot, ability_id)
VALUES (?, ?, ?);

-- name: InsertItemEffect :exec
INSERT INTO item_effects (item_id, effect)
VALUES (?, ?);

-- name: InsertAbilityEffect :exec
INSERT INTO ability_effects (ability_id, effect)
VALUES (?, ?);

-- name: InsertLearnset :exec
INSERT INTO learnsets (species_key, move_id)
VALUES (?, ?);

-- name: InsertRegulation :exec
INSERT INTO regulations (id, name_ja, is_default, starts_on, ends_on)
VALUES (?, ?, ?, ?, ?);

-- name: InsertRegulationSpecies :exec
INSERT INTO regulation_species (regulation_id, species_key)
VALUES (?, ?);

-- name: InsertRegulationMove :exec
INSERT INTO regulation_moves (regulation_id, move_id)
VALUES (?, ?);

-- name: InsertRegulationItem :exec
INSERT INTO regulation_items (regulation_id, item_id)
VALUES (?, ?);

-- name: InsertRegulationAbility :exec
INSERT INTO regulation_abilities (regulation_id, ability_id)
VALUES (?, ?);

-- name: InsertDataVersion :exec
INSERT INTO data_versions (source, version, checksum, imported_at)
VALUES (?, ?, ?, ?);
