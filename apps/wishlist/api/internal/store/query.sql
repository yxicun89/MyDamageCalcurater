-- wishlist の DB アクセス(sqlc)。商品×ジャンル×サイトの CRUD(フェーズ1)。
-- 並び: 商品は sort_order 昇順・同順は id 降順、ジャンルは sort_order 昇順・同順は id 昇順、サイトは id 昇順。

-- name: ListGenres :many
SELECT id, name, query_template, sort_order FROM genres ORDER BY sort_order, id;

-- name: GetGenre :one
SELECT id, name, query_template, sort_order FROM genres WHERE id = ?;

-- name: GetGenreForUpdate :one
SELECT id, name, query_template, sort_order FROM genres WHERE id = ? FOR UPDATE;

-- name: CreateGenre :execlastid
INSERT INTO genres (name, query_template, sort_order) VALUES (?, ?, ?);

-- name: UpdateGenre :execrows
UPDATE genres SET name = ?, query_template = ?, sort_order = ? WHERE id = ?;

-- name: ListGenreSites :many
SELECT genre_id, site_id, sort_order FROM genre_sites ORDER BY genre_id, sort_order, site_id;

-- name: ListGenreSitesByGenre :many
SELECT genre_id, site_id, sort_order FROM genre_sites WHERE genre_id = ? ORDER BY sort_order, site_id;

-- name: DeleteGenreSites :exec
DELETE FROM genre_sites WHERE genre_id = ?;

-- name: InsertGenreSite :exec
INSERT INTO genre_sites (genre_id, site_id, sort_order) VALUES (?, ?, ?);

-- name: ListSites :many
SELECT id, name, search_url_template, fetch_type, is_reference FROM sites ORDER BY id;

-- name: GetSite :one
SELECT id, name, search_url_template, fetch_type, is_reference FROM sites WHERE id = ?;

-- name: CreateSite :execlastid
INSERT INTO sites (name, search_url_template, fetch_type, is_reference) VALUES (?, ?, ?, ?);

-- name: UpdateSite :execrows
UPDATE sites SET name = ?, search_url_template = ?, fetch_type = ?, is_reference = ? WHERE id = ?;

-- name: ListItems :many
SELECT id, genre_id, name, option_text, query_override, image_path, source_url, min_price, sort_order, created_at, updated_at
FROM items ORDER BY sort_order, id DESC;

-- name: ListItemsByGenre :many
SELECT id, genre_id, name, option_text, query_override, image_path, source_url, min_price, sort_order, created_at, updated_at
FROM items WHERE genre_id = ? ORDER BY sort_order, id DESC;

-- name: GetItem :one
SELECT id, genre_id, name, option_text, query_override, image_path, source_url, min_price, sort_order, created_at, updated_at
FROM items WHERE id = ?;

-- name: GetItemForUpdate :one
SELECT id, genre_id, name, option_text, query_override, image_path, source_url, min_price, sort_order, created_at, updated_at
FROM items WHERE id = ? FOR UPDATE;

-- name: CreateItem :execlastid
INSERT INTO items (genre_id, name, option_text, query_override, image_path, source_url, min_price, sort_order)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateItem :exec
UPDATE items
SET genre_id = ?, name = ?, option_text = ?, query_override = ?, image_path = ?, source_url = ?, min_price = ?, sort_order = ?
WHERE id = ?;

-- name: TouchItem :exec
UPDATE items SET updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: DeleteItem :execrows
DELETE FROM items WHERE id = ?;

-- name: ListItemSiteOverrides :many
SELECT item_id, site_id, query, enabled FROM item_site_overrides ORDER BY item_id, site_id;

-- name: ListItemSiteOverridesByItem :many
SELECT item_id, site_id, query, enabled FROM item_site_overrides WHERE item_id = ? ORDER BY site_id;

-- name: DeleteItemSiteOverrides :exec
DELETE FROM item_site_overrides WHERE item_id = ?;

-- name: InsertItemSiteOverride :exec
INSERT INTO item_site_overrides (item_id, site_id, query, enabled) VALUES (?, ?, ?, ?);
