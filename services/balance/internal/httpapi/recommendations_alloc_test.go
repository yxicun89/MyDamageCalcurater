package httpapi

// issue #298(ADR-0409): 1リクエストあたりのアロケーションの上限。実カタログ規模(1,500件)で
// 64Mi の limit に対し同時 30 が OOMKill された原因は、1リクエストが数 MB を確保することだった
// (削減前の実測: 約 3.1 MB/op・約 4,580 allocs/op)。ベンチマーク BenchmarkRecommendations(別ファイル)と同じ条件で、
// B/op が上限を超えたら失敗する。上限は削減後の実測に余裕を持たせた値で、ADR-0409 に実測値を書く。
// 上限を緩めて通さない(CLAUDE.md 絶対ルール6)。緩める必要が出たら先に ADR に理由を書く。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// maxRecommendationBytesPerRequest is the budget for one recommendations request over a 1,500-pokemon catalog.
const maxRecommendationBytesPerRequest = 1 << 20 // 1 MiB

func TestRecommendationsAllocationBudget(t *testing.T) {
	// Not parallel: testing.Benchmark measures process-wide allocations.
	server := New(benchRecommendationsDependencies(t))
	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, recommendationsPath, strings.NewReader(benchMembersBody))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Device-Id", "bench-device")
			request.Header.Set("X-Session-Id", "bench-session")
			server.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				b.Fatalf("status = %d, want 200", recorder.Code)
			}
		}
	})
	if result.N == 0 {
		t.Fatal("benchmark did not run")
	}
	if got := result.AllocedBytesPerOp(); got > maxRecommendationBytesPerRequest {
		t.Errorf("recommendations allocates %d B/op, want <= %d B/op (1,500 pokemon, 6 members, limit 20)", got, maxRecommendationBytesPerRequest)
	}
}
