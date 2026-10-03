-- 空の DB に seed だけを戻す用。サイト 1・2 の削除は CASCADE で、そのサイトの紐づけ・商品ごとの上書き・取得結果も消す。
-- ジャンル 1〜4 を使う商品があれば外部キー(RESTRICT)で失敗する(利用データを黙って消さない)。
DELETE FROM genre_sites WHERE genre_id IN (1, 2, 3, 4) AND site_id IN (1, 2);
DELETE FROM sites WHERE id IN (1, 2);
DELETE FROM genres WHERE id IN (1, 2, 3, 4);
