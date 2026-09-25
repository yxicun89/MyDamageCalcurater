package wasmapi_test

// issue #272 / ADR-0126: calcBulk の defenderAbilities・calcReverse の unknownAbilities。
//
//   - 送らない(Web がまだ対応していない)ときは、応答がバイト単位で従来と同じ(abilityId / abilityIds を出さない)
//   - 送ると各行・各候補に abilityId(計算に使った特性)と abilityIds(結果が同じ特性)が出て、無効が効く
//   - 件数超過・重複・空の ID・種族が持たない特性は invalid_input

import (
	"encoding/json"
	"strings"
	"testing"

	"example.com/pokecalc/engine/wasmapi"
)

func abilityValue(id string, effect map[string]any) map[string]any {
	a := map[string]any{"id": id, "nameJa": "テストとくせい"}
	if effect != nil {
		a["effect"] = effect
	}
	return a
}

func normalImmuneAbility() map[string]any {
	return abilityValue("test-immune", map[string]any{"defImmuneTypes": []any{"normal"}})
}

type abilityRowView struct {
	Preset     string   `json:"preset"`
	AbilityID  *string  `json:"abilityId"`
	AbilityIDs []string `json:"abilityIds"`
	Result     struct {
		Rolls []int `json:"rolls"`
	} `json:"result"`
}

func bulkRowsOf(t *testing.T, resp string) []abilityRowView {
	t.Helper()
	var env struct {
		Result *struct {
			Rows []abilityRowView `json:"rows"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil || env.Result == nil {
		t.Fatalf("成功の封筒でない: %v %s", err, resp)
	}
	return env.Result.Rows
}

func TestWasmBulkWithoutDefenderAbilitiesOmitsAbilityFields(t *testing.T) {
	resp := wasmapi.CalcBulk(mustJSON(t, baseBulk()))
	if strings.Contains(resp, `"abilityId`) {
		t.Fatalf("defenderAbilities を送らないのに abilityId が出た(従来の応答と変わる): %s", truncate(resp))
	}
	withEmpty := baseBulk()
	withEmpty["defenderAbilities"] = []any{}
	if got := wasmapi.CalcBulk(mustJSON(t, withEmpty)); got != resp {
		t.Errorf("空配列と省略で応答が違う")
	}
}

func TestWasmBulkDefenderAbilitiesSplitRows(t *testing.T) {
	req := baseBulk()
	sub(t, req, "defenderSpecies")["abilities"] = []any{"test-immune", "test-plain"}
	req["defenderAbilities"] = []any{normalImmuneAbility(), abilityValue("test-plain", nil)}
	legacy := bulkRowsOf(t, wasmapi.CalcBulk(mustJSON(t, baseBulk())))
	rows := bulkRowsOf(t, wasmapi.CalcBulk(mustJSON(t, req)))
	if len(rows) != 2*len(legacy) {
		t.Fatalf("行数=%d want %d(ノーマル技は無効の特性だけ結果が違う)", len(rows), 2*len(legacy))
	}
	for i, row := range rows {
		want := "test-immune"
		if i%2 == 1 {
			want = "test-plain"
		}
		if row.AbilityID == nil || *row.AbilityID != want || len(row.AbilityIDs) != 1 || row.AbilityIDs[0] != want {
			t.Fatalf("rows[%d] abilityId=%v abilityIds=%v want %s", i, row.AbilityID, row.AbilityIDs, want)
		}
		zero := true
		for _, d := range row.Result.Rolls {
			zero = zero && d == 0
		}
		if zero != (want == "test-immune") {
			t.Errorf("rows[%d](%s) rolls=%v", i, want, row.Result.Rolls)
		}
		if want == "test-plain" && !equalInts(row.Result.Rolls, legacy[i/2].Result.Rolls) {
			t.Errorf("rows[%d] 効果の無い特性の結果が従来の行と違う", i)
		}
	}
}

func TestWasmReverseUnknownAbilities(t *testing.T) {
	legacy := wasmapi.CalcReverse(mustJSON(t, baseReverse()))
	if strings.Contains(legacy, `"abilityId`) {
		t.Fatalf("unknownAbilities を送らないのに abilityId が出た: %s", truncate(legacy))
	}
	req := baseReverse()
	req["maxCandidates"] = 0
	req["unknownAbilities"] = []any{abilityValue("test-plain", nil), normalImmuneAbility()}
	var env struct {
		Result *struct {
			Candidates []struct {
				AbilityID  string   `json:"abilityId"`
				AbilityIDs []string `json:"abilityIds"`
				Exact      bool     `json:"exact"`
			} `json:"candidates"`
		} `json:"result"`
	}
	resp := wasmapi.CalcReverse(mustJSON(t, req))
	if err := json.Unmarshal([]byte(resp), &env); err != nil || env.Result == nil {
		t.Fatalf("成功の封筒でない: %v %s", err, resp)
	}
	seen := map[string]bool{}
	for _, c := range env.Result.Candidates {
		seen[c.AbilityID] = true
		if len(c.AbilityIDs) != 1 || c.AbilityIDs[0] != c.AbilityID {
			t.Errorf("abilityIds=%v abilityId=%q", c.AbilityIDs, c.AbilityID)
		}
	}
	if !seen["test-plain"] || !seen["test-immune"] {
		t.Errorf("特性ごとの候補が無い: %v", seen)
	}
	if first := env.Result.Candidates[0]; first.AbilityID != "test-plain" {
		t.Errorf("先頭候補の特性=%q want test-plain(無効の特性ではダメージが出ない)", first.AbilityID)
	}
}

func TestWasmAbilityCandidatesRejected(t *testing.T) {
	four := []any{abilityValue("a1", nil), abilityValue("a2", nil), abilityValue("a3", nil), abilityValue("a4", nil)}
	cases := []struct {
		name      string
		species   []any
		abilities []any
	}{
		{"件数超過", nil, four},
		{"重複", nil, []any{abilityValue("a1", nil), abilityValue("a1", nil)}},
		{"空の ID", nil, []any{abilityValue("", nil)}},
		{"種族が持たない特性", []any{"other"}, []any{abilityValue("a1", nil)}},
	}
	for _, tc := range cases {
		t.Run("calcBulk/"+tc.name, func(t *testing.T) {
			req := baseBulk()
			if tc.species != nil {
				sub(t, req, "defenderSpecies")["abilities"] = tc.species
			}
			req["defenderAbilities"] = tc.abilities
			if got := decodeError(t, wasmapi.CalcBulk(mustJSON(t, req))); got.Code != wasmapi.CodeInvalidInput {
				t.Errorf("code=%q want invalid_input(%s)", got.Code, got.Message)
			}
		})
		t.Run("calcReverse/"+tc.name, func(t *testing.T) {
			req := baseReverse()
			if tc.species != nil {
				sub(t, req, "unknownSpecies")["abilities"] = tc.species
			}
			req["unknownAbilities"] = tc.abilities
			if got := decodeError(t, wasmapi.CalcReverse(mustJSON(t, req))); got.Code != wasmapi.CodeInvalidInput {
				t.Errorf("code=%q want invalid_input(%s)", got.Code, got.Message)
			}
		})
	}
	// 不正な効果定義は calc と同じコードに写る(境界が効果を検証している)。
	req := baseBulk()
	req["defenderAbilities"] = []any{abilityValue("a1", map[string]any{"defImmuneTypes": []any{"nosuchtype"}})}
	if got := decodeError(t, wasmapi.CalcBulk(mustJSON(t, req))); got.Code != wasmapi.CodeInvalidEnum {
		t.Errorf("code=%q want invalid_enum", got.Code)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
