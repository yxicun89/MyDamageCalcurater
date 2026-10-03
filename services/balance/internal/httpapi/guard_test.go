package httpapi

// issue #299(ADR-0801): サービス全体の同時実行の上限(recommendations の上限 ADR-0409 とは別。全操作で数える)と、
// ハンドラ全体の締め切り。上限超過は待たせず 503 overloaded + Retry-After、締め切り後は計算を新しく始めない。

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/pokecalc/services/balance/internal/httpguard"
)

func TestGuardRejectsAnyOperationOverTheInflightLimit(t *testing.T) {
	t.Parallel()

	g := newGatedCatalog(2)
	deps := gatedDependencies(g, 4)
	deps.Guard = httpguard.Config{MaxInflight: 1, Timeout: 5 * time.Second}
	server := New(deps)
	wg := holdSlots(t, server, g, 1)

	// 別の操作(analyze)も、全体の枠が埋まっていれば待たせず 503。
	assertOverloaded(t, postAnalyze(t, server, `{"members":[{"pokemonId":"9001-000"}]}`))
	// 運用エンドポイントは対象外。
	for _, path := range []string{"/healthz", "/metrics"} {
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("%s = %d, want 200(上限の対象外)", path, recorder.Code)
		}
	}

	close(g.release)
	wg.Wait()
	if recorder := postRecommendations(t, server, recBody); recorder.Code != http.StatusOK {
		t.Fatalf("解放後 = %d, want 200", recorder.Code)
	}
}

// 締め切りが過ぎた後は engine(balance.Analyze* / RecommendTypes)を新しく呼ばない。
func TestExpiredDeadlineSkipsComputation(t *testing.T) {
	t.Parallel()

	guard := httpguard.Config{MaxInflight: 4, Timeout: time.Nanosecond}

	analyzeDeps := Dependencies{TypeChart: testTypeChart(), PokemonTypes: fictionalPokemonTypes, Guard: guard}
	assertOverloaded(t, postAnalyze(t, New(analyzeDeps), `{"members":[{"pokemonId":"9001-000"}]}`))

	recDeps := recFullDependencies()
	recDeps.Guard = guard
	assertOverloaded(t, postRecommendations(t, New(recDeps), recBody))
}
