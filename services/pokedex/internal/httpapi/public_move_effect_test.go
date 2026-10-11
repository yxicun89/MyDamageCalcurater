package httpapi_test

// 公開 API の技(Move)に確率 100% のランク変化(move_effects)を出すことのテスト(ADR-0252。連続技 CB-4)。
//
// 受け入れ条件(pokedex-svc):
//   - AC-E1 getMove・getMovesByIds・searchMoves の Move.effect は、move_effects の行の Chance が 100 のときだけ、
//     内部 API の MasterMove.effect と同じ値(Chance・Target・Stages)。確率 100% 未満の効果・効果の行が無い技は
//     キーごと省く(null を返さない)。攻撃技・変化技のどちらにも付く。応答は契約に合う。
//   - AC-E2 検証を通らない効果(共通マスタの DecodeMoveEffect が拒否する JSON)があれば 503 master_unavailable
//     (黙って捨てない。rule・効果定義と同じ扱い)。確率 100% 未満の行でも検証する(黙って省かない)。

import (
	"encoding/json"
	"net/http"
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

const (
	effectSelfSpeed = `{"Chance":100,"Target":"self","Stages":{"spe":1}}`         // 攻撃技(ニトロチャージ相当)
	effectDance     = `{"Chance":100,"Target":"self","Stages":{"atk":1,"spe":1}}` // 変化技(りゅうのまい相当)
	effectChance20  = `{"Chance":20,"Target":"target","Stages":{"def":-1}}`       // 確率 100% 未満(出さない)
)

// publicEffectQuerier は storetest の既定に move_effects の行を足す。
// testflame(攻撃技)は確率 100%、testglare(変化技)は確率 100%、teststrike は 20%、testbanned は行なし。
func publicEffectQuerier() *storetest.Querier {
	q := storetest.New()
	q.MoveEffects = []store.MoveEffect{
		{MoveID: "testflame", Effect: json.RawMessage(effectSelfSpeed)},
		{MoveID: "testglare", Effect: json.RawMessage(effectDance)},
		{MoveID: "teststrike", Effect: json.RawMessage(effectChance20)},
	}
	return q
}

// wantPublicEffect は技ごとの期待(""はキーなし)。
var wantPublicEffect = map[string]string{
	"testflame":  effectSelfSpeed,
	"testglare":  effectDance,
	"teststrike": "",
	"testbanned": "",
}

func checkPublicEffect(t *testing.T, path string, m map[string]any) {
	t.Helper()
	id, _ := m["id"].(string)
	want, known := wantPublicEffect[id]
	if !known {
		return
	}
	got, has := m["effect"]
	switch {
	case want == "" && has:
		t.Errorf("%s: %s.effect = %v, want キーなし(確率 100%% 未満・効果なし)", path, id, got)
	case want != "" && (!has || !sameJSON(t, got, want)):
		t.Errorf("%s: %s.effect = %v(ある=%v), want %s", path, id, got, has, want)
	}
}

// AC-E1
func TestPublicMoveEffectOnlyGuaranteed(t *testing.T) {
	h := newHandler(t, publicEffectQuerier())
	for id := range wantPublicEffect {
		path := "/api/pokedex/moves/" + id
		t.Run(path, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, path, true)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
			}
			validateAgainstContract(t, http.MethodGet, path, true, rec)
			var m map[string]any
			decodeStrict(t, rec.Body.Bytes(), &m)
			checkPublicEffect(t, path, m)
		})
	}
	for _, path := range []string{
		"/api/pokedex/moves/batch?ids=testflame&ids=testglare&ids=teststrike&ids=testbanned",
		"/api/pokedex/moves?limit=200",
	} {
		t.Run(path, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, path, true)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
			}
			validateAgainstContract(t, http.MethodGet, path, true, rec)
			var list []map[string]any
			decodeStrict(t, rec.Body.Bytes(), &list)
			seen := false
			for _, m := range list {
				checkPublicEffect(t, path, m)
				seen = seen || m["id"] == "testflame"
			}
			if !seen {
				t.Errorf("testflame が応答に無い")
			}
		})
	}
}

// AC-E1: 生成型でも読める(effect は MasterEffect)。
func TestPublicMoveEffectDecodesAsGeneratedType(t *testing.T) {
	h := newHandler(t, publicEffectQuerier())
	rec := do(t, h, http.MethodGet, "/api/pokedex/moves/testglare", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	var mv api.Move
	if err := json.Unmarshal(rec.Body.Bytes(), &mv); err != nil {
		t.Fatalf("api.Move として読めない: %v", err)
	}
	if mv.Effect == nil || !sameJSON(t, *mv.Effect, effectDance) {
		t.Errorf("effect = %v, want %s", mv.Effect, effectDance)
	}
}

// AC-E2
func TestPublicMoveEffectRejectsInvalidEffect(t *testing.T) {
	invalid := []struct{ name, effect string }{
		{"未知のキー", `{"Chance":100,"Target":"self","Stages":{"spe":1},"Extra":1}`},
		{"変化量 0", `{"Chance":100,"Target":"self","Stages":{"spe":0}}`},
		{"確率 100% 未満でも検証する", `{"Chance":10,"Target":"self","Stages":{"hp":1}}`},
	}
	for _, inv := range invalid {
		for _, path := range []string{
			"/api/pokedex/moves/testflame",
			"/api/pokedex/moves/batch?ids=testflame",
			"/api/pokedex/moves?limit=200",
		} {
			t.Run(inv.name+" "+path, func(t *testing.T) {
				q := publicEffectQuerier()
				q.MoveEffects = []store.MoveEffect{{MoveID: "testflame", Effect: json.RawMessage(inv.effect)}}
				h := newHandler(t, q)
				rec := do(t, h, http.MethodGet, path, true)
				assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
			})
		}
	}
}
