package itemtest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// フェーズ4-1 表記揺れの辞書の Repository 契約(docs/phase4-spec.md AC-A1〜A3)。
// 入力の検査(空の語・1 語だけのグループ・長さ)は Service の役目。Repository は「正規化後の語がジャンル内で重複しない」ことだけを守る。

func aliasesEqual(a, b [][]string) bool {
	return fmt.Sprintf("%q", a) == fmt.Sprintf("%q", b)
}

func genreByID(t *testing.T, r item.Repository, id int64) item.Genre {
	t.Helper()
	list, err := r.ListGenres(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range list {
		if g.ID == id {
			return g
		}
	}
	t.Fatalf("ジャンル %d が一覧に無い", id)
	return item.Genre{}
}

func runAliasContract(t *testing.T, newRepo func(t *testing.T) item.Repository) {
	ctx := context.Background()
	figuarts := [][]string{{"S.H.Figuarts", "SHフィギュアーツ", "フィギュアーツ"}, {"HG", "ハイグレード"}}

	// AC-A1: 作成で渡した別名グループを、グループの順・語の順のまま返す(作成結果と一覧)。
	// 渡さなければ長さ 0。同じ語でも別のジャンルなら重複ではない。
	t.Run("CreateGenreWithAliases", func(t *testing.T) {
		f := setup(t, newRepo)
		g, err := f.repo.CreateGenre(ctx, item.NewGenre{Name: "契約テスト 別名", QueryTemplate: "{name}", Aliases: figuarts})
		if err != nil {
			t.Fatal(err)
		}
		if !aliasesEqual(g.Aliases, figuarts) {
			t.Errorf("作成結果の Aliases = %q, want %q", g.Aliases, figuarts)
		}
		if got := genreByID(t, f.repo, g.ID).Aliases; !aliasesEqual(got, figuarts) {
			t.Errorf("一覧の Aliases = %q, want %q", got, figuarts)
		}
		if got := genreByID(t, f.repo, f.genreA.ID).Aliases; len(got) != 0 {
			t.Errorf("別名を渡していないジャンルの Aliases = %q, want 長さ 0", got)
		}
		if len(f.genreA.Aliases) != 0 {
			t.Errorf("別名を渡していない作成結果の Aliases = %q, want 長さ 0", f.genreA.Aliases)
		}
		other, err := f.repo.CreateGenre(ctx, item.NewGenre{Name: "契約テスト 別名2", QueryTemplate: "{name}", Aliases: [][]string{{"HG", "ハイグレード"}}})
		if err != nil {
			t.Fatalf("別のジャンルの同じ語: %v", err)
		}
		if !aliasesEqual(other.Aliases, [][]string{{"HG", "ハイグレード"}}) {
			t.Errorf("別のジャンルの Aliases = %q", other.Aliases)
		}
	})

	// AC-A2: 更新で Aliases を渡すと全件置き換え。省略すれば変えない。空なら全部消す。存在しないジャンルは ErrNotFound。
	t.Run("UpdateGenreAliases", func(t *testing.T) {
		f := setup(t, newRepo)
		g, err := f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{Aliases: &figuarts})
		if err != nil {
			t.Fatal(err)
		}
		if !aliasesEqual(g.Aliases, figuarts) || g.Name != f.genreA.Name {
			t.Errorf("置き換え後 = %+v", g)
		}
		next := [][]string{{"MG", "マスターグレード"}}
		if g, err = f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{Aliases: &next}); err != nil {
			t.Fatal(err)
		}
		if !aliasesEqual(g.Aliases, next) {
			t.Errorf("2 回目の置き換え後 Aliases = %q, want %q", g.Aliases, next)
		}
		if g, err = f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{Name: strp("契約テスト 別名を残す"), SiteIDs: &[]int64{f.site1.ID}}); err != nil {
			t.Fatal(err)
		}
		if !aliasesEqual(g.Aliases, next) {
			t.Errorf("Aliases を省略した更新で変わった: %q", g.Aliases)
		}
		if got := genreByID(t, f.repo, f.genreA.ID).Aliases; !aliasesEqual(got, next) {
			t.Errorf("一覧の Aliases = %q, want %q", got, next)
		}
		if g, err = f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{Aliases: &[][]string{}}); err != nil {
			t.Fatal(err)
		}
		if len(g.Aliases) != 0 || len(genreByID(t, f.repo, f.genreA.ID).Aliases) != 0 {
			t.Errorf("空で置き換え後 Aliases = %q", g.Aliases)
		}
		if _, err := f.repo.UpdateGenre(ctx, 99999999, item.GenrePatch{Aliases: &next}); !errors.Is(err, item.ErrNotFound) {
			t.Errorf("存在しないジャンル err = %v, want ErrNotFound", err)
		}
		if got := genreByID(t, f.repo, f.genreB.ID).Aliases; len(got) != 0 {
			t.Errorf("他のジャンルの Aliases が変わった: %q", got)
		}
	})

	// AC-A3: 正規化(query.Normalize)後の語がジャンル内で重複すれば ErrInvalid で、何も変えない(同じ呼び出しの他の項目も)。
	// 正規化後に違う語(濁点・ひらがなとカタカナ)は重複ではない(DB の照合順序で同一視しない)。
	t.Run("AliasDuplicates", func(t *testing.T) {
		f := setup(t, newRepo)
		base := [][]string{{"HG", "ハイグレード"}}
		if _, err := f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{Aliases: &base}); err != nil {
			t.Fatal(err)
		}
		dups := []struct {
			name    string
			aliases [][]string
		}{
			{"別のグループに同じ語", [][]string{{"HG", "ハイグレード"}, {"HG", "エイチジー"}}},
			{"同じグループに同じ語", [][]string{{"MG", "MG", "マスターグレード"}}},
			{"大文字小文字・全角(NFKC)", [][]string{{"HG", "ハイグレード"}, {"ｈｇ", "エイチジー"}}},
			{"記号の有無", [][]string{{"S.H.Figuarts", "SHFiguarts"}}},
			{"半角カナ(NFKC)", [][]string{{"ハイグレード", "ﾊｲｸﾞﾚｰﾄﾞ"}}},
		}
		for _, c := range dups {
			t.Run("更新/"+c.name, func(t *testing.T) {
				a := c.aliases
				if _, err := f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{Name: strp("契約テスト 入らない"), Aliases: &a}); !errors.Is(err, item.ErrInvalid) {
					t.Errorf("err = %v, want ErrInvalid", err)
				}
				g := genreByID(t, f.repo, f.genreA.ID)
				if g.Name != f.genreA.Name || !aliasesEqual(g.Aliases, base) {
					t.Errorf("失敗した更新が反映された: %+v", g)
				}
			})
			t.Run("作成/"+c.name, func(t *testing.T) {
				before, _ := f.repo.ListGenres(ctx)
				if _, err := f.repo.CreateGenre(ctx, item.NewGenre{Name: "契約テスト 重複 " + c.name, QueryTemplate: "{name}", Aliases: c.aliases}); !errors.Is(err, item.ErrInvalid) {
					t.Errorf("err = %v, want ErrInvalid", err)
				}
				if after, _ := f.repo.ListGenres(ctx); len(after) != len(before) {
					t.Errorf("失敗したのにジャンルが増えた(%d → %d)", len(before), len(after))
				}
			})
		}

		distinct := [][]string{{"ガンダム", "カンダム"}, {"はいぐれーど", "ハイグレード"}, {"HG", "H G 1"}}
		g, err := f.repo.UpdateGenre(ctx, f.genreA.ID, item.GenrePatch{Aliases: &distinct})
		if err != nil {
			t.Fatalf("正規化後に違う語: %v(DB の照合順序で同一視していないか)", err)
		}
		if !aliasesEqual(g.Aliases, distinct) || !aliasesEqual(genreByID(t, f.repo, f.genreA.ID).Aliases, distinct) {
			t.Errorf("Aliases = %q, want %q", g.Aliases, distinct)
		}
	})
}
