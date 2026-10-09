package httpapi_test

// 技の処理の定義(move_rules)・機構の中身・種族の重さを内部 API・公開 API に出すことのテスト(ADR-0143 §6)。
//
// 受け入れ条件(pokedex-svc):
//   - AC-R1 内部 API(getMasterExport): MasterMove.rule は move_rules の JSON(定義の無い技はキーを省く)。
//     MasterSpecies.weightHg は species.weight_hg(NULL はキーを省く)。応答は契約に合う。
//   - AC-R2 公開 API(getMove・getMovesByIds・searchMoves): Move.rule・Move.mechanismParams は内部 API と同じ値。
//     無い技はキーを省く。getSpecies の SpeciesDetail.weightHg も同じ(NULL は省く)。
//   - AC-R3 公開 API は検証を通らない定義(共通マスタの DecodeMoveRule が拒否する JSON)があれば 503 master_unavailable
//     (黙って捨てない。効果定義の public effect と同じ扱い)。
//
// fixture: storetest.New() の種族 9001-000(テストモン)の重さは 905 hg、9002-000(テストリーフ)は NULL(実装で storetest に入れる)。
// 技の定義・機構の中身はこのテストで q に入れる。

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// testflame は storetest で variable_power・multi_hit を持つ(move_flags_test.go の wantMoveMechanisms)。
const testflameRule = `{"PowerFormula":"hit_index"}`

func querierWithRules() *storetest.Querier {
	q := storetest.New()
	q.MoveRules = []store.MoveRule{{MoveID: "testflame", Rule: json.RawMessage(testflameRule)}}
	q.MoveMechanismParams = []store.MoveMechanismParam{{MoveID: "testflame",
		MultiHitMin: sql.NullInt16{Int16: 3, Valid: true}, MultiHitMax: sql.NullInt16{Int16: 3, Valid: true}}}
	return q
}

func sameJSON(t *testing.T, got any, want string) bool {
	t.Helper()
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var g, w any
	_ = json.Unmarshal(b, &g)
	_ = json.Unmarshal([]byte(want), &w)
	gb, _ := json.Marshal(g)
	wb, _ := json.Marshal(w)
	return string(gb) == string(wb)
}

// AC-R1
func TestMasterExportCarriesRuleAndWeight(t *testing.T) {
	h := newHandler(t, querierWithRules())
	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, masterPath, false, rec)
	raw := rawExport(t, rec.Body.Bytes())
	for _, v := range raw["moves"].([]any) {
		m := v.(map[string]any)
		rule, ok := m["rule"]
		switch m["id"] {
		case "testflame":
			if !ok || !sameJSON(t, rule, testflameRule) {
				t.Errorf("testflame.rule = %v, want %s", rule, testflameRule)
			}
		default:
			if ok {
				t.Errorf("%v.rule = %v, want キーなし(定義の無い技)", m["id"], rule)
			}
		}
	}
	for _, v := range raw["species"].([]any) {
		s := v.(map[string]any)
		w, ok := s["weightHg"]
		switch s["key"] {
		case "9001-000":
			if !ok || w != float64(905) {
				t.Errorf("9001-000.weightHg = %v, want 905", w)
			}
		case "9002-000":
			if ok {
				t.Errorf("9002-000.weightHg = %v, want キーなし(NULL = まだ取り込んでいない)", w)
			}
		}
	}
}

// AC-R2
func TestPublicMoveRuleAndMechanismParams(t *testing.T) {
	h := newHandler(t, querierWithRules())
	check := func(t *testing.T, m map[string]any) {
		t.Helper()
		rule, hasRule := m["rule"]
		params, hasParams := m["mechanismParams"]
		if m["id"] == "testflame" {
			if !hasRule || !sameJSON(t, rule, testflameRule) {
				t.Errorf("testflame.rule = %v, want %s", rule, testflameRule)
			}
			want := `{"multiHit":{"min":3,"max":3},"fixedDamage":null,"ohko":null,"offenseStat":null,"offensePokemon":null,"defenseStat":null}`
			if !hasParams || !sameJSON(t, params, want) {
				t.Errorf("testflame.mechanismParams = %v, want %s", params, want)
			}
			return
		}
		if hasRule || hasParams {
			t.Errorf("%v: rule = %v・mechanismParams = %v, want どちらもキーなし", m["id"], rule, params)
		}
	}
	for _, path := range []string{
		"/api/pokedex/moves/testflame",
		"/api/pokedex/moves/teststrike",
	} {
		rec := do(t, h, http.MethodGet, path, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d\nbody=%s", path, rec.Code, rec.Body.String())
		}
		validateAgainstContract(t, http.MethodGet, path, true, rec)
		var m map[string]any
		decodeStrict(t, rec.Body.Bytes(), &m)
		check(t, m)
	}
	for _, path := range []string{
		"/api/pokedex/moves/batch?ids=testflame&ids=teststrike",
		"/api/pokedex/moves?limit=200",
	} {
		rec := do(t, h, http.MethodGet, path, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d\nbody=%s", path, rec.Code, rec.Body.String())
		}
		validateAgainstContract(t, http.MethodGet, path, true, rec)
		var list []map[string]any
		decodeStrict(t, rec.Body.Bytes(), &list)
		seen := false
		for _, m := range list {
			check(t, m)
			seen = seen || m["id"] == "testflame"
		}
		if !seen {
			t.Errorf("%s: testflame が応答に無い", path)
		}
	}
}

// AC-R2(種族)
func TestPublicSpeciesWeight(t *testing.T) {
	h := newHandler(t, querierWithRules())
	for key, want := range map[string]*int{"9001-000": intPtr(905), "9002-000": nil} {
		path := "/api/pokedex/species/" + key
		rec := do(t, h, http.MethodGet, path, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d\nbody=%s", path, rec.Code, rec.Body.String())
		}
		validateAgainstContract(t, http.MethodGet, path, true, rec)
		var d api.SpeciesDetail
		decodeStrict(t, rec.Body.Bytes(), &d)
		switch {
		case want == nil && d.WeightHg != nil:
			t.Errorf("%s.weightHg = %d, want キーなし", key, *d.WeightHg)
		case want != nil && (d.WeightHg == nil || *d.WeightHg != *want):
			t.Errorf("%s.weightHg = %v, want %d", key, d.WeightHg, *want)
		}
	}
}

// AC-R3
func TestPublicMoveRuleRejectsInvalidDefinition(t *testing.T) {
	for _, path := range []string{
		"/api/pokedex/moves/testflame",
		"/api/pokedex/moves/batch?ids=testflame",
		"/api/pokedex/moves?limit=200",
	} {
		t.Run(path, func(t *testing.T) {
			q := querierWithRules()
			q.MoveRules = []store.MoveRule{{MoveID: "testflame", Rule: json.RawMessage(`{"PowerFormula":"hp_ratio"}`)}}
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, path, true)
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
		})
	}
}

func intPtr(v int) *int { return &v }
