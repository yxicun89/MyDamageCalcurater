package item

// GuessGenre は商品名 name と URL のホスト host からジャンルを推定する(フェーズ5 5-1c。docs/phase5-spec.md)。純粋な関数。
//  1. name を query.Normalize した文字列が、MatchKeywords の語(query.Normalize したもの。空になる語は使わない)を含むジャンル。
//     複数当たれば、当たった語(正規化後)が最も長い(rune 数)もの、同じ長さなら SortOrder が小さいもの、さらに同じなら ID が小さいもの
//  2. 1 で決まらなければ、host(大文字小文字・末尾の . は無視)が MatchHosts のどれかと一致するジャンル。
//     複数なら SortOrder が小さいもの、同じなら ID が小さいもの
//
// 決まらなければ (0, false)。genres の並びには依らない。
//
// TODO(5-1c 実装者): 未実装。spec-writer が置いた空の実装(guess_test.go が失敗する)。
func GuessGenre(genres []Genre, name, host string) (int64, bool) {
	_, _, _ = genres, name, host
	return 0, false
}
