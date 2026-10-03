package httpapi_test

// searchItems・getSpecies は、一覧と効果の検証用の相性表を1つの読み取り専用トランザクションで読む
// (ADR-0218 §3。ADR-0127 と同じ)。成功したら Commit で閉じ、失敗したら Rollback で閉じる(開いたまま残さない)。

import (
	"net/http"
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

func TestPublicEffectReadsInOneReadOnlySnapshot(t *testing.T) {
	tests := []struct {
		name, target string
		reads        []string
	}{
		{"searchItems", "/api/pokedex/items", []string{"GetDefaultRegulation", "SearchItems", "ListTypes", "ListTypeChart"}},
		{"getSpecies", "/api/pokedex/species/9001-000", []string{
			"GetDefaultRegulation", "GetSpeciesByKey", "ListSpeciesAbilityNames", "ListSpeciesLearnset", "ListTypes", "ListTypeChart",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			rec := do(t, newHandler(t, q), http.MethodGet, tt.target, true)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
			}
			for _, p := range q.SnapshotViolations(tt.reads) {
				t.Error(p)
			}
		})
	}
}

func TestPublicEffectRollsBackOnFailure(t *testing.T) {
	tests := []struct {
		name, target string
		mutate       func(q *storetest.Querier)
	}{
		{"searchItems: クエリが失敗", "/api/pokedex/items", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"SearchItems": storetest.ErrDB}
		}},
		{"searchItems: 効果が不正", "/api/pokedex/items", setItemEffect("testorb", `{"NoSuchField": 1}`)},
		{"getSpecies: 相性表を読めない", "/api/pokedex/species/9001-000", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"ListTypeChart": storetest.ErrDB}
		}},
		{"getSpecies: 効果が不正", "/api/pokedex/species/9001-000", setAbilityEffect("testblaze", `{"NoSuchField": 1}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			tt.mutate(q)
			rec := do(t, newHandler(t, q), http.MethodGet, tt.target, true)
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
			for _, p := range q.RollbackViolations() {
				t.Error(p)
			}
		})
	}
}
