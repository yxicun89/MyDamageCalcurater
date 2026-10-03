package httpapi

// issue #298: recommendations の1リクエストあたりのアロケーションを測るベンチマーク。
// 実行: cd services/balance && go test -run '^$' -bench BenchmarkRecommendations -benchmem ./internal/httpapi
// 結果(B/op・allocs/op)は ADR-0409 に記録する。カタログは架空 ID(9000-000〜9149-009 の1,500件。issue の再現条件と同じ)。

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
	"example.com/pokecalc/services/balance/internal/master"
)

const (
	benchCatalogSize = 1500
	// benchMembersBody is the issue's reproduction request: 6 members, limit 20.
	benchMembersBody = `{"members":[` +
		`{"pokemonId":"9000-000","moveIds":["move-9001","move-9005"]},` +
		`{"pokemonId":"9000-001","moveIds":["move-9003"]},` +
		`{"pokemonId":"9000-002","moveIds":[]},` +
		`{"pokemonId":"9000-003","moveIds":["move-9004"],"abilityId":"ability-9001"},` +
		`{"pokemonId":"9000-004","moveIds":["move-9007"]},` +
		`{"pokemonId":"9000-005","moveIds":["move-9001"]}` +
		`],"limit":20}`
)

// benchCatalog builds n fictional pokemon whose types and abilities cycle through the real type list.
func benchCatalog(n int) testCatalog {
	types := balance.AllTypes()
	abilityIDs := []string{"ability-9001", "ability-9002", "ability-9003"}
	catalog := make(testCatalog, n)
	for i := range catalog {
		pokemonTypes := []balance.TypeID{types[i%len(types)]}
		if i%3 != 0 {
			pokemonTypes = append(pokemonTypes, types[(i/len(types)+1+i)%len(types)])
		}
		if pokemonTypes[len(pokemonTypes)-1] == pokemonTypes[0] && len(pokemonTypes) == 2 {
			pokemonTypes = pokemonTypes[:1]
		}
		catalog[i] = balance.CatalogPokemon{
			PokemonID:  fmt.Sprintf("%04d-%03d", 9000+i/10, i%10),
			NameJa:     fmt.Sprintf("テスト%04d", i),
			Types:      pokemonTypes,
			AbilityIDs: []string{abilityIDs[i%len(abilityIDs)]},
		}
	}
	return catalog
}

func benchRecommendationsDependencies(b testing.TB) Dependencies {
	b.Helper()
	chart, err := master.EmbeddedTypeChart()
	if err != nil {
		b.Fatalf("EmbeddedTypeChart: %v", err)
	}
	catalog := benchCatalog(benchCatalogSize)
	return Dependencies{
		TypeChart:      chart,
		PokemonTypes:   catalog.types(),
		PokemonCatalog: catalog,
		Moves:          recMoves,
		Abilities:      recAbilities,
	}
}

func BenchmarkRecommendations(b *testing.B) {
	server := New(benchRecommendationsDependencies(b))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, recommendationsPath, strings.NewReader(benchMembersBody))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Device-Id", "11111111-1111-4111-8111-111111111111")
		request.Header.Set("X-Session-Id", "22222222-2222-4222-a222-222222222222")
		server.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			b.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
	}
}
