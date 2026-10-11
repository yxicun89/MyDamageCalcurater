package httpapi

// issue #232 案B のダブル分・#288(ユーザー決定 2026-10-03)・ADR-0222: calc-svc は format=double を engine に渡し、
// 計算に反映された結果を返す(黙ってシングルの数値を返さない)。
//   - 防御側の壁 2732/4096、全体技(マスタの技の対象が spread)3072/4096
//   - マスタが技の対象を持たない間(issue #288)は、ダブルの攻撃技に技の印 move_target_unknown を付ける
// テラスタルは計算に使わない(ポケモンチャンピオンズに無い)。teraType の印は PR #497 / ADR-0160 の担当なので、
// ここでは技の印(target=move)だけを比べる(#497 の format の印とも独立にする)。

import (
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/engine/wasmapi"
)

const moveSpread = "test-quake" // normal / physical / 80 / 全体技(マスタが技の対象を持つ体の fake)

func newStoreWithSpread(t *testing.T) *fakeStore {
	t.Helper()
	store := newFakeStore(t)
	store.moves[moveSpread] = engine.Move{
		ID: moveSpread, NameJa: "テストじしん", Type: engine.TypeNormal, Category: engine.CategoryPhysical,
		Power: 80, Target: engine.MoveTargetSpread,
	}
	return store
}

type calcView struct {
	MaxDamage   int               `json:"maxDamage"`
	Unsupported []unsupportedMark `json:"unsupported"`
}

// moveMarks は印のうち技の印(target=move)だけを返す。
func moveMarks(ms []unsupportedMark) []unsupportedMark {
	out := []unsupportedMark{}
	for _, m := range ms {
		if m.Target == "move" {
			out = append(out, m)
		}
	}
	return out
}

func postCalc(t *testing.T, store *fakeStore, body map[string]any) calcView {
	t.Helper()
	rec := post(t, NewHandler(store, nil), "/api/calc", mustJSON(t, body), true)
	var got calcView
	decodeInto(t, rec, &got)
	return got
}

func doubleBody(format, moveID string, reflectOn bool) map[string]any {
	c := calcCase{attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
		defender: indiv{speciesKey: speciesLeaf, natureID: natureNeutral}, moveID: moveID}
	b := c.httpBody()
	b["format"] = format
	if reflectOn {
		b["field"] = map[string]any{"defenderScreens": map[string]any{"reflect": true}}
	}
	return b
}

func TestCalcFormatDouble(t *testing.T) {
	store := newStoreWithSpread(t)

	// 壁: シングル 1/2 < ダブル 2732/4096 < 壁なし。
	sRef := postCalc(t, store, doubleBody("single", movePhysical, true))
	dRef := postCalc(t, store, doubleBody("double", movePhysical, true))
	noRef := postCalc(t, store, doubleBody("single", movePhysical, false))
	if !(sRef.MaxDamage < dRef.MaxDamage && dRef.MaxDamage < noRef.MaxDamage) {
		t.Errorf("壁: single=%d double=%d 壁なし=%d(single < double < 壁なし のはず)", sRef.MaxDamage, dRef.MaxDamage, noRef.MaxDamage)
	}

	// 全体技: マスタの技の対象が spread ならダブルで ×0.75、シングルは等倍。技の印は付かない。
	sSpread := postCalc(t, store, doubleBody("single", moveSpread, false))
	dSpread := postCalc(t, store, doubleBody("double", moveSpread, false))
	if !(dSpread.MaxDamage < sSpread.MaxDamage) || sSpread.MaxDamage != noRef.MaxDamage {
		t.Errorf("全体技: double=%d single=%d 単体技=%d", dSpread.MaxDamage, sSpread.MaxDamage, noRef.MaxDamage)
	}
	if ms := moveMarks(dSpread.Unsupported); len(ms) != 0 {
		t.Errorf("技の対象が分かる技に技の印が付いた: %+v", ms)
	}

	// マスタが技の対象を持たない技(現状の全技。issue #288): ダブルでは技の印、シングルでは印なし。
	want := []unsupportedMark{{"move", "move_target_unknown", movePhysical}}
	if got := moveMarks(dRef.Unsupported); !reflect.DeepEqual(got, want) {
		t.Errorf("ダブル・対象不明: 技の印=%+v want %+v", got, want)
	}
	if got := moveMarks(sRef.Unsupported); len(got) != 0 {
		t.Errorf("シングル・対象不明: 技の印=%+v want []", got)
	}
}

// HTTP の結果は、同じ入力(Format=double)を engine に直接渡した結果の写しである。
func TestCalcDoubleMatchesEngine(t *testing.T) {
	store := newStoreWithSpread(t)
	c := calcCase{attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
		defender: indiv{speciesKey: speciesLeaf, natureID: natureNeutral}, moveID: moveSpread,
		field:    map[string]any{"defenderScreens": map[string]any{"lightScreen": true}},
		engField: engine.Field{DefenderScreens: engine.Screens{LightScreen: true}}}
	// 物理の全体技 × ひかりのかべ(効かない): 全体技の ×0.75 だけが single との差になる
	// (全体技 × リフレクターは 0.75 × 2732/4096 ≒ 0.5 でシングルの壁とロールが一致しうるので使わない)。
	body := c.httpBody()
	body["format"] = "double"
	in := c.engineInput(t, store)
	in.Format = engine.FormatDouble
	want, err := engine.CalcDamage(in)
	if err != nil {
		t.Fatal(err)
	}
	if got := postCalc(t, store, body); got.MaxDamage != want.MaxDamage() {
		t.Errorf("maxDamage = %d, engine = %d", got.MaxDamage, want.MaxDamage())
	}
	single := in
	single.Format = engine.FormatSingle
	ws, err := engine.CalcDamage(single)
	if err != nil {
		t.Fatal(err)
	}
	if ws.MaxDamage() == want.MaxDamage() {
		t.Errorf("single と double で engine の結果が変わらない(%d)", ws.MaxDamage())
	}
}

// 一括・逆算でも format=double が効き、対象不明の技には各行・各候補に技の印が付く。
func TestBulkAndReverseFormatDouble(t *testing.T) {
	store := newStoreWithSpread(t)
	h := NewHandler(store, nil)
	bulk := func(format, moveID string) []int {
		body := map[string]any{
			"format":             format,
			"attacker":           indiv{speciesKey: speciesAttacker, natureID: natureNeutral}.http(),
			"defenderSpeciesKey": speciesLeaf,
			"moveId":             moveID,
		}
		rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)
		var got struct {
			Rows []struct {
				Result calcView `json:"result"`
			} `json:"rows"`
		}
		decodeInto(t, rec, &got)
		out := make([]int, 0, len(got.Rows))
		for i, r := range got.Rows {
			out = append(out, r.Result.MaxDamage)
			wantMarks := 0
			if format == "double" && moveID == movePhysical {
				wantMarks = 1
			}
			if ms := moveMarks(r.Result.Unsupported); len(ms) != wantMarks {
				t.Errorf("bulk %s/%s rows[%d] の技の印=%+v(件数 want %d)", format, moveID, i, ms, wantMarks)
			}
		}
		return out
	}
	s, d := bulk("single", moveSpread), bulk("double", moveSpread)
	for i := range s {
		if !(d[i] < s[i]) {
			t.Errorf("bulk rows[%d]: double %d が single %d より小さくない(全体技 ×0.75)", i, d[i], s[i])
		}
	}
	bulk("double", movePhysical)

	rc := reverseCase{
		name: "double", side: engine.SideDefender,
		known:          indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
		unknownSpecies: speciesLeaf, moveID: movePhysical,
		observations: []engine.Observation{{Percent: 20}},
	}
	body := rc.httpBody()
	body["format"] = "double"
	rec := post(t, h, "/api/calc/reverse", mustJSON(t, body), true)
	var got struct {
		Candidates []struct {
			Unsupported []unsupportedMark `json:"unsupported"`
		} `json:"candidates"`
	}
	decodeInto(t, rec, &got)
	want := []unsupportedMark{{"move", "move_target_unknown", movePhysical}}
	if len(got.Candidates) == 0 {
		t.Fatal("candidates が空")
	}
	for i, c := range got.Candidates {
		if ms := moveMarks(c.Unsupported); !reflect.DeepEqual(ms, want) {
			t.Errorf("reverse candidates[%d] の技の印=%+v want %+v", i, ms, want)
		}
	}
}

// HTTP と WASM 境界は、ダブル・技の対象(既知/不明)でも同じ数値と同じ印を返す(ADR-0200 AC-9 のパリティ)。
func TestParityDouble(t *testing.T) {
	store := newStoreWithSpread(t)
	h := NewHandler(store, nil)
	for _, moveID := range []string{moveSpread, movePhysical} {
		t.Run(moveID, func(t *testing.T) {
			c := calcCase{attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
				defender: indiv{speciesKey: speciesLeaf, natureID: natureNeutral}, moveID: moveID,
				field: map[string]any{"defenderScreens": map[string]any{"reflect": true}}}
			hb := c.httpBody()
			hb["format"] = "double"
			wb := calcWasmBody(t, store, c)
			wb["format"] = "double"

			rec := post(t, h, "/api/calc", mustJSON(t, hb), true)
			var got map[string]any
			decodeInto(t, rec, &got)
			want, ok := wasmResult(t, wasmapi.Calc(string(mustJSON(t, wb)))).(map[string]any)
			if !ok {
				t.Fatal("wasmapi の result が object でない")
			}
			for _, k := range []string{"rolls", "minDamage", "maxDamage", "stab", "effectiveness", "unsupported"} {
				if !reflect.DeepEqual(got[k], want[k]) {
					t.Errorf("%s: HTTP=%v WASM=%v", k, got[k], want[k])
				}
			}
		})
	}
}
