package importer_test

// ADR-0139: コミットされた本番の効果定義 data/importer/effects.json の素早さの節(speedItems・speedAbilities)が、
// ADR に書いた対象(Champions の既定のレギュレーションで使用可能なもののうち、素早さに効くもの。
// こだわりスカーフは対象外)と一致すること。ID はテストに書かず、件数と「条件 → 倍率」の組で固定する
// (値の出典は @smogon/calc 0.12.0 の getFinalSpeed と Showdown の onModifySpe。ADR-0139 §対象)。

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/importer"
)

func TestRepoSpeedEffectsMatchADR(t *testing.T) {
	raw, err := os.ReadFile(repoDataPath("effects.json"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := importer.DecodeEffectsFile(raw)
	if err != nil {
		t.Fatalf("data/importer/effects.json: %v", err)
	}
	chart := fullTypeChart(t)

	type speedRow struct {
		Condition    engine.SpeedCondition
		Modifier     int
		IgnoresParal bool
	}
	collect := func(label string, defs map[string]json.RawMessage, decode func(json.RawMessage) ([]engine.SpeedMod, bool, error)) []speedRow {
		var rows []speedRow
		for id, def := range defs {
			mods, ignores, err := decode(def)
			if err != nil {
				t.Errorf("%s %s の定義を読めない: %v", label, id, err)
				continue
			}
			if len(mods) != 1 {
				t.Errorf("%s %s: SpeedMods は1要素(対象はどれも条件が1つ): %+v", label, id, mods)
				continue
			}
			rows = append(rows, speedRow{Condition: mods[0].Condition, Modifier: mods[0].Modifier, IgnoresParal: ignores})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Condition < rows[j].Condition })
		return rows
	}

	abilities := collect("特性", file.SpeedAbilities, func(def json.RawMessage) ([]engine.SpeedMod, bool, error) {
		e, err := master.DecodeAbilityEffect(def, chart)
		if err != nil {
			return nil, false, err
		}
		return e.SpeedMods, e.IgnoresParalysisSpeedDrop, nil
	})
	// 特性 7 件: 雨・晴れ・砂・雪・エレキフィールドで ×2、状態異常で ×1.5(まひの半減を受けない)、持ち物を失った後に ×2。
	wantAbilities := []speedRow{
		{engine.SpeedConditionHasStatus, 6144, true},
		{engine.SpeedConditionItemLost, 8192, false},
		{engine.SpeedConditionTerrainElectric, 8192, false},
		{engine.SpeedConditionWeatherRain, 8192, false},
		{engine.SpeedConditionWeatherSand, 8192, false},
		{engine.SpeedConditionWeatherSnow, 8192, false},
		{engine.SpeedConditionWeatherSun, 8192, false},
	}
	if !reflect.DeepEqual(abilities, wantAbilities) {
		t.Errorf("speedAbilities = %+v, want %+v(ADR-0139 §対象)", abilities, wantAbilities)
	}

	items := collect("持ち物", file.SpeedItems, func(def json.RawMessage) ([]engine.SpeedMod, bool, error) {
		e, err := master.DecodeItemEffect(def, chart)
		if err != nil {
			return nil, false, err
		}
		return e.SpeedMods, false, nil
	})
	// 持ち物 1 件: 常に ×0.5(こだわりスカーフは対象外。判定の設定 ADR-0701 のまま)。
	if want := []speedRow{{engine.SpeedConditionAlways, 2048, false}}; !reflect.DeepEqual(items, want) {
		t.Errorf("speedItems = %+v, want %+v(ADR-0139 §対象)", items, want)
	}

	// 素早さの節の ID は、ダメージの節と重なってよい(取り込みで1つの効果 JSON に合わせる)が、
	// ダメージの節に素早さの項目を書かない。
	for kind, defs := range map[string]map[string]json.RawMessage{"items": file.Items, "abilities": file.Abilities} {
		for id, def := range defs {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(def, &fields); err != nil {
				t.Fatalf("%s %s: %v", kind, id, err)
			}
			for _, k := range []string{"SpeedMods", "IgnoresParalysisSpeedDrop"} {
				if _, ok := fields[k]; ok {
					t.Errorf("%s %s: ダメージの節に素早さの項目 %s がある(speedItems・speedAbilities に置く)", kind, id, k)
				}
			}
		}
	}
}
