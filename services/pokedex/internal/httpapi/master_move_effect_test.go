package httpapi_test

// GET /internal/pokedex/master が move_effects の行を MasterMove.effect として返すこと(ADR-0107 決定7)。
// 持ち物・特性の effect と同じ「そのまま運ぶ」扱い(値の検証は受け取った calc-svc が共通マスタで行う)。

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// moveEffectQuerier は storetest の既定に move_effects の行を足す。
// testflame は自分の素早さ+1、testglare は効果なし(行が無い)。
func moveEffectQuerier() *storetest.Querier {
	q := storetest.New()
	q.MoveEffects = []store.MoveEffect{
		{MoveID: "testflame", Effect: json.RawMessage(`{"Chance": 100, "Target": "self", "Stages": {"spe": 1}}`)},
		{MoveID: "teststrike", Effect: json.RawMessage(`{"Chance": 20, "Target": "target", "Stages": {"def": -1}}`)},
	}
	return q
}

func TestMasterExportCarriesMoveEffect(t *testing.T) {
	h := newHandler(t, moveEffectQuerier())
	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, masterPath, false, rec)

	raw := rawExport(t, rec.Body.Bytes())
	moves, ok := raw["moves"].([]any)
	if !ok {
		t.Fatal("moves が配列でない")
	}

	tests := []struct{ name, id, want string }{
		{"自分のランク変化", "testflame", `{"Chance":100,"Target":"self","Stages":{"spe":1}}`},
		{"相手のランク変化", "teststrike", `{"Chance":20,"Target":"target","Stages":{"def":-1}}`},
		{"効果の行が無い技", "testglare", `null`},
		{"効果の行が無い技(レギュレーション外)", "testbanned", `null`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findByID(t, moves, tt.id)
			effect, ok := got["effect"]
			if !ok {
				t.Fatalf("effect キーが無い(null でもキーは必須。MasterItem/MasterAbility と同じ)")
			}
			var want any
			dec := json.NewDecoder(strings.NewReader(tt.want))
			dec.UseNumber()
			if err := dec.Decode(&want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(effect, want) {
				t.Errorf("effect = %#v, want %#v", effect, want)
			}
		})
	}
}

// TestMasterExportMoveEffectIsVerbatim は、pokedex-svc が効果定義を解釈しないこと
// (数値の字面もそのまま。検証は受け取った側の責務)。
func TestMasterExportMoveEffectIsVerbatim(t *testing.T) {
	q := moveEffectQuerier()
	q.MoveEffects = append(q.MoveEffects, store.MoveEffect{
		MoveID: "testglare", Effect: json.RawMessage(`{"Chance": 100.0, "Stages": {"spe": 9007199254740993}}`),
	})
	h := newHandler(t, q)
	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	moves := rawExport(t, rec.Body.Bytes())["moves"].([]any)
	effect := findByID(t, moves, "testglare")["effect"]

	var want any
	dec := json.NewDecoder(strings.NewReader(`{"Chance":100.0,"Stages":{"spe":9007199254740993}}`))
	dec.UseNumber()
	if err := dec.Decode(&want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(effect, want) {
		t.Errorf("effect = %#v, want %#v(数値の字面をそのまま)", effect, want)
	}
}
