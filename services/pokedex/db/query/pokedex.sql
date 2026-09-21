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
