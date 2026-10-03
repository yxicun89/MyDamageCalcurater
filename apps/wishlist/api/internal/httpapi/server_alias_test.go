package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// フェーズ4-1 表記揺れの辞書の API(docs/phase4-spec.md AC-A10・A11)。

// aliasesJSON は応答の aliases を正規の JSON 文字列にする(無ければ "<missing>")。
func aliasesJSON(t *testing.T, g map[string]any) string {
	t.Helper()
	v, ok := g["aliases"]
	if !ok {
		return "<missing>"
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// AC-A10: Genre は常に aliases(グループの配列。無ければ [])を返す。POST で渡すと保存、PATCH で渡すと全件置き換え・省略で変えない・[] で消す。
func TestGenreAliases(t *testing.T) {
	e := newEnv(t)
	gs := decode[map[string][]map[string]any](t, e.do(t, req{method: http.MethodGet, path: "/api/genres"}))["genres"]
	if len(gs) != 1 || aliasesJSON(t, gs[0]) != "[]" {
		t.Errorf("一覧の aliases = %s, want [](無くても常に返す)", aliasesJSON(t, gs[0]))
	}

	w := e.json(t, http.MethodPost, "/api/genres", map[string]any{"name": "ガンプラ", "aliases": [][]string{{"HG", "ハイグレード"}, {"MG", "マスターグレード"}}})
	if w.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", w.Code, w.Body.String())
	}
	g := decode[map[string]any](t, w)
	if got := aliasesJSON(t, g); got != `[["HG","ハイグレード"],["MG","マスターグレード"]]` {
		t.Errorf("POST の aliases = %s", got)
	}
	w = e.json(t, http.MethodPost, "/api/genres", map[string]any{"name": "デュエマ"})
	if w.Code != http.StatusCreated {
		t.Fatalf("POST = %d %s", w.Code, w.Body.String())
	}
	if got := aliasesJSON(t, decode[map[string]any](t, w)); got != "[]" {
		t.Errorf("aliases なしの POST の aliases = %s, want []", got)
	}

	path := fmt.Sprintf("/api/genres/%d", idOf(g))
	w = e.json(t, http.MethodPatch, path, map[string]any{"aliases": [][]string{{" RG ", "リアルグレード"}}})
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s", w.Code, w.Body.String())
	}
	if got := aliasesJSON(t, decode[map[string]any](t, w)); got != `[["RG","リアルグレード"]]` {
		t.Errorf("PATCH の aliases = %s, want 全件置き換え(前後の空白は除く)", got)
	}
	w = e.json(t, http.MethodPatch, path, map[string]any{"name": "ガンプラ改"})
	if got := aliasesJSON(t, decode[map[string]any](t, w)); w.Code != http.StatusOK || got != `[["RG","リアルグレード"]]` {
		t.Errorf("aliases を省略した PATCH = %d, aliases %s(変えない)", w.Code, got)
	}
	gs = decode[map[string][]map[string]any](t, e.do(t, req{method: http.MethodGet, path: "/api/genres"}))["genres"]
	for _, x := range gs {
		if idOf(x) == idOf(g) && aliasesJSON(t, x) != `[["RG","リアルグレード"]]` {
			t.Errorf("一覧の aliases = %s", aliasesJSON(t, x))
		}
	}
	w = e.json(t, http.MethodPatch, path, map[string]any{"aliases": []any{}})
	if got := aliasesJSON(t, decode[map[string]any](t, w)); w.Code != http.StatusOK || got != "[]" {
		t.Errorf("aliases: [] の PATCH = %d, aliases %s(全部消す)", w.Code, got)
	}
}

// AC-A11: 空の語・1 語だけのグループ・正規化後の重複・長すぎる語は 422 unprocessable(POST・PATCH とも。何も変えない)。
// 形が違う(文字列の配列の配列でない)本文は 400 bad_request。
func TestGenreAliases_Errors(t *testing.T) {
	e := newEnv(t)
	path := fmt.Sprintf("/api/genres/%d", e.genre.ID)
	long := ""
	for range 65 {
		long += "あ"
	}
	bad := []struct {
		name    string
		aliases any
	}{
		{"空の語", [][]string{{"HG", ""}}},
		{"1 語だけのグループ", [][]string{{"HG"}}},
		{"正規化後の重複", [][]string{{"HG", "ハイグレード"}, {"ｈｇ", "エイチジー"}}},
		{"長すぎる語", [][]string{{"HG", long}}},
	}
	for _, c := range bad {
		t.Run(c.name, func(t *testing.T) {
			expectError(t, e.json(t, http.MethodPost, "/api/genres", map[string]any{"name": "新ジャンル " + c.name, "aliases": c.aliases}), 422, "unprocessable")
			expectError(t, e.json(t, http.MethodPatch, path, map[string]any{"name": "変わらない", "aliases": c.aliases}), 422, "unprocessable")
		})
	}
	expectError(t, e.json(t, http.MethodPatch, path, `{"aliases": ["HG", "ハイグレード"]}`), 400, "bad_request")
	expectError(t, e.json(t, http.MethodPatch, path, `{"aliases": [[1, 2]]}`), 400, "bad_request")

	gs := decode[map[string][]map[string]any](t, e.do(t, req{method: http.MethodGet, path: "/api/genres"}))["genres"]
	if len(gs) != 1 || gs[0]["name"] != "S.H.Figuarts" || aliasesJSON(t, gs[0]) != "[]" {
		t.Errorf("失敗したのに変わった: %v", gs)
	}
}
