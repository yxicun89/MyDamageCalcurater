-- フェーズ5 5-1b・5-1c・5-1e(docs/phase5-spec.md。ユーザー決定 2026-10-11)。
-- 1. genres に色(color。#rrggbb の小文字)・絵柄(icon)・URL からジャンルを推定する手がかり(match_hosts・match_keywords。JSON の文字列配列)を足す
-- 2. 既存の 4 ジャンルに初期値を入れる(名前で引く。無ければ何もしない)
-- 3. ジャンル「METAL BUILD・ROBOT魂」を足す(同名があれば足さない)。表示するサイトは S.H.Figuarts と同じ(足したジャンルにだけ写す)
-- 4. Yahoo!ショッピングを api(appid)から scrape(検索ページの __NEXT_DATA__)に替える(000004 の名前と URL が一致する行だけ)
-- すでに使われている DB に流しても壊れないように、id を書かず名前で引く(000004・000006 と同じ作り方)。
ALTER TABLE genres
  ADD COLUMN color CHAR(7) NOT NULL DEFAULT '#1d5fd1',
  ADD COLUMN icon ENUM('card', 'figure', 'robot', 'goods', 'star', 'box') NOT NULL DEFAULT 'box',
  ADD COLUMN match_hosts JSON NOT NULL DEFAULT (JSON_ARRAY()),
  ADD COLUMN match_keywords JSON NOT NULL DEFAULT (JSON_ARRAY()),
  ADD CONSTRAINT chk_genres_color CHECK (REGEXP_LIKE(color, '^#[0-9a-f]{6}$', 'c'));

-- 空白を含む語(HG など)は正規化で空白が消えるので、空白なしで入れる。
UPDATE genres SET color = '#e0393e', icon = 'card',
  match_hosts = JSON_ARRAY('dm.takaratomy.co.jp'), match_keywords = JSON_ARRAY('デュエル・マスターズ', 'デュエマ')
WHERE name = 'デュエル・マスターズ';
UPDATE genres SET color = '#7c4dff', icon = 'figure',
  match_hosts = JSON_ARRAY(), match_keywords = JSON_ARRAY('S.H.Figuarts', 'フィギュアーツ')
WHERE name = 'S.H.Figuarts';
UPDATE genres SET color = '#1f6fe5', icon = 'robot',
  match_hosts = JSON_ARRAY('bandai-hobby.net'), match_keywords = JSON_ARRAY('ガンプラ', 'HG', 'MG', 'RG', 'PG', '30MF', '30MM')
WHERE name = 'ガンプラ';
UPDATE genres SET color = '#f2b705', icon = 'goods',
  match_hosts = JSON_ARRAY('www.pokemoncenter-online.com'), match_keywords = JSON_ARRAY()
WHERE name = 'ポケモングッズ';

INSERT INTO genres (name, query_template, sort_order, color, icon, match_hosts, match_keywords)
SELECT 'METAL BUILD・ROBOT魂', '{name} {option}', 50, '#5b6b7f', 'robot', JSON_ARRAY(), JSON_ARRAY('METAL BUILD', 'ROBOT魂', 'METAL ROBOT魂')
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM genres g WHERE g.name = 'METAL BUILD・ROBOT魂');

-- 紐づけは、METAL BUILD・ROBOT魂 に 1 つも紐づけが無いときだけ S.H.Figuarts から写す(ユーザーが作っていた同名ジャンルの紐づけは変えない)。
INSERT INTO genre_sites (genre_id, site_id, sort_order)
SELECT m.id, gs.site_id, gs.sort_order
FROM genres m
JOIN genres f ON f.name = 'S.H.Figuarts'
JOIN genre_sites gs ON gs.genre_id = f.id
WHERE m.name = 'METAL BUILD・ROBOT魂'
  AND NOT EXISTS (SELECT 1 FROM (SELECT genre_id FROM genre_sites) x WHERE x.genre_id = m.id);

UPDATE sites SET fetch_type = 'scrape'
WHERE name = 'Yahoo!ショッピング' AND search_url_template = 'https://shopping.yahoo.co.jp/search/{q}/0/?X=2' AND fetch_type = 'api';
