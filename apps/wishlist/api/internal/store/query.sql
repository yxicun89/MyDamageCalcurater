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

-- 目安価格(フェーズ3)。listings は商品×サイトごとに最新の取得分だけ残す(取得のたびに消して入れ直す)。

-- name: DeleteListingsBySite :exec
DELETE FROM listings WHERE item_id = ? AND site_id = ?;

-- name: InsertListing :exec
INSERT INTO listings (item_id, site_id, title, price, url, image_url, in_stock, suspicious_reasons, fetched_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpsertEstimate :exec
INSERT INTO estimates (item_id, site_id, low, mid, `count`, suspicious_count, in_stock_count, status, fetched_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE low = VALUES(low), mid = VALUES(mid), `count` = VALUES(`count`), suspicious_count = VALUES(suspicious_count),
  in_stock_count = VALUES(in_stock_count), status = VALUES(status), fetched_at = VALUES(fetched_at);

-- name: MarkEstimateFailed :exec
INSERT INTO estimates (item_id, site_id, low, mid, `count`, suspicious_count, in_stock_count, status, fetched_at)
VALUES (?, ?, NULL, NULL, 0, 0, 0, 'failed', ?)
ON DUPLICATE KEY UPDATE status = 'failed';

-- name: ListEstimatesByItem :many
SELECT item_id, site_id, low, mid, `count`, suspicious_count, in_stock_count, status, fetched_at
FROM estimates WHERE item_id = ? ORDER BY site_id;

-- name: ListListingsByItem :many
SELECT id, item_id, site_id, title, price, url, image_url, in_stock, suspicious_reasons, fetched_at
FROM listings WHERE item_id = ? ORDER BY price, id;

-- name: ListListingsByItemSite :many
SELECT id, item_id, site_id, title, price, url, image_url, in_stock, suspicious_reasons, fetched_at
FROM listings WHERE item_id = ? AND site_id = ? ORDER BY price, id;

-- name: ListGenreAliases :many
SELECT id, genre_id, group_no, alias, normalized FROM genre_aliases ORDER BY genre_id, group_no, id;

-- name: ListGenreAliasesByGenre :many
SELECT id, genre_id, group_no, alias, normalized FROM genre_aliases WHERE genre_id = ? ORDER BY group_no, id;

-- name: DeleteGenreAliases :exec
DELETE FROM genre_aliases WHERE genre_id = ?;

-- name: InsertGenreAlias :exec
INSERT INTO genre_aliases (genre_id, group_no, alias, normalized) VALUES (?, ?, ?, ?);

-- 価格の推移(フェーズ4-2)。day は JST の日付。
-- name: UpsertPriceHistory :exec
INSERT INTO price_history (item_id, site_id, day, low, mid, `count`, recorded_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE low = VALUES(low), mid = VALUES(mid), `count` = VALUES(`count`), recorded_at = VALUES(recorded_at);

-- name: ListPriceHistory :many
SELECT item_id, site_id, day, low, mid, `count`, recorded_at
FROM price_history WHERE item_id = ? AND day >= ? ORDER BY site_id, day;

-- name: PrunePriceHistory :execrows
DELETE FROM price_history WHERE day < ?;
