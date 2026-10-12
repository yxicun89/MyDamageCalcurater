package httpapi

// 公開の検索 API(/api/pokedex/*。ADR-0105 §3)の HTTP 境界。生成型 ⇄ app の型の変換だけを行い、
// 入力の検証・既定のレギュレーション・読み出しの手順は app.Service が持つ(G7)。

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/app"
	"example.com/pokecalc/services/pokedex/internal/domain"
)

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// formatEnum は format の値と、それが契約の列挙に含まれるか(生成型の Valid)を app に渡す形にする。
func formatEnum(p *api.Format) *domain.Enum {
	if p == nil {
		return nil
	}
	return &domain.Enum{Value: string(*p), Known: p.Valid()}
}

func typesOf(t1 string, t2 *string) []api.PokeType {
	out := []api.PokeType{api.PokeType(t1)}
	if t2 != nil && *t2 != "" {
		out = append(out, api.PokeType(*t2))
	}
	return out
}

func speciesSummaries(rows []app.SpeciesSummary) []api.SpeciesSummary {
	out := make([]api.SpeciesSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.SpeciesSummary{Key: r.Key, DexNo: r.DexNo, Form: r.Form, NameJa: r.NameJa, Types: typesOf(r.Type1, r.Type2)})
	}
	return out
}

// SearchSpecies は GET /api/pokedex/species。
func (s *Server) SearchSpecies(ctx *echo.Context, params api.SearchSpeciesParams) error {
	rows, err := s.app.SearchSpecies(ctx.Request().Context(), app.SpeciesQuery{
		Q: derefStr(params.Q), Limit: params.Limit, Format: formatEnum(params.Format),
	})
	if err != nil {
		return fail(err)
	}
	return ctx.JSON(http.StatusOK, speciesSummaries(rows))
}

// SearchMoves は GET /api/pokedex/moves。
func (s *Server) SearchMoves(ctx *echo.Context, params api.SearchMovesParams) error {
	moves, err := s.app.SearchMoves(ctx.Request().Context(), app.NameQuery{Q: derefStr(params.Q), Limit: params.Limit})
	if err != nil {
		return fail(err)
	}
	return ctx.JSON(http.StatusOK, publicMoves(moves))
}

func publicMoves(moves []app.PublicMove) []api.Move {
	out := make([]api.Move, 0, len(moves))
	for _, m := range moves {
		out = append(out, publicMove(m))
	}
	return out
}

// publicMove は app の技を契約の Move にする(mechanisms は常にキーを出す。flags は不明なら省く。ADR-0178 §4)。
func publicMove(m app.PublicMove) api.Move {
	priority := m.Priority
	var target *api.MoveTarget
	if m.Target != nil {
		t := api.MoveTarget(*m.Target)
		target = &t
	}
	mechanisms := m.Mechanisms
	var flags *[]api.MoveFlag
	if m.Flags != nil {
		f := make([]api.MoveFlag, 0, len(*m.Flags))
		for _, v := range *m.Flags {
			f = append(f, api.MoveFlag(v))
		}
		flags = &f
	}
	return api.Move{
		Id: m.ID, NameJa: m.NameJa, Type: api.PokeType(m.Type), Category: api.MoveCategory(m.Category),
		Power: m.Power, Priority: &priority, Target: target,
		Mechanisms: &mechanisms, Flags: flags,
		MechanismParams: masterMechanismParams(m.MechanismParams), Rule: masterEffect(m.Rule), Effect: masterEffect(m.Effect),
	}
}

// SearchItems は GET /api/pokedex/items。
func (s *Server) SearchItems(ctx *echo.Context, params api.SearchItemsParams) error {
	items, err := s.app.SearchItems(ctx.Request().Context(), app.NameQuery{Q: derefStr(params.Q), Limit: params.Limit})
	if err != nil {
		return fail(err)
	}
	out := make([]api.Item, 0, len(items))
	for _, it := range items {
		// roles は空でも JSON で [] を出す。isMegaStone は常にキーごと出す(ADR-0175)。
		roles := make([]api.ItemRole, 0, len(it.Roles))
		for _, role := range it.Roles {
			roles = append(roles, api.ItemRole(role))
		}
		isMegaStone := it.IsMegaStone
		out = append(out, api.Item{Id: it.ID, NameJa: it.NameJa, Effect: masterEffect(it.Effect), Roles: &roles, IsMegaStone: &isMegaStone, FlingPower: it.FlingPower})
	}
	return ctx.JSON(http.StatusOK, out)
}

// GetSpecies は GET /api/pokedex/species/{key}。
func (s *Server) GetSpecies(ctx *echo.Context, key api.SpeciesKey, params api.GetSpeciesParams) error {
	d, err := s.app.GetSpecies(ctx.Request().Context(), key)
	if err != nil {
		return fail(err)
	}
	sp := d.Species
	abilities := make([]api.Ability, 0, len(d.Abilities))
	for _, a := range d.Abilities {
		abilities = append(abilities, api.Ability{Id: a.ID, NameJa: a.NameJa, Effect: masterEffect(a.Effect)})
	}
	learnset := d.Learnset
	detail := api.SpeciesDetail{
		Key: sp.Key, NameJa: sp.NameJa, DexNo: sp.DexNo, Form: sp.Form, Types: typesOf(sp.Type1, sp.Type2),
		BaseStats: statBlock(sp.BaseStats),
		Abilities: abilities, Learnset: &learnset,
	}
	isMega := sp.IsMega
	detail.IsMega = &isMega
	detail.WeightHg = sp.WeightHg
	return ctx.JSON(http.StatusOK, speciesDetailBody{
		SpeciesDetail: detail, RequiredItemId: sp.RequiredItemID,
		BaseSpeciesKey: sp.BaseSpeciesKey, BaseSpeciesNameJa: d.BaseSpeciesNameJa,
	})
}

// speciesDetailBody は SpeciesDetail の応答本文。requiredItemId は契約上 optional なので生成型は omitempty だが、
// 公開 API はメガでなくてもキーを出す(null)。外側の同名フィールドが埋め込み側より優先され、キーが重複しない。
type speciesDetailBody struct {
	api.SpeciesDetail
	RequiredItemId *string `json:"requiredItemId"`
	// BaseSpeciesKey・BaseSpeciesNameJa も同じ扱い(ADR-0175 §3)。
	BaseSpeciesKey    *string `json:"baseSpeciesKey"`
	BaseSpeciesNameJa *string `json:"baseSpeciesNameJa"`
}

// GetMove は GET /api/pokedex/moves/{key}。
func (s *Server) GetMove(ctx *echo.Context, key string, params api.GetMoveParams) error {
	m, err := s.app.GetMove(ctx.Request().Context(), key)
	if err != nil {
		return fail(err)
	}
	return ctx.JSON(http.StatusOK, publicMove(m))
}

// GetMovesByIds は GET /api/pokedex/moves/batch。
func (s *Server) GetMovesByIds(ctx *echo.Context, params api.GetMovesByIdsParams) error {
	moves, err := s.app.GetMovesByIDs(ctx.Request().Context(), params.Ids)
	if err != nil {
		return fail(err)
	}
	return ctx.JSON(http.StatusOK, publicMoves(moves))
}

// ListNatures は GET /api/pokedex/natures。
func (s *Server) ListNatures(ctx *echo.Context, params api.ListNaturesParams) error {
	rows, err := s.app.ListNatures(ctx.Request().Context())
	if err != nil {
		return fail(err)
	}
	out := make([]api.Nature, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.Nature{Id: r.ID, NameJa: r.NameJa, Plus: statKeyPtr(r.Plus), Minus: statKeyPtr(r.Minus)})
	}
	return ctx.JSON(http.StatusOK, out)
}

// ListMoveLearners は GET /api/pokedex/moves/{key}/learners(ADR-0251)。
func (s *Server) ListMoveLearners(ctx *echo.Context, key string, params api.ListMoveLearnersParams) error {
	rows, err := s.app.ListMoveLearners(ctx.Request().Context(), app.LearnersQuery{MoveKey: key, Limit: params.Limit, Offset: params.Offset})
	if err != nil {
		return fail(err)
	}
	return ctx.JSON(http.StatusOK, speciesSummaries(rows))
}
