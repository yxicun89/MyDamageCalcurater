-- 000004 で足した 5 サイト(Yahoo!フリマ・カードラッシュ・あみあみ・Yahoo!ショッピング・駿河屋)だけを消す。
-- 名前と URL の両方が一致する行だけが対象(ユーザーが同じ名前を別の URL で登録していた行は消さない)。
-- 紐づけを先に消す。ジャンルは消さない。
DELETE gs FROM genre_sites gs JOIN sites s ON s.id = gs.site_id
WHERE (s.name = 'Yahoo!フリマ' AND s.search_url_template = 'https://paypayfleamarket.yahoo.co.jp/search/{q}')
   OR (s.name = 'カードラッシュ' AND s.search_url_template = 'https://www.cardrush-dm.jp/product-list?keyword={q}&order=asc&available=1&num=20')
   OR (s.name = 'あみあみ' AND s.search_url_template = 'https://slist.amiami.jp/top/search/list?s_keywords={q}&s_sortkey=pricea')
   OR (s.name = 'Yahoo!ショッピング' AND s.search_url_template = 'https://shopping.yahoo.co.jp/search/{q}/0/?X=2')
   OR (s.name = '駿河屋' AND s.search_url_template = 'https://www.suruga-ya.jp/search?category=&search_word={q}&rankBy=price%3Aascending&inStock=On');

DELETE FROM sites
WHERE (name = 'Yahoo!フリマ' AND search_url_template = 'https://paypayfleamarket.yahoo.co.jp/search/{q}')
   OR (name = 'カードラッシュ' AND search_url_template = 'https://www.cardrush-dm.jp/product-list?keyword={q}&order=asc&available=1&num=20')
   OR (name = 'あみあみ' AND search_url_template = 'https://slist.amiami.jp/top/search/list?s_keywords={q}&s_sortkey=pricea')
   OR (name = 'Yahoo!ショッピング' AND search_url_template = 'https://shopping.yahoo.co.jp/search/{q}/0/?X=2')
   OR (name = '駿河屋' AND search_url_template = 'https://www.suruga-ya.jp/search?category=&search_word={q}&rankBy=price%3Aascending&inStock=On');
