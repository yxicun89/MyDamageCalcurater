package httpapi

// 計算履歴(listCalcHistory。ADR-0230)の契約テスト(test-strategy.md L4)。応答を api/openapi.yaml に照らし、
// 契約そのものに ADR-0230 の決定(経路・上限・返す項目のホワイトリスト・CalcRequest の再利用)が入っていることを固定する。

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/calcevents"
)

func TestCalcHistoryResponsesMatchContract(t *testing.T) {
	seed := func(n int) func(*testing.T, *fakeStore) {
		return func(t *testing.T, f *fakeStore) {
			base := historyNow().Add(-time.Hour)
			for i := 0; i < n; i++ {
				f.addCalc(t, deviceA, "e"+strings.Repeat("x", i), base.Add(time.Duration(i)*time.Minute), "m")
			}
			f.addEvent(deviceA, "bulk", calcevents.OperationBulk, base, envelopePayload(t, deviceA, calcevents.OperationBulk, base))
		}
	}
	none := func(*testing.T, *fakeStore) {}
	tests := []struct {
		name         string
		setup        func(*testing.T, *fakeStore)
		path         string
		wantStatus   int
		checkRequest bool // false: リクエスト自体が意図的に契約の範囲外(assertMatchesContract 参照)
	}{
		{"200(続きあり)", seed(3), pathCalcHistory + "?limit=2", http.StatusOK, true},
		{"200(空)", none, pathCalcHistory, http.StatusOK, true},
		{"200(任意項目まで埋めた計算)", func(t *testing.T, f *fakeStore) {
			at := historyNow().Add(-time.Minute)
			f.addEvent(deviceA, "full", calcevents.OperationCalc, at, calcEventPayload(t, deviceA, at, fullCalcInput(), 87.6, 137))
		}, pathCalcHistory, http.StatusOK, true},
		{"400(limit 範囲外)", none, pathCalcHistory + "?limit=51", http.StatusBadRequest, false},
		{"400(cursor の文字が契約外)", none, pathCalcHistory + "?cursor=%40%40", http.StatusBadRequest, false},
		{"400(cursor の形式は契約内だが読めない)", none, pathCalcHistory + "?cursor=aGVsbG8", http.StatusBadRequest, true},
		{"503", func(_ *testing.T, f *fakeStore) { f.unavailable = true }, pathCalcHistory, http.StatusServiceUnavailable, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			tt.setup(t, st)
			rec := serve(t, historyHandler(st), http.MethodGet, tt.path, headers(deviceA), nil)
			// 「契約にある応答のどれか」(default を含む)ではなく、期待のステータスそのものを確かめる
			// (未実装の 404 が default に当たって素通りしないように)。
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			assertMatchesContract(t, http.MethodGet, tt.path, headers(deviceA), nil, rec, tt.checkRequest)
		})
	}
}

// 2ページ目(1ページ目の nextCursor を渡した要求)も契約どおり(サーバーが作ったカーソルが契約の pattern・長さに収まる)。
func TestCalcHistorySecondPageMatchesContract(t *testing.T) {
	st := newFakeStore()
	base := historyNow().Add(-time.Hour)
	for i := 0; i < 3; i++ {
		st.addCalc(t, deviceA, "e"+strings.Repeat("x", i), base.Add(time.Duration(i)*time.Minute), "m")
	}
	h := historyHandler(st)
	p1 := decodeHistory(t, serve(t, h, http.MethodGet, historyPageURL(2, nil), headers(deviceA), nil))
	if p1.NextCursor == nil {
		t.Fatal("1ページ目に続きが無い")
	}
	path := historyPageURL(2, p1.NextCursor)
	rec := serve(t, h, http.MethodGet, path, headers(deviceA), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("2ページ目 status = %d; body=%s", rec.Code, rec.Body.String())
	}
	assertMatchesContract(t, http.MethodGet, path, headers(deviceA), nil, rec, true)
}

// 契約そのもの: 経路・operationId・タグ・応答・必須ヘッダ・端末 ID を他の場所で受けないこと・limit と cursor の範囲・
// 返す項目のホワイトリスト・calc が CalcRequest そのものであること(ADR-0230 §1・§2・§6)。
func TestContractHasCalcHistoryOperation(t *testing.T) {
	doc, _ := loadContract(t)
	item := doc.Paths.Find("/api/record/calc-history")
	if item == nil {
		t.Fatal("契約に /api/record/calc-history が無い")
	}
	op := item.Get
	if op == nil {
		t.Fatal("契約に GET /api/record/calc-history が無い")
	}
	if !strings.EqualFold(op.OperationID, "listCalcHistory") {
		t.Errorf("operationId = %q, want listCalcHistory", op.OperationID)
	}
	if len(op.Tags) != 1 || op.Tags[0] != "record" {
		t.Errorf("tags = %v, want [record]", op.Tags)
	}
	for _, s := range []int{200, 400, 503} {
		if op.Responses.Status(s) == nil {
			t.Errorf("%d のレスポンスが無い", s)
		}
	}
	if r := op.Responses.Status(http.StatusServiceUnavailable); r == nil || r.Value == nil || r.Value.Description == nil ||
		!strings.Contains(*r.Value.Description, "store_unavailable") {
		t.Error("503 の description に store_unavailable が書かれていない(ADR-0209 §5.3)")
	}
	// 個別の削除・更新は持たない(ADR-0230 §5)。
	if item.Delete != nil || item.Put != nil || item.Patch != nil || item.Post != nil {
		t.Error("/api/record/calc-history に GET 以外の操作がある(個別の削除・更新は持たない。ADR-0230 §5)")
	}

	var hasDevice, hasSession bool
	params := map[string]bool{}
	for _, p := range op.Parameters {
		if p.Value == nil {
			continue
		}
		v := p.Value
		switch {
		case v.In == "header" && v.Name == "X-Device-Id" && v.Required:
			hasDevice = true
		case v.In == "header" && v.Name == "X-Session-Id" && v.Required:
			hasSession = true
		case v.In != "header" && isDeviceIDName(v.Name):
			t.Errorf("%s で %q を受け取っている(ADR-0209 §6-4)", v.In, v.Name)
		}
		if v.In == "query" {
			params[v.Name] = true
			s := v.Schema.Value
			switch v.Name {
			case "limit":
				if s.Min == nil || *s.Min != 1 || s.Max == nil || *s.Max != 50 || fmt.Sprint(s.Default) != "20" {
					t.Errorf("limit の schema = min %v max %v default %v, want 1〜50・既定 20(ADR-0230 §1)", s.Min, s.Max, s.Default)
				}
			case "cursor":
				if v.Required {
					t.Error("cursor が必須になっている(省略で先頭から)")
				}
				if s.MaxLength == nil || *s.MaxLength != 200 || s.Pattern != "^[A-Za-z0-9_-]+$" {
					t.Errorf("cursor の schema = maxLength %v pattern %q, want 200・URL にそのまま入る文字だけ", s.MaxLength, s.Pattern)
				}
			}
		}
	}
	if !hasDevice || !hasSession {
		t.Errorf("必須ヘッダが足りない(X-Device-Id=%v X-Session-Id=%v)", hasDevice, hasSession)
	}
	if !params["limit"] || !params["cursor"] || len(params) != 2 {
		t.Errorf("クエリ = %v, want limit と cursor だけ", params)
	}

	schemas := doc.Components.Schemas
	page := schemas["CalcHistoryPage"]
	if page == nil || page.Value == nil {
		t.Fatal("CalcHistoryPage が契約に無い")
	}
	assertStringSet(t, "CalcHistoryPage.required", page.Value.Required, "items", "nextCursor")
	items := page.Value.Properties["items"]
	if items == nil || items.Value.MaxItems == nil || *items.Value.MaxItems != 50 {
		t.Error("CalcHistoryPage.items の maxItems が 50 でない(limit の上限と同じ)")
	}
	if nc := page.Value.Properties["nextCursor"]; nc == nil || !nc.Value.Nullable {
		t.Error("CalcHistoryPage.nextCursor が nullable でない(続きが無ければ null)")
	}

	entry := schemas["CalcHistoryEntry"]
	if entry == nil || entry.Value == nil {
		t.Fatal("CalcHistoryEntry が契約に無い")
	}
	// ホワイトリスト: 行が持つ項目はこの3つだけ(端末 ID・セッション ID・イベント ID を足さない。ADR-0230 §2)。
	assertStringSet(t, "CalcHistoryEntry.properties", keysOfSchemaProps(entry.Value.Properties), "occurredAt", "calc", "result")
	assertStringSet(t, "CalcHistoryEntry.required", entry.Value.Required, "occurredAt", "calc", "result")
	if c := entry.Value.Properties["calc"]; c == nil || c.Ref != "#/components/schemas/CalcRequest" {
		t.Errorf("CalcHistoryEntry.calc が $ref CalcRequest でない(生成型も CalcRequest そのものにする。ADR-0228 と同じ)")
	}
	result := schemas["CalcHistoryResult"]
	if result == nil || result.Value == nil {
		t.Fatal("CalcHistoryResult が契約に無い")
	}
	assertStringSet(t, "CalcHistoryResult.properties", keysOfSchemaProps(result.Value.Properties), "minPercent", "maxPercent")
}

func keysOfSchemaProps[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func assertStringSet(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	if strings.Join(g, ",") != strings.Join(w, ",") {
		t.Errorf("%s = %v, want %v", what, g, w)
	}
}
