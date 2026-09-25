package engine

// 防御側プリセットの JSON カタログ(engine/presets/defender.json。ADR-0009 2026-09-25 追記・ADR-0114)の読み込みを固定する。
// カタログの値そのものの期待値は bulk_test.go の TestDefenderPresetCatalogDefinitions が正。

import (
	"errors"
	"strings"
	"testing"
)

// 埋め込んだ JSON のキーは Go の PresetKey 定数と同じ順で並ぶ(定数は OpenAPI enum との対応のために残している)。
func TestDefenderPresetJSONKeysMatchConstants(t *testing.T) {
	want := []PresetKey{PresetNone, PresetHP, PresetHBBoost, PresetHB, PresetHBFull, PresetHDBoost, PresetHD, PresetHDFull}
	c, err := parseDefenderPresetCatalog(defenderPresetJSON)
	if err != nil {
		t.Fatalf("embedded catalog rejected: %v", err)
	}
	got := presetKeys(c)
	if len(got) != len(want) {
		t.Fatalf("keys=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("keys[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestParseDefenderPresetCatalogRejectsBrokenData(t *testing.T) {
	valid := `{"schemaVersion":1,"presets":[
{"key":"none","label":"無振り","sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"nature":{"plus":"","minus":""},"applies":""},
{"key":"hb_full","label":"HB特化","sp":{"hp":32,"atk":0,"def":32,"spa":0,"spd":0,"spe":0},"nature":{"plus":"def","minus":"atk"},"applies":"physical"}]}`
	c, err := parseDefenderPresetCatalog([]byte(valid))
	if err != nil {
		t.Fatalf("valid catalog rejected: %v", err)
	}
	if len(c) != 2 || c[1].SP != (Stats{HP: 32, Def: 32}) || c[1].Nature != (Nature{Plus: StatDef, Minus: StatAtk}) || c[1].Applies != CategoryPhysical {
		t.Fatalf("parsed=%+v", c)
	}
	tests := []struct {
		name     string
		old, new string
	}{
		{"unknown field", `"schemaVersion":1,`, `"schemaVersion":1,"extra":true,`},
		{"unknown preset field", `"label":"HB特化",`, `"label":"HB特化","extra":1,`},
		{"unknown sp field", `"spe":0},"nature":{"plus":"def"`, `"spe":0,"hpp":1},"nature":{"plus":"def"`},
		{"missing sp field", `"atk":0,"def":32,`, `"def":32,`},
		{"missing nature", `,"nature":{"plus":"def","minus":"atk"}`, ``},
		{"missing applies", `,"applies":"physical"`, ``},
		{"missing label", `"label":"HB特化",`, ``},
		{"schema version", `"schemaVersion":1`, `"schemaVersion":2`},
		{"duplicate key", `"key":"hb_full"`, `"key":"none"`},
		{"empty key", `"key":"hb_full"`, `"key":""`},
		{"sp over max", `"def":32`, `"def":33`},
		{"sp negative", `"hp":0`, `"hp":-1`},
		{"sp total over max", `"spa":0,"spd":0,"spe":0},"nature":{"plus":"def"`, `"spa":32,"spd":0,"spe":0},"nature":{"plus":"def"`},
		{"nature hp", `"plus":"def"`, `"plus":"hp"`},
		{"nature unknown stat", `"plus":"def"`, `"plus":"foo"`},
		{"nature half empty", `"minus":"atk"`, `"minus":""`},
		{"nature same stat", `"minus":"atk"`, `"minus":"def"`},
		{"unknown applies", `"applies":"physical"`, `"applies":"other"`},
		{"trailing data", `"applies":"physical"}]}`, `"applies":"physical"}]}{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(valid, tt.old) {
				t.Fatalf("test setup: %q not in catalog", tt.old)
			}
			broken := strings.Replace(valid, tt.old, tt.new, 1)
			if _, err := parseDefenderPresetCatalog([]byte(broken)); !errors.Is(err, ErrInvalidPreset) {
				t.Fatalf("err=%v want ErrInvalidPreset", err)
			}
		})
	}

	empty := `{"schemaVersion":1,"presets":[]}`
	if _, err := parseDefenderPresetCatalog([]byte(empty)); !errors.Is(err, ErrInvalidPreset) {
		t.Fatalf("empty presets: err=%v want ErrInvalidPreset", err)
	}
	var many strings.Builder
	many.WriteString(`{"schemaVersion":1,"presets":[`)
	for i := 0; i <= MaxBulkPresets; i++ {
		if i > 0 {
			many.WriteString(",")
		}
		many.WriteString(`{"key":"k` + string(rune('a'+i)) + `","label":"","sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"nature":{"plus":"","minus":""},"applies":""}`)
	}
	many.WriteString(`]}`)
	if _, err := parseDefenderPresetCatalog([]byte(many.String())); !errors.Is(err, ErrInvalidPreset) {
		t.Fatalf("too many presets: err=%v want ErrInvalidPreset", err)
	}
}
