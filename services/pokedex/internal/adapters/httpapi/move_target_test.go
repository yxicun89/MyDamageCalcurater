package httpapi_test

// 技の対象(moves.target。ADR-0136)を内部 API と公開 API に出すことのテスト(issue 288・ADR-0223)。
//
//   - 内部 API(getMasterExport の MasterMove.target): DB の Showdown の文字列をそのまま返す。NULL は JSON の null で、
//     キーは省かない(必須キー)。値の検証は受け取った calc-svc がする(mechanisms と同じ。ADR-0121)。
//   - 公開 API(getMove・getMovesByIds・searchMoves の Move.target): engine の分類(全体技 → spread、その他 → single)。
//     NULL はキーごと省く。DB に未知の値があれば分類できないので 503 master_unavailable(効果の検証と同じ扱い。ADR-0218)。
//
// fixture(storetest.New)の対象: testflame = allAdjacentFoes、teststrike = normal、testglare = self、testbanned = NULL。

import (
	"database/sql"
	"net/http"
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// AC-T1: 内部 API の MasterMove.target は DB の文字列そのまま。NULL は null(キーを省かない)。
func TestMasterExportCarriesMoveTarget(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)
	rec := do(t, h, http.MethodGet, masterPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\nbody=%s", rec.Code, rec.Body.String())
	}
	validateAgainstContract(t, http.MethodGet, masterPath, false, rec)

	ex := decodeExport(t, rec.Body.Bytes())
	got := map[string]*string{}
	for _, m := range ex.Moves {
		got[m.Id] = m.Target
	}
	want := map[string]string{"testflame": "allAdjacentFoes", "teststrike": "normal", "testglare": "self"}
	for id, w := range want {
		if got[id] == nil || *got[id] != w {
			t.Errorf("%s.target = %v, want %q(DB の文字列のまま。分類しない)", id, deref(got[id]), w)
		}
	}
	if got["testbanned"] != nil {
		t.Errorf("testbanned.target = %q, want null(まだ取り込んでいない)", *got["testbanned"])
	}

	// キーの有無は生の JSON で確かめる(*string は「欠落」と「null」を区別できない)。
	raw := rawExport(t, rec.Body.Bytes())
	for _, v := range raw["moves"].([]any) {
		m := v.(map[string]any)
		if _, ok := m["target"]; !ok {
			t.Errorf("技 %v に target キーが無い(null でもキーは省かない)", m["id"])
		}
	}
}

// AC-T4: 公開 API の Move.target は分類(spread / single)。NULL の技はキーごと省く。応答は契約に合う。
func TestPublicMoveTarget(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)

	wantTarget := map[string]string{"testflame": "spread", "teststrike": "single", "testglare": "single"}

	t.Run("getMove", func(t *testing.T) {
		for id, w := range wantTarget {
			path := "/api/pokedex/moves/" + id
			rec := do(t, h, http.MethodGet, path, true)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: status = %d\nbody=%s", id, rec.Code, rec.Body.String())
			}
			validateAgainstContract(t, http.MethodGet, path, true, rec)
			var m api.Move
			decodeStrict(t, rec.Body.Bytes(), &m)
			if m.Target == nil || string(*m.Target) != w {
				t.Errorf("%s.target = %v, want %q", id, targetString(m.Target), w)
			}
		}
		rec := do(t, h, http.MethodGet, "/api/pokedex/moves/testbanned", true)
		if rec.Code != http.StatusOK {
			t.Fatalf("testbanned: status = %d\nbody=%s", rec.Code, rec.Body.String())
		}
		assertNoTargetKey(t, rec.Body.Bytes(), "testbanned")
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
			if m.Id == "testbanned" {
				if m.Target != nil {
					t.Errorf("testbanned.target = %q, want キーなし(対象が不明)", *m.Target)
				}
				continue
			}
			if m.Target == nil || string(*m.Target) != wantTarget[m.Id] {
				t.Errorf("%s.target = %v, want %q", m.Id, targetString(m.Target), wantTarget[m.Id])
			}
		}
		assertNoTargetKeyInList(t, rec.Body.Bytes(), "testbanned")
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
		seen := 0
		for _, m := range moves {
			w, ok := wantTarget[m.Id]
			if !ok {
				continue
			}
			seen++
			if m.Target == nil || string(*m.Target) != w {
				t.Errorf("%s.target = %v, want %q", m.Id, targetString(m.Target), w)
			}
		}
		if seen == 0 {
			t.Fatalf("検索の応答に fixture の技が無い: %+v", moves)
		}
	})
}

// AC-T5: DB に未知の対象(CHECK をすり抜けた値)があると、公開 API は分類できないので 503 master_unavailable。
// 黙って single にしない・キーを省いて不明に見せない。
func TestPublicMoveTargetRejectsUnknownValue(t *testing.T) {
	for _, path := range []string{
		"/api/pokedex/moves/teststrike",
		"/api/pokedex/moves/batch?ids=teststrike",
		"/api/pokedex/moves?limit=200",
	} {
		t.Run(path, func(t *testing.T) {
			q := storetest.New()
			for i := range q.Moves {
				if q.Moves[i].ID == "teststrike" {
					q.Moves[i].Target = sql.NullString{String: "teleport", Valid: true}
				}
			}
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, path, true)
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
		})
	}
}

func deref(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func targetString(p *api.MoveTarget) any {
	if p == nil {
		return nil
	}
	return string(*p)
}

// assertNoTargetKey は Move 1件の本文に target キーが無いこと(null も返さない)を確かめる。
func assertNoTargetKey(t *testing.T, body []byte, id string) {
	t.Helper()
	var m map[string]any
	decodeStrict(t, body, &m)
	if v, ok := m["target"]; ok {
		t.Errorf("%s に target キーがある(%v)。対象が不明な技はキーごと省く", id, v)
	}
}

// assertNoTargetKeyInList は Move の配列の本文で、id の要素に target キーが無いことを確かめる。
func assertNoTargetKeyInList(t *testing.T, body []byte, id string) {
	t.Helper()
	var list []map[string]any
	decodeStrict(t, body, &list)
	for _, m := range list {
		if m["id"] != id {
			continue
		}
		if v, ok := m["target"]; ok {
			t.Errorf("%s に target キーがある(%v)。対象が不明な技はキーごと省く", id, v)
		}
	}
}
