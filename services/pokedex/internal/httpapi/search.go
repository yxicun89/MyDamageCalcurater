package httpapi

// 公開の検索 API(/api/pokedex/*。ADR-0105 §3)。入力の検証(limit・format・種族キーの形式)は
// DB を呼ぶ前に行う。既定のレギュレーションは GetDefaultRegulation で毎回 DB から引く
// (レギュレーション ID をコードに書かない)。

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/store"
)

const (
	defaultSearchLimit = 50
	minSearchLimit     = 1
	maxSearchLimit     = 200
)

// speciesKeyPattern は getSpecies の path パラメータの形式({図鑑番号4桁}-{フォルム3桁})。
var speciesKeyPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{3}$`)

// likeEscaper は LIKE の特殊文字(\ % _)を \ でエスケープする(1回の走査。エスケープの責任はここ1か所)。
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// likePattern は q を前方一致のパターンにする(省略・空は全件 "%")。
func likePattern(q string) string {
	return likeEscaper.Replace(q) + "%"
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// resolveLimit は limit の既定値・範囲(1〜200)を検証する。DB を呼ぶ前に行う。
func resolveLimit(p *int) (int32, error) {
	if p == nil {
		return defaultSearchLimit, nil
	}
	if *p < minSearchLimit || *p > maxSearchLimit {
		return 0, newError(api.InvalidInput, "limit は %d〜%d でなければならない: %d", minSearchLimit, maxSearchLimit, *p)
	}
	return int32(*p), nil
}

// checkFormat は format の値を検証する(v1 では結果に影響しない。ADR-0105 §3)。
func checkFormat(p *api.Format) error {
	if p == nil {
		return nil
	}
	switch *p {
	case api.Single, api.Double:
		return nil
	default:
		return newError(api.InvalidEnum, "format が不正: %q", string(*p))
	}
}

func typesOf(t1 string, t2 sql.NullString) []api.PokeType {
	out := []api.PokeType{api.PokeType(t1)}
	if t2.Valid && t2.String != "" {
		out = append(out, api.PokeType(t2.String))
	}
	return out
}

// SearchSpecies は GET /api/pokedex/species。
func (s *Server) SearchSpecies(ctx *echo.Context, params api.SearchSpeciesParams) error {
	limit, err := resolveLimit(params.Limit)
	if err != nil {
		return err
	}
	if err := checkFormat(params.Format); err != nil {
		return err
	}
	reqCtx := ctx.Request().Context()
	reg, err := s.q.GetDefaultRegulation(reqCtx)
	if err != nil {
		return unavailable("SearchSpecies/GetDefaultRegulation", err)
	}
	rows, err := s.q.SearchSpecies(reqCtx, store.SearchSpeciesParams{
		RegulationID: reg.ID, Pattern: likePattern(derefStr(params.Q)), Limit: limit,
	})
	if err != nil {
		return unavailable("SearchSpecies", err)
	}
	out := make([]api.SpeciesSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.SpeciesSummary{Key: r.Key, DexNo: int(r.DexNo), Form: int(r.Form), NameJa: r.NameJa, Types: typesOf(r.Type1, r.Type2)})
	}
	return ctx.JSON(http.StatusOK, out)
}

// SearchMoves は GET /api/pokedex/moves。
func (s *Server) SearchMoves(ctx *echo.Context, params api.SearchMovesParams) error {
	limit, err := resolveLimit(params.Limit)
	if err != nil {
		return err
	}
	reqCtx := ctx.Request().Context()
	reg, err := s.q.GetDefaultRegulation(reqCtx)
	if err != nil {
		return unavailable("SearchMoves/GetDefaultRegulation", err)
	}
	rows, err := s.q.SearchMoves(reqCtx, store.SearchMovesParams{
		RegulationID: reg.ID, Pattern: likePattern(derefStr(params.Q)), Limit: limit,
	})
	if err != nil {
		return unavailable("SearchMoves", err)
	}
	out := make([]api.Move, 0, len(rows))
	for _, r := range rows {
		priority := int(r.Priority)
		out = append(out, api.Move{
			Id: r.ID, NameJa: r.NameJa, Type: api.PokeType(r.Type), Category: api.MoveCategory(r.Category),
			Power: int(r.Power), Priority: &priority,
		})
	}
	return ctx.JSON(http.StatusOK, out)
}

// SearchItems は GET /api/pokedex/items。
func (s *Server) SearchItems(ctx *echo.Context, params api.SearchItemsParams) error {
	limit, err := resolveLimit(params.Limit)
	if err != nil {
		return err
	}
	reqCtx := ctx.Request().Context()
	reg, err := s.q.GetDefaultRegulation(reqCtx)
	if err != nil {
		return unavailable("SearchItems/GetDefaultRegulation", err)
	}
	rows, err := s.q.SearchItems(reqCtx, store.SearchItemsParams{
		RegulationID: reg.ID, Pattern: likePattern(derefStr(params.Q)), Limit: limit,
	})
	if err != nil {
		return unavailable("SearchItems", err)
	}
	out := make([]api.Item, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.Item{Id: r.ID, NameJa: r.NameJa})
	}
	return ctx.JSON(http.StatusOK, out)
}

// GetSpecies は GET /api/pokedex/species/{key}。使用可能集合の外の種族も返す(詳細はマスタの参照)。
// learnset は習得技 ∩ 既定のレギュレーションの使用可能な技(ID 昇順)。
func (s *Server) GetSpecies(ctx *echo.Context, key api.SpeciesKey, params api.GetSpeciesParams) error {
	if !speciesKeyPattern.MatchString(key) {
		return newError(api.InvalidInput, "種族キーの形式が不正: %q", key)
	}
	reqCtx := ctx.Request().Context()
	reg, err := s.q.GetDefaultRegulation(reqCtx)
	if err != nil {
		return unavailable("GetSpecies/GetDefaultRegulation", err)
	}
	sp, err := s.q.GetSpeciesByKey(reqCtx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return newError(api.NotFound, "種族が無い: %s", key)
		}
		return unavailable("GetSpeciesByKey", err)
	}
	abilityRows, err := s.q.ListSpeciesAbilityNames(reqCtx, key)
	if err != nil {
		return unavailable("ListSpeciesAbilityNames", err)
	}
	sort.Slice(abilityRows, func(i, j int) bool { return abilityRows[i].Slot < abilityRows[j].Slot })
	abilities := make([]api.Ability, 0, len(abilityRows))
	for _, a := range abilityRows {
		abilities = append(abilities, api.Ability{Id: a.ID, NameJa: a.NameJa})
	}
	learnset, err := s.q.ListSpeciesLearnset(reqCtx, store.ListSpeciesLearnsetParams{SpeciesKey: key, RegulationID: reg.ID})
	if err != nil {
		return unavailable("ListSpeciesLearnset", err)
	}
	sort.Strings(learnset)
	if learnset == nil {
		learnset = []string{}
	}

	detail := api.SpeciesDetail{
		Key: sp.Key, NameJa: sp.NameJa, DexNo: int(sp.DexNo), Form: int(sp.Form), Types: typesOf(sp.Type1, sp.Type2),
		BaseStats: api.StatBlock{Hp: int(sp.BaseHp), Atk: int(sp.BaseAtk), Def: int(sp.BaseDef), Spa: int(sp.BaseSpa), Spd: int(sp.BaseSpd), Spe: int(sp.BaseSpe)},
		Abilities: abilities, Learnset: &learnset,
	}
	return ctx.JSON(http.StatusOK, detail)
}

// GetMove は GET /api/pokedex/moves/{key}。使用可能集合の外の技も返す(絞り込みは検索の仕事)。
// 判定レーンが技の優先度(priority)を個別に引くための経路(2026-09-22 の依頼。DECISIONS.md)。
func (s *Server) GetMove(ctx *echo.Context, key string, params api.GetMoveParams) error {
	row, err := s.q.GetMove(ctx.Request().Context(), key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return newError(api.NotFound, "技が無い: %s", key)
		}
		return unavailable("GetMove", err)
	}
	priority := int(row.Priority)
	move := api.Move{
		Id: row.ID, NameJa: row.NameJa, Type: api.PokeType(row.Type), Category: api.MoveCategory(row.Category),
		Power: int(row.Power), Priority: &priority,
	}
	return ctx.JSON(http.StatusOK, move)
}

// ListNatures は GET /api/pokedex/natures。0行なら 503 master_unavailable。
func (s *Server) ListNatures(ctx *echo.Context, params api.ListNaturesParams) error {
	rows, err := s.q.ListNatures(ctx.Request().Context())
	if err != nil {
		return unavailable("ListNatures", err)
	}
	if len(rows) == 0 {
		return unavailable("ListNatures", errEmpty("natures"))
	}
	out := make([]api.Nature, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.Nature{Id: r.ID, NameJa: r.NameJa, Plus: statKeyPtr(r.Plus), Minus: statKeyPtr(r.Minus)})
	}
	return ctx.JSON(http.StatusOK, out)
}
