//go:build mysql

package importer_test

// GET /readyz が実 MySQL の状態に連動することの統合テスト(issue #107・ADR-0129 §1)。`make test-db` だけが実行する。
// migrate 済み・未投入 → 503、import 後 → 同じハンドラのまま 200(再起動なしで Ready)、
// migrate 前(テーブルが無い)→ 503。どの場合も /healthz は 200(liveness は DB に連動させない)。

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	pokedexdb "example.com/pokecalc/services/pokedex/db"
	"example.com/pokecalc/services/pokedex/importer"
	"example.com/pokecalc/services/pokedex/internal/httpapi"
	"example.com/pokecalc/services/pokedex/internal/readtx"
)

func getStatus(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code, rec.Body.String()
}

func TestReadyzFollowsDatabaseState(t *testing.T) {
	conn := freshImportDB(t) // migrate 済み・未投入
	h := httpapi.NewHandler(readtx.NewDB(conn))

	if code, body := getStatus(t, h, "/readyz"); code != http.StatusServiceUnavailable {
		t.Fatalf("未投入の /readyz = %d, want 503\nbody=%s", code, body)
	}
	if code, _ := getStatus(t, h, "/healthz"); code != http.StatusOK {
		t.Errorf("未投入の /healthz = %d, want 200", code)
	}

	out, versions := fixtureOutput(t)
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := importer.Apply(context.Background(), conn, out, versions, now); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if code, body := getStatus(t, h, "/readyz"); code != http.StatusOK {
		t.Fatalf("import 後の /readyz = %d, want 200(同じハンドラのまま Ready になる)\nbody=%s", code, body)
	}

	// migrate 前(全テーブルを戻す)。テーブルが無いので 503。DB のエラー文(テーブル名・DSN)を出さない。
	dsn := os.Getenv("POKEDEX_TEST_DSN")
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := pokedexdb.DownAll(dsn, cfg.DBName); err != nil {
		t.Fatalf("DownAll: %v", err)
	}
	code, body := getStatus(t, h, "/readyz")
	if code != http.StatusServiceUnavailable {
		t.Fatalf("migrate 前の /readyz = %d, want 503\nbody=%s", code, body)
	}
	for _, leak := range []string{"data_versions", "doesn't exist", cfg.DBName, cfg.User} {
		if leak != "" && strings.Contains(body, leak) {
			t.Errorf("/readyz の本文に内部情報 %q がある: %s", leak, body)
		}
	}
	if code, _ := getStatus(t, h, "/healthz"); code != http.StatusOK {
		t.Errorf("migrate 前の /healthz = %d, want 200", code)
	}
	// 後続のテストのために戻す(freshImportDB は毎回 DownAll → Up するので必須ではないが、DB を空のまま残さない)。
	if err := pokedexdb.Up(dsn); err != nil {
		t.Fatalf("Up: %v", err)
	}
}
