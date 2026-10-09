package wasmapi

// 技の処理の定義(MoveRule。ADR-0143 §6)の境界 DTO。キーは camelCase で、Web が公開 API の PascalCase のまま渡しても
// 受ける(encoding/json は大小を区別せずに照合する)。語彙に無い値は invalid_enum、値域・機構との対応の誤りは engine の検証
// (ValidateRule)で invalid_input。

import (
	"fmt"

	"example.com/pokecalc/engine"
)

type moveRuleDTO struct {
	PowerFormula             string               `json:"powerFormula"`
	PowerBoosts              []movePowerBoostDTO  `json:"powerBoosts"`
	IgnoresBurn              bool                 `json:"ignoresBurn"`
	TerrainPowerMods         []terrainPowerModDTO `json:"terrainPowerMods"`
	TypeByWeather            map[string]string    `json:"typeByWeather"`
	TypeByTerrain            map[string]string    `json:"typeByTerrain"`
	ExtraEffectivenessType   string               `json:"extraEffectivenessType"`
	SuperEffectiveAgainst    []string             `json:"superEffectiveAgainst"`
	PriorityBoost            *priorityBoostDTO    `json:"priorityBoost"`
	BreaksScreens            bool                 `json:"breaksScreens"`
	FailsWithoutDefenderItem bool                 `json:"failsWithoutDefenderItem"`
	SpreadInTerrain          string               `json:"spreadInTerrain"`
	MoveSpecificResolved     bool                 `json:"moveSpecificResolved"`
}

type movePowerBoostDTO struct {
	Condition      string   `json:"condition"`
	Statuses       []string `json:"statuses"`
	Weathers       []string `json:"weathers"`
	Terrains       []string `json:"terrains"`
	BaseMultiplier int      `json:"baseMultiplier"`
	Modifier       int      `json:"modifier"`
}

type terrainPowerModDTO struct {
	Terrain  string `json:"terrain"`
	Modifier int    `json:"modifier"`
}

type priorityBoostDTO struct {
	Terrain string `json:"terrain"`
	Delta   int    `json:"delta"`
}

// toEngine は nil(定義なし)を nil のまま返す。
func (r *moveRuleDTO) toEngine(path string) (*engine.MoveRule, error) {
	if r == nil {
		return nil, nil
	}
	out := &engine.MoveRule{
		PowerFormula: engine.PowerFormula(r.PowerFormula), IgnoresBurn: r.IgnoresBurn,
		BreaksScreens: r.BreaksScreens, FailsWithoutDefenderItem: r.FailsWithoutDefenderItem,
		MoveSpecificResolved: r.MoveSpecificResolved,
	}
	if out.PowerFormula != "" && !out.PowerFormula.Known() {
		return nil, enumError(path+".powerFormula", r.PowerFormula)
	}
	var err error
	if r.PowerBoosts != nil {
		out.PowerBoosts = make([]engine.MovePowerBoost, 0, len(r.PowerBoosts))
		for i, b := range r.PowerBoosts {
			boost, err := b.toEngine(fmt.Sprintf("%s.powerBoosts[%d]", path, i))
			if err != nil {
				return nil, err
			}
			out.PowerBoosts = append(out.PowerBoosts, boost)
		}
	}
	for i, m := range r.TerrainPowerMods {
		t, err := parseTerrainName(fmt.Sprintf("%s.terrainPowerMods[%d].terrain", path, i), m.Terrain)
		if err != nil {
			return nil, err
		}
		out.TerrainPowerMods = append(out.TerrainPowerMods, engine.TerrainPowerMod{Terrain: t, Modifier: m.Modifier})
	}
	if r.TypeByWeather != nil {
		out.TypeByWeather = make(map[engine.Weather]engine.Type, len(r.TypeByWeather))
		for _, k := range sortedKeys(r.TypeByWeather) {
			w := engine.Weather(k)
			if !validWeathers[w] {
				return nil, enumError(path+".typeByWeather", k)
			}
			if out.TypeByWeather[w], err = parseType(path+".typeByWeather."+k, r.TypeByWeather[k], false); err != nil {
				return nil, err
			}
		}
	}
	if r.TypeByTerrain != nil {
		out.TypeByTerrain = make(map[engine.Terrain]engine.Type, len(r.TypeByTerrain))
		for _, k := range sortedKeys(r.TypeByTerrain) {
			t, err := parseTerrainName(path+".typeByTerrain", k)
			if err != nil {
				return nil, err
			}
			if out.TypeByTerrain[t], err = parseType(path+".typeByTerrain."+k, r.TypeByTerrain[k], false); err != nil {
				return nil, err
			}
		}
	}
	if out.ExtraEffectivenessType, err = parseType(path+".extraEffectivenessType", r.ExtraEffectivenessType, true); err != nil {
		return nil, err
	}
	for i, v := range r.SuperEffectiveAgainst {
		t, err := parseType(fmt.Sprintf("%s.superEffectiveAgainst[%d]", path, i), v, false)
		if err != nil {
			return nil, err
		}
		out.SuperEffectiveAgainst = append(out.SuperEffectiveAgainst, t)
	}
	if pb := r.PriorityBoost; pb != nil {
		t, err := parseTerrainName(path+".priorityBoost.terrain", pb.Terrain)
		if err != nil {
			return nil, err
		}
		out.PriorityBoost = &engine.PriorityBoost{Terrain: t, Delta: pb.Delta}
	}
	if r.SpreadInTerrain != "" {
		if out.SpreadInTerrain, err = parseTerrainName(path+".spreadInTerrain", r.SpreadInTerrain); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (b movePowerBoostDTO) toEngine(path string) (engine.MovePowerBoost, error) {
	out := engine.MovePowerBoost{
		Condition: engine.MoveCondition(b.Condition), BaseMultiplier: b.BaseMultiplier, Modifier: b.Modifier,
	}
	if !out.Condition.Known() {
		return engine.MovePowerBoost{}, enumError(path+".condition", b.Condition)
	}
	for i, v := range b.Statuses {
		s := engine.Status(v)
		if !validStatuses[s] {
			return engine.MovePowerBoost{}, enumError(fmt.Sprintf("%s.statuses[%d]", path, i), v)
		}
		out.Statuses = append(out.Statuses, s)
	}
	for i, v := range b.Weathers {
		w := engine.Weather(v)
		if !validWeathers[w] {
			return engine.MovePowerBoost{}, enumError(fmt.Sprintf("%s.weathers[%d]", path, i), v)
		}
		out.Weathers = append(out.Weathers, w)
	}
	for i, v := range b.Terrains {
		t, err := parseTerrainName(fmt.Sprintf("%s.terrains[%d]", path, i), v)
		if err != nil {
			return engine.MovePowerBoost{}, err
		}
		out.Terrains = append(out.Terrains, t)
	}
	return out, nil
}

// parseTerrainName はフィールド名を語彙で検証する(none は engine の検証が invalid_input にする)。
func parseTerrainName(path, v string) (engine.Terrain, error) {
	t := engine.Terrain(v)
	if !validTerrains[t] {
		return "", enumError(path, v)
	}
	return t, nil
}
