-- リンク用サイト(取得しない。人がブラウザで開く検索リンク)の初期データ(docs/sites.md)。魂ウェブ・ポケモンセンターオンライン・プレバンの 3 サイト。すべて link_only・基準サイトにしない。
-- プレバンの検索ページは robots.txt で機械取得が禁止なので、取得はせずリンクだけを出す(検索語の文字コードは未確認。sites.md)。
-- すでに使われている DB に流しても壊れないように、sites は id を書かず、同じ名前があれば足さない(既存の行は変えない)。
-- genre_sites は名前で引き、無い組だけを足す(既存の行・sort_order は変えない)。ジャンルは足さない。

INSERT INTO sites (name, search_url_template, fetch_type, is_reference)
SELECT v.name, v.search_url_template, v.fetch_type, v.is_reference
FROM (
  SELECT '魂ウェブ' AS name, 'https://tamashiiweb.com/item/?wo={q}' AS search_url_template, 'link_only' AS fetch_type, FALSE AS is_reference
  UNION ALL
  SELECT 'ポケモンセンターオンライン', 'https://www.pokemoncenter-online.com/search/?q={q}', 'link_only', FALSE
  UNION ALL
  SELECT 'プレバン', 'https://p-bandai.jp/search_bst/?q={q}', 'link_only', FALSE
) AS v
WHERE NOT EXISTS (SELECT 1 FROM sites s WHERE s.name = v.name);

-- 表示順。各ジャンルの末尾(Amazon 20 の後)に置く。
INSERT INTO genre_sites (genre_id, site_id, sort_order)
SELECT g.id, s.id, v.sort_order
FROM (
  SELECT 'S.H.Figuarts' AS genre_name, '魂ウェブ' AS site_name, 30 AS sort_order
  UNION ALL SELECT 'S.H.Figuarts', 'プレバン', 25
  UNION ALL SELECT 'ガンプラ', 'プレバン', 25
  UNION ALL SELECT 'ポケモングッズ', 'ポケモンセンターオンライン', 25
) AS v
JOIN genres g ON g.name = v.genre_name
JOIN sites s ON s.name = v.site_name
WHERE NOT EXISTS (SELECT 1 FROM genre_sites x WHERE x.genre_id = g.id AND x.site_id = s.id);
