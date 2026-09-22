package httpapi

// 境界値の受け入れテスト(critic 指摘 R4・R6)。SP・ランクの端点、無効相性(0倍)の成功ケース、
// テラスタイプが相性表に無いときの実行時エラーを固定する。

import (
	"net/http"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

// R4: SP は各 0..32・合計 <=66 の境界(単体ちょうど32、合計ちょうど66)で成功する。
func TestCalcDamageSPBoundaries(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	tests := []struct {
		name string
		sp   engine.Stats
	}{
		{"単体ちょうど32", engine.Stats{Atk: 32}},
		{"合計ちょうど66", engine.Stats{Atk: 32, Def: 32, Spe: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := calcCases()[0]
			c.attacker.sp = tt.sp
			want, err := engine.CalcDamage(c.engineInput(t, store))
			if err != nil {
				t.Fatalf("engine.CalcDamage = %v", err)
			}
			rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), true)
			var got api.CalcResult
			decodeInto(t, rec, &got)
			assertCalcResultMatchesEngine(t, got, want)
		})
	}
}

// R4: ランクは -6..+6 の境界。+6/-6 は成功、範囲外(-7)は invalid_input。
func TestCalcDamageRankBoundaries(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	okTests := []struct {
		name  string
		ranks engine.Ranks
	}{
		{"Atk +6", engine.Ranks{Atk: 6}},
		{"Atk -6", engine.Ranks{Atk: -6}},
	}
	for _, tt := range okTests {
		t.Run(tt.name, func(t *testing.T) {
			c := calcCases()[0]
			c.attacker.ranks = tt.ranks
			want, err := engine.CalcDamage(c.engineInput(t, store))
			if err != nil {
				t.Fatalf("engine.CalcDamage = %v", err)
			}
			rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), true)
			var got api.CalcResult
			decodeInto(t, rec, &got)
			assertCalcResultMatchesEngine(t, got, want)
		})
	}

	t.Run("Atk -7 は invalid_input", func(t *testing.T) {
		c := calcCases()[0]
		c.attacker.ranks = engine.Ranks{Atk: -7}
		rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), false)
		assertError(t, rec, http.StatusBadRequest, "invalid_input")
	})
}

// R4: 無効相性(0倍)は黙って弾かず 200 になり、effectiveness 0・ko.hits 0・表示% 0.0 になる。
func TestCalcDamageImmuneMatchup(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	c := calcCase{
		name:     "無効相性(ノーマル→ゴースト)",
		attacker: indiv{speciesKey: speciesAttacker, natureID: natureNeutral},
		defender: indiv{speciesKey: speciesGhost, natureID: natureNeutral},
		moveID:   movePhysical,
	}
	want, err := engine.CalcDamage(c.engineInput(t, store))
	if err != nil {
		t.Fatalf("engine.CalcDamage = %v", err)
	}
	if want.Effectiveness != 0 || want.KO.Hits != 0 {
		t.Fatalf("fixture が無効相性になっていない(前提が崩れている): %+v", want)
	}

	rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), true)
	var got api.CalcResult
	decodeInto(t, rec, &got)
	assertCalcResultMatchesEngine(t, got, want)
	if got.Effectiveness != 0 {
		t.Errorf("effectiveness = %v, want 0", got.Effectiveness)
	}
	if got.MinPercent != 0 || got.MaxPercent != 0 {
		t.Errorf("min/maxPercent = %v/%v, want 0.0/0.0", got.MinPercent, got.MaxPercent)
	}
	if got.Ko.Hits != 0 || got.Ko.Guaranteed || got.Ko.DisplayChancePercent != 0 {
		t.Errorf("ko = %+v, want hits/guaranteed/displayChancePercent すべて 0/false/0.0", got.Ko)
	}
}

// R6: teraType は種族・技のタイプと違いリクエスト由来の値で、master.New の起動時検査を
// 経ないため、相性表に無ければ実行時に 400 unknown_type になる(黙って等倍にしない)。
func TestCalcDamageUnknownTeraType(t *testing.T) {
	store := newFakeStore(t)
	// 種族・技のタイプがすべて収まる、意図的に小さい相性表に差し替える(psychic を含まない)。
	reduced, err := engine.NewTypeChart(engine.TypeChartData{
		Types: []engine.Type{engine.TypeNormal, engine.TypeWater, engine.TypeSteel},
	})
	if err != nil {
		t.Fatalf("engine.NewTypeChart = %v", err)
	}
	store.chart = reduced
	h := NewHandler(store)

	c := calcCases()[0]
	c.attacker.tera = engine.TypePsychic // 相性表(上)に無いタイプ
	rec := post(t, h, "/api/calc", mustJSON(t, c.httpBody()), false)
	assertError(t, rec, http.StatusBadRequest, "unknown_type")
}
