package engine

import (
	"errors"
	"strings"
	"testing"
)

// TestAttackerPresetCatalogDefinitions は攻撃側プリセットの正(engine/presets/attacker.json)の
// キー・順序・SP・性格規則を固定する(ADR-0114。issue #71)。Web・iOS は同じ JSON と契約テストで突き合わせる。
func TestAttackerPresetCatalogDefinitions(t *testing.T) {
	want := []AttackerPreset{
		{Key: "none", RelevantSP: 0, Nature: AttackerNatureNeutral},
		{Key: "x_full", RelevantSP: MaxSPPerStat, Nature: AttackerNatureBoost},
		{Key: "x", RelevantSP: MaxSPPerStat, Nature: AttackerNatureNeutral},
	}
	got := AttackerPresetCatalog()
	if len(got) != len(want) {
		t.Fatalf("catalog=%+v want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("catalog[%d]=%+v want %+v", i, got[i], want[i])
		}
	}
	if d := DefaultAttackerPreset(); d != "none" {
		t.Errorf("default=%q want none", d)
	}
}

func TestAttackerPresetCatalogReturnsCopy(t *testing.T) {
	c := AttackerPresetCatalog()
	c[0].Key = "changed"
	if AttackerPresetCatalog()[0].Key != "none" {
		t.Fatal("caller mutation leaked into the catalog")
	}
}

func TestResolveAttackerPreset(t *testing.T) {
	tests := []struct {
		key      AttackerPresetKey
		category MoveCategory
		sp       Stats
		nature   Nature
	}{
		{"none", CategoryPhysical, Stats{}, NatureNeutral},
		{"none", CategorySpecial, Stats{}, NatureNeutral},
		{"x_full", CategoryPhysical, Stats{Atk: MaxSPPerStat}, Nature{Plus: StatAtk, Minus: StatSpA}},
		{"x_full", CategorySpecial, Stats{SpA: MaxSPPerStat}, Nature{Plus: StatSpA, Minus: StatAtk}},
		// 変化技は物理と同じ atk 扱い(ADR-0300 §5・ADR-0500 §6)
		{"x_full", CategoryStatus, Stats{Atk: MaxSPPerStat}, Nature{Plus: StatAtk, Minus: StatSpA}},
		{"x", CategoryPhysical, Stats{Atk: MaxSPPerStat}, NatureNeutral},
		{"x", CategorySpecial, Stats{SpA: MaxSPPerStat}, NatureNeutral},
	}
	for _, tt := range tests {
		t.Run(string(tt.key)+"/"+string(tt.category), func(t *testing.T) {
			sp, nature, err := ResolveAttackerPreset(tt.key, tt.category)
			if err != nil {
				t.Fatal(err)
			}
			if sp != tt.sp || nature != tt.nature {
				t.Fatalf("got sp=%+v nature=%+v want sp=%+v nature=%+v", sp, nature, tt.sp, tt.nature)
			}
		})
	}
}

func TestResolveAttackerPresetRejectsUnknown(t *testing.T) {
	if _, _, err := ResolveAttackerPreset("a_full", CategoryPhysical); !errors.Is(err, ErrUnknownAttackerPreset) {
		t.Errorf("unknown key err=%v want ErrUnknownAttackerPreset", err)
	}
	if _, _, err := ResolveAttackerPreset("none", "bogus"); !errors.Is(err, ErrInvalidAttackerPreset) {
		t.Errorf("unknown category err=%v want ErrInvalidAttackerPreset", err)
	}
}

// 解決結果は engine の個体として有効(SP 範囲・合計、HP に性格補正なし)。
func TestResolvedAttackerPresetIsValidIndividual(t *testing.T) {
	for _, p := range AttackerPresetCatalog() {
		for _, c := range []MoveCategory{CategoryPhysical, CategorySpecial, CategoryStatus} {
			sp, nature, err := ResolveAttackerPreset(p.Key, c)
			if err != nil {
				t.Fatal(err)
			}
			in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
			in.Attacker.SP, in.Attacker.Nature = sp, nature
			if err := in.Attacker.Validate(); err != nil {
				t.Errorf("%s/%s: %v", p.Key, c, err)
			}
		}
	}
}

// TestParseAttackerPresetCatalogRejectsBrokenData は壊れたカタログを読み込み時に拒否する(黙ってゼロ値にしない)。
func TestParseAttackerPresetCatalogRejectsBrokenData(t *testing.T) {
	valid := `{"schemaVersion":1,"default":"none",
"relevantStat":{"physical":"atk","special":"spa","status":"atk"},
"boostMinus":{"atk":"spa","spa":"atk"},
"presets":[{"key":"none","relevantSp":0,"nature":"neutral"},{"key":"x_full","relevantSp":32,"nature":"boost"}]}`
	if _, err := parseAttackerPresetCatalog([]byte(valid)); err != nil {
		t.Fatalf("valid catalog rejected: %v", err)
	}
	tests := []struct {
		name     string
		old, new string
	}{
		{"unknown field", `"schemaVersion":1,`, `"schemaVersion":1,"extra":true,`},
		{"schema version", `"schemaVersion":1`, `"schemaVersion":2`},
		{"default not in presets", `"default":"none"`, `"default":"x"`},
		{"duplicate key", `"key":"x_full"`, `"key":"none"`},
		{"empty key", `"key":"x_full"`, `"key":""`},
		{"sp over max", `"relevantSp":32`, `"relevantSp":33`},
		{"sp negative", `"relevantSp":0`, `"relevantSp":-1`},
		{"unknown nature rule", `"nature":"boost"`, `"nature":"minus"`},
		{"missing category", `,"status":"atk"`, ``},
		{"unknown category", `"status":"atk"`, `"other":"atk"`},
		{"relevant hp", `"physical":"atk"`, `"physical":"hp"`},
		{"unknown stat", `"physical":"atk"`, `"physical":"foo"`},
		{"boost minus missing", `,"spa":"atk"}`, `}`},
		{"boost minus same stat", `"atk":"spa",`, `"atk":"atk",`},
		{"boost minus hp", `"atk":"spa",`, `"atk":"hp",`},
		{"boost minus unused key", `"spa":"atk"}`, `"spa":"atk","def":"atk"}`},
		{"no presets", `"presets":[{"key":"none","relevantSp":0,"nature":"neutral"},{"key":"x_full","relevantSp":32,"nature":"boost"}]`, `"presets":[]`},
		{"trailing data", `"nature":"boost"}]}`, `"nature":"boost"}]}{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(valid, tt.old) {
				t.Fatalf("test setup: %q not in catalog", tt.old)
			}
			broken := strings.Replace(valid, tt.old, tt.new, 1)
			if _, err := parseAttackerPresetCatalog([]byte(broken)); !errors.Is(err, ErrInvalidAttackerPreset) {
				t.Fatalf("err=%v want ErrInvalidAttackerPreset", err)
			}
		})
	}
}
