-- 000006 で足した 3 サイト(魂ウェブ・ポケモンセンターオンライン・プレバン)だけを消す。
-- 名前と URL の両方が一致する行だけが対象(ユーザーが同じ名前を別の URL で登録していた行は消さない)。
-- 紐づけを先に消す。ジャンルは消さない。
DELETE gs FROM genre_sites gs JOIN sites s ON s.id = gs.site_id
WHERE (s.name = '魂ウェブ' AND s.search_url_template = 'https://tamashiiweb.com/item/?wo={q}')
   OR (s.name = 'ポケモンセンターオンライン' AND s.search_url_template = 'https://www.pokemoncenter-online.com/search/?q={q}')
   OR (s.name = 'プレバン' AND s.search_url_template = 'https://p-bandai.jp/search_bst/?q={q}');

DELETE FROM sites
WHERE (name = '魂ウェブ' AND search_url_template = 'https://tamashiiweb.com/item/?wo={q}')
   OR (name = 'ポケモンセンターオンライン' AND search_url_template = 'https://www.pokemoncenter-online.com/search/?q={q}')
   OR (name = 'プレバン' AND search_url_template = 'https://p-bandai.jp/search_bst/?q={q}');
