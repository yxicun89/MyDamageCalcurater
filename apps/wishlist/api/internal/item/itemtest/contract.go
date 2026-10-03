// Package itemtest は item.Repository の契約テスト。メモリ実装と MySQL 実装の両方に同じテストを流し、
// 振る舞い(外部キー・UNIQUE・並び・部分更新・全件置き換え・不可分性)をそろえる。
//
// newRepo は呼ばれるたびに独立した状態の Repository を返すこと。MySQL の seed(ジャンル 4・サイト 2)が
// 入っていてもよいように、テストは自分で作った行だけを見る(名前に一意な接頭辞を付ける)。
package itemtest

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/oapi-codegen/nullable"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// テスト用の画像ファイル名(Repository は中身を見ない)。
const (
	img1 = "11111111-1111-4111-8111-111111111111.png"
	img2 = "22222222-2222-4222-8222-222222222222.jpg"
)

func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }
func i64p(i int64) *int64   { return &i }

type fixture struct {
	repo   item.Repository
	genreA item.Genre
	genreB item.Genre
	site1  item.Site
	site2  item.Site
}

func setup(t *testing.T, newRepo func(t *testing.T) item.Repository) fixture {
	t.Helper()
	ctx := context.Background()
	r := newRepo(t)
	s1, err := r.CreateSite(ctx, item.NewSite{Name: "契約テスト サイト1", SearchURLTemplate: "https://one.example.com/s?q={q}", FetchType: item.FetchLinkOnly})
	if err != nil {
		t.Fatalf("CreateSite: %v", err)
	}
	s2, err := r.CreateSite(ctx, item.NewSite{Name: "契約テスト サイト2", SearchURLTemplate: "https://two.example.com/s?q={q}", FetchType: item.FetchScrape, IsReference: true})
	if err != nil {
		t.Fatalf("CreateSite: %v", err)
	}
	ga, err := r.CreateGenre(ctx, item.NewGenre{Name: "契約テスト ジャンルA", QueryTemplate: "{name} {option}", SortOrder: 1000})
	if err != nil {
		t.Fatalf("CreateGenre: %v", err)
	}
	gb, err := r.CreateGenre(ctx, item.NewGenre{Name: "契約テスト ジャンルB", QueryTemplate: "B {name}", SortOrder: 1000})
	if err != nil {
		t.Fatalf("CreateGenre: %v", err)
	}
	return fixture{repo: r, genreA: ga, genreB: gb, site1: s1, site2: s2}
}

func (f fixture) newItem(t *testing.T, genreID int64, name string, sortOrder int) item.Item {
	t.Helper()
	it, err := f.repo.CreateItem(context.Background(), item.NewItem{GenreID: genreID, Name: name, ImagePath: img1, SortOrder: sortOrder})
	if err != nil {
		t.Fatalf("CreateItem(%s): %v", name, err)
	}
	return it
}

func ids(items []item.Item) []int64 {
	out := make([]int64, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func only[T any](xs []T, keep func(T) bool) []T {
	var out []T
	for _, x := range xs {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}

// RunRepositoryContract は Repository の契約テストを流す。
func RunRepositoryContract(t *testing.T, newRepo func(t *testing.T) item.Repository) {
	ctx := context.Background()

	// AC-R1: ジャンルの作成と一覧。site_ids の順序を保つ。一覧は sort_order 昇順・同順は id 昇順。
	t.Run("CreateAndListGenres", func(t *testing.T) {
		f := setup(t, newRepo)
		g, err := f.repo.CreateGenre(ctx, item.NewGenre{Name: "契約テスト ジャンルC", QueryTemplate: "C {name}", SortOrder: 999, SiteIDs: []int64{f.site2.ID, f.site1.ID}})
		if err != nil {
			t.Fatal(err)
		}
		if g.ID == 0 || g.Name != "契約テスト ジャンルC" || g.QueryTemplate != "C {name}" || g.SortOrder != 999 {
			t.Errorf("作成結果 = %+v", g)
		}
		if !slices.Equal(g.SiteIDs, []int64{f.site2.ID, f.site1.ID}) {
			t.Errorf("SiteIDs = %v, want 指定順 [%d %d]", g.SiteIDs, f.site2.ID, f.site1.ID)
		}
		list, err := f.repo.ListGenres(ctx)
		if err != nil {
			t.Fatal(err)
		}
		mine := only(list, func(x item.Genre) bool { return x.ID == g.ID || x.ID == f.genreA.ID || x.ID == f.genreB.ID })
		var got []int64
		for _, x := range mine {
			got = append(got, x.ID)
		}
		want := []int64{g.ID, f.genreA.ID, f.genreB.ID} // 999 < 1000、A と B は同順で id 昇順
		if !slices.Equal(got, want) {
			t.Errorf("並び = %v, want %v", got, want)
		}
		for _, x := range mine {
			if x.ID == g.ID && !slices.Equal(x.SiteIDs, []int64{f.site2.ID, f.site1.ID}) {
				t.Errorf("一覧の SiteIDs = %v", x.SiteIDs)
			}
		}
	})

	// AC-R2: ジャンル名の重複・存在しないサイト・site_ids 内の重複。
	t.Run("CreateGenreErrors", func(t *testing.T) {
		f := setup(t, newRepo)
		cases := []struct {
			name string
			in   item.NewGenre
			want error
		}{
			{"重複名", item.NewGenre{Name: f.genreA.Name, QueryTemplate: "{name}"}, item.ErrDuplicateName},
			{"存在しないサイト", item.NewGenre{Name: "契約テスト X", QueryTemplate: "{name}", SiteIDs: []int64{f.site1.ID, 99999999}}, item.ErrSiteNotFound},
			{"site_ids の重複", item.NewGenre{Name: "契約テスト Y", QueryTemplate: "{name}", SiteIDs: []int64{f.site1.ID, f.site1.ID}}, item.ErrInvalid},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				before, _ := f.repo.ListGenres(ctx)
				if _, err := f.repo.CreateGenre(ctx, c.in); !errors.Is(err, c.want) {
					t.Errorf("err = %v, want %v", err, c.want)
				}
				after, _ := f.repo.ListGenres(ctx)
				if len(after) != len(before) {
					t.Errorf("失敗したのにジャンルが増えた(%d → %d)", len(before), len(after))
				}
			})
		}
	})

	// AC-R3: ジャンルの部分更新と site_ids の全件置き換え。
	t.Run("UpdateGenre", func(t *testing.T) {
		f := setup(t, newRepo)
		g, err := f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{SiteIDs: &[]int64{f.site2.ID, f.site1.ID}})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(g.SiteIDs, []int64{f.site2.ID, f.site1.ID}) {
			t.Errorf("置き換え後 SiteIDs = %v", g.SiteIDs)
		}
		if g.Name != f.genreA.Name || g.QueryTemplate != f.genreA.QueryTemplate || g.SortOrder != f.genreA.SortOrder {
			t.Errorf("指定していない項目が変わった: %+v", g)
		}
		g, err = f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{SiteIDs: &[]int64{f.site1.ID}})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(g.SiteIDs, []int64{f.site1.ID}) {
			t.Errorf("2 回目の置き換え後 SiteIDs = %v", g.SiteIDs)
		}
		g, err = f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{Name: strp("契約テスト 改名"), SortOrder: intp(5)})
		if err != nil {
			t.Fatal(err)
		}
		if g.Name != "契約テスト 改名" || g.SortOrder != 5 || !slices.Equal(g.SiteIDs, []int64{f.site1.ID}) {
			t.Errorf("SiteIDs を省略した更新 = %+v", g)
		}
		g, err = f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{SiteIDs: &[]int64{}})
		if err != nil {
			t.Fatal(err)
		}
		if len(g.SiteIDs) != 0 {
			t.Errorf("空で置き換え後 SiteIDs = %v", g.SiteIDs)
		}
		// 自分と同じ名前への更新は重複ではない
		if _, err := f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{Name: strp("契約テスト 改名")}); err != nil {
			t.Errorf("同名への更新: %v", err)
		}
	})

	// AC-R3: ジャンル更新のエラーと不可分性。
	t.Run("UpdateGenreErrors", func(t *testing.T) {
		f := setup(t, newRepo)
		if _, err := f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{SiteIDs: &[]int64{f.site1.ID}}); err != nil {
			t.Fatal(err)
		}
		cases := []struct {
			name string
			id   int64
			p    item.GenrePatch
			want error
		}{
			{"存在しないジャンル", 99999999, item.GenrePatch{Name: strp("契約テスト Z")}, item.ErrNotFound},
			{"他と重複する名前", f.genreA.ID, item.GenrePatch{Name: &f.genreB.Name}, item.ErrDuplicateName},
			{"存在しないサイト(名前の変更も入れない)", f.genreA.ID, item.GenrePatch{Name: strp("契約テスト 入らない"), SiteIDs: &[]int64{f.site2.ID, 99999999}}, item.ErrSiteNotFound},
			{"site_ids の重複", f.genreA.ID, item.GenrePatch{SiteIDs: &[]int64{f.site2.ID, f.site2.ID}}, item.ErrInvalid},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				if _, err := f.repo.UpdateGenre(ctx, c.id, c.p); !errors.Is(err, c.want) {
					t.Errorf("err = %v, want %v", err, c.want)
				}
			})
		}
		list, _ := f.repo.ListGenres(ctx)
		for _, g := range list {
			if g.ID == f.genreA.ID && (g.Name != f.genreA.Name || !slices.Equal(g.SiteIDs, []int64{f.site1.ID})) {
				t.Errorf("失敗した更新が一部だけ反映された: %+v", g)
			}
		}
	})

	// AC-R4: サイトの作成・一覧(id 昇順)・部分更新・重複名・存在しない id。
	t.Run("Sites", func(t *testing.T) {
		f := setup(t, newRepo)
		if f.site2.FetchType != item.FetchScrape || !f.site2.IsReference || f.site2.SearchURLTemplate != "https://two.example.com/s?q={q}" {
			t.Errorf("作成結果 = %+v", f.site2)
		}
		list, err := f.repo.ListSites(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(list); i++ {
			if list[i-1].ID >= list[i].ID {
				t.Errorf("id 昇順でない: %d, %d", list[i-1].ID, list[i].ID)
			}
		}
		if _, err := f.repo.CreateSite(ctx, item.NewSite{Name: f.site1.Name, SearchURLTemplate: "https://x.example.com/{q}", FetchType: item.FetchLinkOnly}); !errors.Is(err, item.ErrDuplicateName) {
			t.Errorf("重複名 err = %v", err)
		}
		ft := item.FetchHeadless
		s, err := f.repo.UpdateSite(ctx, f.site1.ID, item.SitePatch{FetchType: &ft})
		if err != nil {
			t.Fatal(err)
		}
		if s.FetchType != item.FetchHeadless || s.Name != f.site1.Name || s.SearchURLTemplate != f.site1.SearchURLTemplate || s.IsReference != f.site1.IsReference {
			t.Errorf("部分更新 = %+v", s)
		}
		if _, err := f.repo.UpdateSite(ctx, f.site1.ID, item.SitePatch{Name: &f.site2.Name}); !errors.Is(err, item.ErrDuplicateName) {
			t.Errorf("更新で重複名 err = %v", err)
		}
		if _, err := f.repo.UpdateSite(ctx, 99999999, item.SitePatch{Name: strp("契約テスト 無い")}); !errors.Is(err, item.ErrNotFound) {
			t.Errorf("存在しない id err = %v", err)
		}
	})

	// AC-R5: 商品の作成・取得。存在しない genre_id は ErrGenreNotFound、存在しない id は ErrNotFound。
	t.Run("CreateGetItem", func(t *testing.T) {
		f := setup(t, newRepo)
		in := item.NewItem{GenreID: f.genreA.ID, Name: "ボルシャック", OptionText: strp("銀トレジャー"), QueryOverride: strp("ボルシャック 銀"),
			ImagePath: img1, SourceURL: strp("https://shop.example.com/1"), MinPrice: intp(300), SortOrder: 3}
		it, err := f.repo.CreateItem(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		if it.ID == 0 || it.GenreID != in.GenreID || it.Name != in.Name || it.ImagePath != img1 || it.SortOrder != 3 ||
			it.OptionText == nil || *it.OptionText != "銀トレジャー" || it.QueryOverride == nil || *it.QueryOverride != "ボルシャック 銀" ||
			it.SourceURL == nil || *it.SourceURL != "https://shop.example.com/1" || it.MinPrice == nil || *it.MinPrice != 300 {
			t.Errorf("作成結果 = %+v", it)
		}
		if it.CreatedAt.IsZero() || it.UpdatedAt.IsZero() {
			t.Errorf("CreatedAt/UpdatedAt が空: %+v", it)
		}
		if len(it.SiteOverrides) != 0 {
			t.Errorf("SiteOverrides = %v, want 空", it.SiteOverrides)
		}
		got, err := f.repo.GetItem(ctx, it.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != it.ID || got.Name != it.Name || !got.CreatedAt.Equal(it.CreatedAt) {
			t.Errorf("GetItem = %+v, want %+v", got, it)
		}
		if _, err := f.repo.GetItem(ctx, 99999999); !errors.Is(err, item.ErrNotFound) {
			t.Errorf("存在しない id err = %v", err)
		}
		if _, err := f.repo.CreateItem(ctx, item.NewItem{GenreID: 99999999, Name: "x", ImagePath: img1}); !errors.Is(err, item.ErrGenreNotFound) {
			t.Errorf("存在しない genre err = %v", err)
		}
		// 任意項目を省略すれば nil のまま
		bare := f.newItem(t, f.genreA.ID, "素の商品", 0)
		if bare.OptionText != nil || bare.QueryOverride != nil || bare.SourceURL != nil || bare.MinPrice != nil {
			t.Errorf("省略した項目が nil でない: %+v", bare)
		}
	})

	// AC-R6: 一覧の並び(sort_order 昇順、同順は id 降順)とジャンルでの絞り込み。
	t.Run("ListItems", func(t *testing.T) {
		f := setup(t, newRepo)
		a1 := f.newItem(t, f.genreA.ID, "a1", 2)
		a2 := f.newItem(t, f.genreA.ID, "a2", 1)
		a3 := f.newItem(t, f.genreA.ID, "a3", 1)
		b1 := f.newItem(t, f.genreB.ID, "b1", 0)
		all, err := f.repo.ListItems(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if want := []int64{b1.ID, a3.ID, a2.ID, a1.ID}; !slices.Equal(ids(all), want) {
			t.Errorf("全件 = %v, want %v", ids(all), want)
		}
		onlyA, err := f.repo.ListItems(ctx, &f.genreA.ID)
		if err != nil {
			t.Fatal(err)
		}
		if want := []int64{a3.ID, a2.ID, a1.ID}; !slices.Equal(ids(onlyA), want) {
			t.Errorf("ジャンル A = %v, want %v", ids(onlyA), want)
		}
		none, err := f.repo.ListItems(ctx, i64p(99999999))
		if err != nil {
			t.Errorf("存在しないジャンルでの絞り込みはエラーにしない: %v", err)
		}
		if len(none) != 0 {
			t.Errorf("存在しないジャンル = %v", ids(none))
		}
	})

	// AC-R7: PATCH の nullable(未指定は変えない・null で消す・値で設定)。
	t.Run("UpdateItemNullable", func(t *testing.T) {
		f := setup(t, newRepo)
		it, err := f.repo.CreateItem(ctx, item.NewItem{GenreID: f.genreA.ID, Name: "グリス", OptionText: strp("o"), QueryOverride: strp("q"),
			ImagePath: img1, SourceURL: strp("https://s.example.com/"), MinPrice: intp(100), SortOrder: 1})
		if err != nil {
			t.Fatal(err)
		}
		// 何も指定しない → 変わらない
		got, err := f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{})
		if err != nil {
			t.Fatal(err)
		}
		if got.OptionText == nil || *got.OptionText != "o" || got.QueryOverride == nil || got.SourceURL == nil || got.MinPrice == nil || *got.MinPrice != 100 || got.Name != "グリス" || got.SortOrder != 1 {
			t.Errorf("空の patch で変わった: %+v", got)
		}
		// 値で設定
		got, err = f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{
			Name: strp("グリス改"), SortOrder: intp(7), GenreID: &f.genreB.ID,
			OptionText: nullable.NewNullableWithValue("o2"), MinPrice: nullable.NewNullableWithValue(0),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "グリス改" || got.SortOrder != 7 || got.GenreID != f.genreB.ID || got.OptionText == nil || *got.OptionText != "o2" ||
			got.MinPrice == nil || *got.MinPrice != 0 || got.QueryOverride == nil || *got.QueryOverride != "q" {
			t.Errorf("値で設定 = %+v", got)
		}
		// null で消す(指定しなかった項目は残る)
		got, err = f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{
			OptionText: nullable.NewNullNullable[string](), QueryOverride: nullable.NewNullNullable[string](),
			SourceURL: nullable.NewNullNullable[string](), MinPrice: nullable.NewNullNullable[int](),
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.OptionText != nil || got.QueryOverride != nil || got.SourceURL != nil || got.MinPrice != nil {
			t.Errorf("null で消えていない: %+v", got)
		}
		if got.Name != "グリス改" || got.ImagePath != img1 {
			t.Errorf("指定していない項目が変わった: %+v", got)
		}
		// 画像のパス
		got, err = f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{ImagePath: strp(img2)})
		if err != nil {
			t.Fatal(err)
		}
		if got.ImagePath != img2 {
			t.Errorf("ImagePath = %q", got.ImagePath)
		}
		// GetItem でも同じ
		again, _ := f.repo.GetItem(ctx, it.ID)
		if again.OptionText != nil || again.ImagePath != img2 || again.Name != "グリス改" {
			t.Errorf("GetItem = %+v", again)
		}
	})

	// AC-R8: site_overrides の全件置き換え(site_id 昇順で返す)。
	t.Run("UpdateItemSiteOverrides", func(t *testing.T) {
		f := setup(t, newRepo)
		it := f.newItem(t, f.genreA.ID, "x", 0)
		got, err := f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{SiteOverrides: &[]item.SiteOverride{
			{SiteID: f.site2.ID, Query: nil, Enabled: false},
			{SiteID: f.site1.ID, Query: strp("サイト1だけの語"), Enabled: true},
		}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.SiteOverrides) != 2 || got.SiteOverrides[0].SiteID != f.site1.ID || got.SiteOverrides[1].SiteID != f.site2.ID ||
			got.SiteOverrides[0].Query == nil || *got.SiteOverrides[0].Query != "サイト1だけの語" || !got.SiteOverrides[0].Enabled ||
			got.SiteOverrides[1].Query != nil || got.SiteOverrides[1].Enabled {
			t.Errorf("1 回目 = %+v", got.SiteOverrides)
		}
		// 省略は変えない
		got, err = f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{Name: strp("y")})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.SiteOverrides) != 2 {
			t.Errorf("省略で変わった: %+v", got.SiteOverrides)
		}
		// 置き換え
		got, err = f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{SiteOverrides: &[]item.SiteOverride{{SiteID: f.site2.ID, Enabled: true}}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.SiteOverrides) != 1 || got.SiteOverrides[0].SiteID != f.site2.ID || !got.SiteOverrides[0].Enabled {
			t.Errorf("置き換え後 = %+v", got.SiteOverrides)
		}
		// 存在しないサイト → ErrSiteNotFound、名前の変更も含めて何も変えない
		if _, err := f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{Name: strp("入らない"), SiteOverrides: &[]item.SiteOverride{{SiteID: 99999999, Enabled: true}}}); !errors.Is(err, item.ErrSiteNotFound) {
			t.Errorf("存在しないサイト err = %v", err)
		}
		// site_id の重複 → ErrInvalid
		if _, err := f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{SiteOverrides: &[]item.SiteOverride{{SiteID: f.site1.ID, Enabled: true}, {SiteID: f.site1.ID, Enabled: false}}}); !errors.Is(err, item.ErrInvalid) {
			t.Errorf("重複 err = %v", err)
		}
		again, _ := f.repo.GetItem(ctx, it.ID)
		if again.Name != "y" || len(again.SiteOverrides) != 1 || again.SiteOverrides[0].SiteID != f.site2.ID {
			t.Errorf("失敗した更新が反映された: %+v", again)
		}
		// 空で全部消す
		got, err = f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{SiteOverrides: &[]item.SiteOverride{}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.SiteOverrides) != 0 {
			t.Errorf("空で置き換え後 = %+v", got.SiteOverrides)
		}
		// 一覧にも site_overrides が載る
		if _, err := f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{SiteOverrides: &[]item.SiteOverride{{SiteID: f.site1.ID, Enabled: false}}}); err != nil {
			t.Fatal(err)
		}
		list, _ := f.repo.ListItems(ctx, nil)
		for _, x := range list {
			if x.ID == it.ID && (len(x.SiteOverrides) != 1 || x.SiteOverrides[0].SiteID != f.site1.ID) {
				t.Errorf("一覧の SiteOverrides = %+v", x.SiteOverrides)
			}
		}
	})

	// AC-R9: 商品更新のエラー(存在しない id は ErrNotFound、存在しない genre_id は ErrGenreNotFound)。
	t.Run("UpdateItemErrors", func(t *testing.T) {
		f := setup(t, newRepo)
		it := f.newItem(t, f.genreA.ID, "x", 0)
		if _, err := f.repo.UpdateItem(ctx, 99999999, item.ItemPatch{Name: strp("z")}); !errors.Is(err, item.ErrNotFound) {
			t.Errorf("存在しない id err = %v", err)
		}
		if _, err := f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{GenreID: i64p(99999999)}); !errors.Is(err, item.ErrGenreNotFound) {
			t.Errorf("存在しない genre err = %v", err)
		}
		again, _ := f.repo.GetItem(ctx, it.ID)
		if again.GenreID != f.genreA.ID {
			t.Errorf("失敗した更新が反映された: %+v", again)
		}
	})

	// AC-R10: 削除は ImagePath を返し、以後は ErrNotFound。サイトの紐づけも消える。
	t.Run("DeleteItem", func(t *testing.T) {
		f := setup(t, newRepo)
		it := f.newItem(t, f.genreA.ID, "x", 0)
		if _, err := f.repo.UpdateItem(ctx, it.ID, item.ItemPatch{SiteOverrides: &[]item.SiteOverride{{SiteID: f.site1.ID, Enabled: true}}}); err != nil {
			t.Fatal(err)
		}
		p, err := f.repo.DeleteItem(ctx, it.ID)
		if err != nil {
			t.Fatal(err)
		}
		if p != img1 {
			t.Errorf("imagePath = %q, want %q", p, img1)
		}
		if _, err := f.repo.GetItem(ctx, it.ID); !errors.Is(err, item.ErrNotFound) {
			t.Errorf("削除後 GetItem err = %v", err)
		}
		if _, err := f.repo.DeleteItem(ctx, it.ID); !errors.Is(err, item.ErrNotFound) {
			t.Errorf("2 回目の削除 err = %v", err)
		}
		list, _ := f.repo.ListItems(ctx, nil)
		if slices.Contains(ids(list), it.ID) {
			t.Error("削除した商品が一覧に残っている")
		}
	})
}
