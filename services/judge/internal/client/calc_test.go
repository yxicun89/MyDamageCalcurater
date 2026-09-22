package client

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

// exampleCalcRequest は架空の個体・架空の技での計算要求(実マスタは使わない)。
func exampleCalcRequest() CalcRequest {
	return CalcRequest{
		Format: "single",
		Attacker: Individual{
			SpeciesKey: "9001-000",
			NatureID:   "test-nature-plus-spe",
			ItemID:     "test-item-scarf",
			SP:         StatBlock{Atk: 32, Spe: 32},
			Ranks:      RankBlock{Spe: 1},
		},
		Defender: Individual{
			SpeciesKey: "9002-000",
			NatureID:   "test-nature-neutral",
			SP:         StatBlock{HP: 32, Def: 32},
		},
		MoveID: "test-move",
	}
}

// validCalcBody は calc-svc の POST /api/calc の 200 の本文(ルートの api/openapi.yaml の
// CalcResult)を模した架空データ。
func validCalcBody() []byte {
	return []byte(`{
      "rolls": [100,101,102,103,104,105,106,107,108,109,110,111,112,113,114,115],
      "minDamage": 100,
      "maxDamage": 115,
      "minPercent": 58.1,
      "maxPercent": 66.8,
      "defenderHP": 172,
      "effectiveness": 2,
      "stab": true,
      "category": "physical",
      "ko": {"hits": 2, "guaranteed": true, "chancePercent": 0, "displayChancePercent": 100}
    }`)
}

func newCalc(t *testing.T, baseURL string, timeout time.Duration) *Calc {
	t.Helper()
	calc, err := NewCalc(Config{BaseURL: baseURL, Timeout: timeout})
	if err != nil {
		t.Fatalf("NewCalc: %v", err)
	}
	return calc
}

// TestDamageSendsContractShapedRequest: judge が送る body は、ルートの api/openapi.yaml の
// CalcRequest の形(format・attacker・defender・moveId)であること(ADR-0700 §4)。
// 空の abilityId / itemId は送らない(null と「指定なし」を取り違えさせない)。
func TestDamageSendsContractShapedRequest(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath, gotDevice, gotSession, gotContentType string
	var gotBody map[string]any
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotDevice = r.Header.Get("X-Device-Id")
		gotSession = r.Header.Get("X-Session-Id")
		gotContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(validCalcBody())
	})

	if _, err := newCalc(t, server.URL, testTimeout).Damage(t.Context(), requestContext, exampleCalcRequest()); err != nil {
		t.Fatalf("Damage: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/calc" {
		t.Errorf("path = %q, want /api/calc", gotPath)
	}
	if !strings.HasPrefix(gotContentType, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotDevice != requestContext.DeviceID || gotSession != requestContext.SessionID {
		t.Errorf("headers = (%q, %q), want (%q, %q)", gotDevice, gotSession, requestContext.DeviceID, requestContext.SessionID)
	}

	if got := gotBody["format"]; got != "single" {
		t.Errorf("format = %v, want single", got)
	}
	if got := gotBody["moveId"]; got != "test-move" {
		t.Errorf("moveId = %v, want test-move", got)
	}
	attacker, ok := gotBody["attacker"].(map[string]any)
	if !ok {
		t.Fatalf("attacker = %v, want an object", gotBody["attacker"])
	}
	if got := attacker["speciesKey"]; got != "9001-000" {
		t.Errorf("attacker.speciesKey = %v, want 9001-000", got)
	}
	if got := attacker["natureId"]; got != "test-nature-plus-spe" {
		t.Errorf("attacker.natureId = %v, want test-nature-plus-spe", got)
	}
	if got := attacker["itemId"]; got != "test-item-scarf" {
		t.Errorf("attacker.itemId = %v, want test-item-scarf", got)
	}
	// sp は 6 ステータスすべてを送る(StatBlock の required)。ranks は指定した値がそのまま乗る。
	sp, ok := attacker["sp"].(map[string]any)
	if !ok {
		t.Fatalf("attacker.sp = %v, want an object", attacker["sp"])
	}
	for _, key := range []string{"hp", "atk", "def", "spa", "spd", "spe"} {
		if _, present := sp[key]; !present {
			t.Errorf("attacker.sp に %q が無い(ルートの StatBlock は6つとも必須)", key)
		}
	}
	if got := sp["spe"]; got != float64(32) {
		t.Errorf("attacker.sp.spe = %v, want 32", got)
	}
	ranks, ok := attacker["ranks"].(map[string]any)
	if !ok {
		t.Fatalf("attacker.ranks = %v, want an object", attacker["ranks"])
	}
	if got := ranks["spe"]; got != float64(1) {
		t.Errorf("attacker.ranks.spe = %v, want 1", got)
	}
	// 未指定の abilityId は送らない(defender は abilityId も itemId も指定していない)。
	defender, ok := gotBody["defender"].(map[string]any)
	if !ok {
		t.Fatalf("defender = %v, want an object", gotBody["defender"])
	}
	for _, key := range []string{"abilityId", "itemId"} {
		if _, present := defender[key]; present {
			t.Errorf("defender に %q を送っている。未指定の欄は送らない", key)
		}
	}
}

// TestDamageDecodesUpstreamResponse: 200 の本文から judge が使う欄を取り出す。
// ko は calc-svc の KOChance をそのまま転記する(意味の正はルートの api/openapi.yaml と ADR-0010。
// judge は確定数を再計算しない)。
func TestDamageDecodesUpstreamResponse(t *testing.T) {
	t.Parallel()

	server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(validCalcBody())
	})

	got, err := newCalc(t, server.URL, testTimeout).Damage(t.Context(), requestContext, exampleCalcRequest())
	if err != nil {
		t.Fatalf("Damage: %v", err)
	}

	want := CalcResult{
		MinDamage:  100,
		MaxDamage:  115,
		DefenderHP: 172,
		KO:         KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Damage = %+v, want %+v", got, want)
	}
}

// TestDamageRequiresRequestContext: 端末 ID・セッション ID が無いまま上流を呼ばない(pokedex と同じ)。
func TestDamageRequiresRequestContext(t *testing.T) {
	t.Parallel()

	called := false
	server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(validCalcBody())
	})

	_, err := newCalc(t, server.URL, testTimeout).Damage(t.Context(), RequestContext{}, exampleCalcRequest())
	if !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("err = %v, want ErrInvalidRequest", err)
	}
	if called {
		t.Error("上流を呼んでいる。端末 ID が無い要求は judge の中で止める")
	}
}

// TestDamageNormalizesUpstreamStatus: 上流のステータスを ADR-0700 §3 の番兵エラーに畳む。
// calc-svc は 404 を返さないので、対応表のうち 5xx と 400 を確かめる。
func TestDamageNormalizesUpstreamStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"500", http.StatusInternalServerError, `{"code":"internal_error","message":"boom"}`, ErrUpstreamUnavailable},
		{"502", http.StatusBadGateway, "", ErrUpstreamUnavailable},
		// calc-svc がマスタを引けないときの 503 も「上流が使えない」に畳む。
		{"503", http.StatusServiceUnavailable, `{"code":"master_unavailable","message":"no master"}`, ErrUpstreamUnavailable},
		{"400", http.StatusBadRequest, `{"code":"invalid_input","message":"unknown move"}`, ErrInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			_, err := newCalc(t, server.URL, testTimeout).Damage(t.Context(), requestContext, exampleCalcRequest())
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestDamageRejectsInvalidBody: 200 でも本文が契約に合わなければ ErrUpstreamInvalidResponse。
// 特に ko の欄が欠けた応答を「0 発で倒せない」と読み替えない(判定を静かに間違える)。
func TestDamageRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{"壊れた JSON", `{"minDamage": 100`},
		{"JSON ですらない", `upstream connect error`},
		{"ko が無い", `{"minDamage":100,"maxDamage":115,"defenderHP":172}`},
		{"ko.hits が無い", `{"minDamage":100,"maxDamage":115,"defenderHP":172,
		  "ko":{"guaranteed":true,"displayChancePercent":100}}`},
		{"ko.guaranteed が無い", `{"minDamage":100,"maxDamage":115,"defenderHP":172,
		  "ko":{"hits":2,"displayChancePercent":100}}`},
		{"ko.displayChancePercent が無い", `{"minDamage":100,"maxDamage":115,"defenderHP":172,
		  "ko":{"hits":2,"guaranteed":true}}`},
		{"defenderHP が無い", `{"minDamage":100,"maxDamage":115,
		  "ko":{"hits":2,"guaranteed":true,"displayChancePercent":100}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			})

			_, err := newCalc(t, server.URL, testTimeout).Damage(t.Context(), requestContext, exampleCalcRequest())
			if !errors.Is(err, ErrUpstreamInvalidResponse) {
				t.Errorf("err = %v, want ErrUpstreamInvalidResponse", err)
			}
		})
	}
}

// TestDamageOnConnectionError: 誰も待ち受けていない上流は ErrUpstreamUnavailable。
func TestDamageOnConnectionError(t *testing.T) {
	t.Parallel()

	_, err := newCalc(t, deadBaseURL, testTimeout).Damage(t.Context(), requestContext, exampleCalcRequest())
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Fatalf("err = %v, want ErrUpstreamUnavailable", err)
	}
}

// TestDamageTimesOut: 答えない上流を設定のタイムアウトで打ち切る(ADR-0700 §2)。
func TestDamageTimesOut(t *testing.T) {
	t.Parallel()

	server := blockingServer(t)

	start := time.Now()
	_, err := newCalc(t, server.URL, shortTimeout).Damage(t.Context(), requestContext, exampleCalcRequest())
	elapsed := time.Since(start)

	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Errorf("err = %v, want ErrUpstreamUnavailable", err)
	}
	if elapsed > time.Second {
		t.Errorf("%v 待った。設定のタイムアウト %v で打ち切ること", elapsed, shortTimeout)
	}
}
