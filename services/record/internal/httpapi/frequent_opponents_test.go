package httpapi

// GET /api/record/frequent-opponents の受け入れテスト(requirements.md §2「よく使うポケモン」・
// ADR-0209 §3 #2・§6-3)。

import (
	"net/http"
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/record/internal/store"
)

// 並びはスコアの降順、同点は speciesKey の昇順(契約の 200 の description)。
func TestFrequentOpponentsOrdering(t *testing.T) {
	st := newFakeStore()
	st.mu.Lock()
	d := st.state(deviceA)
	d.aggregates = []store.FrequentOpponent{
		{SpeciesKey: speciesLeaf, Score: 1.5, Count: 2, LastCalculatedAt: st.now},
		{SpeciesKey: "9004-000", Score: 4.0, Count: 5, LastCalculatedAt: st.now},
		{SpeciesKey: speciesGuard, Score: 4.0, Count: 4, LastCalculatedAt: st.now},
	}
	st.mu.Unlock()

	got := decodeFrequent(t, serve(t, NewHandler(st), http.MethodGet, pathFrequent, headers(deviceA), nil))
	want := []string{speciesGuard, "9004-000", speciesLeaf} // 4.0(9002 < 9004)→ 1.5
	if len(got) != len(want) {
		t.Fatalf("件数 = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if string(got[i].SpeciesKey) != want[i] {
			t.Errorf("[%d] speciesKey = %q, want %q", i, got[i].SpeciesKey, want[i])
		}
	}
}

// limit の既定は 10、範囲外(0・51・整数でない)は 400 invalid_input(契約の schema どおり)。
func TestFrequentOpponentsLimit(t *testing.T) {
	st := newFakeStore()
	st.seed(deviceA, 0, 20, 0)
	h := NewHandler(st)

	t.Run("既定は10件", func(t *testing.T) {
		got := decodeFrequent(t, serve(t, h, http.MethodGet, pathFrequent, headers(deviceA), nil))
		if len(got) != 10 {
			t.Errorf("件数 = %d, want 10(既定)", len(got))
		}
	})
	t.Run("指定した件数", func(t *testing.T) {
		got := decodeFrequent(t, serve(t, h, http.MethodGet, pathFrequent+"?limit=3", headers(deviceA), nil))
		if len(got) != 3 {
			t.Errorf("件数 = %d, want 3", len(got))
		}
	})
	// critic 指摘 R-11: 範囲の両端(1・50)は有効な値として 200 になること(範囲外テストが
	// 隣接する境界だけを見ており、有効側の境界がテストされていなかった)。
	t.Run("有効な境界 limit=1", func(t *testing.T) {
		got := decodeFrequent(t, serve(t, h, http.MethodGet, pathFrequent+"?limit=1", headers(deviceA), nil))
		if len(got) != 1 {
			t.Errorf("件数 = %d, want 1", len(got))
		}
	})
	t.Run("有効な境界 limit=50", func(t *testing.T) {
		got := decodeFrequent(t, serve(t, h, http.MethodGet, pathFrequent+"?limit=50", headers(deviceA), nil))
		if len(got) != 20 { // 種の集計自体が20件しか無いので、50件要求しても20件しか返らない
			t.Errorf("件数 = %d, want 20(集計の全件)", len(got))
		}
	})
	for _, bad := range []string{"0", "51", "-1", "abc"} {
		t.Run("範囲外 limit="+bad, func(t *testing.T) {
			rec := serve(t, h, http.MethodGet, pathFrequent+"?limit="+bad, headers(deviceA), nil)
			assertErrorBody(t, rec, http.StatusBadRequest, api.InvalidInput)
		})
	}
}

// DB に届かなければ 503 store_unavailable(ADR-0209 §5.3)。
func TestFrequentOpponentsStoreUnavailable(t *testing.T) {
	st := newFakeStore()
	st.unavailable = true
	rec := serve(t, NewHandler(st), http.MethodGet, pathFrequent, headers(deviceA), nil)
	assertErrorBody(t, rec, http.StatusServiceUnavailable, api.StoreUnavailable)
}

// /healthz は DB に触れず常に 200、/readyz は DB に届かなければ 503 store_unavailable。
// record-svc が落ちていても calc-svc に波及しないこと(CLAUDE.md 絶対ルール5)の record 側の担保は、
// 「record-svc が自分のプロセス内で 503 に収めて他サービスを呼ばない」ことなので、
// ここでは「record-svc は下流サービスを1つも持たない(store だけに依存する)」形を固定する。
func TestHealthzAndReadyz(t *testing.T) {
	t.Run("DB が生きている", func(t *testing.T) {
		h := NewHandler(newFakeStore())
		if rec := serve(t, h, http.MethodGet, "/healthz", http.Header{}, nil); rec.Code != http.StatusOK {
			t.Errorf("/healthz = %d, want 200", rec.Code)
		}
		if rec := serve(t, h, http.MethodGet, "/readyz", http.Header{}, nil); rec.Code != http.StatusOK {
			t.Errorf("/readyz = %d, want 200", rec.Code)
		}
	})
	t.Run("DB に届かない", func(t *testing.T) {
		st := newFakeStore()
		st.unavailable = true
		h := NewHandler(st)
		if rec := serve(t, h, http.MethodGet, "/healthz", http.Header{}, nil); rec.Code != http.StatusOK {
			t.Errorf("/healthz = %d, want 200(DB に触れない)", rec.Code)
		}
		rec := serve(t, h, http.MethodGet, "/readyz", http.Header{}, nil)
		assertErrorBody(t, rec, http.StatusServiceUnavailable, api.StoreUnavailable)
	})
}
