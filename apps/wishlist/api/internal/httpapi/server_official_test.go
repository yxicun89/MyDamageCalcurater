package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/testimg"
)

// フェーズ4-3 公式サイトの販売状況の API(docs/phase4-spec.md AC-O26〜O28)。

// AC-O26: watch_official は作成(multipart・JSON)と PATCH で切り替える。source_url が無いのに ON なら 422(何も作らない・変えない)。
// multipart の値は true / false だけ(それ以外は 400)。
func TestItemWatchOfficial(t *testing.T) {
	e := newEnv(t)
	it := e.createItem(t, map[string]string{"source_url": "https://tamashii.example/item/1/", "watch_official": "true"})
	if it["watch_official"] != true {
		t.Errorf("作成結果の watch_official = %v", it["watch_official"])
	}
	if v, ok := it["official_status"]; !ok || v != nil {
		t.Errorf("作成結果の official_status = %v(有無 %v), want null(常に返す)", v, ok)
	}
	off := e.createItem(t, nil)
	if off["watch_official"] != false {
		t.Errorf("既定の watch_official = %v, want false(常に返す)", off["watch_official"])
	}
	if f := e.createItem(t, map[string]string{"watch_official": "false"}); f["watch_official"] != false {
		t.Errorf("watch_official=false = %v", f["watch_official"])
	}

	// multipart:source_url なしで ON → 422、値が不正 → 400。どちらも商品・画像を作らない
	before := countFiles(t, e.dir)
	for _, c := range []struct {
		fields map[string]string
		status int
		code   string
	}{
		{map[string]string{"watch_official": "true"}, http.StatusUnprocessableEntity, "unprocessable"},
		{map[string]string{"watch_official": "yes", "source_url": "https://tamashii.example/item/1/"}, http.StatusBadRequest, "bad_request"},
		{map[string]string{"watch_official": "1", "source_url": "https://tamashii.example/item/1/"}, http.StatusBadRequest, "bad_request"},
	} {
		c.fields["genre_id"] = fmt.Sprint(e.genre.ID)
		c.fields["name"] = "x"
		body, ct := multipartBody(t, c.fields, testimg.PNG())
		expectError(t, e.do(t, req{method: http.MethodPost, path: "/api/items", body: body, ctype: ct}), c.status, c.code)
	}
	if n := countFiles(t, e.dir); n != before {
		t.Errorf("失敗したのに画像が増えた(%d → %d)", before, n)
	}

	// JSON:source_url なしで ON → 422。画像を取りに行かない
	e.remote.image = testimg.JPEG()
	w := e.json(t, http.MethodPost, "/api/items", map[string]any{"genre_id": e.genre.ID, "name": "x", "image_url": "https://cdn.example.com/a.jpg", "watch_official": true})
	expectError(t, w, http.StatusUnprocessableEntity, "unprocessable")
	if len(e.remote.calls) != 0 {
		t.Errorf("検査の前に画像を取った: %v", e.remote.calls)
	}
	w = e.json(t, http.MethodPost, "/api/items", map[string]any{"genre_id": e.genre.ID, "name": "x", "image_url": "https://cdn.example.com/a.jpg", "watch_official": true, "source_url": "https://tamashii.example/item/9/"})
	if w.Code != http.StatusCreated || decode[map[string]any](t, w)["watch_official"] != true {
		t.Errorf("JSON で ON = %d %s", w.Code, w.Body.String())
	}

	// PATCH
	offPath := fmt.Sprintf("/api/items/%d", idOf(off))
	expectError(t, e.json(t, http.MethodPatch, offPath, map[string]any{"watch_official": true}), http.StatusUnprocessableEntity, "unprocessable")
	w = e.json(t, http.MethodPatch, offPath, map[string]any{"watch_official": true, "source_url": "https://tamashii.example/item/2/"})
	if w.Code != http.StatusOK || decode[map[string]any](t, w)["watch_official"] != true {
		t.Errorf("source_url と同時に ON = %d %s", w.Code, w.Body.String())
	}
	expectError(t, e.json(t, http.MethodPatch, offPath, `{"source_url": null}`), http.StatusUnprocessableEntity, "unprocessable")
	w = e.json(t, http.MethodPatch, offPath, `{"source_url": null, "watch_official": false}`)
	if m := decode[map[string]any](t, w); w.Code != http.StatusOK || m["watch_official"] != false || m["source_url"] != nil {
		t.Errorf("source_url を消して OFF = %d %v", w.Code, m)
	}
	expectError(t, e.json(t, http.MethodPatch, offPath, `{"watch_official": "true"}`), http.StatusBadRequest, "bad_request")
}

// AC-O27: Item(GET 1 件・一覧・PATCH の応答)に official_status を出す。無ければ null。
// 時刻は RFC 3339 の UTC、changed_at・previous_status は変化が無ければ null。
func TestItemOfficialStatus(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	it := e.createItem(t, map[string]string{"source_url": "https://tamashii.example/item/1/", "watch_official": "true"})
	other := e.createItem(t, nil)
	id := idOf(it)
	jst := time.FixedZone("JST", 9*60*60)
	t1 := time.Date(2026, 10, 3, 3, 0, 0, 0, jst)
	t2 := time.Date(2026, 10, 4, 3, 0, 0, 0, jst)
	if _, err := e.repo.SaveOfficialCheck(ctx, id, item.OfficialCheck{State: item.OfficialPreorder, Evidence: []string{"予約受付中", "予約する"}, At: t1}); err != nil {
		t.Fatal(err)
	}
	want := `{"changed_at":null,"checked_at":"2026-10-02T18:00:00Z","evidence":["予約受付中","予約する"],"last_attempt_at":"2026-10-02T18:00:00Z","last_result":"preorder","previous_status":null,"status":"preorder"}`
	check := func(label string, m map[string]any, want string) {
		t.Helper()
		got, err := jsonString(m["official_status"])
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s の official_status = %s\nwant %s", label, got, want)
		}
	}
	check("GET", decode[map[string]any](t, e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d", id)})), want)
	list := decode[map[string]any](t, e.do(t, req{method: http.MethodGet, path: "/api/items"}))
	for _, x := range list["items"].([]any) {
		m := x.(map[string]any)
		switch idOf(m) {
		case id:
			check("一覧", m, want)
		case idOf(other):
			if v, ok := m["official_status"]; !ok || v != nil {
				t.Errorf("状態の無い商品の official_status = %v(有無 %v), want null", v, ok)
			}
			if m["watch_official"] != false {
				t.Errorf("一覧の watch_official = %v", m["watch_official"])
			}
		}
	}

	// failed は status を上書きしない。そのあと変化すると changed_at・previous_status が付く
	if _, err := e.repo.SaveOfficialCheck(ctx, id, item.OfficialCheck{State: item.OfficialFailed, At: t2}); err != nil {
		t.Fatal(err)
	}
	want = `{"changed_at":null,"checked_at":"2026-10-02T18:00:00Z","evidence":["予約受付中","予約する"],"last_attempt_at":"2026-10-03T18:00:00Z","last_result":"failed","previous_status":null,"status":"preorder"}`
	check("failed のあと", decode[map[string]any](t, e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d", id)})), want)
	if _, err := e.repo.SaveOfficialCheck(ctx, id, item.OfficialCheck{State: item.OfficialEnded, Evidence: []string{"予約受付終了"}, At: t2.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	want = `{"changed_at":"2026-10-03T19:00:00Z","checked_at":"2026-10-03T19:00:00Z","evidence":["予約受付終了"],"last_attempt_at":"2026-10-03T19:00:00Z","last_result":"ended","previous_status":"preorder","status":"ended"}`
	check("変化のあと", decode[map[string]any](t, e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d", id)})), want)

	// PATCH の応答にも出る。source_url を変えると消える
	w := e.json(t, http.MethodPatch, fmt.Sprintf("/api/items/%d", id), map[string]any{"name": "グリス改"})
	check("PATCH", decode[map[string]any](t, w), want)
	w = e.json(t, http.MethodPatch, fmt.Sprintf("/api/items/%d", id), map[string]any{"source_url": "https://tamashii.example/item/99/"})
	if m := decode[map[string]any](t, w); m["official_status"] != nil {
		t.Errorf("source_url を変えたあとの official_status = %v, want null", m["official_status"])
	}
}

// AC-O28: 販売状況を取りに行く API は無い(夜間だけ)。GET estimates・POST refresh は公式ページの状態を変えない。
func TestOfficialNotFetchedByAPI(t *testing.T) {
	e := newEnv(t)
	it := e.createItem(t, map[string]string{"source_url": "https://tamashii.example/item/1/", "watch_official": "true"})
	id := idOf(it)
	e.do(t, req{method: http.MethodGet, path: fmt.Sprintf("/api/items/%d/estimates", id)})
	e.do(t, req{method: http.MethodPost, path: fmt.Sprintf("/api/items/%d/estimates/refresh", id)})
	e.est.Wait()
	got, err := e.repo.GetItem(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Official != nil {
		t.Errorf("API の更新で販売状況ができた: %+v", got.Official)
	}
	if !got.WatchOfficial {
		t.Error("watch_official が保存されていない")
	}
}

func jsonString(v any) (string, error) {
	if v == nil {
		return "null", nil
	}
	b, err := json.Marshal(v)
	return string(b), err
}
