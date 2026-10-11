package item_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// フェーズ5 5-1b: ジャンルの色と絵柄、URL の手がかり(docs/phase5-spec.md)。Service の検査と既定値。

// AC-5-1b-2: 作成で色・絵柄を省略したら #1d5fd1・box。手がかりは省略したら空。
func TestService_GenreStyleDefaults(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	g, err := e.svc.CreateGenre(ctx, item.NewGenre{Name: "既定の色"})
	if err != nil {
		t.Fatal(err)
	}
	if item.DefaultGenreColor != "#1d5fd1" || item.DefaultGenreIcon != item.IconBox {
		t.Errorf("既定の定数 = %q / %q", item.DefaultGenreColor, item.DefaultGenreIcon)
	}
	if g.Color != "#1d5fd1" || g.Icon != item.IconBox {
		t.Errorf("既定値 = %q / %q, want #1d5fd1 / box", g.Color, g.Icon)
	}
	if len(g.MatchHosts) != 0 || len(g.MatchKeywords) != 0 {
		t.Errorf("手がかり = %q / %q, want 空", g.MatchHosts, g.MatchKeywords)
	}
	// newEnv のジャンル(色を渡していない)も既定値で読める
	list, err := e.svc.ListGenres(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range list {
		if x.Color != "#1d5fd1" || x.Icon != item.IconBox {
			t.Errorf("一覧の %s = %q / %q", x.Name, x.Color, x.Icon)
		}
	}
}

// AC-5-1b-1・2: 色は大文字も受けて小文字に揃える。絵柄は 6 つのどれでも通る。手がかりは渡した順のまま保存する。
func TestService_GenreStyleValid(t *testing.T) {
	ctx := context.Background()
	icons := []item.GenreIcon{item.IconCard, item.IconFigure, item.IconRobot, item.IconGoods, item.IconStar, item.IconBox}
	for i, icon := range icons {
		t.Run(string(icon), func(t *testing.T) {
			e := newEnv(t)
			g, err := e.svc.CreateGenre(ctx, item.NewGenre{Name: fmt.Sprintf("色%d", i), Color: "#E0393E", Icon: icon,
				MatchHosts: []string{"dm.takaratomy.co.jp"}, MatchKeywords: []string{"デュエマ", "デュエル・マスターズ"}})
			if err != nil {
				t.Fatal(err)
			}
			if g.Color != "#e0393e" || g.Icon != icon {
				t.Errorf("作成 = %q / %q, want #e0393e / %s", g.Color, g.Icon, icon)
			}
			if fmt.Sprint(g.MatchHosts) != "[dm.takaratomy.co.jp]" || fmt.Sprintf("%q", g.MatchKeywords) != `["デュエマ" "デュエル・マスターズ"]` {
				t.Errorf("手がかり = %q / %q", g.MatchHosts, g.MatchKeywords)
			}
		})
	}
	e := newEnv(t)
	color, icon := "#7C4DFF", item.IconFigure
	hosts, kws := []string{"tamashiiweb.com"}, []string{"S.H.Figuarts"}
	g, err := e.svc.UpdateGenre(ctx, e.genre.ID, item.GenrePatch{Color: &color, Icon: &icon, MatchHosts: &hosts, MatchKeywords: &kws})
	if err != nil {
		t.Fatal(err)
	}
	if g.Color != "#7c4dff" || g.Icon != item.IconFigure || fmt.Sprint(g.MatchHosts) != "[tamashiiweb.com]" || fmt.Sprint(g.MatchKeywords) != "[S.H.Figuarts]" {
		t.Errorf("更新 = %+v", g)
	}
	// 省略した項目は変えない。空のスライスは全部外す。
	name := "名前だけ変える"
	empty := []string{}
	if g, err = e.svc.UpdateGenre(ctx, e.genre.ID, item.GenrePatch{Name: &name, MatchHosts: &empty}); err != nil {
		t.Fatal(err)
	}
	if g.Color != "#7c4dff" || g.Icon != item.IconFigure || len(g.MatchHosts) != 0 || fmt.Sprint(g.MatchKeywords) != "[S.H.Figuarts]" {
		t.Errorf("部分更新 = %+v", g)
	}
}

// AC-5-1b-2: 色の形式違反・列挙に無い絵柄は ErrInvalid(422)。作成・更新とも何も変えない。
func TestService_GenreStyleInvalid(t *testing.T) {
	ctx := context.Background()
	badColors := []string{"1d5fd1", "#1d5fd", "#1d5fd1a", "#1d5fg1", "#fff", "red", " #1d5fd1", "#1d5fd1 ", "＃1d5fd1"}
	for _, c := range badColors {
		t.Run("color "+c, func(t *testing.T) {
			e := newEnv(t)
			if _, err := e.svc.CreateGenre(ctx, item.NewGenre{Name: "色の検査", Color: c}); !errors.Is(err, item.ErrInvalid) {
				t.Errorf("CreateGenre err = %v, want ErrInvalid", err)
			}
			cc := c
			if _, err := e.svc.UpdateGenre(ctx, e.genre.ID, item.GenrePatch{Color: &cc}); !errors.Is(err, item.ErrInvalid) {
				t.Errorf("UpdateGenre err = %v, want ErrInvalid", err)
			}
			list, _ := e.svc.ListGenres(ctx)
			if len(list) != 1 || list[0].Color != "#1d5fd1" {
				t.Errorf("失敗したのに変わった: %+v", list)
			}
		})
	}
	for _, ic := range []item.GenreIcon{"heart", "Card", "BOX", " "} {
		t.Run("icon "+string(ic), func(t *testing.T) {
			e := newEnv(t)
			if _, err := e.svc.CreateGenre(ctx, item.NewGenre{Name: "絵柄の検査", Icon: ic}); !errors.Is(err, item.ErrInvalid) {
				t.Errorf("CreateGenre err = %v, want ErrInvalid", err)
			}
			icc := ic
			if _, err := e.svc.UpdateGenre(ctx, e.genre.ID, item.GenrePatch{Icon: &icc}); !errors.Is(err, item.ErrInvalid) {
				t.Errorf("UpdateGenre err = %v, want ErrInvalid", err)
			}
		})
	}
	// 更新で空文字の色・絵柄は「省略」ではなく形式違反
	e := newEnv(t)
	emptyColor, emptyIcon := "", item.GenreIcon("")
	if _, err := e.svc.UpdateGenre(ctx, e.genre.ID, item.GenrePatch{Color: &emptyColor}); !errors.Is(err, item.ErrInvalid) {
		t.Errorf("空の色 err = %v, want ErrInvalid", err)
	}
	if _, err := e.svc.UpdateGenre(ctx, e.genre.ID, item.GenrePatch{Icon: &emptyIcon}); !errors.Is(err, item.ErrInvalid) {
		t.Errorf("空の絵柄 err = %v, want ErrInvalid", err)
	}
}
