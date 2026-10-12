package readmodel_test

// ADR-0139 §5: 素早さの補正(SpeedMods・IgnoresParalysisSpeedDrop)は read model(balance・speed の6ファイル)に出さない。
// balance の abilities.json の effects は防御側のタイプ相性の効果だけ(ADR-0017 §2)で、loader は未知の kind・フィールドを拒否する
// (ADR-0128・ADR-0138)。素早さの補正の利用者(判定)は内部 API(getMasterExport)の effect から読む。
//   - 素早さの項目だけの特性は effects が空配列(特性の行は従来どおり出る)。
//   - 防御の項目と同居していても、防御の項目だけが出る(並び・値は従来どおり)。
//   - 同じ data_versions なら、6ファイルは素早さの項目を足しても足さなくてもバイト単位で同じ(形は変わらない。
//     実運用では effects.json の変更で取り込みの checksum が変わり dataVersion は変わる。ADR-0128 のとおり)。

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// speedEffectQuerier は storetest.New() の特性に素早さの項目を足す。
//   - teststance: 素早さの項目だけ(行が無かった特性に足す)
//   - testguard: 既存の防御の項目(DefResistType)に素早さの項目を足す
func speedEffectQuerier() *storetest.Querier {
	q := storetest.New()
	q.AbilityEffects = append(q.AbilityEffects,
		store.AbilityEffect{AbilityID: "teststance", Effect: json.RawMessage(
			`{"SpeedMods":[{"Condition":"weather_rain","Modifier":8192}]}`)},
	)
	for i, e := range q.AbilityEffects {
		if e.AbilityID == "testguard" {
			q.AbilityEffects[i].Effect = json.RawMessage(
				`{"DefResistType": {"water": 2048, "fire": 2048}, "SpeedMods": [{"Condition": "has_status", "Modifier": 6144}], "IgnoresParalysisSpeedDrop": true}`)
		}
	}
	return q
}

func TestExportDoesNotCarrySpeedEffects(t *testing.T) {
	files, _ := export(t, speedEffectQuerier())

	if got := abilityEffectsFor(t, files.Abilities, "teststance"); len(got) != 0 {
		t.Errorf("teststance の effects = %+v, want [](素早さの補正は read model に出さない)", got)
	}
	want := []abilityEffect{
		{Kind: "type_multiplier", AttackType: "fire", Numerator: 1, Denominator: 2},
		{Kind: "type_multiplier", AttackType: "water", Numerator: 1, Denominator: 2},
	}
	if got := abilityEffectsFor(t, files.Abilities, "testguard"); !reflect.DeepEqual(got, want) {
		t.Errorf("testguard の effects = %+v, want %+v(防御の項目だけ)", got, want)
	}
	if err := validate(t, compileSchema(t, "abilities.schema.json"), files.Abilities); err != nil {
		t.Errorf("abilities.json が balance の schema に合わない: %v", err)
	}

	outputs := map[string][]byte{
		"pokemon-types.json": files.PokemonTypes,
		"moves.json":         files.Moves,
		"abilities.json":     files.Abilities,
		"speed-pokemon.json": files.SpeedPokemon,
		"type-chart.json":    files.TypeChart,
		"metadata.json":      files.Metadata,
	}
	for name, data := range outputs {
		for _, needle := range []string{"SpeedMods", "speedMods", "IgnoresParalysisSpeedDrop", "weather_rain", "has_status"} {
			if bytes.Contains(data, []byte(needle)) {
				t.Errorf("%s に %s が出た(read model は素早さの補正を持たない。ADR-0139 §5)", name, needle)
			}
		}
	}
}

// 素早さの項目を足しても、read model の6ファイルはバイト単位で変わらない。
func TestExportIsIndependentOfSpeedEffects(t *testing.T) {
	base := storetest.New()
	// 比較の基準: 同じ特性の行から素早さの項目だけを除いたもの(teststance は行なし)。
	for i, e := range base.AbilityEffects {
		if e.AbilityID == "testguard" {
			base.AbilityEffects[i].Effect = json.RawMessage(`{"DefResistType": {"water": 2048, "fire": 2048}}`)
		}
	}
	without, _ := export(t, base)
	with, _ := export(t, speedEffectQuerier())
	if !bytes.Equal(without.PokemonTypes, with.PokemonTypes) || !bytes.Equal(without.Moves, with.Moves) ||
		!bytes.Equal(without.Abilities, with.Abilities) || !bytes.Equal(without.SpeedPokemon, with.SpeedPokemon) ||
		!bytes.Equal(without.TypeChart, with.TypeChart) || !bytes.Equal(without.Metadata, with.Metadata) {
		t.Error("素早さの項目を足したら read model が変わった(read model は素早さの補正に依存しない)")
	}
}
