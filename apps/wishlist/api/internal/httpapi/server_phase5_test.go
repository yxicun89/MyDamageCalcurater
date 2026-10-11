package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/ogp"
)

// フェーズ5 5-1b〜5-1d の HTTP(docs/phase5-spec.md)。

// ---- 5-1b ジャンルの色と絵柄 ----

// AC-5-1b-1・2: Genre は color(小文字)・icon・match_hosts・match_keywords を必ず返す。作成で省略したら #1d5fd1・box・[]。
func TestGenreStyle(t *testing.T) {
	e := newEnv(t)
	w := e.json(t, http.MethodPost, "/api/genres", map[string]any{"name": "既定の色"})
	if w.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", w.Code, w.Body.String())
	}
	g := decode[map[string]any](t, w)
	if g["color"] != "#1d5fd1" || g["icon"] != "box" || fmt.Sprint(g["match_hosts"]) != "[]" || fmt.Sprint(g["match_keywords"]) != "[]" {
		t.Errorf("既定値 = %v", g)
	}
	if _, ok := g["match_hosts"].([]any); !ok {
		t.Errorf("match_hosts が配列でない: %#v", g["match_hosts"])
	}

	w = e.json(t, http.MethodPost, "/api/genres", map[string]any{
		"name": "デュエマ", "color": "#E0393E", "icon": "card",
		"match_hosts": []string{"dm.takaratomy.co.jp"}, "match_keywords": []string{"デュエル・マスターズ", "デュエマ"},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", w.Code, w.Body.String())
	}
	g = decode[map[string]any](t, w)
	if g["color"] != "#e0393e" || g["icon"] != "card" || fmt.Sprint(g["match_hosts"]) != "[dm.takaratomy.co.jp]" || fmt.Sprint(g["match_keywords"]) != "[デュエル・マスターズ デュエマ]" {
		t.Errorf("作成 = %v", g)
	}

	path := fmt.Sprintf("/api/genres/%d", idOf(g))
	w = e.json(t, http.MethodPatch, path, map[string]any{"color": "#7C4DFF", "icon": "star", "match_keywords": []string{}})
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s", w.Code, w.Body.String())
	}
	g = decode[map[string]any](t, w)
	if g["color"] != "#7c4dff" || g["icon"] != "star" || fmt.Sprint(g["match_hosts"]) != "[dm.takaratomy.co.jp]" || fmt.Sprint(g["match_keywords"]) != "[]" {
		t.Errorf("PATCH 後 = %v", g)
	}

	// 一覧にも載る(newEnv のジャンルは既定値)
	gs := decode[map[string][]map[string]any](t, e.do(t, req{method: http.MethodGet, path: "/api/genres"}))["genres"]
	for _, x := range gs {
		for _, k := range []string{"color", "icon", "match_hosts", "match_keywords"} {
			if _, ok := x[k]; !ok {
				t.Errorf("ジャンル %v に %s が無い", x["name"], k)
			}
		}
	}
}

// AC-5-1b-2: 色の形式違反・列挙に無い絵柄は 422 unprocessable(作成・更新とも)。
func TestGenreStyle_Errors(t *testing.T) {
	e := newEnv(t)
	path := fmt.Sprintf("/api/genres/%d", e.genre.ID)
	for _, c := range []string{"1d5fd1", "#1d5fd", "#1d5fg1", "red", "#fff"} {
		expectError(t, e.json(t, http.MethodPost, "/api/genres", map[string]any{"name": "色 " + c, "color": c}), 422, "unprocessable")
		expectError(t, e.json(t, http.MethodPatch, path, map[string]any{"color": c}), 422, "unprocessable")
	}
	for _, ic := range []string{"heart", "Card", ""} {
		expectError(t, e.json(t, http.MethodPost, "/api/genres", map[string]any{"name": "絵柄 " + ic, "icon": ic}), 422, "unprocessable")
		expectError(t, e.json(t, http.MethodPatch, path, map[string]any{"icon": ic}), 422, "unprocessable")
	}
	gs := decode[map[string][]map[string]any](t, e.do(t, req{method: http.MethodGet, path: "/api/genres"}))["genres"]
	if len(gs) != 1 || gs[0]["color"] != "#1d5fd1" || gs[0]["icon"] != "box" {
		t.Errorf("失敗したのに変わった: %v", gs)
	}
}

// ---- 5-1c URL からの下書き ----

// phase5Genres は初期値(migration 000009)と同じ手がかりのジャンルを足し、名前 → id を返す。
func phase5Genres(t *testing.T, e *env) map[string]int64 {
	t.Helper()
	ctx := context.Background()
	out := map[string]int64{}
	// newEnv の「S.H.Figuarts」(sort_order 0)に語を足す
	kws := []string{"S.H.Figuarts", "フィギュアーツ"}
	if _, err := e.svc.UpdateGenre(ctx, e.genre.ID, item.GenrePatch{MatchKeywords: &kws}); err != nil {
		t.Fatal(err)
	}
	out["S.H.Figuarts"] = e.genre.ID
	for _, g := range []item.NewGenre{
		{Name: "デュエル・マスターズ", SortOrder: 10, MatchHosts: []string{"dm.takaratomy.co.jp"}, MatchKeywords: []string{"デュエル・マスターズ", "デュエマ"}},
		{Name: "ガンプラ", SortOrder: 30, MatchHosts: []string{"bandai-hobby.net"}, MatchKeywords: []string{"ガンプラ", "HG", "MG", "RG", "PG", "30MF", "30MM"}},
		{Name: "ポケモングッズ", SortOrder: 40, MatchHosts: []string{"www.pokemoncenter-online.com"}},
	} {
		c, err := e.svc.CreateGenre(ctx, g)
		if err != nil {
			t.Fatal(err)
		}
		out[g.Name] = c.ID
	}
	return out
}

// AC-5-1c-1・4・6: 取得した og:title からサイト名を除いた名前、ジャンルの推定と genre_guessed。
func TestDraftFromURL_Guess(t *testing.T) {
	cases := []struct {
		name        string
		url         string
		genre       string // リクエストに付けるジャンル(空なら付けない)
		title       string // Remote が返す og:title
		wantName    string
		wantGenre   string // 空なら null
		wantGuessed bool
	}{
		{"デュエマ(ホストで推定・型番とサイト名を除く)", "https://dm.takaratomy.co.jp/card/detail/?id=dm26rp3-077", "",
			"光る！流れる!!ベンザマン!!!(DM26RP3 77/77) | デュエル・マスターズ", "光る！流れる!!ベンザマン!!!", "デュエル・マスターズ", true},
		{"魂ウェブ(語で推定)", "https://tamashiiweb.com/item/15889/", "",
			"S.H.Figuarts ウルトラマンノア | 商品一覧 | 魂ウェブ", "S.H.Figuarts ウルトラマンノア", "S.H.Figuarts", true},
		{"ポケセン(ホストで推定)", "https://www.pokemoncenter-online.com/4521329419527.html", "",
			"ぬいぐるみ メガアブソル ｜【公式】ポケモンセンターオンライン", "ぬいぐるみ メガアブソル", "ポケモングッズ", true},
		{"バンダイ ホビーサイト(語で推定)", "https://bandai-hobby.net/item/01_7007/", "",
			"30MF カスタマイズストラクチャー3｜バンダイ ホビーサイト", "30MF カスタマイズストラクチャー3", "ガンプラ", true},
		{"genre_id を渡せばそれ(推定しない)", "https://dm.takaratomy.co.jp/card/detail/?id=x", "ガンプラ",
			"架空カード | デュエル・マスターズ", "架空カード", "ガンプラ", false},
		{"決まらなければ null", "https://shop.example.com/p/1", "",
			"ふつうの商品 | 架空ショップ", "ふつうの商品", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			ids := phase5Genres(t, e)
			img := "https://cdn.example.com/og.jpg"
			e.remote.draft = ogp.Draft{Title: c.title, ImageURL: &img}
			body := map[string]any{"url": c.url}
			if c.genre != "" {
				body["genre_id"] = ids[c.genre]
			}
			w := e.json(t, http.MethodPost, "/api/items/from-url", body)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d %s", w.Code, w.Body.String())
			}
			m := decode[map[string]any](t, w)
			if m["name"] != c.wantName || m["image_url"] != img || m["source_url"] != c.url {
				t.Errorf("draft = %v, want name %q", m, c.wantName)
			}
			var want any
			if c.wantGenre != "" {
				want = float64(ids[c.wantGenre])
			}
			if m["genre_id"] != want {
				t.Errorf("genre_id = %v, want %v(%s)", m["genre_id"], want, c.wantGenre)
			}
			if g, ok := m["genre_guessed"].(bool); !ok || g != c.wantGuessed {
				t.Errorf("genre_guessed = %#v, want %v", m["genre_guessed"], c.wantGuessed)
			}
			if len(e.remote.calls) != 1 {
				t.Errorf("取得 = %v, want 1 回", e.remote.calls)
			}
		})
	}
}

// AC-5-1c-7: title を渡せばページを取りに行かず、title を整えた名前・image_url(http/https の絶対 URL だけ)を使い、ジャンルを推定する。
func TestDraftFromURL_TitleHint(t *testing.T) {
	cases := []struct {
		name      string
		body      map[string]any
		wantName  string
		wantImage any // nil は null
		wantGenre string
	}{
		{"title と image_url", map[string]any{"url": "https://tamashiiweb.com/item/15889/", "title": "S.H.Figuarts ウルトラマンノア | 商品一覧 | 魂ウェブ",
			"image_url": "https://tamashiiweb.com/storage/images/products/thumbnail/x.webp"},
			"S.H.Figuarts ウルトラマンノア", "https://tamashiiweb.com/storage/images/products/thumbnail/x.webp", "S.H.Figuarts"},
		{"image_url なし → null", map[string]any{"url": "https://www.pokemoncenter-online.com/1.html", "title": "ぬいぐるみ ｜【公式】ポケモンセンターオンライン"},
			"ぬいぐるみ", nil, "ポケモングッズ"},
		{"相対 URL の image_url → null", map[string]any{"url": "https://shop.example.com/p/1", "title": "架空フィギュア", "image_url": "/img/a.jpg"},
			"架空フィギュア", nil, ""},
		{"javascript: の image_url → null", map[string]any{"url": "https://shop.example.com/p/1", "title": "架空フィギュア", "image_url": "javascript:alert(1)"},
			"架空フィギュア", nil, ""},
		{"data: の image_url → null", map[string]any{"url": "https://shop.example.com/p/1", "title": "架空フィギュア", "image_url": "data:image/png;base64,AAAA"},
			"架空フィギュア", nil, ""},
		{"駿河屋のように読めないページでも登録できる", map[string]any{"url": "https://www.suruga-ya.jp/product/detail/602290829", "title": "中古 架空フィギュア",
			"image_url": "https://www.suruga-ya.jp/database/pics_light/game/602290829.jpg"},
			"中古 架空フィギュア", "https://www.suruga-ya.jp/database/pics_light/game/602290829.jpg", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			ids := phase5Genres(t, e)
			e.remote.draftErr = fmt.Errorf("取りに行ってはいけない")
			w := e.json(t, http.MethodPost, "/api/items/from-url", c.body)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d %s", w.Code, w.Body.String())
			}
			if len(e.remote.calls) != 0 {
				t.Errorf("ページを取りに行った: %v", e.remote.calls)
			}
			m := decode[map[string]any](t, w)
			if m["name"] != c.wantName || m["image_url"] != c.wantImage || m["source_url"] != c.body["url"] {
				t.Errorf("draft = %v, want name %q image %v", m, c.wantName, c.wantImage)
			}
			var want any
			if c.wantGenre != "" {
				want = float64(ids[c.wantGenre])
			}
			if m["genre_id"] != want || m["genre_guessed"] != (c.wantGenre != "") {
				t.Errorf("genre_id = %v genre_guessed = %v, want %v %v", m["genre_id"], m["genre_guessed"], want, c.wantGenre != "")
			}
		})
	}
	// title があっても url の検査は今までどおり(取得はしない)
	e := newEnv(t)
	expectError(t, e.json(t, http.MethodPost, "/api/items/from-url", map[string]any{"url": "ftp://example.com/x", "title": "x"}), 422, "unprocessable")
	if len(e.remote.calls) != 0 {
		t.Errorf("取得した: %v", e.remote.calls)
	}
}

// AC-5-1c-3: 駿河屋の商品ページは取りに行かず、名前は空・画像は photo.php で 200。それ以外の駿河屋のページは今までどおり取得する。
func TestDraftFromURL_Surugaya(t *testing.T) {
	e := newEnv(t)
	e.remote.draftErr = fmt.Errorf("取りに行ってはいけない")
	w := e.json(t, http.MethodPost, "/api/items/from-url", map[string]any{"url": "https://www.suruga-ya.jp/product/detail/602290829"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	if len(e.remote.calls) != 0 {
		t.Errorf("ページを取りに行った: %v", e.remote.calls)
	}
	m := decode[map[string]any](t, w)
	if m["name"] != "" || m["image_url"] != "https://www.suruga-ya.jp/database/photo.php?shinaban=602290829&size=l" || m["genre_id"] != nil || m["genre_guessed"] != false {
		t.Errorf("draft = %v", m)
	}

	e = newEnv(t)
	e.remote.draft = ogp.Draft{Title: "検索 | 駿河屋"}
	w = e.json(t, http.MethodPost, "/api/items/from-url", map[string]any{"url": "https://www.suruga-ya.jp/search?search_word=x"})
	if w.Code != http.StatusOK || len(e.remote.calls) != 1 {
		t.Errorf("駿河屋の検索ページ = %d %s、取得 %v(今までどおり 1 回取得する)", w.Code, w.Body.String(), e.remote.calls)
	}
}

// ---- 5-1d 一覧に目安を載せる ----

// AC-5-1d-1: GET /api/items の各 Item に summary。目安が無ければ {low:null, status:"none", fetched_at:null}、
// 保存済みの目安があれば low・status・fetched_at(UTC の RFC 3339)。ジャンルに無いサイトの目安は使わない。
func TestListItems_Summary(t *testing.T) {
	e := newEnv(t)
	a := idOf(e.createItem(t, map[string]string{"name": "目安あり"}))
	b := idOf(e.createItem(t, map[string]string{"name": "目安なし"}))
	c := idOf(e.createItem(t, map[string]string{"name": "出品なし"}))
	ctx := context.Background()
	at := time.Date(2026, 10, 10, 3, 4, 5, 0, time.UTC)
	low := 4800
	other, err := e.svc.CreateSite(ctx, item.NewSite{Name: "ジャンルに無いサイト", SearchURLTemplate: "https://other.example.com/s?q={q}"})
	if err != nil {
		t.Fatal(err)
	}
	cheap := 100
	for _, s := range []item.Estimate{
		{ItemID: a, SiteID: e.site1.ID, Low: &low, Status: item.EstimateOK, FetchedAt: at},
		{ItemID: a, SiteID: other.ID, Low: &cheap, Status: item.EstimateOK, FetchedAt: at},
		{ItemID: c, SiteID: e.site1.ID, Status: item.EstimateNoResult, FetchedAt: at},
		{ItemID: c, SiteID: e.site2.ID, Status: item.EstimateNoResult, FetchedAt: at},
	} {
		if err := e.repo.SaveSiteResult(ctx, s, nil); err != nil {
			t.Fatal(err)
		}
	}
	list := decode[struct {
		Items []map[string]any `json:"items"`
	}](t, e.do(t, req{method: http.MethodGet, path: "/api/items"}))
	want := map[int64]string{
		a: `{"fetched_at":"2026-10-10T03:04:05Z","low":4800,"status":"ok"}`,
		b: `{"fetched_at":null,"low":null,"status":"none"}`,
		c: `{"fetched_at":null,"low":null,"status":"no_result"}`,
	}
	if len(list.Items) != 3 {
		t.Fatalf("items = %v", list.Items)
	}
	for _, it := range list.Items {
		got, ok := it["summary"]
		if !ok || got == nil {
			t.Errorf("%v の summary が無い・null", it["name"])
			continue
		}
		if s := canonicalAny(t, got); s != want[idOf(it)] {
			t.Errorf("%v の summary = %s, want %s", it["name"], s, want[idOf(it)])
		}
	}
}

// canonicalAny は decode 済みの値をキー順の JSON にする(比較用)。
func canonicalAny(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// AC-5-1d-1: 一覧の summary は詳細(GET /api/items/{id}/estimates)のまとめと同じ値。
// このプロセスに Fetcher が無いサイト(Renderer の無い api のメルカリ。夜間の CronJob が保存した目安)も両方に入る。
func TestListItems_SummaryEqualsDetail(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	st, err := e.svc.CreateSite(ctx, item.NewSite{Name: "メルカリ", SearchURLTemplate: "https://jp.mercari.com/search?keyword={q}", FetchType: item.FetchHeadless})
	if err != nil {
		t.Fatal(err)
	}
	g, err := e.svc.CreateGenre(ctx, item.NewGenre{Name: "取れないサイトだけ", SiteIDs: []int64{st.ID}})
	if err != nil {
		t.Fatal(err)
	}
	id := idOf(e.createItem(t, map[string]string{"name": "目安あり", "genre_id": fmt.Sprint(g.ID)}))
	at := time.Date(2026, 10, 10, 3, 0, 0, 0, time.UTC)
	low := 3000
	if err := e.repo.SaveSiteResult(ctx, item.Estimate{ItemID: id, SiteID: st.ID, Low: &low, Status: item.EstimateOK, FetchedAt: at}, nil); err != nil {
		t.Fatal(err)
	}
	detail := decode[map[string]any](t, e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/estimates", id)}))
	list := decode[struct {
		Items []map[string]any `json:"items"`
	}](t, e.do(t, req{method: http.MethodGet, path: "/api/items?genre_id=" + fmt.Sprint(g.ID)}))
	if len(list.Items) != 1 {
		t.Fatalf("items = %v", list.Items)
	}
	sum, _ := list.Items[0]["summary"].(map[string]any)
	if detail["summary_low"] != float64(low) {
		t.Errorf("詳細の summary_low = %v, want %d", detail["summary_low"], low)
	}
	if sum["low"] != detail["summary_low"] || sum["fetched_at"] != detail["summary_fetched_at"] || sum["status"] != "ok" {
		t.Errorf("一覧 %v と詳細 %v/%v が違う", sum, detail["summary_low"], detail["summary_fetched_at"])
	}
}
