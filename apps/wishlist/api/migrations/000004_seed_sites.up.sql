-- 確認済みサイトの初期データ(docs/sites.md)。Yahoo!フリマ・カードラッシュ・あみあみ・Yahoo!ショッピング・駿河屋の 5 サイト。
-- すでに使われている DB に流しても壊れないように、sites は id を書かず、同じ名前があれば足さない(既存の行は変えない)。
-- genre_sites は名前で引き、無い組だけを足す(既存の行・sort_order は変えない)。ジャンルは足さない。
-- 未確認のサイトは入れない(sites.md に人が登録する候補として残す)。

INSERT INTO sites (name, search_url_template, fetch_type, is_reference)
SELECT v.name, v.search_url_template, v.fetch_type, v.is_reference
FROM (
  SELECT 'Yahoo!フリマ' AS name, 'https://paypayfleamarket.yahoo.co.jp/search/{q}' AS search_url_template, 'scrape' AS fetch_type, FALSE AS is_reference
  UNION ALL
  SELECT 'カードラッシュ', 'https://www.cardrush-dm.jp/product-list?keyword={q}&order=asc&available=1&num=20', 'scrape', TRUE
  UNION ALL
  SELECT 'あみあみ', 'https://slist.amiami.jp/top/search/list?s_keywords={q}&s_sortkey=pricea', 'scrape', TRUE
  UNION ALL
  SELECT 'Yahoo!ショッピング', 'https://shopping.yahoo.co.jp/search/{q}/0/?X=2', 'api', TRUE
  UNION ALL
  SELECT '駿河屋', 'https://www.suruga-ya.jp/search?category=&search_word={q}&rankBy=price%3Aascending&inStock=On', 'link_only', FALSE
) AS v
WHERE NOT EXISTS (SELECT 1 FROM sites s WHERE s.name = v.name);

-- 表示順。メルカリ 10・Amazon 20(000002)の間と前に、各ジャンル内で重ならない値を置く。
INSERT INTO genre_sites (genre_id, site_id, sort_order)
SELECT g.id, s.id, v.sort_order
FROM (
  SELECT 'デュエル・マスターズ' AS genre_name, 'カードラッシュ' AS site_name, 5 AS sort_order
  UNION ALL SELECT 'デュエル・マスターズ', 'Yahoo!フリマ', 8
  UNION ALL SELECT 'デュエル・マスターズ', 'Yahoo!ショッピング', 15
  UNION ALL SELECT 'S.H.Figuarts', 'あみあみ', 2
  UNION ALL SELECT 'S.H.Figuarts', '駿河屋', 5
  UNION ALL SELECT 'S.H.Figuarts', 'Yahoo!フリマ', 8
  UNION ALL SELECT 'S.H.Figuarts', 'Yahoo!ショッピング', 15
  UNION ALL SELECT 'ガンプラ', 'あみあみ', 2
  UNION ALL SELECT 'ガンプラ', '駿河屋', 5
  UNION ALL SELECT 'ガンプラ', 'Yahoo!フリマ', 8
  UNION ALL SELECT 'ガンプラ', 'Yahoo!ショッピング', 15
  UNION ALL SELECT 'ポケモングッズ', '駿河屋', 5
  UNION ALL SELECT 'ポケモングッズ', 'Yahoo!フリマ', 8
  UNION ALL SELECT 'ポケモングッズ', 'Yahoo!ショッピング', 15
) AS v
JOIN genres g ON g.name = v.genre_name
JOIN sites s ON s.name = v.site_name
WHERE NOT EXISTS (SELECT 1 FROM genre_sites x WHERE x.genre_id = g.id AND x.site_id = s.id);
