package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/healthz", "/api/balance/healthz"} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			newTestServer().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if got := strings.TrimSpace(recorder.Body.String()); got != `{"status":"ok"}` {
				t.Errorf("body = %s", got)
			}
		})
	}
}

func TestAnalyzeRequiresRequestContextHeaders(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/balance/v1/team-balance/analyze", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	newTestServer().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"missing_request_context"`) {
		t.Errorf("body = %s", recorder.Body.String())
	}
}

func TestAnalyzeNormalizesGeneratedParameterErrors(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/balance/v1/team-balance/analyze", strings.NewReader(`{"members":[{"pokemonId":"9001-000"}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("X-Device-Id", "first-device")
	request.Header.Add("X-Device-Id", "second-device")
	request.Header.Set("X-Session-Id", "test-session")
	newTestServer().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"invalid_request"`) {
		t.Errorf("body = %s", recorder.Body.String())
	}
}

func TestAnalyzeReturnsDefenseBalance(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/balance/v1/team-balance/analyze", strings.NewReader(`{"members":[{"pokemonId":"9001-000"}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Device-Id", "test-device")
	request.Header.Set("X-Session-Id", "test-session")
	newTestServer().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Errorf("Content-Type = %q, want application/json", recorder.Header().Get("Content-Type"))
	}
	if strings.Contains(recorder.Body.String(), "tb1_not_implemented") {
		t.Errorf("body = %s", recorder.Body.String())
	}
}

func TestAnalyzeRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"members":`},
		{name: "empty party", body: `{"members":[]}`},
		{name: "too many members", body: `{"members":[{"pokemonId":"0001-000"},{"pokemonId":"0002-000"},{"pokemonId":"0003-000"},{"pokemonId":"0004-000"},{"pokemonId":"0005-000"},{"pokemonId":"0006-000"},{"pokemonId":"0007-000"}]}`},
		{name: "invalid pokemon ID", body: `{"members":[{"pokemonId":"pokemon-a"}]}`},
		{name: "unknown property", body: `{"members":[{"pokemonId":"9001-000","types":["dragon"]}]}`},
		{name: "trailing JSON", body: `{"members":[{"pokemonId":"9001-000"}]} {}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/balance/v1/team-balance/analyze", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Device-Id", "test-device")
			request.Header.Set("X-Session-Id", "test-session")
			newTestServer().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"code":"invalid_request"`) {
				t.Errorf("body = %s", recorder.Body.String())
			}
		})
	}
}

func TestAnalyzeRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	body := `{"members":[{"pokemonId":"` + strings.Repeat("1", maxAnalyzeBodyBytes) + `"}]}`
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/balance/v1/team-balance/analyze", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Device-Id", "test-device")
	request.Header.Set("X-Session-Id", "test-session")
	newTestServer().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"request_too_large"`) {
		t.Errorf("body = %s", recorder.Body.String())
	}
}

func TestAnalyzeRejectsPartialOrBlankRequestContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		deviceID  string
		sessionID string
	}{
		{name: "missing device", sessionID: "test-session"},
		{name: "missing session", deviceID: "test-device"},
		{name: "blank device", deviceID: "   ", sessionID: "test-session"},
		{name: "blank session", deviceID: "test-device", sessionID: "\t"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/balance/v1/team-balance/analyze", strings.NewReader(`{"members":[{"pokemonId":"9001-000"}]}`))
			request.Header.Set("Content-Type", "application/json")
			if tt.deviceID != "" {
				request.Header.Set("X-Device-Id", tt.deviceID)
			}
			if tt.sessionID != "" {
				request.Header.Set("X-Session-Id", tt.sessionID)
			}
			newTestServer().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"code":"missing_request_context"`) {
				t.Errorf("body = %s", recorder.Body.String())
			}
		})
	}
}

func TestAnalyzeAcceptsBoundaryInputs(t *testing.T) {
	t.Parallel()

	sixMembers := `{"members":[{"pokemonId":"9001-000"},{"pokemonId":"9002-000"},{"pokemonId":"9003-000"},{"pokemonId":"9004-000"},{"pokemonId":"9005-000"},{"pokemonId":"9006-000"}]}`
	oneMember := `{"members":[{"pokemonId":"9001-000"}]}`
	exactLimit := oneMember + strings.Repeat(" ", maxAnalyzeBodyBytes-len(oneMember))

	tests := []struct {
		name string
		body string
	}{
		{name: "six members", body: sixMembers},
		{name: "body exactly at limit", body: exactLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.name == "body exactly at limit" && len(tt.body) != maxAnalyzeBodyBytes {
				t.Fatalf("fixture length = %d, want %d", len(tt.body), maxAnalyzeBodyBytes)
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/balance/v1/team-balance/analyze", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Device-Id", "test-device")
			request.Header.Set("X-Session-Id", "test-session")
			newTestServer().ServeHTTP(recorder, request)

			// TB1 accepts boundary inputs and returns the analysis.
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
		})
	}
}
