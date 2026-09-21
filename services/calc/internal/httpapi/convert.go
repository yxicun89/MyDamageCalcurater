package httpapi

// 生成型(api.*)と engine 型の変換、および Store を使った ID 解決(ADR-0018)。
//
// 1リクエストの流れ(パッケージ doc と同じ): 厳格デコード → 列挙の検証(invalid_enum)
// → ID 解決(unknown_*)→ engine の入力検証(invalid_input)→ engine 呼び出し → 生成型への写し。
// 列挙は「format・weather・terrain・status・teraType」だけを生成型の Valid() で検証する。
// side・presets は engine の sentinel に一本化する(WASM 境界と同じ失敗にするため。ADR-0018 §4)。

import (
	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

// parseFormat は形式を検証する(必須。欠落は zero 値 "" が Valid() で弾かれる)。
func parseFormat(f api.Format) (engine.Format, error) {
	if !f.Valid() {
		return "", newError(api.InvalidEnum, "format に未知の値 %q", string(f))
	}
	return engine.Format(f), nil
}

func parseStatusCondition(label string, s *api.StatusCondition) (engine.Status, error) {
	if s == nil {
		return engine.StatusNone, nil
	}
	if !s.Valid() {
		return "", newError(api.InvalidEnum, "%s.status に未知の値 %q", label, string(*s))
	}
	return engine.Status(*s), nil
}

func parseTeraType(label string, t *api.PokeType) (engine.Type, error) {
	if t == nil {
		return engine.TypeNone, nil
	}
	if !t.Valid() {
		return "", newError(api.InvalidEnum, "%s.teraType に未知の値 %q", label, string(*t))
	}
	return engine.Type(*t), nil
}

func statsFromBlock(s api.StatBlock) engine.Stats {
	return engine.Stats{HP: s.Hp, Atk: s.Atk, Def: s.Def, SpA: s.Spa, SpD: s.Spd, Spe: s.Spe}
}

func statBlockFrom(s engine.Stats) api.StatBlock {
	return api.StatBlock{Hp: s.HP, Atk: s.Atk, Def: s.Def, Spa: s.SpA, Spd: s.SpD, Spe: s.Spe}
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func derefBool(p *bool) bool { return p != nil && *p }

func ranksFromBlock(r *api.RankBlock) engine.Ranks {
	if r == nil {
		return engine.Ranks{}
	}
	return engine.Ranks{Atk: derefInt(r.Atk), Def: derefInt(r.Def), SpA: derefInt(r.Spa), SpD: derefInt(r.Spd), Spe: derefInt(r.Spe)}
}

// natureModifierFrom は性格補正の構造値を契約の NatureModifier(null 許容)に写す。
func natureModifierFrom(n engine.Nature) api.NatureModifier {
	var plus, minus *api.StatKey
	if n.Plus != "" {
		v := api.StatKey(n.Plus)
		plus = &v
	}
	if n.Minus != "" {
		v := api.StatKey(n.Minus)
		minus = &v
	}
	return api.NatureModifier{Plus: plus, Minus: minus}
}

func screensFrom(s *api.Screens) engine.Screens {
	if s == nil {
		return engine.Screens{}
	}
	return engine.Screens{Reflect: derefBool(s.Reflect), LightScreen: derefBool(s.LightScreen), AuroraVeil: derefBool(s.AuroraVeil)}
}

// parseField は場の状態を検証する(weather / terrain は列挙。省略は既定値)。
func parseField(f *api.FieldState) (engine.Field, error) {
	if f == nil {
		return engine.Field{}, nil
	}
	weather := engine.WeatherNone
	if f.Weather != nil {
		if !f.Weather.Valid() {
			return engine.Field{}, newError(api.InvalidEnum, "field.weather に未知の値 %q", string(*f.Weather))
		}
		weather = engine.Weather(*f.Weather)
	}
	terrain := engine.TerrainNone
	if f.Terrain != nil {
		if !f.Terrain.Valid() {
			return engine.Field{}, newError(api.InvalidEnum, "field.terrain に未知の値 %q", string(*f.Terrain))
		}
		terrain = engine.Terrain(*f.Terrain)
	}
	return engine.Field{
		Weather: weather, Terrain: terrain,
		AttackerScreens: screensFrom(f.AttackerScreens), DefenderScreens: screensFrom(f.DefenderScreens),
	}, nil
}

func criticalFrom(o *api.CalcOptions) bool { return o != nil && derefBool(o.Critical) }

// resolveIndividual は個体を解決する。enum(status・teraType)を先に検証し、
// その後で ID(speciesKey・natureId・abilityId・itemId)を Store で解決する。
func (s *Server) resolveIndividual(label string, in api.Individual) (engine.Individual, error) {
	status, err := parseStatusCondition(label, in.Status)
	if err != nil {
		return engine.Individual{}, err
	}
	tera, err := parseTeraType(label, in.TeraType)
	if err != nil {
		return engine.Individual{}, err
	}
	species, ok := s.store.Species(in.SpeciesKey)
	if !ok {
		return engine.Individual{}, newError(api.UnknownSpecies, "%s.speciesKey が見つからない: %q", label, in.SpeciesKey)
	}
	nature, ok := s.store.Nature(in.NatureId)
	if !ok {
		return engine.Individual{}, newError(api.UnknownNature, "%s.natureId が見つからない: %q", label, in.NatureId)
	}
	var ability engine.Ability
	if in.AbilityId != nil {
		ability, ok = s.store.Ability(*in.AbilityId)
		if !ok {
			return engine.Individual{}, newError(api.UnknownAbility, "%s.abilityId が見つからない: %q", label, *in.AbilityId)
		}
	}
	var item *engine.Item
	if in.ItemId != nil {
		it, ok := s.store.Item(*in.ItemId)
		if !ok {
			return engine.Individual{}, newError(api.UnknownItem, "%s.itemId が見つからない: %q", label, *in.ItemId)
		}
		item = &it
	}
	return engine.Individual{
		Species: species, Level: derefInt(in.Level), Nature: nature, Ability: ability, Item: item,
		SP: statsFromBlock(in.Sp), Ranks: ranksFromBlock(in.Ranks), TeraType: tera, Status: status,
	}, nil
}

// resolveSpecies は種族キーを解決する(bulk の defenderSpeciesKey / reverse の unknownSpeciesKey)。
func (s *Server) resolveSpecies(label, key string) (engine.Species, error) {
	sp, ok := s.store.Species(key)
	if !ok {
		return engine.Species{}, newError(api.UnknownSpecies, "%s が見つからない: %q", label, key)
	}
	return sp, nil
}

// resolveMove は技 ID を解決する。
func (s *Server) resolveMove(id string) (engine.Move, error) {
	mv, ok := s.store.Move(id)
	if !ok {
		return engine.Move{}, newError(api.UnknownMove, "moveId が見つからない: %q", id)
	}
	return mv, nil
}

// resolveItems は持ち物 ID の配列(null は持ち物なし)を解決する。省略(nil)は nil のまま返す
// (engine 側が「持ち物なしの1通り」に既定するため。ADR-0018)。
func (s *Server) resolveItems(label string, ids *[]*string) ([]*engine.Item, error) {
	if ids == nil {
		return nil, nil
	}
	out := make([]*engine.Item, 0, len(*ids))
	for i, id := range *ids {
		if id == nil {
			out = append(out, nil)
			continue
		}
		it, ok := s.store.Item(*id)
		if !ok {
			return nil, newError(api.UnknownItem, "%s[%d] が見つからない: %q", label, i, *id)
		}
		out = append(out, &it)
	}
	return out, nil
}

// presetKeysFrom は契約の DefenderPreset 列挙値をそのまま engine.PresetKey に写す。
// 未知の値の検証は engine.CalcBulk の sentinel に任せる(WASM 境界と同じ失敗にするため)。
func presetKeysFrom(ps *[]api.DefenderPreset) []engine.PresetKey {
	if ps == nil {
		return nil
	}
	out := make([]engine.PresetKey, 0, len(*ps))
	for _, p := range *ps {
		out = append(out, engine.PresetKey(p))
	}
	return out
}

// convertObservations は観測を検証して engine.Observation に写す(ADR-0018: キーの有無で数える)。
// engine.CalcReverse 自体は値の有無を区別できない(0 を未指定とみなす)ため、ここで独立に検証する。
func convertObservations(obs []api.Observation) ([]engine.Observation, error) {
	if len(obs) == 0 {
		return nil, newError(api.NoObservation, "観測が1件も無い")
	}
	out := make([]engine.Observation, 0, len(obs))
	for i, o := range obs {
		n := 0
		if o.Percent != nil {
			n++
		}
		if o.PercentTenths != nil {
			n++
		}
		if o.Damage != nil {
			n++
		}
		switch {
		case n != 1:
			return nil, newError(api.InvalidObservation, "観測 %d は percent・percentTenths・damage のうちちょうど1つを指定すること", i+1)
		case o.Percent != nil && (*o.Percent < 1 || *o.Percent > 100):
			return nil, newError(api.InvalidObservation, "観測 %d の percent は 1..100 の範囲外: %d", i+1, *o.Percent)
		case o.PercentTenths != nil && (*o.PercentTenths < 1 || *o.PercentTenths > 1000):
			return nil, newError(api.InvalidObservation, "観測 %d の percentTenths は 1..1000 の範囲外: %d", i+1, *o.PercentTenths)
		case o.Damage != nil && *o.Damage < 1:
			return nil, newError(api.InvalidObservation, "観測 %d の damage は正でなければならない: %d", i+1, *o.Damage)
		}
		e := engine.Observation{}
		if o.Percent != nil {
			e.Percent = *o.Percent
		}
		if o.PercentTenths != nil {
			e.PercentTenths = *o.PercentTenths
		}
		if o.Damage != nil {
			e.Damage = *o.Damage
		}
		if o.Note != nil {
			e.Note = *o.Note
		}
		out = append(out, e)
	}
	return out, nil
}

// --- 結果の写し ---------------------------------------------------------------

// percentFromTenths は engine の 0.1% 単位の整数(tenths)を表示%(float64)にする。
// 10 で割るだけで、float で近似し直さない(CLAUDE.md 絶対ルール3)。
func percentFromTenths(v int) float64 { return float64(v) / 10 }

// calcResultFrom は engine の結果を契約の CalcResult に写す。表示%は tenths を 10 で割るだけ
// (float で近似しない。CLAUDE.md 絶対ルール3)。
func calcResultFrom(r engine.DamageResult) api.CalcResult {
	minT, maxT := r.DisplayPercentRangeTenths()
	rolls := make([]int, len(r.Rolls))
	copy(rolls, r.Rolls[:])
	chance := r.KO.ChancePercent
	return api.CalcResult{
		Rolls: rolls, MinDamage: r.MinDamage(), MaxDamage: r.MaxDamage(),
		MinPercent: percentFromTenths(minT), MaxPercent: percentFromTenths(maxT),
		DefenderHP: r.DefenderHP, Effectiveness: r.Effectiveness, Stab: r.STAB,
		Category: api.MoveCategory(r.Category),
		Ko: api.KOChance{
			Hits: r.KO.Hits, Guaranteed: r.KO.Guaranteed, ChancePercent: &chance,
			DisplayChancePercent: percentFromTenths(r.KO.DisplayChancePercentTenths()),
		},
	}
}

// bulkResultFrom は engine.BulkResult を契約の BulkCalcResult に写す(natureId は Store で写像)。
func (s *Server) bulkResultFrom(res engine.BulkResult) api.BulkCalcResult {
	rows := make([]api.BulkCalcRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		var itemID *string
		if row.ItemID != "" {
			id := row.ItemID
			itemID = &id
		}
		var natureID *string
		if id, ok := s.store.NatureID(row.Defender.Nature); ok {
			natureID = &id
		}
		rows = append(rows, api.BulkCalcRow{
			Preset: api.DefenderPreset(row.Preset), PresetLabel: row.PresetLabel, ItemId: itemID,
			Defender: api.BulkDefender{
				Sp: statBlockFrom(row.Defender.SP), Nature: natureModifierFrom(row.Defender.Nature),
				NatureId: natureID, Stats: statBlockFrom(engine.RealStats(row.Defender)),
			},
			Result: calcResultFrom(row.Result),
		})
	}
	return api.BulkCalcResult{DefenderSpeciesKey: res.DefenderSpeciesKey, Rows: rows}
}

// reverseResultFrom は engine.ReverseResult を契約の ReverseResult に写す(natureId は Store で写像)。
func (s *Server) reverseResultFrom(res engine.ReverseResult) api.ReverseResult {
	cands := make([]api.ReverseCandidate, 0, len(res.Candidates))
	for _, c := range res.Candidates {
		ranges := make([]api.SPRange, 0, len(c.Ranges))
		for _, r := range c.Ranges {
			ranges = append(ranges, api.SPRange{Min: r.Min, Max: r.Max})
		}
		var itemID *string
		if c.ItemID != "" {
			id := c.ItemID
			itemID = &id
		}
		var natureID *string
		if id, ok := s.store.NatureID(c.Nature); ok {
			natureID = &id
		}
		cands = append(cands, api.ReverseCandidate{
			NatureClass: api.NatureClass(c.NatureClass), Nature: natureModifierFrom(c.Nature), NatureId: natureID,
			ItemId: itemID, Ranges: ranges, SpCount: c.SPCount, Exact: c.Exact, Mismatch: c.Mismatch, Support: c.Support,
			MinPercent: percentFromTenths(c.MinPercentTenths), MaxPercent: percentFromTenths(c.MaxPercentTenths),
		})
	}
	return api.ReverseResult{
		Side: api.ReverseSide(res.Side), Stat: api.StatKey(res.Stat), AssumedHpSp: res.AssumedHPSP,
		ExactCount: res.ExactCount, Candidates: cands,
	}
}
