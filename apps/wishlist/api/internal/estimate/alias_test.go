package estimate_test

import (
	"slices"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/estimate"
)

// フェーズ4-1 表記揺れの辞書(docs/phase4-spec.md AC-A4〜A7)。

// figuarts・gunpla は seed(000005)と同じ既定の辞書。
var (
	figuarts = [][]string{{"S.H.Figuarts", "SHフィギュアーツ", "フィギュアーツ"}}
	gunpla   = [][]string{{"HG", "ハイグレード"}, {"MG", "マスターグレード"}, {"RG", "リアルグレード"}}
)

// AC-A4: トークンが属するグループ(トークンと正規化して完全一致する語を持つグループ)の語を、正規化して返す。
// 順はグループ内の順で重複を除く。属するグループが無ければトークン自身の正規化だけ。正規化して空の語は除く。
func TestAliasVariants(t *testing.T) {
	cases := []struct {
		name   string
		token  string
		groups [][]string
		want   []string
	}{
		{"辞書の先頭の語", "S.H.Figuarts", figuarts, []string{"shfiguarts", "shフィギュアーツ", "フィギュアーツ"}},
		{"辞書の途中の語", "フィギュアーツ", figuarts, []string{"shfiguarts", "shフィギュアーツ", "フィギュアーツ"}},
		{"全角・小文字でも正規化して一致(NFKC)", "ｓ．ｈ．ｆｉｇｕａｒｔｓ", figuarts, []string{"shfiguarts", "shフィギュアーツ", "フィギュアーツ"}},
		{"複数グループのうち属するものだけ", "HG", gunpla, []string{"hg", "ハイグレード"}},
		{"別のグループ", "マスターグレード", gunpla, []string{"mg", "マスターグレード"}},
		{"属さないトークンは自身の正規化だけ", "グリス", figuarts, []string{"グリス"}},
		{"部分一致ではグループに属さない(HGUC は HG ではない)", "HGUC", gunpla, []string{"hguc"}},
		{"辞書が nil", "ＨＧ", nil, []string{"hg"}},
		{"正規化して同じ語は 1 つにまとめる", "HG", [][]string{{"HG", "ＨＧ", "H.G.", "ハイグレード"}}, []string{"hg", "ハイグレード"}},
		{"正規化して空の語は除く", "HG", [][]string{{"HG", "・・", "ハイグレード"}}, []string{"hg", "ハイグレード"}},
		{"正規化して空のトークンは空", "-", gunpla, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := estimate.AliasVariants(c.token, c.groups)
			if len(got) != len(c.want) || (len(c.want) > 0 && !slices.Equal(got, c.want)) {
				t.Errorf("AliasVariants(%q) = %q, want %q", c.token, got, c.want)
			}
		})
	}
}

// AC-A5: 辞書つきのタイトル照合。各トークンについて、AliasVariants のどれかが正規化タイトルに含まれれば一致。
func TestTitleMatchesWithAliases(t *testing.T) {
	cases := []struct {
		name, item, title string
		groups            [][]string
		want              bool
	}{
		{"指示の例: S.H.Figuarts と SHフィギュアーツ", "S.H.Figuarts グリス", "SHフィギュアーツ 仮面ライダーグリス", figuarts, true},
		{"指示の例: HG と ハイグレード", "HG エアリアル", "ハイグレード ガンダムエアリアル", gunpla, true},
		{"逆向き(name 側が別名)", "フィギュアーツ グリス", "S.H.Figuarts 仮面ライダーグリス", figuarts, true},
		{"カタカナ表記(フェーズ3 では不一致だった例)", "S.H.Figuarts グリス", "フィギュアーツ グリス", figuarts, true},
		{"別名でも、他のトークンが欠ければ不一致", "S.H.Figuarts グリス", "SHフィギュアーツ 仮面ライダーローグ", figuarts, false},
		{"別のグループの語では一致しない", "HG エアリアル", "マスターグレード ガンダムエアリアル", gunpla, false},
		{"別名が無いトークンは従来どおり", "ボルシャック 銀トレジャー", "ボルシャック・ドラゴン 通常版", figuarts, false},
		{"辞書が空なら従来どおり(カタカナは不一致)", "S.H.Figuarts グリス", "フィギュアーツ グリス", nil, false},
		{"辞書が空でも記号の揺れは吸収", "S.H.Figuarts グリス", "SHFiguarts グリス", nil, true},
		{"トークンが無ければ true", "  ", "なんでも", figuarts, true},
		{"タイトルが空", "HG エアリアル", "", gunpla, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := estimate.TitleMatchesWithAliases(c.item, c.title, c.groups); got != c.want {
				t.Errorf("TitleMatchesWithAliases(%q, %q) = %v, want %v", c.item, c.title, got, c.want)
			}
		})
	}
}

// AC-A5: 辞書が nil・空なら TitleMatches と同じ結果(フェーズ3 の AC-E1 の例)。
func TestTitleMatchesWithAliases_NoAliasesSameAsBefore(t *testing.T) {
	pairs := [][2]string{
		{"グリス", "S.H.Figuarts 仮面ライダーグリス 開封品"},
		{"ボルシャック 銀トレジャー", "【銀トレジャー】ボルシャック・ドラゴン"},
		{"ボルシャック 銀トレジャー", "ボルシャック・ドラゴン 通常版"},
		{"HG ガンダムエアリアル", "ＨＧ　ガンダム・エアリアル 1/144"},
		{"S.H.Figuarts グリス", "S.H.フィギュアーツ グリス"},
		{"グリス -", "仮面ライダーグリス"},
		{"グリス", ""},
	}
	for _, p := range pairs {
		want := estimate.TitleMatches(p[0], p[1])
		for _, g := range [][][]string{nil, {}} {
			if got := estimate.TitleMatchesWithAliases(p[0], p[1], g); got != want {
				t.Errorf("TitleMatchesWithAliases(%q, %q, %v) = %v, want TitleMatches と同じ %v", p[0], p[1], g, got, want)
			}
		}
	}
}

// AC-A6: Judge は Item.Aliases を使って title_mismatch を判定する。
func TestJudge_Aliases(t *testing.T) {
	it := estimate.Item{Name: "S.H.Figuarts グリス", Aliases: figuarts}
	if rs := estimate.Judge(it, nil, estimate.Listing{Title: "SHフィギュアーツ 仮面ライダーグリス", Price: 5000}); len(rs) != 0 {
		t.Errorf("別名のタイトル: 理由 = %v, want なし", rs)
	}
	if rs := estimate.Judge(it, nil, estimate.Listing{Title: "仮面ライダーグリス ソフビ", Price: 5000}); !slices.Equal(rs, []estimate.Reason{estimate.ReasonTitleMismatch}) {
		t.Errorf("別名も無いタイトル: 理由 = %v, want [title_mismatch]", rs)
	}
	noDict := estimate.Item{Name: "S.H.Figuarts グリス"}
	if rs := estimate.Judge(noDict, nil, estimate.Listing{Title: "SHフィギュアーツ 仮面ライダーグリス", Price: 5000}); !slices.Equal(rs, []estimate.Reason{estimate.ReasonTitleMismatch}) {
		t.Errorf("辞書なし: 理由 = %v, want [title_mismatch](従来どおり)", rs)
	}
}

// AC-A7: Evaluate の基準価格も辞書で照合する(別名のタイトルの基準サイトの出品が基準に入る)。
// 仕様 §6 の例は辞書があっても変わらない。
func TestEvaluate_Aliases(t *testing.T) {
	t.Run("別名のタイトルが基準に入る", func(t *testing.T) {
		it := estimate.Item{Name: "HG エアリアル", Aliases: gunpla}
		r := estimate.Evaluate(it, []estimate.SiteInput{
			{SiteID: 1, Reference: true, Listings: []estimate.Listing{
				{Title: "ハイグレード ガンダムエアリアル", Price: 2000, InStock: true},
				{Title: "HG 1/144 ガンダムエアリアル", Price: 2200, InStock: true},
			}},
			{SiteID: 2, Listings: []estimate.Listing{
				{Title: "ハイグレード エアリアル 未組立", Price: 1800, InStock: true},
				{Title: "エアリアル キーホルダー", Price: 300, InStock: true},
			}},
		})
		if !eqIntPtr(r.BasePrice, ip(2100)) {
			t.Errorf("BasePrice = %v, want 2100(別名のタイトルも基準に入る)", fmtPtr(r.BasePrice))
		}
		s1 := findSite(t, r, 1)
		if len(s1.Reasons) != 2 || len(s1.Reasons[0]) != 0 || len(s1.Reasons[1]) != 0 {
			t.Errorf("基準サイトの理由 = %v, want [[] []]", s1.Reasons)
		}
		s2 := findSite(t, r, 2)
		if len(s2.Reasons) != 2 || len(s2.Reasons[0]) != 0 ||
			!slices.Equal(s2.Reasons[1], []estimate.Reason{estimate.ReasonTitleMismatch, estimate.ReasonTooCheap}) {
			t.Errorf("フリマの理由 = %v, want [[] [title_mismatch too_cheap]]", s2.Reasons)
		}
		if s2.Estimate.Count != 1 || s2.Estimate.SuspiciousCount != 1 || !eqIntPtr(s2.Estimate.Low, ip(1800)) {
			t.Errorf("フリマの目安 = %+v", s2.Estimate)
		}
	})
	t.Run("仕様 §6 の例は辞書があっても同じ", func(t *testing.T) {
		const cardrush, dragonstar, mercari = 11, 12, 13
		it := estimate.Item{Name: "ボルシャック 銀トレジャー", Aliases: [][]string{{"銀トレジャー", "銀トレ"}}}
		r := estimate.Evaluate(it, []estimate.SiteInput{
			{SiteID: cardrush, Reference: true, Listings: []estimate.Listing{{Title: "ボルシャック・ドラゴン 銀トレジャー", Price: 5000, InStock: true}}},
			{SiteID: dragonstar, Reference: true, Listings: []estimate.Listing{{Title: "【銀トレジャー】ボルシャック", Price: 4800, InStock: true}}},
			{SiteID: mercari, Reference: false, Listings: []estimate.Listing{{Title: "まとめ売り カード", Price: 300, InStock: true}}},
		})
		if !eqIntPtr(r.BasePrice, ip(4900)) {
			t.Errorf("BasePrice = %v, want 4900", fmtPtr(r.BasePrice))
		}
		m := findSite(t, r, mercari)
		if len(m.Reasons) != 1 || !slices.Equal(m.Reasons[0], []estimate.Reason{estimate.ReasonTitleMismatch, estimate.ReasonTooCheap}) {
			t.Errorf("メルカリの理由 = %v, want [[title_mismatch too_cheap]]", m.Reasons)
		}
	})
}
