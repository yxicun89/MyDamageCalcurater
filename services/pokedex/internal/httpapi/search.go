package httpapi

// 公開の検索 API(/api/pokedex/*。ADR-0105 §3)。入力の検証(limit・format・種族キーの形式)は
// DB を呼ぶ前に行う。既定のレギュレーションは GetDefaultRegulation で毎回 DB から引く
// (レギュレーション ID をコードに書かない)。

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/internal/store"
)

const (
	defaultSearchLimit = 50
	minSearchLimit     = 1
	maxSearchLimit     = 200

	// maxSearchOffset は learners の offset の上限(契約の maximum。int32 に収まる安全側の値)。
	maxSearchOffset = 10000

	// maxBatchIDsCount は getMovesByIds の ids の件数上限(契約の maxItems と同じ。ADR-0208 の
	// 前例どおり、生成ラッパは配列のスキーマを検証しないためハンドラで自前に検査する)。
	maxBatchIDsCount = 64
)

// speciesKeyPattern は getSpecies の path パラメータの形式({図鑑番号4桁}-{フォルム3桁})。
var speciesKeyPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{3}$`)

// likeEscaper は LIKE の特殊文字(\ % _)を \ でエスケープする(1回の走査。エスケープの責任はここ1か所)。
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// likePattern は q を前方一致のパターンにする(省略・空は全件 "%")。
func likePattern(q string) string {
	return likeEscaper.Replace(q) + "%"
}

// SpeciesSearchPatterns は種族検索の LIKE のパターンの組を返す(ADR-0324)。
// pattern は q の前方一致、megaPattern は「メガ + q」の前方一致(メガ種族だけに使う)。
// 後者で、メガを除いた基本種名(ルカリオ)でもメガ種族(メガルカリオ)が当たる。
func SpeciesSearchPatterns(q string) (pattern, megaPattern string) {
	return likePattern(q), likeEscaper.Replace(master.MegaNamePrefix) + likePattern(q)
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

// resolveOffset は offset の既定値(0)・範囲(0〜10000)を検証する。DB を呼ぶ前に行う。
func resolveOffset(p *int) (int32, error) {
	if p == nil {
		return 0, nil
	}
	if *p < 0 || *p > maxSearchOffset {
		return 0, newError(api.InvalidInput, "offset は 0〜%d でなければならない: %d", maxSearchOffset, *p)
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
	pattern, megaPattern := SpeciesSearchPatterns(derefStr(params.Q))
	rows, err := s.q.SearchSpecies(reqCtx, store.SearchSpeciesParams{
		RegulationID: reg.ID, Pattern: pattern, MegaPattern: megaPattern, Limit: limit,
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
		target, err := publicMoveTarget(r.ID, r.Target)
		if err != nil {
			return err
		}
		out = append(out, api.Move{
			Id: r.ID, NameJa: r.NameJa, Type: api.PokeType(r.Type), Category: api.MoveCategory(r.Category),
			Power: int(r.Power), Priority: &priority, Target: target,
		})
	}
	return ctx.JSON(http.StatusOK, out)
}

// publicMoveTarget は技の対象を公開 API の分類(single/spread)にする。NULL はキーを省く(nil)。
// 未知の値は分類できないので 503 master_unavailable(ADR-0223 §4。効果と同じ扱い)。
func publicMoveTarget(moveID string, v sql.NullString) (*api.MoveTarget, error) {
	if !v.Valid {
		return nil, nil
	}
	if !master.IsMoveTarget(v.String) {
		return nil, unavailable("move target: "+moveID, fmt.Errorf("未知の技の対象: %q", v.String))
	}
	out := api.MoveTarget(master.MoveTarget(v.String).Engine())
	return &out, nil
}

// SearchItems は GET /api/pokedex/items。
func (s *Server) SearchItems(ctx *echo.Context, params api.SearchItemsParams) error {
	limit, err := resolveLimit(params.Limit)
	if err != nil {
		return err
	}
	reqCtx := ctx.Request().Context()
	// 一覧と相性表(効果の検証用)は1つの読み取り専用トランザクションで読む(ADR-0127・ADR-0218 §3)。
	tx, err := s.q.BeginTx(reqCtx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return unavailable("SearchItems/BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }()
	reg, err := tx.GetDefaultRegulation(reqCtx)
	if err != nil {
		return unavailable("SearchItems/GetDefaultRegulation", err)
	}
	rows, err := tx.SearchItems(reqCtx, store.SearchItemsParams{
		RegulationID: reg.ID, Pattern: likePattern(derefStr(params.Q)), Limit: limit,
	})
	if err != nil {
		return unavailable("SearchItems", err)
	}
	var chart engine.TypeChart // 効果を持つ行があるときだけ読む
	out := make([]api.Item, 0, len(rows))
	for _, r := range rows {
		effect, decoded, err := publicEffect(reqCtx, tx, &chart, r.Effect, master.DecodeItemEffect)
		if err != nil {
			return unavailable("SearchItems/effect:"+r.ID, err)
		}
		// ItemRoles は nil を返さない(空でも JSON で [] を出す)。isMegaStone は常にキーごと出す(ADR-0175)。
		roles := master.ItemRoles(decoded, r.IsMegaStone)
		apiRoles := make([]api.ItemRole, 0, len(roles))
		for _, role := range roles {
			apiRoles = append(apiRoles, api.ItemRole(role))
		}
		isMegaStone := r.IsMegaStone
		out = append(out, api.Item{Id: r.ID, NameJa: r.NameJa, Effect: effect, Roles: &apiRoles, IsMegaStone: &isMegaStone})
	}
	// 読み終えたらすぐ閉じて接続を返す(応答の書き込みを待たない)。エラー経路は defer の Rollback が閉じる。
	if err := tx.Commit(); err != nil {
		return unavailable("SearchItems/Commit", err)
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
	tx, err := s.q.BeginTx(reqCtx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return unavailable("GetSpecies/BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }()
	reg, err := tx.GetDefaultRegulation(reqCtx)
	if err != nil {
		return unavailable("GetSpecies/GetDefaultRegulation", err)
	}
	sp, err := tx.GetSpeciesByKey(reqCtx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return newError(api.NotFound, "種族が無い: %s", key)
		}
		return unavailable("GetSpeciesByKey", err)
	}
	abilityRows, err := tx.ListSpeciesAbilityNames(reqCtx, key)
	if err != nil {
		return unavailable("ListSpeciesAbilityNames", err)
	}
	sort.Slice(abilityRows, func(i, j int) bool { return abilityRows[i].Slot < abilityRows[j].Slot })
	abilities := make([]api.Ability, 0, len(abilityRows))
	var chart engine.TypeChart // 効果を持つ行があるときだけ読む
	for _, a := range abilityRows {
		effect, _, err := publicEffect(reqCtx, tx, &chart, a.Effect, master.DecodeAbilityEffect)
		if err != nil {
			return unavailable("GetSpecies/effect:"+a.ID, err)
		}
		abilities = append(abilities, api.Ability{Id: a.ID, NameJa: a.NameJa, Effect: effect})
	}
	learnset, err := tx.ListSpeciesLearnset(reqCtx, store.ListSpeciesLearnsetParams{SpeciesKey: key, RegulationID: reg.ID})
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
	isMega := sp.IsMega
	detail.IsMega = &isMega
	// 基本種の名前も同じ読み取り専用トランザクションで読む(1スナップショット。ADR-0127・ADR-0175 §3)。
	var baseNameJa *string
	if sp.BaseSpeciesKey.Valid {
		base, err := tx.GetSpeciesByKey(reqCtx, sp.BaseSpeciesKey.String)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return unavailable("GetSpecies/GetBaseSpecies", err)
		}
		if err == nil {
			baseNameJa = &base.NameJa
		}
	}
	if err := tx.Commit(); err != nil {
		return unavailable("GetSpecies/Commit", err)
	}
	return ctx.JSON(http.StatusOK, speciesDetailBody{
		SpeciesDetail: detail, RequiredItemId: nullStringPtr(sp.RequiredItemID),
		BaseSpeciesKey: nullStringPtr(sp.BaseSpeciesKey), BaseSpeciesNameJa: baseNameJa,
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
	target, err := publicMoveTarget(row.ID, row.Target)
	if err != nil {
		return err
	}
	move := api.Move{
		Id: row.ID, NameJa: row.NameJa, Type: api.PokeType(row.Type), Category: api.MoveCategory(row.Category),
		Power: int(row.Power), Priority: &priority, Target: target,
	}
	return ctx.JSON(http.StatusOK, move)
}

// GetMovesByIds は GET /api/pokedex/moves/batch。getMove の複数版(ADR-0304 §3)。
// getMove と同様に既定のレギュレーションで絞らない。見つからなかった ID は黙って省き、
// 応答は ids と同じ順にする(DB の IN 句は順序を保証しないため、ここで並べ替える)。
func (s *Server) GetMovesByIds(ctx *echo.Context, params api.GetMovesByIdsParams) error {
	if len(params.Ids) == 0 {
		return newError(api.InvalidInput, "ids が空: 1〜%d 件でなければならない", maxBatchIDsCount)
	}
	if len(params.Ids) > maxBatchIDsCount {
		return newError(api.InvalidInput, "ids は %d 件以下でなければならない: %d 件", maxBatchIDsCount, len(params.Ids))
	}

	rows, err := s.q.GetMovesByIDs(ctx.Request().Context(), params.Ids)
	if err != nil {
		return unavailable("GetMovesByIDs", err)
	}
	byID := make(map[string]store.GetMovesByIDsRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	out := make([]api.Move, 0, len(params.Ids))
	for _, id := range params.Ids {
		r, ok := byID[id]
		if !ok {
			continue
		}
		priority := int(r.Priority)
		target, err := publicMoveTarget(r.ID, r.Target)
		if err != nil {
			return err
		}
		out = append(out, api.Move{
			Id: r.ID, NameJa: r.NameJa, Type: api.PokeType(r.Type), Category: api.MoveCategory(r.Category),
			Power: int(r.Power), Priority: &priority, Target: target,
		})
	}
	return ctx.JSON(http.StatusOK, out)
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

// publicEffect は公開 API の effect(ADR-0218)。行が無ければ nil(キーを省く)。あれば共通マスタで
// 厳格に検証してから、数値の字面を保ったまま api.MasterEffect にする。検証に通らなければ error
// (呼び出し側が 503 にする。error の文面は本文に出ない)。相性表は最初に必要になったときだけ chart に読む。
func publicEffect[T any](ctx context.Context, q store.Querier, chart *engine.TypeChart, rawp *json.RawMessage,
	decode func([]byte, engine.TypeChart) (*T, error)) (*api.MasterEffect, *T, error) {
	if rawp == nil {
		return nil, nil, nil // 効果の行が無い
	}
	raw := *rawp
	if *chart == (engine.TypeChart{}) {
		c, err := loadTypeChart(ctx, q)
		if err != nil {
			return nil, nil, err
		}
		*chart = c
	}
	decoded, err := decode(raw, *chart)
	if err != nil {
		return nil, nil, err
	}
	me, err := masterEffectFor(raw)
	return me, decoded, err
}

// loadTypeChart は types / type_chart から engine.TypeChart を作る(services/internal/master 経由)。
func loadTypeChart(ctx context.Context, q store.Querier) (engine.TypeChart, error) {
	types, err := q.ListTypes(ctx)
	if err != nil {
		return engine.TypeChart{}, err
	}
	chart, err := q.ListTypeChart(ctx)
	if err != nil {
		return engine.TypeChart{}, err
	}
	typeRows := make([]master.TypeRow, 0, len(types))
	for _, t := range types {
		typeRows = append(typeRows, master.TypeRow{ID: t.ID, SortOrder: int(t.SortOrder), NameJa: t.NameJa})
	}
	chartRows := make([]master.TypeChartRow, 0, len(chart))
	for _, c := range chart {
		chartRows = append(chartRows, master.TypeChartRow{AttackType: c.AttackType, DefenseType: c.DefenseType, Code: int(c.Code)})
	}
	return master.TypeChart(typeRows, chartRows)
}

// ListMoveLearners は GET /api/pokedex/moves/{key}/learners(技を覚える種族の一覧。ADR-0251)。
// 判定の順: 入力の検証 → 既定のレギュレーション(無ければ 503) → 技の存在(無ければ 404) → 逆引き。
// 技がマスタにあって使用可能集合の外なら 200 []。
func (s *Server) ListMoveLearners(ctx *echo.Context, key string, params api.ListMoveLearnersParams) error {
	limit, err := resolveLimit(params.Limit)
	if err != nil {
		return err
	}
	offset, err := resolveOffset(params.Offset)
	if err != nil {
		return err
	}
	reqCtx := ctx.Request().Context()
	reg, err := s.q.GetDefaultRegulation(reqCtx)
	if err != nil {
		return unavailable("ListMoveLearners/GetDefaultRegulation", err)
	}
	move, err := s.q.GetMove(reqCtx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return newError(api.NotFound, "技が無い: %s", key)
		}
		return unavailable("ListMoveLearners/GetMove", err)
	}
	rows, err := s.q.ListMoveLearners(reqCtx, store.ListMoveLearnersParams{
		RegulationID: reg.ID, MoveID: move.ID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return unavailable("ListMoveLearners", err)
	}
	out := make([]api.SpeciesSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.SpeciesSummary{Key: r.Key, DexNo: int(r.DexNo), Form: int(r.Form), NameJa: r.NameJa, Types: typesOf(r.Type1, r.Type2)})
	}
	return ctx.JSON(http.StatusOK, out)
}
