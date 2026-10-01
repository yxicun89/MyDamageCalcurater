package wasmapi_test

// issue #274/#272 の残り / ADR-0216: calcBulk の defenderOverride(ranks・status)。
//
//   - 送らない・空オブジェクト・ゼロ値(ranks 0・status none / "")は、応答がバイト単位で従来と同じ
//   - ranks は全行の防御側に一律で当たる(物理は def、特殊は spd だけが効く)。行の形(フィールド)は変えない
//   - status は全行に当たるが、今の式では防御側の状態はダメージを変えない(ADR-0216 §3)ので応答はバイト同一
//   - ranks の -6..+6 外は invalid_input、未知の status は invalid_enum(individual.status と同じ語彙)、
//     契約に無いキー(abilityId・ranks.hp 等)は unknown_field。HTTP(calc-svc)と同じ code(parity)

import (
	"strings"
	"testing"

	"example.com/pokecalc/engine/wasmapi"
)

func bulkWithOverride(t *testing.T, ov any) string {
	t.Helper()
	req := baseBulk()
	req["defenderOverride"] = ov
	return wasmapi.CalcBulk(mustJSON(t, req))
}

func TestWasmBulkDefenderOverrideZeroValueIsByteIdentical(t *testing.T) {
	legacy := wasmapi.CalcBulk(mustJSON(t, baseBulk()))
	decodeSuccess(t, legacy)
	for name, ov := range map[string]any{
		"空オブジェクト":     map[string]any{},
		"ranks 空":     map[string]any{"ranks": map[string]any{}},
		"ranks 全0":    map[string]any{"ranks": map[string]any{"atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": 0}},
		"status none": map[string]any{"status": "none"},
		"status 空文字":  map[string]any{"status": ""},
	} {
		if got := bulkWithOverride(t, ov); got != legacy {
			t.Errorf("%s: 応答が従来と違う\n got=%s\nwant=%s", name, truncate(got), truncate(legacy))
		}
	}
}

func TestWasmBulkDefenderOverrideRanks(t *testing.T) {
	legacy := bulkRowsOf(t, wasmapi.CalcBulk(mustJSON(t, baseBulk()))) // のしかかり(物理)
	maxOf := func(r abilityRowView) int { return r.Result.Rolls[len(r.Result.Rolls)-1] }
	tests := []struct {
		name  string
		ranks map[string]any
		dir   int
	}{
		{"防御+2", map[string]any{"def": 2}, -1},
		{"防御+6(境界)", map[string]any{"def": 6}, -1},
		{"防御-6(境界)", map[string]any{"def": -6}, +1},
		{"特防+6は物理に無関係", map[string]any{"spd": 6}, 0},
		{"攻撃・特攻・素早さは無関係", map[string]any{"atk": 6, "spa": -6, "spe": 6}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := bulkRowsOf(t, bulkWithOverride(t, map[string]any{"ranks": tt.ranks}))
			if len(rows) != len(legacy) {
				t.Fatalf("行数=%d want %d(上書きで行数は変わらない)", len(rows), len(legacy))
			}
			for i := range rows {
				if rows[i].Preset != legacy[i].Preset {
					t.Errorf("rows[%d].preset=%q want %q", i, rows[i].Preset, legacy[i].Preset)
				}
				x, y := maxOf(rows[i]), maxOf(legacy[i])
				if ok := (tt.dir > 0 && x > y) || (tt.dir < 0 && x < y) || (tt.dir == 0 && x == y); !ok {
					t.Errorf("rows[%d](%s) 最大ダメージ %d vs 従来 %d、期待 dir=%+d", i, rows[i].Preset, x, y, tt.dir)
				}
			}
		})
	}
}

// 防御側の状態異常は今の式に効かず、行にも出さない(BulkDefender の形を変えない)ので、応答はバイト同一。
func TestWasmBulkDefenderOverrideStatusDoesNotChangeResponse(t *testing.T) {
	legacy := wasmapi.CalcBulk(mustJSON(t, baseBulk()))
	for _, st := range []string{"burn", "paralysis", "poison", "badly_poison", "sleep", "freeze"} {
		if got := bulkWithOverride(t, map[string]any{"status": st}); got != legacy {
			t.Errorf("status=%s: 応答が従来と違う(防御側の状態は式に効かない)\n got=%s", st, truncate(got))
		}
	}
}

// 上書きで行の形(キー)は増えない。ランクを付けても応答に ranks / status を出さない。
func TestWasmBulkDefenderOverrideDoesNotEchoFields(t *testing.T) {
	resp := bulkWithOverride(t, map[string]any{"ranks": map[string]any{"def": 1}, "status": "burn"})
	decodeSuccess(t, resp)
	for _, key := range []string{`"ranks"`, `"status"`, `"defenderOverride"`} {
		if strings.Contains(resp, key) {
			t.Errorf("応答に %s が出た(行の形は変えない): %s", key, truncate(resp))
		}
	}
}

func TestWasmBulkDefenderOverrideErrors(t *testing.T) {
	tests := []struct {
		name string
		ov   any
		want string
	}{
		{"防御+7", map[string]any{"ranks": map[string]any{"def": 7}}, wasmapi.CodeInvalidInput},
		{"特防-7", map[string]any{"ranks": map[string]any{"spd": -7}}, wasmapi.CodeInvalidInput},
		{"素早さ+7(使わない側でも拒否)", map[string]any{"ranks": map[string]any{"spe": 7}}, wasmapi.CodeInvalidInput},
		{"未知の状態異常", map[string]any{"status": "confusion"}, wasmapi.CodeInvalidEnum},
		{"大文字の状態異常", map[string]any{"status": "BURN"}, wasmapi.CodeInvalidEnum},
		{"ランクが小数", map[string]any{"ranks": map[string]any{"def": 1.5}}, wasmapi.CodeInvalidJSON},
		{"ランクが文字列", map[string]any{"ranks": map[string]any{"def": "+1"}}, wasmapi.CodeInvalidJSON},
		{"ranks.hp は契約に無い", map[string]any{"ranks": map[string]any{"hp": 1}}, wasmapi.CodeUnknownField},
		{"abilityId は WASM の上書きに無い(defenderAbilities を使う)", map[string]any{"abilityId": "x"}, wasmapi.CodeUnknownField},
		{"defenderOverride が配列", []any{}, wasmapi.CodeInvalidJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeError(t, bulkWithOverride(t, tt.ov)); got.Code != tt.want {
				t.Errorf("code=%q want %q (%s)", got.Code, tt.want, got.Message)
			}
		})
	}
}

// 件数上限(ADR-0208・ADR-0108)の検査は上書きの検証より先(HTTP と同じ順。parity)。
func TestWasmBulkLimitsCheckedBeforeDefenderOverride(t *testing.T) {
	req := baseBulk()
	keys := make([]any, 0, 9)
	for range 9 {
		keys = append(keys, "none")
	}
	req["presetKeys"] = keys
	req["defenderOverride"] = map[string]any{"status": "confusion"}
	got := decodeError(t, wasmapi.CalcBulk(mustJSON(t, req)))
	if got.Code != wasmapi.CodeInvalidInput || !strings.Contains(got.Message, "presetKeys") {
		t.Errorf("code=%q message=%q want 件数上限の invalid_input が先", got.Code, got.Message)
	}
}
