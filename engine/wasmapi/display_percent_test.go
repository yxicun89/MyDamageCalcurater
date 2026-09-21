package wasmapi_test

// P1-11「表示%の分離」の境界側の受け入れ条件(ADR-0011 §3 / AC-11、ADR-0010 §3)。
//
// 固定すること:
//   - 表示%(minPercent / maxPercent / ko.displayChancePercent)は JSON 上で
//     必ず小数第1位を1桁持つ(73.4 / 100.0 / 0.0)。指数表記にしない。
//     書式が1つに決まるので、Go とWASM のレスポンスはバイト一致する(§7 の一致テストは文字列比較)。
//   - 値は engine の 0.1% 単位の整数と一致する(境界が独自に丸め直さない)。
//   - ko は生値(chancePercent)と表示値(displayChancePercent)の両方を持つ。
//   - 逆算の入力 observations[].percent は観測%(整数)のままで、別概念。

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
)

// displayPercentKeys は「小数第1位で出す」キー(どの入れ子にあっても同じ意味)。
var displayPercentKeys = map[string]bool{
	"minPercent": true, "maxPercent": true, "displayChancePercent": true,
}

// oneDecimal は 12.3 / -0.1 / 137.0 のような「小数第1位をちょうど1桁持つ10進数」。
var oneDecimal = regexp.MustCompile(`^-?(0|[1-9][0-9]*)\.[0-9]$`)

// --- AC-11: 書式 -------------------------------------------------------------

func TestDisplayPercentFieldsAreOneDecimal(t *testing.T) {
	cases := map[string]string{
		"calc":        mustJSON(t, baseCalc()),
		"calcBulk":    mustJSON(t, baseBulk()),
		"calcReverse": mustJSON(t, baseReverse()),
	}
	for fn, req := range cases {
		t.Run(fn, func(t *testing.T) {
			resp := invoke(t, fn, req)
			dec := json.NewDecoder(strings.NewReader(resp))
			dec.UseNumber()
			var v any
			if err := dec.Decode(&v); err != nil {
				t.Fatalf("レスポンスが JSON ではない: %v", err)
			}
			seen := walkDisplayPercents(t, v, "")
			if seen == 0 {
				t.Fatalf("表示%%のフィールドが1つも出てこない(契約が消えている): %s", resp)
			}
		})
	}
}

// walkDisplayPercents は表示%のキーをすべて訪ね、書式を検査した個数を返す。
func walkDisplayPercents(t *testing.T, v any, key string) int {
	t.Helper()
	n := 0
	switch x := v.(type) {
	case map[string]any:
		for k, sv := range x {
			n += walkDisplayPercents(t, sv, k)
		}
	case []any:
		for _, sv := range x {
			n += walkDisplayPercents(t, sv, key)
		}
	case json.Number:
		if !displayPercentKeys[key] {
			return 0
		}
		s := x.String()
		if !oneDecimal.MatchString(s) {
			t.Errorf("%q は小数第1位を1桁だけ持つ10進数であるべき(指数表記・整数表記は不可): %q", key, s)
		}
		n = 1
	}
	return n
}

// --- AC-11: 値が engine の tenths と一致する --------------------------------

func TestDisplayPercentFieldsMatchEngineTenths(t *testing.T) {
	resp := invoke(t, "calc", mustJSON(t, baseCalc()))

	var env struct {
		Result struct {
			Rolls      [16]int     `json:"rolls"`
			DefenderHP int         `json:"defenderHP"`
			MinPercent json.Number `json:"minPercent"`
			MaxPercent json.Number `json:"maxPercent"`
			KO         struct {
				Hits                 int         `json:"hits"`
				Guaranteed           bool        `json:"guaranteed"`
				ChancePercent        float64     `json:"chancePercent"`
				DisplayChancePercent json.Number `json:"displayChancePercent"`
			} `json:"ko"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("レスポンスが読めない: %v\n%s", err, resp)
	}
	r := env.Result

	wantMin := engine.DisplayPercentTenthsFloor(r.Rolls[0], r.DefenderHP)
	wantMax := engine.DisplayPercentTenthsRound(r.Rolls[15], r.DefenderHP)
	if got := parseTenths(t, "minPercent", r.MinPercent); got != wantMin {
		t.Errorf("minPercent = %d, want %d(0.1%%単位。切り捨て)", got, wantMin)
	}
	if got := parseTenths(t, "maxPercent", r.MaxPercent); got != wantMax {
		t.Errorf("maxPercent = %d, want %d(0.1%%単位。四捨五入)", got, wantMax)
	}

	ko := engine.KOChance{Hits: r.KO.Hits, Guaranteed: r.KO.Guaranteed, ChancePercent: r.KO.ChancePercent}
	if got, want := parseTenths(t, "displayChancePercent", r.KO.DisplayChancePercent),
		ko.DisplayChancePercentTenths(); got != want {
		t.Errorf("ko.displayChancePercent = %d, want %d(0.1%%単位)", got, want)
	}
}

// parseTenths は "73.4" を 734 に戻す(書式の検査を兼ねる)。
func parseTenths(t *testing.T, key string, n json.Number) int {
	t.Helper()
	s := n.String()
	if !oneDecimal.MatchString(s) {
		t.Fatalf("%q の書式が契約と違う: %q", key, s)
	}
	v, err := strconv.Atoi(strings.Replace(s, ".", "", 1))
	if err != nil {
		t.Fatalf("%q が数値でない: %q", key, s)
	}
	return v
}

// --- AC-11: ko のキー集合 ----------------------------------------------------

func TestKOObjectShape(t *testing.T) {
	resp := invoke(t, "calc", mustJSON(t, baseCalc()))
	var env struct {
		Result struct {
			KO map[string]json.RawMessage `json:"ko"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("レスポンスが読めない: %v", err)
	}
	want := []string{"hits", "guaranteed", "chancePercent", "displayChancePercent"}
	if got := keysOf(env.Result.KO); !sameSet(got, want) {
		t.Errorf("ko のキー集合が契約と違う\n got %v\nwant %v", got, want)
	}
}

// TestGuaranteedKOIsDisplayedAsHundred は、確定n発のときに画面へ出せる値が
// 100.0% になることを実際のレスポンスで固定する(生値の 0 をそのまま出さない)。
func TestGuaranteedKOIsDisplayedAsHundred(t *testing.T) {
	req := baseCalc()
	// 防御側を極端に柔らかくして確定1〜2発にする。
	sub(t, req, "defender")["sp"] = map[string]any{"hp": 0, "atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": 0}
	sub(t, sub(t, req, "defender"), "species")["baseStats"] = map[string]any{
		"hp": 1, "atk": 1, "def": 1, "spa": 1, "spd": 1, "spe": 1,
	}
	resp := invoke(t, "calc", mustJSON(t, req))

	var env struct {
		Result struct {
			KO struct {
				Guaranteed           bool        `json:"guaranteed"`
				ChancePercent        float64     `json:"chancePercent"`
				DisplayChancePercent json.Number `json:"displayChancePercent"`
			} `json:"ko"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("レスポンスが読めない: %v\n%s", err, resp)
	}
	if !env.Result.KO.Guaranteed {
		t.Fatalf("確定になる入力のはず: %s", resp)
	}
	if env.Result.KO.ChancePercent != 0 {
		t.Errorf("生値は 0 のまま(ADR-0006): got %v", env.Result.KO.ChancePercent)
	}
	if got := env.Result.KO.DisplayChancePercent.String(); got != "100.0" {
		t.Errorf("確定なのに表示用が %q(100.0 であるべき)", got)
	}
}

// --- 観測%は整数のまま(別概念であることの固定) -----------------------------

// TestObservedPercentInputStaysInteger は、逆算の入力 observations[].percent / percentTenths が
// 表示%(小数)ではなく整数のままであることを示す。percent は整数%、percentTenths は 0.1% 単位の整数
// (P1-12。ADR-0010 §R2)。小数を渡したら入力検証で落ちる(表示%と入力を取り違えたときに黙って通らない)。
func TestObservedPercentInputStaysInteger(t *testing.T) {
	for _, obs := range []map[string]any{
		{"percent": 45.5},
		{"percentTenths": 452.5},
	} {
		req := baseReverse()
		req["observations"] = []any{obs}
		resp := invoke(t, "calcReverse", mustJSON(t, req))

		var env struct {
			Error *struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(resp), &env); err != nil {
			t.Fatalf("レスポンスが読めない: %v", err)
		}
		if env.Error == nil {
			t.Fatalf("小数の観測 %v は受け付けないはず(観測は整数。ADR-0010 §R2): %s", obs, resp)
		}
		if env.Error.Code != "invalid_json" && env.Error.Code != "invalid_observation" {
			t.Errorf("%v: error.code = %q(invalid_json か invalid_observation のはず)", obs, env.Error.Code)
		}
	}
}
