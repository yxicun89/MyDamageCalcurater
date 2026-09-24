package wasmapi_test

// P1-9 受け入れ条件 AC-1/AC-3/AC-4/AC-5/AC-6 の検証(境界そのものの振る舞い)。
// 契約は ADR-0011。ここでは engine の計算値ではなく、封筒・エラー写像・ステートレス性・
// 数値の書式・マスタ非依存を固定する。

import (
	"encoding/json"
	"fmt"
	"go/build"
	"math"
	"strings"
	"testing"

	"example.com/pokecalc/engine/wasmapi"
)

// invoke は fn 名で3つの境界関数を呼び分ける。
func invoke(t *testing.T, fn, requestJSON string) string {
	t.Helper()
	switch fn {
	case "calc":
		return wasmapi.Calc(requestJSON)
	case "calcBulk":
		return wasmapi.CalcBulk(requestJSON)
	case "calcReverse":
		return wasmapi.CalcReverse(requestJSON)
	}
	t.Fatalf("未知の fn: %q", fn)
	return ""
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("テスト入力を JSON にできない: %v", err)
	}
	return string(b)
}

func stats(hp, atk, def, spa, spd, spe int) map[string]any {
	return map[string]any{"hp": hp, "atk": atk, "def": def, "spa": spa, "spd": spd, "spe": spe}
}

func snorlaxSpecies() map[string]any {
	return map[string]any{"key": "snorlax", "types": []any{"normal"}, "baseStats": stats(160, 110, 65, 65, 110, 30)}
}

func blisseySpecies() map[string]any {
	return map[string]any{"key": "blissey", "types": []any{"normal"}, "baseStats": stats(255, 10, 10, 75, 135, 55)}
}

func bodySlam() map[string]any {
	return map[string]any{"id": "bodyslam", "nameJa": "テストわざ1", "type": "normal", "category": "physical", "power": 85, "priority": 0}
}

func attackerIndividual() map[string]any {
	return map[string]any{
		"species": snorlaxSpecies(),
		"level":   50,
		"nature":  map[string]any{"plus": "atk", "minus": "spa"},
		"sp":      stats(0, 32, 0, 0, 0, 0),
		"status":  "none",
	}
}

func defenderIndividual() map[string]any {
	return map[string]any{
		"species": blisseySpecies(),
		"level":   50,
		"nature":  map[string]any{"plus": "", "minus": ""},
		"sp":      stats(0, 0, 0, 0, 0, 0),
		"status":  "none",
	}
}

// typeChartRequestValue はリクエストに載せるタイプ相性表(ADR-0011 §13)。
// 表はベクタ(testdata/vectors.json)の先頭で1度だけ定義したものを使い回す。
// 読めないときは nil を返し、TestVectorsDefineSharedTypeChart が理由付きで落とす。
func typeChartRequestValue() any {
	f, err := loadVectorFile()
	if err != nil {
		return nil
	}
	return f.TypeChartRaw
}

func baseCalc() map[string]any {
	return map[string]any{
		"format":    "single",
		"attacker":  attackerIndividual(),
		"defender":  defenderIndividual(),
		"move":      bodySlam(),
		"field":     map[string]any{"weather": "none", "terrain": "none"},
		"critical":  false,
		"typeChart": typeChartRequestValue(),
	}
}

func baseBulk() map[string]any {
	return map[string]any{
		"format":          "single",
		"attacker":        attackerIndividual(),
		"defenderSpecies": blisseySpecies(),
		"move":            bodySlam(),
		"field":           map[string]any{"weather": "none", "terrain": "none"},
		"critical":        false,
		"typeChart":       typeChartRequestValue(),
	}
}

func baseReverse() map[string]any {
	return map[string]any{
		"format":         "single",
		"side":           "defender",
		"known":          attackerIndividual(),
		"unknownSpecies": blisseySpecies(),
		"move":           bodySlam(),
		"field":          map[string]any{"weather": "none", "terrain": "none"},
		"critical":       false,
		"observations":   []any{map[string]any{"percent": 45}},
		"maxCandidates":  5,
		"typeChart":      typeChartRequestValue(),
	}
}

func sub(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("テスト入力の %q が object ではない", key)
	}
	return v
}

// --- AC-1: 成功封筒の形 -----------------------------------------------------

func TestSuccessEnvelopeShape(t *testing.T) {
	cases := []struct {
		fn      string
		request string
		keys    []string
	}{
		{"calc", mustJSON(t, baseCalc()), []string{
			"rolls", "minDamage", "maxDamage", "minPercent", "maxPercent",
			"defenderHP", "effectiveness", "stab", "category", "ko",
		}},
		{"calcBulk", mustJSON(t, baseBulk()), []string{"defenderSpeciesKey", "rows"}},
		{"calcReverse", mustJSON(t, baseReverse()), []string{"side", "stat", "assumedHpSp", "candidates", "exactCount"}},
	}
	for _, c := range cases {
		t.Run(c.fn, func(t *testing.T) {
			resp := invoke(t, c.fn, c.request)

			if strings.HasSuffix(resp, "\n") {
				t.Error("レスポンスが改行で終わっている(Go/WASM のバイト一致比較が壊れる)")
			}
			var env map[string]json.RawMessage
			if err := json.Unmarshal([]byte(resp), &env); err != nil {
				t.Fatalf("レスポンスが JSON ではない: %v\n%s", err, resp)
			}
			if _, bad := env["error"]; bad {
				t.Fatalf("成功するはずが error 封筒: %s", resp)
			}
			if len(env) != 1 {
				t.Errorf("封筒のキーは result だけであるべき: %v", keysOf(env))
			}
			var result map[string]json.RawMessage
			if err := json.Unmarshal(env["result"], &result); err != nil {
				t.Fatalf("result が object ではない: %v", err)
			}
			if got, want := keysOf(result), c.keys; !sameSet(got, want) {
				t.Errorf("result のキー集合が契約と違う\n got %v\nwant %v", got, want)
			}
		})
	}
}

func keysOf[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	set := make(map[string]bool, len(want))
	for _, w := range want {
		set[w] = true
	}
	for _, g := range got {
		if !set[g] {
			return false
		}
	}
	return true
}

// --- AC-3: エラー封筒とコードの写像 -----------------------------------------

func decodeError(t *testing.T, resp string) errorView {
	t.Helper()
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *errorView      `json:"error"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("レスポンスが JSON ではない: %v\n%s", err, resp)
	}
	if env.Error == nil {
		t.Fatalf("エラーになるはずが成功した: %s", resp)
	}
	if len(env.Result) != 0 {
		t.Errorf("error 封筒に result が同居している: %s", resp)
	}
	if env.Error.Message == "" {
		t.Error("message が空(原因が分からない)")
	}
	if strings.Contains(env.Error.Message, "panic") || strings.Contains(env.Error.Message, "goroutine") {
		t.Errorf("message に Go のランタイム情報が漏れている: %q", env.Error.Message)
	}
	return *env.Error
}

func TestErrorEnvelopeCodes(t *testing.T) {
	// 各ケースは request を組み立てる関数で書く(ベース値を壊さないため)。
	cases := []struct {
		name     string
		fn       string
		request  func(t *testing.T) string
		wantCode string
	}{
		{"壊れた JSON", "calc", func(t *testing.T) string { return "{" }, wasmapi.CodeInvalidJSON},
		{"JSON ではない", "calc", func(t *testing.T) string { return "これは JSON ではない" }, wasmapi.CodeInvalidJSON},
		{"空文字", "calc", func(t *testing.T) string { return "" }, wasmapi.CodeInvalidJSON},
		{"型違い(attacker が数値)", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["attacker"] = 1
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidJSON},

		{"未知フィールド(トップ)", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["attackerr"] = attackerIndividual()
			return mustJSON(t, r)
		}, wasmapi.CodeUnknownField},
		{"未知フィールド(ネスト)", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["hiddenPower"] = 70
			return mustJSON(t, r)
		}, wasmapi.CodeUnknownField},

		{"未知の天候", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "field")["weather"] = "sunny"
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},
		{"未知のフィールド状態", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "field")["terrain"] = "electric_terrain"
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},
		{"未知の状態異常", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["status"] = "burned"
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},
		{"未知のタイプ(種族)", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, sub(t, r, "defender"), "species")["types"] = []any{"normal", "light"}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},
		{"未知のタイプ(技)", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "move")["type"] = "sound"
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},
		{"未知の分類", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "move")["category"] = "phys"
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},
		{"未知の形式", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["format"] = "triple"
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},
		{"未知のテラスタイプ", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["teraType"] = "stellar!"
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},
		{"未知のステータスキー(持ち物の実数値補正)", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["item"] = map[string]any{
				"id": "choiceband", "effect": map[string]any{"statMods": map[string]any{"attack": 6144}},
			}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},
		{"未知のタイプ(特性の防御補正)", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "defender")["ability"] = map[string]any{
				"id": "thickfat", "effect": map[string]any{"defResistType": map[string]any{"flame": 2048}},
			}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},

		{"SP が上限超過", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["sp"] = stats(0, 33, 0, 0, 0, 0)
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"SP 合計が上限超過", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["sp"] = stats(32, 32, 32, 0, 0, 0)
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"ランクが範囲外", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "defender")["ranks"] = map[string]any{"def": 7}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"性格補正が HP", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["nature"] = map[string]any{"plus": "hp", "minus": "spa"}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"タイプが0個", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, sub(t, r, "defender"), "species")["types"] = []any{}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"レベルが 50 以外", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "attacker")["level"] = 100
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},

		// 一括・逆算の種族/個体の検証は engine.Individual.Validate を境界で先に呼んで invalid_input にする
		// (呼ばないと engine 内部の素のエラーが internal に化ける)。
		{"一括: 防御側の種族のタイプが3個", "calcBulk", func(t *testing.T) string {
			r := baseBulk()
			sub(t, r, "defenderSpecies")["types"] = []any{"normal", "fire", "water"}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"逆算: 既知の側の SP が上限超過", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			sub(t, r, "known")["sp"] = stats(0, 33, 0, 0, 0, 0)
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"逆算: 推定側の種族のタイプが0個", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			sub(t, r, "unknownSpecies")["types"] = []any{}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},

		// 空文字の列挙。ADR-0011 §4 の表: move.category と species.types[] は空を許さない。
		{"空の技分類", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, r, "move")["category"] = ""
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},
		{"種族のタイプに空文字", "calc", func(t *testing.T) string {
			r := baseCalc()
			sub(t, sub(t, r, "defender"), "species")["types"] = []any{""}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},

		{"未知のプリセットキー", "calcBulk", func(t *testing.T) string {
			r := baseBulk()
			r["presetKeys"] = []any{"none", "hyper_bulk"}
			return mustJSON(t, r)
		}, wasmapi.CodeUnknownPreset},
		{"プリセットキーの重複", "calcBulk", func(t *testing.T) string {
			r := baseBulk()
			r["presetKeys"] = []any{"none", "none"}
			return mustJSON(t, r)
		}, wasmapi.CodeDuplicatePreset},
		{"プリセット定義が不正", "calcBulk", func(t *testing.T) string {
			r := baseBulk()
			r["presets"] = []any{map[string]any{
				"key": "wall", "label": "壁", "sp": stats(40, 0, 0, 0, 0, 0),
				"nature": map[string]any{"plus": "def", "minus": "atk"},
			}}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidPreset},

		// 件数の上限(issue #110。ADR-0208 §4・ADR-0108。HTTP と同じ invalid_input に写す)。
		{"presets が上限超過(9件)", "calcBulk", func(t *testing.T) string {
			r := baseBulk()
			presets := make([]any, 9)
			for i := range presets {
				presets[i] = map[string]any{"key": fmt.Sprintf("p%d", i)}
			}
			r["presets"] = presets
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"presetKeys が上限超過(9件)", "calcBulk", func(t *testing.T) string {
			r := baseBulk()
			keys := make([]any, 9)
			for i := range keys {
				keys[i] = fmt.Sprintf("p%d", i)
			}
			r["presetKeys"] = keys
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"itemVariants が上限超過(65件)", "calcBulk", func(t *testing.T) string {
			r := baseBulk()
			variants := make([]any, 65)
			for i := range variants {
				variants[i] = map[string]any{"id": fmt.Sprintf("item%d", i)}
			}
			r["itemVariants"] = variants
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},

		{"逆算の対象側が不正", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			r["side"] = "both"
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidReverseSide},
		{"観測が無い", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			delete(r, "observations")
			return mustJSON(t, r)
		}, wasmapi.CodeNoObservation},
		{"観測が空配列", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			r["observations"] = []any{}
			return mustJSON(t, r)
		}, wasmapi.CodeNoObservation},
		{"percent と damage の同時指定", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			r["observations"] = []any{map[string]any{"percent": 45, "damage": 150}}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidObservation},
		{"percent も damage も未指定", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			r["observations"] = []any{map[string]any{"note": "見ていない"}}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidObservation},
		{"percent が範囲外", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			r["observations"] = []any{map[string]any{"percent": 101}}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidObservation},
		// 0.1% 精度の観測(P1-12。ADR-0010 §R2)。3種類のうちちょうど1つだけを指定する。
		{"percent と percentTenths の同時指定", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			r["observations"] = []any{map[string]any{"percent": 45, "percentTenths": 452}}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidObservation},
		{"percentTenths と damage の同時指定", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			r["observations"] = []any{map[string]any{"percentTenths": 452, "damage": 150}}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidObservation},
		{"percentTenths が範囲外", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			r["observations"] = []any{map[string]any{"percentTenths": 1001}}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidObservation},

		// 件数・範囲の上限(issue #110。ADR-0208 §4・ADR-0108。HTTP と同じ invalid_input に写す)。
		{"itemCandidates が上限超過(65件)", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			cands := make([]any, 65)
			for i := range cands {
				cands[i] = map[string]any{"id": fmt.Sprintf("item%d", i)}
			}
			r["itemCandidates"] = cands
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"observations が上限超過(17件)", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			obs := make([]any, 17)
			for i := range obs {
				obs[i] = map[string]any{"percent": 40}
			}
			r["observations"] = obs
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"maxCandidates が負", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			r["maxCandidates"] = -1
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},
		{"maxCandidates が上限超過(129)", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			r["maxCandidates"] = 129
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidInput},

		// タイプ相性表(ADR-0011 §13 / ADR-0013)。境界は既定の表を補わない。
		{"相性表が無い(calc)", "calc", func(t *testing.T) string {
			r := baseCalc()
			delete(r, "typeChart")
			return mustJSON(t, r)
		}, wasmapi.CodeTypeChartMissing},
		{"相性表が null(calc)", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["typeChart"] = nil
			return mustJSON(t, r)
		}, wasmapi.CodeTypeChartMissing},
		{"相性表が空(calc)", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["typeChart"] = map[string]any{}
			return mustJSON(t, r)
		}, wasmapi.CodeTypeChartMissing},
		{"相性表が無い(calcBulk)", "calcBulk", func(t *testing.T) string {
			r := baseBulk()
			delete(r, "typeChart")
			return mustJSON(t, r)
		}, wasmapi.CodeTypeChartMissing},
		{"相性表が無い(calcReverse)", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			delete(r, "typeChart")
			return mustJSON(t, r)
		}, wasmapi.CodeTypeChartMissing},

		{"相性表のコードが不正", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["typeChart"] = map[string]any{
				"types":         []any{"normal", "ghost"},
				"effectiveness": map[string]any{"normal": map[string]any{"ghost": 3}},
			}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidTypeChart},
		{"相性表のタイプが重複", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["typeChart"] = map[string]any{"types": []any{"normal", "normal"}}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidTypeChart},
		{"相性表のキーが types に無い", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["typeChart"] = map[string]any{
				"types":         []any{"normal"},
				"effectiveness": map[string]any{"fire": map[string]any{"normal": 4}},
			}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidTypeChart},

		// 綴りの誤り(18 タイプに無い文字列)は invalid_enum、
		// 綴りは正しいが渡された表に無い ID は unknown_type。
		{"相性表のタイプ名が未知の綴り", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["typeChart"] = map[string]any{"types": []any{"normal", "light"}}
			return mustJSON(t, r)
		}, wasmapi.CodeInvalidEnum},
		{"技のタイプが表に無い", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["typeChart"] = map[string]any{"types": []any{"ghost"}}
			return mustJSON(t, r)
		}, wasmapi.CodeUnknownType},
		{"種族のタイプが表に無い", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["typeChart"] = map[string]any{"types": []any{"normal"}}
			sub(t, sub(t, r, "defender"), "species")["types"] = []any{"ghost"}
			return mustJSON(t, r)
		}, wasmapi.CodeUnknownType},
		{"テラスタイプが表に無い", "calc", func(t *testing.T) string {
			r := baseCalc()
			r["typeChart"] = map[string]any{"types": []any{"normal"}}
			sub(t, r, "attacker")["teraType"] = "ghost"
			return mustJSON(t, r)
		}, wasmapi.CodeUnknownType},
		{"一括: 種族のタイプが表に無い", "calcBulk", func(t *testing.T) string {
			r := baseBulk()
			r["typeChart"] = map[string]any{"types": []any{"ghost"}}
			return mustJSON(t, r)
		}, wasmapi.CodeUnknownType},
		{"逆算: 種族のタイプが表に無い", "calcReverse", func(t *testing.T) string {
			r := baseReverse()
			r["typeChart"] = map[string]any{"types": []any{"ghost"}}
			return mustJSON(t, r)
		}, wasmapi.CodeUnknownType},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := decodeError(t, invoke(t, c.fn, c.request(t)))
			if got.Code != c.wantCode {
				t.Errorf("code: got %q want %q(message=%q)", got.Code, c.wantCode, got.Message)
			}
		})
	}
}

// decodeSuccess は成功封筒であることを確かめ、result の生 JSON を返す。
func decodeSuccess(t *testing.T, resp string) json.RawMessage {
	t.Helper()
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *errorView      `json:"error"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("レスポンスが JSON ではない: %v\n%s", err, resp)
	}
	if env.Error != nil {
		t.Fatalf("成功するはずが error 封筒: code=%q message=%q", env.Error.Code, env.Error.Message)
	}
	return env.Result
}

// TestLimitsAtBoundarySucceed は件数・範囲がちょうど上限の calcBulk / calcReverse が
// 成功し、上限内の最大の件数を返すこと(issue #110。ADR-0208 §1・§4・ADR-0108)。
// TestErrorEnvelopeCodes 側は「上限+1 は失敗する」側だけなので、成功側をここで固定する。
func TestLimitsAtBoundarySucceed(t *testing.T) {
	t.Run("calcBulk", func(t *testing.T) {
		r := baseBulk()
		r["presetKeys"] = []any{"none", "hp", "hb_boost", "hb", "hb_full", "hd_boost", "hd", "hd_full"}
		variants := make([]any, 64)
		for i := range variants {
			variants[i] = map[string]any{"id": fmt.Sprintf("item%d", i)}
		}
		r["itemVariants"] = variants

		var result struct {
			Rows []json.RawMessage `json:"rows"`
		}
		if err := json.Unmarshal(decodeSuccess(t, invoke(t, "calcBulk", mustJSON(t, r))), &result); err != nil {
			t.Fatalf("result が契約と違う: %v", err)
		}
		if want := 8 * 64; len(result.Rows) != want {
			t.Errorf("rows の件数 = %d, want %d", len(result.Rows), want)
		}
	})

	t.Run("calcReverse", func(t *testing.T) {
		r := baseReverse()
		cands := make([]any, 64)
		for i := range cands {
			cands[i] = map[string]any{"id": fmt.Sprintf("item%d", i)}
		}
		r["itemCandidates"] = cands
		obs := make([]any, 16)
		for i := range obs {
			obs[i] = map[string]any{"percent": 40}
		}
		r["observations"] = obs
		r["maxCandidates"] = 128

		var result struct {
			Candidates []json.RawMessage `json:"candidates"`
		}
		if err := json.Unmarshal(decodeSuccess(t, invoke(t, "calcReverse", mustJSON(t, r))), &result); err != nil {
			t.Fatalf("result が契約と違う: %v", err)
		}
		// 性格クラス2 × 持ち物候補64 = 128 件がちょうど maxCandidates(128)と一致する。
		if want := 2 * 64; len(result.Candidates) != want {
			t.Errorf("candidates の件数 = %d, want %d", len(result.Candidates), want)
		}
	})
}

// TestEmptyEnumMeansDefault は、空文字を既定値として許す列挙(ADR-0011 §4 の表)が
// 拒否されず、move.type の "" は TypeNone として成功することを確かめる。
func TestEmptyEnumMeansDefault(t *testing.T) {
	r := baseCalc()
	sub(t, r, "move")["type"] = ""
	sub(t, r, "attacker")["teraType"] = ""
	sub(t, r, "field")["weather"] = ""
	sub(t, r, "field")["terrain"] = ""
	sub(t, r, "attacker")["status"] = ""
	r["format"] = ""

	var got calcResultView
	decodeEnvelope(t, wasmapi.Calc(mustJSON(t, r)), &got)
	if got.Category != "physical" {
		t.Errorf("category: got %q want physical", got.Category)
	}
}

// TestTypeChartIsPassedThroughToEngine は、境界が相性表を解釈せず engine に渡すことを
// 確かめる(ADR-0011 §13)。実在の相性と違う表を渡しても、その表どおりの結果になる。
// 等倍の省略も engine と同じ規則で扱われる。
func TestTypeChartIsPassedThroughToEngine(t *testing.T) {
	// 攻撃側・防御側・技のすべてが normal(baseCalc)。
	tests := []struct {
		name  string
		chart map[string]any
		want  float64
	}{
		{"表がいまひとつと言えば 0.5", map[string]any{
			"types":         []any{"normal"},
			"effectiveness": map[string]any{"normal": map[string]any{"normal": 1}},
		}, 0.5},
		{"表が抜群と言えば 2", map[string]any{
			"types":         []any{"normal"},
			"effectiveness": map[string]any{"normal": map[string]any{"normal": 4}},
		}, 2.0},
		{"省略は等倍", map[string]any{"types": []any{"normal"}}, 1.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := baseCalc()
			r["typeChart"] = tt.chart

			var got calcResultView
			decodeEnvelope(t, wasmapi.Calc(mustJSON(t, r)), &got)
			if got.Effectiveness != tt.want {
				t.Errorf("effectiveness = %v, want %v(渡した表が使われていない)", got.Effectiveness, tt.want)
			}
		})
	}
}

// TestTrailingDataIsInvalidJSON は、有効な JSON の後ろに余計なデータがあれば
// invalid_json にすることを確かめる(先頭の値だけを黙って採用しない)。
func TestTrailingDataIsInvalidJSON(t *testing.T) {
	cases := map[string]string{
		"calc":        mustJSON(t, baseCalc()),
		"calcBulk":    mustJSON(t, baseBulk()),
		"calcReverse": mustJSON(t, baseReverse()),
	}
	for fn, req := range cases {
		for _, suffix := range []string{"ゴミ", "{}", " x", "\n[]"} {
			t.Run(fn+"/"+suffix, func(t *testing.T) {
				got := decodeError(t, invoke(t, fn, req+suffix))
				if got.Code != wasmapi.CodeInvalidJSON {
					t.Errorf("code: got %q want %q(message=%q)", got.Code, wasmapi.CodeInvalidJSON, got.Message)
				}
			})
		}
	}
}

// TestHostileInputNeverPanics は壊れた入力でも panic を JS へ漏らさないことを確かめる。
// go test は panic をテスト失敗として捉えるので、ここでは recover を挟まず素で呼ぶ。
func TestHostileInputNeverPanics(t *testing.T) {
	inputs := []string{
		"", " ", "null", "[]", "0", `"文字列"`, "{", "}", `{"`, `{"attacker":}`,
		`{"attacker":null,"defender":null,"move":null}`,
		`{"attacker":{"species":{"baseStats":{"hp":1e400}}}}`,
		`{"move":{"power":99999999999999999999}}`,
		`{"attacker":{"sp":{"hp":-2147483648}}}`,
		`{"observations":[{"percent":-1}]}`,
		strings.Repeat(`{"a":`, 200) + "1" + strings.Repeat("}", 200),
		"\x00\x01\x02",
	}
	for _, fn := range []string{"calc", "calcBulk", "calcReverse"} {
		for _, in := range inputs {
			resp := invoke(t, fn, in)
			var env struct {
				Result json.RawMessage `json:"result"`
				Error  *errorView      `json:"error"`
			}
			if err := json.Unmarshal([]byte(resp), &env); err != nil {
				t.Fatalf("%s(%q): レスポンスが JSON ではない: %v\n%s", fn, truncate(in), err, resp)
			}
			if env.Error == nil {
				t.Errorf("%s(%q): 不正入力が成功している: %s", fn, truncate(in), truncate(resp))
				continue
			}
			if env.Error.Code == "" {
				t.Errorf("%s(%q): code が空", fn, truncate(in))
			}
		}
	}
}

func truncate(s string) string {
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}

// --- AC-4: ステートレス・決定的 ---------------------------------------------

func TestCallsAreStatelessAndDeterministic(t *testing.T) {
	calcReq := mustJSON(t, baseCalc())
	bulkReq := mustJSON(t, baseBulk())
	revReq := mustJSON(t, baseReverse())
	badReq := "{"

	first := map[string]string{
		"calc":        invoke(t, "calc", calcReq),
		"calcBulk":    invoke(t, "calcBulk", bulkReq),
		"calcReverse": invoke(t, "calcReverse", revReq),
	}

	// 他の関数・不正入力・別の入力を挟んでも、同じ入力は常に同じバイト列を返す。
	for i := 0; i < 3; i++ {
		invoke(t, "calc", badReq)
		invoke(t, "calcReverse", revReq)
		invoke(t, "calcBulk", badReq)

		for fn, req := range map[string]string{"calc": calcReq, "calcBulk": bulkReq, "calcReverse": revReq} {
			if got := invoke(t, fn, req); got != first[fn] {
				t.Fatalf("%s の %d 回目が初回と違う(状態を持ち越している)\n初回 %s\n今回 %s",
					fn, i+2, truncate(first[fn]), truncate(got))
			}
		}
	}
}

// --- AC-5: 整数は整数のまま(float 化しない)---------------------------------

func TestIntegerFieldsHaveNoFractionOrExponent(t *testing.T) {
	// minPercent / maxPercent / displayChancePercent は P1-11 で「小数第1位の表示%」になったので
	// ここでは見ない。書式は display_percent_test.go の TestDisplayPercentFieldsAreOneDecimal が
	// より厳しく固定する(小数点が1つ・小数第1位がちょうど1桁・指数表記なし)。
	intKeys := map[string]bool{
		"rolls": true, "minDamage": true, "maxDamage": true,
		"defenderHP": true, "hits": true, "exactCount": true,
		// 逆算候補(P1-12): SP 範囲の両端・SP 数・距離・説明できるロールの延べ数・H の仮定。
		"min": true, "max": true, "spCount": true, "mismatch": true, "support": true, "assumedHpSp": true,
		"hp": true, "atk": true, "def": true, "spa": true, "spd": true, "spe": true,
		"power": true, "priority": true, "level": true,
	}
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
			walkNumbers(t, v, "", intKeys)
		})
	}
}

func walkNumbers(t *testing.T, v any, key string, intKeys map[string]bool) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for k, sv := range x {
			walkNumbers(t, sv, k, intKeys)
		}
	case []any:
		for _, sv := range x {
			walkNumbers(t, sv, key, intKeys)
		}
	case json.Number:
		s := x.String()
		if intKeys[key] && (strings.ContainsAny(s, ".eE")) {
			t.Errorf("%q は整数で出すべきだが %q になっている", key, s)
		}
		if f, err := x.Float64(); err == nil && (math.IsNaN(f) || math.IsInf(f, 0)) {
			t.Errorf("%q が NaN/Inf(JSON として不正): %q", key, s)
		}
	}
}

// --- AC-6: マスタを持ち込まない ---------------------------------------------

// TestOpaqueMasterIDsArePassedThrough は、境界が持ち物・技・特性の ID を解釈せず、
// 解決済みの効果定義だけで計算することを確かめる(マスタは Web 側が解決する)。
func TestOpaqueMasterIDsArePassedThrough(t *testing.T) {
	r := baseCalc()
	sub(t, r, "move")["id"] = "この技IDはマスタに存在しない"
	sub(t, r, "attacker")["item"] = map[string]any{
		"id": "存在しない持ち物", "nameJa": "?", "effect": map[string]any{"damageMod": 5324},
	}
	sub(t, r, "attacker")["ability"] = map[string]any{
		"id": "存在しない特性", "effect": map[string]any{"stabMod": 8192},
	}

	var got calcResultView
	decodeEnvelope(t, wasmapi.Calc(mustJSON(t, r)), &got)
	if got.MaxDamage <= 0 {
		t.Errorf("未知の ID でも効果定義どおりに計算されるべき: %+v", got)
	}
}

// TestBoundaryPackageImportsAreMinimal は境界パッケージが engine と標準ライブラリしか
// 使っていないこと(= WASM 固有の syscall/js を持ち込んでいないこと)を確かめる。
func TestBoundaryPackageImportsAreMinimal(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("パッケージを読めない: %v", err)
	}
	for _, imp := range pkg.Imports {
		switch {
		case imp == "example.com/pokecalc/engine":
		case !strings.Contains(strings.SplitN(imp, "/", 2)[0], "."):
			// 標準ライブラリ(最初の要素にドットが無い)。
		default:
			t.Errorf("境界パッケージが外部依存を持っている: %q", imp)
		}
		if imp == "syscall/js" {
			t.Errorf("syscall/js は engine/cmd/wasm に閉じ込める(このパッケージはネイティブでもテストできること)")
		}
	}
}
