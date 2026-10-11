package item_test

import (
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// フェーズ5 5-1c: URL からの下書きのジャンル推定(docs/phase5-spec.md)。GuessGenre は純粋な関数。
// 初期値(migration 000009)と同じ手がかりを持つジャンルの一覧で確かめる。

func seedGenres() []item.Genre {
	return []item.Genre{
		// 並びは sort_order 順にしない(GuessGenre は並びに依らない)。
		{ID: 5, Name: "METAL BUILD・ROBOT魂", SortOrder: 50, MatchKeywords: []string{"METAL BUILD", "ROBOT魂", "METAL ROBOT魂"}},
		{ID: 1, Name: "デュエル・マスターズ", SortOrder: 10, MatchHosts: []string{"dm.takaratomy.co.jp"}, MatchKeywords: []string{"デュエル・マスターズ", "デュエマ"}},
		{ID: 3, Name: "ガンプラ", SortOrder: 30, MatchHosts: []string{"bandai-hobby.net"}, MatchKeywords: []string{"ガンプラ", "HG", "MG", "RG", "PG", "30MF", "30MM"}},
		{ID: 2, Name: "S.H.Figuarts", SortOrder: 20, MatchKeywords: []string{"S.H.Figuarts", "フィギュアーツ"}},
		{ID: 4, Name: "ポケモングッズ", SortOrder: 40, MatchHosts: []string{"www.pokemoncenter-online.com"}},
	}
}

// AC-5-1c-6: 語(正規化して含む)→ ホスト → 決まらない、の順。
func TestGuessGenre(t *testing.T) {
	cases := []struct {
		name   string
		genres []item.Genre
		title  string
		host   string
		want   int64 // 0 は「決まらない」
	}{
		{"デュエマのカード(語なし・ホストで)", seedGenres(), "光る！流れる!!ベンザマン!!!", "dm.takaratomy.co.jp", 1},
		{"魂ウェブの S.H.Figuarts(語で。ホストは誰の手がかりでもない)", seedGenres(), "S.H.Figuarts ウルトラマンノア", "tamashiiweb.com", 2},
		{"表記揺れも正規化で当たる(全角・記号なし)", seedGenres(), "ＳＨＦｉｇｕａｒｔｓ 仮面ライダーグリス", "shop.example.com", 2},
		{"ポケセン(ホストで)", seedGenres(), "ぬいぐるみ メガアブソル", "www.pokemoncenter-online.com", 4},
		{"バンダイ ホビーサイトの 30MF(語で)", seedGenres(), "30MF カスタマイズストラクチャー3", "bandai-hobby.net", 3},
		{"語はホストより先(ポケセンのホストでもデュエマの語があればデュエマ)", seedGenres(), "デュエマ プレイマット", "www.pokemoncenter-online.com", 1},
		{"最も長い語が勝つ(METAL ROBOT魂 > ROBOT魂、HG より長い)", seedGenres(), "METAL ROBOT魂 ガンダムエアリアル HG", "tamashiiweb.com", 5},
		{"長い語が勝つ(フィギュアーツ 7 文字 > HG 2 文字)", seedGenres(), "フィギュアーツ HG 風", "", 2},
		{"同じ長さなら sort_order が小さいもの", []item.Genre{
			{ID: 7, SortOrder: 20, MatchKeywords: []string{"テスト"}},
			{ID: 8, SortOrder: 10, MatchKeywords: []string{"ゲーム"}},
		}, "テスト ゲーム", "", 8},
		{"同じ長さ・同じ sort_order なら id が小さいもの", []item.Genre{
			{ID: 9, SortOrder: 10, MatchKeywords: []string{"テスト"}},
			{ID: 6, SortOrder: 10, MatchKeywords: []string{"ゲーム"}},
		}, "テスト ゲーム", "", 6},
		{"ホストは大文字小文字を区別しない", seedGenres(), "名前", "DM.TakaraTomy.co.jp", 1},
		{"ホストは完全一致(サブドメインは別)", seedGenres(), "名前", "evil.dm.takaratomy.co.jp", 0},
		{"ホストが複数のジャンルに当たれば sort_order が小さいもの", []item.Genre{
			{ID: 11, SortOrder: 30, MatchHosts: []string{"shop.example.com"}},
			{ID: 12, SortOrder: 5, MatchHosts: []string{"shop.example.com"}},
		}, "名前", "shop.example.com", 12},
		{"どれにも当たらない → 決まらない", seedGenres(), "ふつうの名前", "shop.example.com", 0},
		{"名前が空・ホストも空 → 決まらない", seedGenres(), "", "", 0},
		{"正規化して空になる語は使わない(何にでも当たらない)", []item.Genre{{ID: 13, SortOrder: 1, MatchKeywords: []string{"・ "}}}, "名前", "", 0},
		{"ジャンルが無い → 決まらない", nil, "S.H.Figuarts", "dm.takaratomy.co.jp", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := item.GuessGenre(c.genres, c.title, c.host)
			if c.want == 0 {
				if ok {
					t.Errorf("GuessGenre = %d, true; want 決まらない", got)
				}
				return
			}
			if !ok || got != c.want {
				t.Errorf("GuessGenre = %d, %v; want %d", got, ok, c.want)
			}
		})
	}
}
