package httpapi

// マスタの準備状態の受け入れテスト(ADR-0204 §3)。calc-svc が pokedex-svc からマスタを取得できるまでの間:
//   - calc の3操作は 503 master_unavailable(契約の 503 = Error に準拠)
//   - GET /healthz は 200(liveness。プロセスは生きている)
//   - GET /readyz は 503 master_unavailable(readiness。Service の宛先から外す)
// 取得後は NewHandler と同じに振る舞い、/readyz は 200。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"example.com/pokecalc/services/calc/internal/master"
)

// switchableStore は、テストから「準備中 → 準備済み」を切り替えられる StoreFunc の供給元。
type switchableStore struct {
	ready atomic.Bool
	store master.Store
}

func (s *switchableStore) current() master.Store {
	if !s.ready.Load() {
		return nil
	}
	return s.store
}

// calcOperationBodies は calc の3操作の妥当な本文(準備済みなら 200 になる入力)。
func calcOperationBodies(t *testing.T) map[string][]byte {
	t.Helper()
	return map[string][]byte{
		"/api/calc":         mustJSON(t, calcBody()),
		"/api/calc/bulk":    mustJSON(t, bulkBody(movePhysical, nil, nil)),
		"/api/calc/reverse": mustJSON(t, reverseCases(t, newFakeStore(t))[0].httpBody()),
	}
}

func assertStatusOK(t *testing.T, rec *httptest.ResponseRecorder, what string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Errorf("%s status = %d, want 200; body=%s", what, rec.Code, rec.Body.String())
	}
}

// AC-R1: マスタの準備中は calc の3操作が 503 master_unavailable(Error 形式・契約どおり)。
func TestMasterUnavailableWhileNotReady(t *testing.T) {
	h := NewDeferredHandler(func() master.Store { return nil })
	for path, body := range calcOperationBodies(t) {
		t.Run(path, func(t *testing.T) {
			rec := post(t, h, path, body, true) // 503 の本文も契約(Error)に照らす
			assertError(t, rec, http.StatusServiceUnavailable, "master_unavailable")
		})
	}
}

// AC-R1: 準備中でも liveness(/healthz)は 200、readiness(/readyz)は 503 master_unavailable。
// 担当外のルート(pokedex・未知)は準備状態に関係なく 404 not_found。
func TestProbesWhileNotReady(t *testing.T) {
	h := NewDeferredHandler(func() master.Store { return nil })

	rec := serve(t, h, http.MethodGet, "/healthz", http.Header{}, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("/healthz status = %d, want 200(準備中でもプロセスは生きている); body=%s", rec.Code, rec.Body.String())
	}

	rec = serve(t, h, http.MethodGet, "/readyz", http.Header{}, nil)
	assertError(t, rec, http.StatusServiceUnavailable, "master_unavailable")

	for _, path := range []string{"/api/pokedex/natures", "/api/unknown", "/internal/pokedex/master"} {
		rec := serve(t, h, http.MethodGet, path, validHeaders(), nil)
		assertError(t, rec, http.StatusNotFound, "not_found")
	}
}

// AC-R2: 供給元が Store を返すようになったら、同じハンドラが計算に答え、/readyz が 200 になる
// (ハンドラを作り直さない。バックグラウンドの取得が終わった時点で切り替わる)。
func TestDeferredHandlerBecomesReady(t *testing.T) {
	src := &switchableStore{store: newFakeStore(t)}
	h := NewDeferredHandler(src.current)
	bodies := calcOperationBodies(t)

	assertError(t, post(t, h, "/api/calc", bodies["/api/calc"], true), http.StatusServiceUnavailable, "master_unavailable")

	src.ready.Store(true)
	for path, body := range bodies {
		rec := post(t, h, path, body, true)
		if rec.Code != http.StatusOK {
			t.Errorf("準備後の %s status = %d, want 200; body=%s", path, rec.Code, rec.Body.String())
		}
	}
	rec := serve(t, h, http.MethodGet, "/readyz", http.Header{}, nil)
	assertStatusOK(t, rec, "準備後の /readyz")
	assertReadyBody(t, rec.Body.Bytes())
}

// AC-R2: 準備済みの結果は NewHandler(Store を直接渡す)と同じ(準備状態の包みが計算に影響しない)。
func TestDeferredHandlerMatchesNewHandler(t *testing.T) {
	store := newFakeStore(t)
	direct := NewHandler(store)
	deferred := NewDeferredHandler(func() master.Store { return store })
	for path, body := range calcOperationBodies(t) {
		want := post(t, direct, path, body, true)
		got := post(t, deferred, path, body, true)
		if got.Code != want.Code || got.Body.String() != want.Body.String() {
			t.Errorf("%s: NewDeferredHandler = %d %s, want NewHandler と同じ %d %s",
				path, got.Code, got.Body.String(), want.Code, want.Body.String())
		}
	}
}

// AC-R3: NewHandler(起動時に読み込み済み。ファイル方式)の /readyz は常に 200 {"status":"ok"}。
func TestReadyzWithLoadedStore(t *testing.T) {
	rec := serve(t, NewHandler(newFakeStore(t)), http.MethodGet, "/readyz", http.Header{}, nil)
	assertStatusOK(t, rec, "/readyz")
	assertReadyBody(t, rec.Body.Bytes())
}

// AC-R4: calc-svc は pokedex-svc の内部 API(GET /internal/pokedex/master)を提供しない(担当外は 404 not_found)。
// 生成物 api.ServerInterface に GetMasterExport が増えたが、ルートには登録しない。
func TestMasterExportRouteIsNotFound(t *testing.T) {
	h := NewHandler(newFakeStore(t))
	for _, header := range []http.Header{{}, validHeaders()} {
		rec := serve(t, h, http.MethodGet, "/internal/pokedex/master", header, nil)
		assertError(t, rec, http.StatusNotFound, "not_found")
	}
}

func assertReadyBody(t *testing.T, raw []byte) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("/readyz の本文が JSON でない: %v; body=%s", err, raw)
	}
	if len(body) != 1 || body["status"] != "ok" {
		t.Errorf("/readyz の本文 = %v, want {\"status\":\"ok\"}", body)
	}
}
