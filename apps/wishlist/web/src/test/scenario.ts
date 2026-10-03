import { makeGenre, makeItem, makeSite } from "./factories";

/** 画面テスト共通の初期データ。ジャンルの sort_order は id の逆順(= 並び替えを確かめられる)。 */
export function scenario() {
  const sites = [
    makeSite({
      id: 1,
      name: "メルカリ",
      search_url_template: "https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc",
    }),
    makeSite({
      id: 2,
      name: "Amazon",
      search_url_template: "https://www.amazon.co.jp/s?k={q}&s=price-asc-rank",
    }),
    makeSite({ id: 3, name: "その他", search_url_template: "https://other.example/?q={q}" }),
  ];
  const genres = [
    makeGenre({
      id: 1,
      name: "デュエマ",
      query_template: "{name} {option}",
      sort_order: 2,
      site_ids: [1, 2],
    }),
    makeGenre({
      id: 2,
      name: "S.H.Figuarts",
      query_template: "S.H.Figuarts {name}",
      sort_order: 1,
      site_ids: [2, 1, 3],
    }),
  ];
  const items = [
    makeItem({ id: 11, genre_id: 1, name: "ボルシャック", option_text: "銀トレジャー" }),
    makeItem({ id: 12, genre_id: 2, name: "グリス" }),
    makeItem({ id: 13, genre_id: 2, name: "ビルド" }),
  ];
  return { sites, genres, items };
}
