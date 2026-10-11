package httpapi

// 生成型(api.*)と app・engine の型の変換(ADR-0200・ADR-0811)。
//
// 列挙(format・weather・terrain・status・teraType)が契約に含まれるかは、ここで生成型の Valid() で判定して
// domain.Enum.Known に入れて渡す。失敗にする順番と文言は domain・app が持つ(ADR-0252)。
// side・presets は engine の sentinel に一本化する(WASM 境界と同じ失敗にするため。ADR-0200 §4)。
// 省略の有無(nil)は保ったまま写す(計算イベントにはリクエストのままを載せるため。ADR-0212 §7)。

import (
	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/app"
	"example.com/pokecalc/services/calc/internal/domain"
	"example.com/pokecalc/services/internal/api"
)

// --- 生成型 → app・domain --------------------------------------------------------

func formatFrom(f api.Format) domain.Enum { return domain.Enum{Value: string(f), Known: f.Valid()} }

func statusFrom(s *api.StatusCondition) *domain.Enum {
	if s == nil {
		return nil
	}
	return &domain.Enum{Value: string(*s), Known: s.Valid()}
}

func teraTypeFrom(t *api.PokeType) *domain.Enum {
	if t == nil {
		return nil
	}
	return &domain.Enum{Value: string(*t), Known: t.Valid()}
}

func statsFromBlock(s api.StatBlock) engine.Stats {
	return engine.Stats{HP: s.Hp, Atk: s.Atk, Def: s.Def, SpA: s.Spa, SpD: s.Spd, Spe: s.Spe}
}

func ranksFrom(r *api.RankBlock) *app.Ranks {
	if r == nil {
		return nil
	}
	return &app.Ranks{Atk: r.Atk, Def: r.Def, SpA: r.Spa, SpD: r.Spd, Spe: r.Spe}
}

func individualFrom(in api.Individual) app.Individual {
	return app.Individual{
		SpeciesKey: string(in.SpeciesKey), NatureID: in.NatureId, Level: in.Level,
		AbilityID: in.AbilityId, ItemID: in.ItemId, SP: statsFromBlock(in.Sp), Ranks: ranksFrom(in.Ranks),
		Status: statusFrom(in.Status), TeraType: teraTypeFrom(in.TeraType),
	}
}

func screensFrom(s *api.Screens) *domain.Screens {
	if s == nil {
		return nil
	}
	return &domain.Screens{Reflect: s.Reflect, LightScreen: s.LightScreen, AuroraVeil: s.AuroraVeil}
}

func fieldFrom(f *api.FieldState) *domain.Field {
	if f == nil {
		return nil
	}
	out := &domain.Field{AttackerScreens: screensFrom(f.AttackerScreens), DefenderScreens: screensFrom(f.DefenderScreens)}
	if f.Weather != nil {
		out.Weather = &domain.Enum{Value: string(*f.Weather), Known: f.Weather.Valid()}
	}
	if f.Terrain != nil {
		out.Terrain = &domain.Enum{Value: string(*f.Terrain), Known: f.Terrain.Valid()}
	}
	return out
}

func optionsFrom(o *api.CalcOptions) *app.Options {
	if o == nil {
		return nil
	}
	return &app.Options{Critical: o.Critical}
}

func battleStateFrom(b *api.CalcBattleState) *domain.BattleState {
	if b == nil {
		return nil
	}
	return &domain.BattleState{AttackerCurrentHP: b.AttackerCurrentHp, DefenderCurrentHP: b.DefenderCurrentHp, Hits: b.Hits}
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

func observationsFrom(obs []api.Observation) []domain.Observation {
	if obs == nil {
		return nil
	}
	out := make([]domain.Observation, 0, len(obs))
	for _, o := range obs {
		out = append(out, domain.Observation{Percent: o.Percent, PercentTenths: o.PercentTenths, Damage: o.Damage, Note: o.Note})
	}
	return out
}

func calcRequestFrom(req api.CalcRequest) app.CalcRequest {
	return app.CalcRequest{
		Format: formatFrom(req.Format), Attacker: individualFrom(req.Attacker), Defender: individualFrom(req.Defender),
		MoveID: req.MoveId, Field: fieldFrom(req.Field), Options: optionsFrom(req.Options),
		BattleState: battleStateFrom(req.BattleState),
	}
}

func bulkRequestFrom(req api.BulkCalcRequest) app.BulkRequest {
	out := app.BulkRequest{
		Format: formatFrom(req.Format), Attacker: individualFrom(req.Attacker),
		DefenderSpeciesKey: string(req.DefenderSpeciesKey), MoveID: req.MoveId,
		Field: fieldFrom(req.Field), Options: optionsFrom(req.Options),
		Presets: presetKeysFrom(req.Presets), ItemVariants: req.ItemVariants,
	}
	if ov := req.DefenderOverride; ov != nil {
		out.DefenderOverride = &app.DefenderOverride{AbilityID: ov.AbilityId, Status: statusFrom(ov.Status), Ranks: ranksFrom(ov.Ranks)}
	}
	return out
}

func reverseRequestFrom(req api.ReverseRequest) app.ReverseRequest {
	return app.ReverseRequest{
		Format: formatFrom(req.Format), Side: engine.ReverseSide(req.Side), Known: individualFrom(req.Known),
		UnknownSpeciesKey: string(req.UnknownSpeciesKey), MoveID: req.MoveId,
		Field: fieldFrom(req.Field), Options: optionsFrom(req.Options), ItemCandidates: req.ItemCandidates,
		Observations: observationsFrom(req.Observations), MaxCandidates: req.MaxCandidates,
		UnknownAbilityID: req.UnknownAbilityId,
	}
}

// --- app → 生成型(計算イベントの中身。リクエストのまま) -------------------------

func individualToAPI(in app.Individual) api.Individual {
	out := api.Individual{
		SpeciesKey: api.SpeciesKey(in.SpeciesKey), NatureId: in.NatureID, Level: in.Level,
		AbilityId: in.AbilityID, ItemId: in.ItemID, Sp: statBlockFrom(in.SP),
	}
	if r := in.Ranks; r != nil {
		out.Ranks = &api.RankBlock{Atk: r.Atk, Def: r.Def, Spa: r.SpA, Spd: r.SpD, Spe: r.Spe}
	}
	if in.Status != nil {
		v := api.StatusCondition(in.Status.Value)
		out.Status = &v
	}
	if in.TeraType != nil {
		v := api.PokeType(in.TeraType.Value)
		out.TeraType = &v
	}
	return out
}

func screensToAPI(s *domain.Screens) *api.Screens {
	if s == nil {
		return nil
	}
	return &api.Screens{Reflect: s.Reflect, LightScreen: s.LightScreen, AuroraVeil: s.AuroraVeil}
}

func fieldToAPI(f *domain.Field) *api.FieldState {
	if f == nil {
		return nil
	}
	out := &api.FieldState{AttackerScreens: screensToAPI(f.AttackerScreens), DefenderScreens: screensToAPI(f.DefenderScreens)}
	if f.Weather != nil {
		v := api.Weather(f.Weather.Value)
		out.Weather = &v
	}
	if f.Terrain != nil {
		v := api.Terrain(f.Terrain.Value)
		out.Terrain = &v
	}
	return out
}

func optionsToAPI(o *app.Options) *api.CalcOptions {
	if o == nil {
		return nil
	}
	return &api.CalcOptions{Critical: o.Critical}
}

func battleStateToAPI(b *domain.BattleState) *api.CalcBattleState {
	if b == nil {
		return nil
	}
	return &api.CalcBattleState{AttackerCurrentHp: b.AttackerCurrentHP, DefenderCurrentHp: b.DefenderCurrentHP, Hits: b.Hits}
}

// --- 調整(adjust.go)が使う解決・検証(app・domain に委ねて HTTP のエラーに写す) ---

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// parseFormat は形式を検証する(必須。欠落は zero 値 "" が Valid() で弾かれる)。
func parseFormat(f api.Format) (engine.Format, error) {
	v, err := domain.ParseFormat(formatFrom(f))
	return v, httpErrorFrom(err)
}

// parseField は場の状態を検証する(weather / terrain は列挙。省略は既定値)。
func parseField(f *api.FieldState) (engine.Field, error) {
	v, err := domain.ParseField(fieldFrom(f))
	return v, httpErrorFrom(err)
}

func criticalFrom(o *api.CalcOptions) bool { return o != nil && o.Critical != nil && *o.Critical }

// resolveIndividual は個体を解決する(app.Service.ResolveIndividual。列挙を先に、その後で ID)。
func (s *Server) resolveIndividual(label string, in api.Individual) (engine.Individual, error) {
	v, err := s.app.ResolveIndividual(label, individualFrom(in))
	return v, httpErrorFrom(err)
}

// resolveMove は技 ID を解決する。
func (s *Server) resolveMove(id string) (engine.Move, error) {
	v, err := s.app.ResolveMove(id)
	return v, httpErrorFrom(err)
}

// --- 結果の写し ---------------------------------------------------------------

func statBlockFrom(s engine.Stats) api.StatBlock {
	return api.StatBlock{Hp: s.HP, Atk: s.Atk, Def: s.Def, Spa: s.SpA, Spd: s.SpD, Spe: s.Spe}
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

// percentFromTenths は engine の 0.1% 単位の整数(tenths)を表示%(float64)にする。
// 10 で割るだけで、float で近似し直さない(CLAUDE.md 絶対ルール3)。
func percentFromTenths(v int) float64 { return float64(v) / 10 }

// unsupportedFrom は engine の「未対応」の印を契約の UnsupportedMark に写す(ADR-0123 §7)。
// 印なしは空配列(null にしない。engine/wasmapi の unsupportedFrom と同じ形)。
func unsupportedFrom(ms []engine.UnsupportedMark) []api.UnsupportedMark {
	out := make([]api.UnsupportedMark, 0, len(ms))
	for _, m := range ms {
		out = append(out, api.UnsupportedMark{
			Target: string(m.Target),
			Reason: string(m.Reason),
			Id:     m.ID,
		})
	}
	return out
}

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
		Unsupported: unsupportedFrom(r.Unsupported),
	}
}

// bulkResultFrom は app.BulkOutput を契約の BulkCalcResult に写す(natureId は app が Master で引いたもの)。
func bulkResultFrom(out app.BulkOutput) api.BulkCalcResult {
	res := out.Result
	rows := make([]api.BulkCalcRow, 0, len(res.Rows))
	for i, row := range res.Rows {
		var itemID *string
		if row.ItemID != "" {
			id := row.ItemID
			itemID = &id
		}
		rows = append(rows, api.BulkCalcRow{
			Preset: api.DefenderPreset(row.Preset), PresetLabel: row.PresetLabel, ItemId: itemID,
			Defender: api.BulkDefender{
				Sp: statBlockFrom(row.Defender.SP), Nature: natureModifierFrom(row.Defender.Nature),
				NatureId: out.NatureIDs[i], Stats: statBlockFrom(engine.RealStats(row.Defender)),
			},
			Result: calcResultFrom(row.Result), AbilityId: row.Ability.ID, AbilityIds: row.AbilityIDs,
		})
	}
	return api.BulkCalcResult{DefenderSpeciesKey: res.DefenderSpeciesKey, Rows: rows}
}

// reverseResultFrom は app.ReverseOutput を契約の ReverseResult に写す(natureId は app が Master で引いたもの)。
func reverseResultFrom(out app.ReverseOutput) api.ReverseResult {
	res := out.Result
	cands := make([]api.ReverseCandidate, 0, len(res.Candidates))
	for i, c := range res.Candidates {
		ranges := make([]api.SPRange, 0, len(c.Ranges))
		for _, r := range c.Ranges {
			ranges = append(ranges, api.SPRange{Min: r.Min, Max: r.Max})
		}
		var itemID *string
		if c.ItemID != "" {
			id := c.ItemID
			itemID = &id
		}
		cands = append(cands, api.ReverseCandidate{
			NatureClass: api.NatureClass(c.NatureClass), Nature: natureModifierFrom(c.Nature), NatureId: out.NatureIDs[i],
			ItemId: itemID, Ranges: ranges, SpCount: c.SPCount, Exact: c.Exact, Mismatch: c.Mismatch, Support: c.Support,
			MinPercent: percentFromTenths(c.MinPercentTenths), MaxPercent: percentFromTenths(c.MaxPercentTenths),
			Unsupported: unsupportedFrom(c.Unsupported), AbilityId: c.Ability.ID, AbilityIds: c.AbilityIDs,
		})
	}
	return api.ReverseResult{
		Side: api.ReverseSide(res.Side), Stat: api.StatKey(res.Stat), AssumedHpSp: res.AssumedHPSP,
		ExactCount: res.ExactCount, Candidates: cands,
	}
}
