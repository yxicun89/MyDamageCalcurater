package speedeffects

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/judge/internal/client"
	"example.com/pokecalc/services/judge/internal/judge"
)

// 効果定義の JSON(内部 API の MasterItem.effect / MasterAbility.effect。自由形式)から、素早さの項目
// (SpeedMods・IgnoresParalysisSpeedDrop。ADR-0139)だけを読む(issue 235 第2段・ADR-0714 §1)。
// 素早さ以外のキー(DamageMod など)は読まずに無視する。素早さの項目の形が不正なら、その ID だけを
// 「確定できない」(表に載せない)にして、他の ID は使う。テストの効果は架空の倍率・条件の組。

func TestDecodeAbilitySpeedEffect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    judge.AbilitySpeedEffect
		wantErr bool
	}{
		{"null は素早さ効果なし", `null`, judge.AbilitySpeedEffect{}, false},
		{"空でない素早さ以外の効果だけなら素早さ効果なし", `{"DamageMod":{"Kind":"x","Modifier":5325}}`, judge.AbilitySpeedEffect{}, false},
		{"条件つき ×2", `{"SpeedMods":[{"Condition":"weather_rain","Modifier":8192}]}`,
			judge.AbilitySpeedEffect{Mods: []engine.SpeedMod{{Condition: engine.SpeedConditionWeatherRain, Modifier: 8192}}}, false},
		{"まひの半減を受けない", `{"SpeedMods":[{"Condition":"has_status","Modifier":6144}],"IgnoresParalysisSpeedDrop":true}`,
			judge.AbilitySpeedEffect{
				Mods:                      []engine.SpeedMod{{Condition: engine.SpeedConditionHasStatus, Modifier: 6144}},
				IgnoresParalysisSpeedDrop: true,
			}, false},
		{"配列の順を保つ(並べ替えない)",
			`{"SpeedMods":[{"Condition":"has_status","Modifier":6144},{"Condition":"always","Modifier":2048}]}`,
			judge.AbilitySpeedEffect{Mods: []engine.SpeedMod{
				{Condition: engine.SpeedConditionHasStatus, Modifier: 6144},
				{Condition: engine.SpeedConditionAlways, Modifier: 2048},
			}}, false},
		// 未知の条件は形としては受け付け、評価で「確定できない」にする(ADR-0714 §1。語彙が増えた新しいマスタでも
		// 他の要素・他の ID を巻き込まない)。
		{"未知の条件は受け付ける", `{"SpeedMods":[{"Condition":"weather_fog","Modifier":8192}]}`,
			judge.AbilitySpeedEffect{Mods: []engine.SpeedMod{{Condition: "weather_fog", Modifier: 8192}}}, false},
		{"素早さ以外のキーと並んでも素早さだけ読む",
			`{"DamageMod":{"Kind":"x","Modifier":5325},"SpeedMods":[{"Condition":"always","Modifier":2048}]}`,
			judge.AbilitySpeedEffect{Mods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: 2048}}}, false},
		{"IgnoresParalysisSpeedDrop が false は無いのと同じ", `{"IgnoresParalysisSpeedDrop":false}`, judge.AbilitySpeedEffect{}, false},

		// 不正(その ID は確定できない)。
		{"オブジェクトでも null でもない", `[1]`, judge.AbilitySpeedEffect{}, true},
		{"壊れた JSON", `{"SpeedMods":[`, judge.AbilitySpeedEffect{}, true},
		{"SpeedMods が配列でない", `{"SpeedMods":{"Condition":"always","Modifier":2048}}`, judge.AbilitySpeedEffect{}, true},
		{"SpeedMods が空", `{"SpeedMods":[]}`, judge.AbilitySpeedEffect{}, true},
		{"SpeedMods が null", `{"SpeedMods":null}`, judge.AbilitySpeedEffect{}, true},
		{"要素がオブジェクトでない", `{"SpeedMods":[8192]}`, judge.AbilitySpeedEffect{}, true},
		{"Condition が無い", `{"SpeedMods":[{"Modifier":8192}]}`, judge.AbilitySpeedEffect{}, true},
		{"Modifier が無い", `{"SpeedMods":[{"Condition":"always"}]}`, judge.AbilitySpeedEffect{}, true},
		{"Condition が文字列でない", `{"SpeedMods":[{"Condition":1,"Modifier":8192}]}`, judge.AbilitySpeedEffect{}, true},
		{"Condition が空", `{"SpeedMods":[{"Condition":"","Modifier":8192}]}`, judge.AbilitySpeedEffect{}, true},
		{"Modifier が小数", `{"SpeedMods":[{"Condition":"always","Modifier":1.5}]}`, judge.AbilitySpeedEffect{}, true},
		{"Modifier が文字列", `{"SpeedMods":[{"Condition":"always","Modifier":"8192"}]}`, judge.AbilitySpeedEffect{}, true},
		{"Modifier が 0", `{"SpeedMods":[{"Condition":"always","Modifier":0}]}`, judge.AbilitySpeedEffect{}, true},
		{"Modifier が負", `{"SpeedMods":[{"Condition":"always","Modifier":-8192}]}`, judge.AbilitySpeedEffect{}, true},
		{"Modifier が中立(4096)", `{"SpeedMods":[{"Condition":"always","Modifier":4096}]}`, judge.AbilitySpeedEffect{}, true},
		{"Modifier が上限超え", `{"SpeedMods":[{"Condition":"always","Modifier":2097153}]}`, judge.AbilitySpeedEffect{}, true},
		{"Modifier が int64 を超える", `{"SpeedMods":[{"Condition":"always","Modifier":99999999999999999999}]}`, judge.AbilitySpeedEffect{}, true},
		// encoding/json は大文字小文字違いのキーを既知の欄へ黙って寄せるので、キーは厳密に照合する。
		{"要素のキーの大文字小文字違い", `{"SpeedMods":[{"condition":"always","Modifier":2048}]}`, judge.AbilitySpeedEffect{}, true},
		{"要素に未知のキー", `{"SpeedMods":[{"Condition":"always","Modifier":2048,"Extra":1}]}`, judge.AbilitySpeedEffect{}, true},
		{"同じ条件が2回", `{"SpeedMods":[{"Condition":"always","Modifier":2048},{"Condition":"always","Modifier":8192}]}`,
			judge.AbilitySpeedEffect{}, true},
		{"IgnoresParalysisSpeedDrop が真偽値でない", `{"IgnoresParalysisSpeedDrop":"true"}`, judge.AbilitySpeedEffect{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeAbilitySpeedEffect(json.RawMessage(tt.raw))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DecodeAbilitySpeedEffect(%s) err = nil, want error(got %+v)", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeAbilitySpeedEffect(%s): %v", tt.raw, err)
			}
			if !reflect.DeepEqual(normalizeAbility(got), normalizeAbility(tt.want)) {
				t.Errorf("DecodeAbilitySpeedEffect(%s) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDecodeItemSpeedEffect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    judge.ItemSpeedEffect
		wantErr bool
	}{
		{"null は素早さ効果なし", `null`, judge.ItemSpeedEffect{}, false},
		{"素早さ以外の効果だけ", `{"DamageMod":{"Kind":"x","Modifier":4915}}`, judge.ItemSpeedEffect{}, false},
		{"常に ×0.5", `{"SpeedMods":[{"Condition":"always","Modifier":2048}]}`,
			judge.ItemSpeedEffect{Mods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: 2048}}}, false},
		// 持ち物に IgnoresParalysisSpeedDrop は無い(ADR-0139)。持ち物では読まない(素早さに使わない)。
		{"持ち物の IgnoresParalysisSpeedDrop は読まない",
			`{"SpeedMods":[{"Condition":"always","Modifier":2048}],"IgnoresParalysisSpeedDrop":true}`,
			judge.ItemSpeedEffect{Mods: []engine.SpeedMod{{Condition: engine.SpeedConditionAlways, Modifier: 2048}}}, false},
		{"SpeedMods が空", `{"SpeedMods":[]}`, judge.ItemSpeedEffect{}, true},
		{"Modifier が中立", `{"SpeedMods":[{"Condition":"always","Modifier":4096}]}`, judge.ItemSpeedEffect{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := DecodeItemSpeedEffect(json.RawMessage(tt.raw))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("DecodeItemSpeedEffect(%s) err = nil, want error", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeItemSpeedEffect(%s): %v", tt.raw, err)
			}
			if len(got.Mods) != len(tt.want.Mods) || (len(got.Mods) > 0 && !reflect.DeepEqual(got.Mods, tt.want.Mods)) {
				t.Errorf("DecodeItemSpeedEffect(%s) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}

// TestDecodeRejectsTooManySpeedMods: 1 つの効果の SpeedMods は上限(MaxSpeedModsPerEffect)まで。
// 巨大な配列で 1 件の評価が重くならないようにする(超えたらその ID だけ確定できない)。
func TestDecodeRejectsTooManySpeedMods(t *testing.T) {
	t.Parallel()

	elems := make([]string, 0, MaxSpeedModsPerEffect+1)
	for i := 0; i <= MaxSpeedModsPerEffect; i++ {
		elems = append(elems, `{"Condition":"unknown_`+strings.Repeat("x", i+1)+`","Modifier":8192}`)
	}
	atLimit := `{"SpeedMods":[` + strings.Join(elems[:MaxSpeedModsPerEffect], ",") + `]}`
	if _, err := DecodeAbilitySpeedEffect(json.RawMessage(atLimit)); err != nil {
		t.Errorf("上限ちょうど(%d 件)は受け付ける: %v", MaxSpeedModsPerEffect, err)
	}
	over := `{"SpeedMods":[` + strings.Join(elems, ",") + `]}`
	if _, err := DecodeAbilitySpeedEffect(json.RawMessage(over)); err == nil {
		t.Errorf("上限超え(%d 件)は不正にする", MaxSpeedModsPerEffect+1)
	}
	if MaxSpeedModsPerEffect < len(engine.AllSpeedConditions()) {
		t.Errorf("MaxSpeedModsPerEffect = %d は語彙の件数 %d 未満(正しいデータを拒否する)",
			MaxSpeedModsPerEffect, len(engine.AllSpeedConditions()))
	}
}

// TestBuildTable: 内部 API の一覧から ID → 素早さ効果の表を作る。表には全 ID を載せる
// (素早さ効果が無い ID も「効果なしと確定」として載る)。素早さの項目が不正な ID だけを載せない。
func TestBuildTable(t *testing.T) {
	t.Parallel()

	master := client.MasterEffects{
		Abilities: []client.MasterEffectEntry{
			{ID: "test-rain-ability", Effect: json.RawMessage(`{"SpeedMods":[{"Condition":"weather_rain","Modifier":8192}]}`)},
			{ID: "test-plain-ability", Effect: nil},
			{ID: "test-damage-ability", Effect: json.RawMessage(`{"DamageMod":{"Kind":"x","Modifier":5325}}`)},
			{ID: "test-broken-ability", Effect: json.RawMessage(`{"SpeedMods":[{"Condition":"always","Modifier":4096}]}`)},
		},
		Items: []client.MasterEffectEntry{
			{ID: "test-heavy-item", Effect: json.RawMessage(`{"SpeedMods":[{"Condition":"always","Modifier":2048}]}`)},
			{ID: "test-plain-item", Effect: nil},
			{ID: "test-broken-item", Effect: json.RawMessage(`{"SpeedMods":"x"}`)},
		},
	}
	table := BuildTable(master)

	if got, ok := table.Ability("test-rain-ability"); !ok || len(got.Mods) != 1 || got.Mods[0].Modifier != 8192 {
		t.Errorf("Ability(test-rain-ability) = %+v, %v", got, ok)
	}
	for _, id := range []string{"test-plain-ability", "test-damage-ability"} {
		if got, ok := table.Ability(id); !ok || len(got.Mods) != 0 || got.IgnoresParalysisSpeedDrop {
			t.Errorf("Ability(%s) = %+v, %v, want 効果なしで載る", id, got, ok)
		}
	}
	for _, id := range []string{"test-broken-ability", "test-missing-ability", ""} {
		if _, ok := table.Ability(id); ok {
			t.Errorf("Ability(%q) は載せない(不正・マスタに無い)", id)
		}
	}
	if got, ok := table.Item("test-heavy-item"); !ok || len(got.Mods) != 1 || got.Mods[0].Modifier != 2048 {
		t.Errorf("Item(test-heavy-item) = %+v, %v", got, ok)
	}
	if got, ok := table.Item("test-plain-item"); !ok || len(got.Mods) != 0 {
		t.Errorf("Item(test-plain-item) = %+v, %v, want 効果なしで載る", got, ok)
	}
	for _, id := range []string{"test-broken-item", "test-missing-item"} {
		if _, ok := table.Item(id); ok {
			t.Errorf("Item(%q) は載せない", id)
		}
	}
	// 特性と持ち物の名前空間は別(同じ ID でも取り違えない)。
	if _, ok := table.Item("test-rain-ability"); ok {
		t.Error("特性の ID が持ち物の表に載っている")
	}
}

// TestTableLookupReturnsCopy: 表から返した Mods を書き換えても表は変わらない(複数のリクエストが同じ表を共有する)。
func TestTableLookupReturnsCopy(t *testing.T) {
	t.Parallel()

	table := BuildTable(client.MasterEffects{Abilities: []client.MasterEffectEntry{
		{ID: "test-rain-ability", Effect: json.RawMessage(`{"SpeedMods":[{"Condition":"weather_rain","Modifier":8192}]}`)},
	}})
	got, _ := table.Ability("test-rain-ability")
	got.Mods[0].Modifier = 1
	again, _ := table.Ability("test-rain-ability")
	if again.Mods[0].Modifier != 8192 {
		t.Errorf("表の中身が書き換わった: %+v", again)
	}
}

// TestZeroTableKnowsNothing: ゼロ値の Table はどの ID も知らない(取得失敗時に使っても安全側)。
func TestZeroTableKnowsNothing(t *testing.T) {
	t.Parallel()
	var table Table
	if _, ok := table.Ability("x"); ok {
		t.Error("ゼロ値の Table が特性を知っている")
	}
	if _, ok := table.Item("x"); ok {
		t.Error("ゼロ値の Table が持ち物を知っている")
	}
}

func normalizeAbility(e judge.AbilitySpeedEffect) judge.AbilitySpeedEffect {
	if len(e.Mods) == 0 {
		e.Mods = nil
	}
	return e
}
