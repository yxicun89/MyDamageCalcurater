package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// フェーズ4-2 価格の推移の API(docs/phase4-spec.md AC-H11〜H13)。phase3T0 は JST 2026-10-03 12:00。

// AC-H11: GET price-history は、取得が ok だったサイトの目安を日ごと(JST)に返す。days の既定は 90。
// low・mid は保存した目安と同じ値、day は YYYY-MM-DD。overall は日ごとの最安。
func TestPriceHistory(t *testing.T) {
	e := newEnv(t)
	id, site := e.priceItem(t)
	if w := e.do(t, req{method: http.MethodPost, path: fmt.Sprintf("/api/items/%d/estimates/refresh", id)}); w.Code != http.StatusAccepted {
		t.Fatalf("refresh = %d %s", w.Code, w.Body.String())
	}
	e.est.Wait()
	est := decode[map[string]any](t, e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/estimates", id)}))
	sites, _ := est["sites"].([]any)
	if len(sites) != 1 {
		t.Fatalf("estimates.sites = %v", est["sites"])
	}
	s0 := sites[0].(map[string]any)
	low, mid := s0["low"], s0["mid"]
	if low == nil || mid == nil {
		t.Fatalf("目安 = %v(low・mid があるはず)", s0)
	}

	w := e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/price-history", id)})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d %s", w.Code, w.Body.String())
	}
	want := fmt.Sprintf(`{"days":90,"item_id":%d,"overall":[{"day":"2026-10-03","low":%v}],"sites":[{"points":[{"day":"2026-10-03","low":%v,"mid":%v}],"site_id":%d}]}`,
		id, low, low, mid, site.ID)
	if got := canonicalJSON(t, w.Body.Bytes()); got != want {
		t.Errorf("本文 = %s\nwant   %s", got, want)
	}

	w = e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/price-history?days=180", id)})
	if m := decode[map[string]any](t, w); w.Code != http.StatusOK || m["days"] != float64(180) {
		t.Errorf("days=180 = %d %v", w.Code, m)
	}
	w = e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/price-history?days=1", id)})
	if m := decode[map[string]any](t, w); w.Code != http.StatusOK || m["days"] != float64(1) {
		t.Errorf("days=1 = %d %v", w.Code, m)
	}
}

// AC-H11: 推移の無い商品は sites・overall とも [](null にしない)。mid が無い日は "mid": null。
func TestPriceHistory_EmptyAndNullMid(t *testing.T) {
	e := newEnv(t)
	id := idOf(e.createItem(t, nil))
	w := e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/price-history", id)})
	want := fmt.Sprintf(`{"days":90,"item_id":%d,"overall":[],"sites":[]}`, id)
	if got := canonicalJSON(t, w.Body.Bytes()); w.Code != http.StatusOK || got != want {
		t.Errorf("推移なし = %d %s, want %s", w.Code, got, want)
	}

	// 1 件だけの出品 → 目安は low だけ(mid は null)。
	pid, _ := e.priceItem(t)
	e.fetch.mu.Lock()
	e.fetch.listings = e.fetch.listings[:1]
	e.fetch.mu.Unlock()
	if w := e.do(t, req{method: http.MethodPost, path: fmt.Sprintf("/api/items/%d/estimates/refresh", pid)}); w.Code != http.StatusAccepted {
		t.Fatalf("refresh = %d", w.Code)
	}
	e.est.Wait()
	w = e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/price-history", pid)})
	if !strings.Contains(w.Body.String(), `"mid":null`) {
		t.Errorf("mid の無い日 = %s, want \"mid\":null を含む", w.Body.String())
	}
}

// AC-H12: days が 1〜180 の外・数でないなら 400 bad_request。id が 0 以下・数でないなら 400。存在しない商品は 404。トークンが無ければ 401。
func TestPriceHistory_Errors(t *testing.T) {
	e := newEnv(t)
	id := idOf(e.createItem(t, nil))
	for _, q := range []string{"days=0", "days=-1", "days=181", "days=abc", "days=1.5"} {
		t.Run(q, func(t *testing.T) {
			expectError(t, e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/price-history?%s", id, q)}), http.StatusBadRequest, "bad_request")
		})
	}
	for _, p := range []string{"/api/items/0/price-history", "/api/items/abc/price-history"} {
		expectError(t, e.do(t, req{method: http.MethodGet, path: p}), http.StatusBadRequest, "bad_request")
	}
	expectError(t, e.do(t, req{method: http.MethodGet, path: "/api/items/99999999/price-history"}), http.StatusNotFound, "not_found")
	expectError(t, e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/price-history", id), auth: "-"}), http.StatusUnauthorized, "unauthorized")
}

// canonicalJSON はキーを並べ替えた JSON にする(比較用)。
func canonicalJSON(t *testing.T, b []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("JSON でない: %s", b)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
