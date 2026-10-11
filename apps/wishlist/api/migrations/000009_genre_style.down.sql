-- 000009 を戻す。
-- Yahoo!ショッピングは 000004 の名前と URL が一致し scrape の行だけを api に戻す。
-- METAL BUILD・ROBOT魂 は 000009 が入れた値(色・絵柄)のままで、商品が使っていないときだけ消す(紐づけは CASCADE で消える)。
-- 既存の 4 ジャンルの行は列ごと消える値以外は変えない。
UPDATE sites SET fetch_type = 'api'
WHERE name = 'Yahoo!ショッピング' AND search_url_template = 'https://shopping.yahoo.co.jp/search/{q}/0/?X=2' AND fetch_type = 'scrape';

DELETE FROM genres
WHERE name = 'METAL BUILD・ROBOT魂' AND color = '#5b6b7f' AND icon = 'robot'
  AND NOT EXISTS (SELECT 1 FROM items i WHERE i.genre_id = genres.id);

ALTER TABLE genres
  DROP CHECK chk_genres_color,
  DROP COLUMN match_keywords,
  DROP COLUMN match_hosts,
  DROP COLUMN icon,
  DROP COLUMN color;
