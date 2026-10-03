-- wishlist のテーブル(apps/wishlist/CLAUDE.md §7。追加・具体化は docs/design.md §4)。
-- 削除の扱い: サイトを消すと、そのサイトへの紐づけ(genre_sites・item_site_overrides)と取得結果(listings・estimates)も消える(CASCADE)。
-- 商品を消すと子も消える(CASCADE)。ジャンルは商品が使っていれば消せない(items.genre_id は RESTRICT)。

CREATE TABLE genres (
  id BIGINT NOT NULL AUTO_INCREMENT,
  name VARCHAR(64) NOT NULL,
  query_template VARCHAR(255) NOT NULL DEFAULT '{name} {option}',
  sort_order INT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uq_genres_name (name),
  CONSTRAINT chk_genres_name CHECK (CHAR_LENGTH(name) > 0),
  CONSTRAINT chk_genres_query_template CHECK (CHAR_LENGTH(query_template) > 0)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE sites (
  id BIGINT NOT NULL AUTO_INCREMENT,
  name VARCHAR(64) NOT NULL,
  search_url_template VARCHAR(1024) NOT NULL,
  fetch_type ENUM('api', 'scrape', 'headless', 'link_only') NOT NULL DEFAULT 'link_only',
  -- 基準価格の算出に使うか(ショップ系)
  is_reference BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (id),
  UNIQUE KEY uq_sites_name (name),
  CONSTRAINT chk_sites_name CHECK (CHAR_LENGTH(name) > 0),
  CONSTRAINT chk_sites_search_url_template CHECK (search_url_template LIKE '%{q}%')
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE genre_sites (
  genre_id BIGINT NOT NULL,
  site_id BIGINT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  PRIMARY KEY (genre_id, site_id),
  KEY idx_genre_sites_site (site_id),
  CONSTRAINT fk_genre_sites_genre FOREIGN KEY (genre_id) REFERENCES genres (id) ON DELETE CASCADE,
  CONSTRAINT fk_genre_sites_site FOREIGN KEY (site_id) REFERENCES sites (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE items (
  id BIGINT NOT NULL AUTO_INCREMENT,
  genre_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  option_text VARCHAR(255) NULL,
  query_override VARCHAR(255) NULL,
  -- 画像ストレージ内のファイル名(UUID v4 + 拡張子)。URL は /images/<image_path>
  image_path VARCHAR(512) NOT NULL,
  source_url VARCHAR(1024) NULL,
  min_price INT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_items_genre_sort (genre_id, sort_order),
  -- 使われているジャンルは消せない(RESTRICT)
  CONSTRAINT fk_items_genre FOREIGN KEY (genre_id) REFERENCES genres (id),
  CONSTRAINT chk_items_name CHECK (CHAR_LENGTH(name) > 0),
  CONSTRAINT chk_items_min_price CHECK (min_price IS NULL OR min_price >= 0)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE item_site_overrides (
  item_id BIGINT NOT NULL,
  site_id BIGINT NOT NULL,
  query VARCHAR(255) NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  PRIMARY KEY (item_id, site_id),
  KEY idx_item_site_overrides_site (site_id),
  CONSTRAINT fk_item_site_overrides_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE CASCADE,
  CONSTRAINT fk_item_site_overrides_site FOREIGN KEY (site_id) REFERENCES sites (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

-- 商品×サイトごとに最新の取得分だけ残す(古いものは取得のたびに削除。フェーズ3)
CREATE TABLE listings (
  id BIGINT NOT NULL AUTO_INCREMENT,
  item_id BIGINT NOT NULL,
  site_id BIGINT NOT NULL,
  title VARCHAR(512) NOT NULL,
  price INT NOT NULL,
  url VARCHAR(1024) NOT NULL,
  image_url VARCHAR(1024) NULL,
  in_stock BOOLEAN NOT NULL DEFAULT TRUE,
  -- 例 ["title_mismatch","too_cheap"]。NULL または [] は参考にしている出品
  suspicious_reasons JSON NULL,
  fetched_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  KEY idx_item_site_fetched (item_id, site_id, fetched_at),
  KEY idx_listings_site (site_id),
  CONSTRAINT fk_listings_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE CASCADE,
  CONSTRAINT fk_listings_site FOREIGN KEY (site_id) REFERENCES sites (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;

CREATE TABLE estimates (
  item_id BIGINT NOT NULL,
  site_id BIGINT NOT NULL,
  low INT NULL,
  mid INT NULL,
  count INT NOT NULL DEFAULT 0,
  suspicious_count INT NOT NULL DEFAULT 0,
  status ENUM('ok', 'failed', 'no_result') NOT NULL,
  fetched_at DATETIME NOT NULL,
  PRIMARY KEY (item_id, site_id),
  KEY idx_estimates_site (site_id),
  CONSTRAINT fk_estimates_item FOREIGN KEY (item_id) REFERENCES items (id) ON DELETE CASCADE,
  CONSTRAINT fk_estimates_site FOREIGN KEY (site_id) REFERENCES sites (id) ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
