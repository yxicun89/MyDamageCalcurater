package item_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// フェーズ4-1 表記揺れの辞書の入力の検査(docs/phase4-spec.md AC-A8)。

func quoted(a [][]string) string { return fmt.Sprintf("%q", a) }

// AC-A8: Service は各語の前後の空白を除き、空の語・1 語だけのグループ・MaxAliasLen 超え・正規化して空の語・
// 正規化後の重複を ErrInvalid にする(作成・更新とも。何も変えない)。
func TestService_GenreAliasesInvalid(t *testing.T) {
	ctx := context.Background()
	long := strings.Repeat("あ", item.MaxAliasLen+1)
	cases := []struct {
		name    string
		aliases [][]string
	}{
		{"空の語", [][]string{{"HG", ""}}},
		{"空白だけの語", [][]string{{"HG", " 　"}}},
		{"1 語だけのグループ", [][]string{{"HG", "ハイグレード"}, {"MG"}}},
		{"空のグループ", [][]string{{}}},
		{"空白を除くと 1 語(重複)", [][]string{{"HG", " HG "}}},
		{"長すぎる語", [][]string{{"HG", long}}},
		{"正規化して空になる語", [][]string{{"HG", "・・"}}},
		{"正規化して 1 文字の語", [][]string{{"HG", "Ｇ"}}},
		{"半角カンマを含む語", [][]string{{"HG", "ハイ,グレード"}}},
		{"全角カンマを含む語", [][]string{{"HG", "ハイ，グレード"}}},
		{"読点を含む語", [][]string{{"HG", "ハイ、グレード"}}},
		{"正規化後の重複(別グループ)", [][]string{{"HG", "ハイグレード"}, {"ｈｇ", "エイチジー"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			a := c.aliases
			if _, err := e.svc.UpdateGenre(ctx, e.genre.ID, item.GenrePatch{Aliases: &a}); !errors.Is(err, item.ErrInvalid) {
				t.Errorf("UpdateGenre err = %v, want ErrInvalid", err)
			}
			if _, err := e.svc.CreateGenre(ctx, item.NewGenre{Name: "別名の検査", Aliases: c.aliases}); !errors.Is(err, item.ErrInvalid) {
				t.Errorf("CreateGenre err = %v, want ErrInvalid", err)
			}
			list, _ := e.svc.ListGenres(ctx)
			if len(list) != 1 || len(list[0].Aliases) != 0 {
				t.Errorf("失敗したのに変わった: %+v", list)
			}
		})
	}
}

// AC-A8: ちょうど MaxAliasLen 文字は通る。各語の前後の空白は除いて保存する。空のスライスは「全部消す」。
func TestService_GenreAliasesValid(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	edge := strings.Repeat("あ", item.MaxAliasLen)
	in := [][]string{{" HG ", "ハイグレード\t"}, {edge, "MG"}}
	g, err := e.svc.UpdateGenre(ctx, e.genre.ID, item.GenrePatch{Aliases: &in})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"HG", "ハイグレード"}, {edge, "MG"}}
	if quoted(g.Aliases) != quoted(want) {
		t.Errorf("Aliases = %q, want %q", g.Aliases, want)
	}
	if g, err = e.svc.UpdateGenre(ctx, e.genre.ID, item.GenrePatch{Aliases: &[][]string{}}); err != nil || len(g.Aliases) != 0 {
		t.Errorf("空で置き換え = %+v, %v", g.Aliases, err)
	}
	c, err := e.svc.CreateGenre(ctx, item.NewGenre{Name: "ガンプラ", Aliases: [][]string{{"RG", " リアルグレード "}}})
	if err != nil {
		t.Fatal(err)
	}
	if quoted(c.Aliases) != quoted([][]string{{"RG", "リアルグレード"}}) {
		t.Errorf("作成の Aliases = %q", c.Aliases)
	}
}
