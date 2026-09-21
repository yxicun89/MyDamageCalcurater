package master

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"example.com/pokecalc/services/balance/internal/balance"
)

// 正はダメージ計算レーンの P1-13 のデータ(ADR-0013)。balance の複製はバイト一致でなければならない。
const sharedTypeChartPath = "../../../../testdata/golden/typechart.json"

func TestEmbeddedTypeChartMatchesSharedData(t *testing.T) {
	t.Parallel()

	shared, err := os.ReadFile(sharedTypeChartPath)
	if err != nil {
		t.Fatalf("read shared type chart: %v", err)
	}
	if !bytes.Equal(shared, embeddedTypeChartJSON) {
		t.Fatalf("internal/master/data/typechart.json differs from %s; run make balance-sync-typechart", sharedTypeChartPath)
	}
}

func TestEmbeddedTypeChartLoads(t *testing.T) {
	t.Parallel()

	chart, err := EmbeddedTypeChart()
	if err != nil {
		t.Fatalf("EmbeddedTypeChart() error = %v", err)
	}
	tests := []struct {
		attack, defense balance.TypeID
		want            balance.Multiplier
	}{
		{balance.TypeFire, balance.TypeGrass, balance.MultiplierDouble},
		{balance.TypeFire, balance.TypeWater, balance.MultiplierHalf},
		{balance.TypeNormal, balance.TypeGhost, balance.MultiplierZero},
		{balance.TypeElectric, balance.TypeGround, balance.MultiplierZero},
		{balance.TypeDragon, balance.TypeFairy, balance.MultiplierZero},
		{balance.TypeNormal, balance.TypePsychic, balance.MultiplierNormal},
	}
	for _, tt := range tests {
		got, err := chart.Matchup(tt.attack, tt.defense)
		if err != nil || got != tt.want {
			t.Errorf("Matchup(%s, %s) = %d, %v; want %d", tt.attack, tt.defense, got, err, tt.want)
		}
	}
}

// 移行の確認: データの表は TB0 の TemporaryTypeChart と 18×18 全件で一致する。
func TestEmbeddedTypeChartMatchesTemporaryChart(t *testing.T) {
	t.Parallel()

	chart, err := EmbeddedTypeChart()
	if err != nil {
		t.Fatalf("EmbeddedTypeChart() error = %v", err)
	}
	temporary := NewTemporaryTypeChart()
	for _, attack := range balance.AllTypes() {
		for _, defense := range balance.AllTypes() {
			got, err := chart.Matchup(attack, defense)
			want, _ := temporary.Matchup(attack, defense)
			if err != nil || got != want {
				t.Errorf("Matchup(%s, %s) = %d, %v; temporary = %d", attack, defense, got, err, want)
			}
		}
	}
}

func TestTypeChartMatchupRejectsUnknownTypes(t *testing.T) {
	t.Parallel()

	chart, err := EmbeddedTypeChart()
	if err != nil {
		t.Fatalf("EmbeddedTypeChart() error = %v", err)
	}
	if _, err := chart.Matchup("stellar", balance.TypeFire); !errors.Is(err, balance.ErrInvalidType) {
		t.Errorf("attack stellar: err = %v, want ErrInvalidType", err)
	}
	if _, err := chart.Matchup(balance.TypeFire, "stellar"); !errors.Is(err, balance.ErrInvalidType) {
		t.Errorf("defense stellar: err = %v, want ErrInvalidType", err)
	}
}

func TestLoadTypeChartOmittedPairIsNeutral(t *testing.T) {
	t.Parallel()

	chart, err := LoadTypeChart(strings.NewReader(minimalChart(`"fire":{"grass":4}`)))
	if err != nil {
		t.Fatalf("LoadTypeChart() error = %v", err)
	}
	if got, _ := chart.Matchup(balance.TypeFire, balance.TypeGrass); got != balance.MultiplierDouble {
		t.Errorf("fire→grass = %d, want double", got)
	}
	if got, _ := chart.Matchup(balance.TypeWater, balance.TypeFire); got != balance.MultiplierNormal {
		t.Errorf("omitted water→fire = %d, want neutral", got)
	}
}

func TestLoadTypeChartRejectsInvalidData(t *testing.T) {
	t.Parallel()

	allTypes := `["normal","fire","water","electric","grass","ice","fighting","poison","ground","flying","psychic","bug","rock","ghost","dragon","dark","steel","fairy"]`
	tests := []struct {
		name string
		json string
	}{
		{name: "empty", json: ``},
		{name: "broken JSON", json: `{"schemaVersion":1,`},
		{name: "trailing JSON", json: minimalChart(``) + ` {}`},
		{name: "schema version 2", json: strings.Replace(minimalChart(``), `"schemaVersion":1`, `"schemaVersion":2`, 1)},
		{name: "unknown field", json: strings.Replace(minimalChart(``), `"schemaVersion":1`, `"schemaVersion":1,"extra":true`, 1)},
		{name: "missing a type", json: `{"schemaVersion":1,"types":["fire"],"effectiveness":{}}`},
		{name: "extra type", json: `{"schemaVersion":1,"types":` + strings.Replace(allTypes, `"fairy"`, `"fairy","stellar"`, 1) + `,"effectiveness":{}}`},
		{name: "duplicate type", json: `{"schemaVersion":1,"types":` + strings.Replace(allTypes, `"fairy"`, `"fire"`, 1) + `,"effectiveness":{}}`},
		{name: "unknown attack key", json: minimalChart(`"stellar":{"fire":2}`)},
		{name: "unknown defense key", json: minimalChart(`"fire":{"stellar":2}`)},
		{name: "invalid code 3", json: minimalChart(`"fire":{"grass":3}`)},
		{name: "invalid code -1", json: minimalChart(`"fire":{"grass":-1}`)},
		{name: "invalid code 8", json: minimalChart(`"fire":{"grass":8}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			chart, err := LoadTypeChart(strings.NewReader(tt.json))
			if !errors.Is(err, ErrInvalidTypeChart) {
				t.Fatalf("err = %v, want ErrInvalidTypeChart", err)
			}
			if chart != nil {
				t.Errorf("chart = %v, want nil on error", chart)
			}
		})
	}
}

// minimalChart は 18 タイプを持ち、effectiveness に entries だけを載せた表を返す(その他の項目は typechart.json と同じ形)。
func minimalChart(entries string) string {
	return `{"schemaVersion":1,"source":"fixture","version":"0","generation":9,"note":"test","excludedTypes":[],` +
		`"types":["bug","dark","dragon","electric","fairy","fighting","fire","flying","ghost","grass","ground","ice","normal","poison","psychic","rock","steel","water"],` +
		`"effectiveness":{` + entries + `}}`
}
