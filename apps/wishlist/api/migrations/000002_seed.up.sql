-- 初期データ。サイトは URL を確認済みの 2 件だけ(apps/wishlist/CLAUDE.md §5)。
-- 他サイト(Yahoo!フリマ・カードラッシュ・ドラゴンスター・あみあみ・駿河屋・Yahoo!ショッピング・
-- プレバン・魂ウェブ・ポケモンセンターオンライン)は、人が実際に検索して URL を確かめてから設定画面で登録する。
-- id を明示するのは genre_sites の紐づけと down の削除を確実にするため。

INSERT INTO genres (id, name, query_template, sort_order) VALUES
  (1, 'デュエル・マスターズ', '{name} {option}', 10),
  (2, 'S.H.Figuarts', 'S.H.Figuarts {name}', 20),
  (3, 'ガンプラ', '{name}', 30),
  (4, 'ポケモングッズ', '{name} {option}', 40);

INSERT INTO sites (id, name, search_url_template, fetch_type, is_reference) VALUES
  (1, 'メルカリ', 'https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc', 'headless', FALSE),
  (2, 'Amazon', 'https://www.amazon.co.jp/s?k={q}&s=price-asc-rank', 'link_only', FALSE);

INSERT INTO genre_sites (genre_id, site_id, sort_order) VALUES
  (1, 1, 10), (1, 2, 20),
  (2, 1, 10), (2, 2, 20),
  (3, 1, 10), (3, 2, 20),
  (4, 1, 10), (4, 2, 20);
