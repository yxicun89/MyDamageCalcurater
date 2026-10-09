package httpapi_test

// 技のフラグ(move_flags。ADR-0178)と機構を内部 API・公開 API に出すことのテスト。
//
// 受け入れ条件(pokedex-svc):
//   - AC-P1 内部 API(getMasterExport の MasterMove.flags): 技ごとのフラグを昇順で返す(SQL の並びに頼らない)。
//     フラグの無い技は空配列。値は DB の文字列のまま(検証は calc-svc。mechanisms と同じ)。
//   - AC-P2 move_flags が空(まだ取り込んでいない)なら、内部 API・公開 API とも全技で flags キーを省く(不明。空配列にしない)。
//   - AC-P3 公開 API(getMove・getMovesByIds・searchMoves の Move.flags・Move.mechanisms): flags は AC-P1 と同じ規則、
//     mechanisms は常に返す(昇順・通常の技は空配列)。応答は契約に合う。
//   - AC-P4 公開 API は DB に語彙に無い値(CHECK をすり抜けた値)があれば 503 master_unavailable(黙って捨てない)。
//
// fixture(storetest.New)のフラグ: testflame = sound・secondary(逆順に入れてある)、teststrike = contact・punch、
// testglare・testbanned = フラグなし。機構: testflame = variable_power・multi_hit(逆順)、他は無し。

import (
	"net/http"
	"reflect"
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

var wantMoveFlags = map[string][]string{
	"testflame":  {"secondary", "sound"},
	"teststrike": {"contact", "punch"},
	"testglare":  {},
	"testbanned": {},
}

var wantMoveMechanisms = map[string][]string{
	"testflame":  {"multi_hit", "variable_power"},
	"teststrike": {},
	"testglare":  {},
	"testbanned": {},
}

func publicFlagStrings(p *[]api.MoveFlag) []string {
	if p == nil {
		return nil
	}
	out := make([]string, 0, len(*p))
	for _, f := range *p {
		out = append(out, string(f))
	}
	return out
}

// AC-P1
func TestMasterExportCarriesMoveFlags(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)
	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, masterPath, false, rec)

	ex := decodeExport(t, rec.Body.Bytes())
	for _, m := range ex.Moves {
		want, ok := wantMoveFlags[m.Id]
		if !ok {
			continue
		}
		if m.Flags == nil {
			t.Errorf("%s.flags が無い(フラグを取り込み済みなのにキーを省いた)", m.Id)
			continue
		}
		if !reflect.DeepEqual(append([]string{}, *m.Flags...), want) {
			t.Errorf("%s.flags = %v, want %v(昇順・フラグなしは空配列)", m.Id, *m.Flags, want)
		}
	}
}

// AC-P2
func TestMoveFlagsOmittedBeforeImport(t *testing.T) {
	q := storetest.New()
	q.MoveFlags = nil
	h := newHandler(t, q)

	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, masterPath, false, rec)
	raw := rawExport(t, rec.Body.Bytes())
	for _, v := range raw["moves"].([]any) {
		m := v.(map[string]any)
		if f, ok := m["flags"]; ok {
			t.Errorf("内部 API: 技 %v に flags キーがある(%v)。取り込み前は不明としてキーを省く", m["id"], f)
		}
	}

	for _, path := range []string{"/api/pokedex/moves/teststrike"} {
		rec := do(t, h, http.MethodGet, path, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d\nbody=%s", path, rec.Code, rec.Body.String())
		}
		var m map[string]any
		decodeStrict(t, rec.Body.Bytes(), &m)
		if f, ok := m["flags"]; ok {
			t.Errorf("公開 API: flags キーがある(%v)。取り込み前は不明としてキーを省く", f)
		}
		if _, ok := m["mechanisms"]; !ok {
			t.Error("公開 API: mechanisms キーが無い(機構は不明の状態が無いので常に返す)")
		}
	}
	rec = do(t, h, http.MethodGet, "/api/pokedex/moves/batch?ids=testflame&ids=teststrike", true)
	var list []map[string]any
	decodeStrict(t, rec.Body.Bytes(), &list)
	for _, m := range list {
		if f, ok := m["flags"]; ok {
			t.Errorf("公開 API(batch): %v に flags キーがある(%v)", m["id"], f)
		}
	}
}

// AC-P3
func TestPublicMoveFlagsAndMechanisms(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)

	check := func(t *testing.T, m api.Move) {
		t.Helper()
		if want, ok := wantMoveFlags[m.Id]; ok {
			if m.Flags == nil {
				t.Errorf("%s.flags が無い", m.Id)
			} else if got := publicFlagStrings(m.Flags); !reflect.DeepEqual(got, want) {
				t.Errorf("%s.flags = %v, want %v", m.Id, got, want)
			}
		}
		if want, ok := wantMoveMechanisms[m.Id]; ok {
			if m.Mechanisms == nil {
				t.Errorf("%s.mechanisms が無い", m.Id)
			} else if !reflect.DeepEqual(append([]string{}, *m.Mechanisms...), want) {
				t.Errorf("%s.mechanisms = %v, want %v", m.Id, *m.Mechanisms, want)
			}
		}
	}

	t.Run("getMove", func(t *testing.T) {
		for id := range wantMoveFlags {
			path := "/api/pokedex/moves/" + id
			rec := do(t, h, http.MethodGet, path, true)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: status = %d\nbody=%s", id, rec.Code, rec.Body.String())
			}
			validateAgainstContract(t, http.MethodGet, path, true, rec)
			var m api.Move
			decodeStrict(t, rec.Body.Bytes(), &m)
			check(t, m)
		}
	})
	t.Run("getMovesByIds", func(t *testing.T) {
		path := "/api/pokedex/moves/batch?ids=testflame&ids=teststrike&ids=testglare&ids=testbanned"
		rec := do(t, h, http.MethodGet, path, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
		}
		validateAgainstContract(t, http.MethodGet, path, true, rec)
		var moves []api.Move
		decodeStrict(t, rec.Body.Bytes(), &moves)
		if len(moves) != 4 {
			t.Fatalf("moves = %+v, want 4件", moves)
		}
		for _, m := range moves {
			check(t, m)
		}
	})
	t.Run("searchMoves", func(t *testing.T) {
		path := "/api/pokedex/moves?limit=200"
		rec := do(t, h, http.MethodGet, path, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
		}
		validateAgainstContract(t, http.MethodGet, path, true, rec)
		var moves []api.Move
		decodeStrict(t, rec.Body.Bytes(), &moves)
		if len(moves) == 0 {
			t.Fatal("検索の応答が空")
		}
		for _, m := range moves {
			check(t, m)
		}
	})
}

// AC-P4
func TestPublicMoveFlagsRejectsUnknownValue(t *testing.T) {
	for _, path := range []string{
		"/api/pokedex/moves/teststrike",
		"/api/pokedex/moves/batch?ids=teststrike",
		"/api/pokedex/moves?limit=200",
	} {
		t.Run("flags"+path, func(t *testing.T) {
			q := storetest.New()
			q.MoveFlags = append(q.MoveFlags, store.MoveFlag{MoveID: "teststrike", Flag: "wind"})
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, path, true)
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
		})
		t.Run("mechanisms"+path, func(t *testing.T) {
			q := storetest.New()
			q.MoveMechanisms = append(q.MoveMechanisms, store.MoveMechanism{MoveID: "teststrike", Mechanism: "teleport"})
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, path, true)
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
		})
	}
}
