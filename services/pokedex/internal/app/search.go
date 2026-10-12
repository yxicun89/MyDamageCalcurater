package app

// 公開の検索(/api/pokedex/*。ADR-0105 §3)の手順。入力の検証(limit・format・種族キーの形式)は
// DB を呼ぶ前に行う。既定のレギュレーションは GetDefaultRegulation で毎回 DB から引く
// (レギュレーション ID をコードに書かない)。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/internal/domain"
)

// Service は公開の検索・内部 API・readiness の手順。
type Service struct {
	store Store
}

// New は store を使う Service を作る。
func New(store Store) *Service {
	return &Service{store: store}
}

// SpeciesQuery は searchSpecies の入力。
type SpeciesQuery struct {
	Q      string
	Limit  *int
	Format *domain.Enum
}

// NameQuery は searchMoves・searchItems の入力。
type NameQuery struct {
	Q     string
	Limit *int
}

// LearnersQuery は listMoveLearners の入力。
type LearnersQuery struct {
	MoveKey string
	Limit   *int
	Offset  *int
}

// PublicMove は公開 API の技(機構・フラグ・中身・処理の定義・確率 100% の追加効果つき。ADR-0178 §4・ADR-0253)。
type PublicMove struct {
	ID       string
	NameJa   string
	Type     string
	Category string
	Power    int
	Priority int
	// Target は公開 API の分類(single/spread)。NULL は nil(キーを省く)。
	Target *string
	// Mechanisms は常に非 nil(通常の技は空)。
	Mechanisms []string
	// Flags はまだ取り込んでいない(move_flags が空)なら nil(キーを省く)。
	Flags           *[]string
	MechanismParams *MechanismParams
	Rule            *domain.Effect
	Effect          *domain.Effect
}

// MechanismParams は技の機構の中身(ADR-0142 §8)。無い項目は nil。
type MechanismParams struct {
	OffenseStat    *string
	OffensePokemon *string
	DefenseStat    *string
	MultiHit       *MultiHit
	FixedDamage    *FixedDamage
	Ohko           *OHKO
}

// MultiHit は連続技の回数。
type MultiHit struct{ Min, Max int }

// FixedDamage は固定ダメージ。
type FixedDamage struct {
	Level bool
	Value int
}

// OHKO は一撃必殺(ImmuneType は効かないタイプ)。
type OHKO struct{ ImmuneType *string }

// PublicItem は公開 API の持ち物(ADR-0175・ADR-0218)。Roles は常に非 nil。
type PublicItem struct {
	ID          string
	NameJa      string
	Effect      *domain.Effect
	Roles       []string
	IsMegaStone bool
	FlingPower  *int
}

// PublicAbility は getSpecies の特性(ADR-0218)。
type PublicAbility struct {
	ID     string
	NameJa string
	Effect *domain.Effect
}

// SpeciesDetail は getSpecies の結果。Learnset は常に非 nil(ID 昇順)。
type SpeciesDetail struct {
	Species           Species
	Abilities         []PublicAbility
	Learnset          []string
	BaseSpeciesNameJa *string
}

// SearchSpecies は GET /api/pokedex/species の手順。
func (s *Service) SearchSpecies(ctx context.Context, in SpeciesQuery) ([]SpeciesSummary, error) {
	limit, err := domain.ResolveLimit(in.Limit)
	if err != nil {
		return nil, err
	}
	if err := domain.CheckFormat(in.Format); err != nil {
		return nil, err
	}
	reg, err := s.store.GetDefaultRegulation(ctx)
	if err != nil {
		return nil, domain.Unavailable("SearchSpecies/GetDefaultRegulation", err)
	}
	pattern, megaPattern := domain.SpeciesSearchPatterns(in.Q)
	rows, err := s.store.SearchSpecies(ctx, SpeciesSearch{
		RegulationID: reg.ID, Pattern: pattern, MegaPattern: megaPattern, Limit: limit,
	})
	if err != nil {
		return nil, domain.Unavailable("SearchSpecies", err)
	}
	return rows, nil
}

// SearchMoves は GET /api/pokedex/moves の手順。
func (s *Service) SearchMoves(ctx context.Context, in NameQuery) ([]PublicMove, error) {
	limit, err := domain.ResolveLimit(in.Limit)
	if err != nil {
		return nil, err
	}
	reg, err := s.store.GetDefaultRegulation(ctx)
	if err != nil {
		return nil, domain.Unavailable("SearchMoves/GetDefaultRegulation", err)
	}
	rows, err := s.store.SearchMoves(ctx, NameSearch{
		RegulationID: reg.ID, Pattern: domain.LikePattern(in.Q), Limit: limit,
	})
	if err != nil {
		return nil, domain.Unavailable("SearchMoves", err)
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	extras, err := publicMoveExtras(ctx, s.store, ids)
	if err != nil {
		return nil, err
	}
	out := make([]PublicMove, 0, len(rows))
	for _, r := range rows {
		target, err := publicMoveTarget(r.ID, r.Target)
		if err != nil {
			return nil, err
		}
		out = append(out, extras.move(r, target))
	}
	return out, nil
}

// moveExtras は公開 API の技の機構・フラグ(ADR-0178 §4)。値は昇順・検証済み。
type moveExtras struct {
	mechanismsByID map[string][]string
	flagsKnown     bool
	flagsByID      map[string][]string
	// paramsByID・ruleByID は機構の中身と処理の定義(ADR-0143 §6)。行の無い技はキーごと省く。
	paramsByID map[string]*MechanismParams
	ruleByID   map[string]*domain.Effect
	// effectByID は確率 100% の追加効果(ランク変化。ADR-0253)。確率 100% 未満・行の無い技はキーごと省く。
	effectByID map[string]*domain.Effect
}

// move は技の行と分類済みの対象に、機構・フラグなどを付ける。
func (e moveExtras) move(r Move, target *string) PublicMove {
	return PublicMove{
		ID: r.ID, NameJa: r.NameJa, Type: r.Type, Category: r.Category,
		Power: r.Power, Priority: r.Priority, Target: target,
		Mechanisms: e.mechanisms(r.ID), Flags: e.flags(r.ID),
		MechanismParams: e.paramsByID[r.ID], Rule: e.ruleByID[r.ID], Effect: e.effectByID[r.ID],
	}
}

// mechanisms は技の機構を返す(常に返す。通常の技は空配列)。
func (e moveExtras) mechanisms(id string) []string {
	m := e.mechanismsByID[id]
	if m == nil {
		m = []string{}
	}
	return m
}

// flags は技のフラグを返す。まだ取り込んでいない(move_flags が空)なら nil でキーを省く。
func (e moveExtras) flags(id string) *[]string {
	if !e.flagsKnown {
		return nil
	}
	f := e.flagsByID[id]
	if f == nil {
		f = []string{}
	}
	return &f
}

// publicMoveExtras は技の ID で機構・フラグを引く。語彙に無い値(CHECK をすり抜けた値)は黙って捨てず
// 503 master_unavailable(target と同じ扱い。ADR-0178 §4)。
func publicMoveExtras(ctx context.Context, q Reader, ids []string) (moveExtras, error) {
	out := moveExtras{
		mechanismsByID: map[string][]string{}, flagsByID: map[string][]string{},
		paramsByID: map[string]*MechanismParams{}, ruleByID: map[string]*domain.Effect{},
		effectByID: map[string]*domain.Effect{},
	}
	if len(ids) == 0 {
		return out, nil
	}
	mechanisms, err := q.ListMoveMechanismsByMoveIDs(ctx, ids)
	if err != nil {
		return moveExtras{}, domain.Unavailable("ListMoveMechanismsByMoveIDs", err)
	}
	for _, m := range mechanisms {
		if !engine.MoveMechanism(m.Mechanism).Known() {
			return moveExtras{}, domain.Unavailable("move mechanism: "+m.MoveID, fmt.Errorf("未知の技の機構: %q", m.Mechanism))
		}
		out.mechanismsByID[m.MoveID] = append(out.mechanismsByID[m.MoveID], m.Mechanism)
	}
	for id := range out.mechanismsByID {
		sort.Strings(out.mechanismsByID[id])
	}
	params, err := q.ListMoveMechanismParamsByMoveIDs(ctx, ids)
	if err != nil {
		return moveExtras{}, domain.Unavailable("ListMoveMechanismParamsByMoveIDs", err)
	}
	for _, p := range params {
		out.paramsByID[p.MoveID] = mechanismParams(p)
	}
	rules, err := q.ListMoveRulesByMoveIDs(ctx, ids)
	if err != nil {
		return moveExtras{}, domain.Unavailable("ListMoveRulesByMoveIDs", err)
	}
	var chart engine.TypeChart // 定義を持つ行があるときだけ読む
	for i := range rules {
		// 共通マスタで厳格に検証してから返す(検証に通らない定義は黙って捨てず 503。効果と同じ扱い)。
		rule, _, err := publicEffect(ctx, q, &chart, &rules[i].Rule, master.DecodeMoveRule)
		if err != nil {
			return moveExtras{}, domain.Unavailable("move rule: "+rules[i].MoveID, err)
		}
		out.ruleByID[rules[i].MoveID] = rule
	}
	effects, err := q.ListMoveEffectsByMoveIDs(ctx, ids)
	if err != nil {
		return moveExtras{}, domain.Unavailable("ListMoveEffectsByMoveIDs", err)
	}
	for _, e := range effects {
		effect, err := publicGuaranteedMoveEffect(e.Effect)
		if err != nil {
			return moveExtras{}, domain.Unavailable("move effect: "+e.MoveID, err)
		}
		if effect != nil {
			out.effectByID[e.MoveID] = effect
		}
	}
	if out.flagsKnown, err = q.HasMoveFlags(ctx); err != nil {
		return moveExtras{}, domain.Unavailable("HasMoveFlags", err)
	}
	if !out.flagsKnown {
		return out, nil
	}
	flags, err := q.ListMoveFlagsByMoveIDs(ctx, ids)
	if err != nil {
		return moveExtras{}, domain.Unavailable("ListMoveFlagsByMoveIDs", err)
	}
	for _, f := range flags {
		if !master.IsMoveFlag(f.Flag) {
			return moveExtras{}, domain.Unavailable("move flag: "+f.MoveID, fmt.Errorf("未知の技のフラグ: %q", f.Flag))
		}
		out.flagsByID[f.MoveID] = append(out.flagsByID[f.MoveID], f.Flag)
	}
	for id := range out.flagsByID {
		sort.Slice(out.flagsByID[id], func(i, j int) bool { return out.flagsByID[id][i] < out.flagsByID[id][j] })
	}
	return out, nil
}

// publicGuaranteedMoveEffect は技の追加効果の行を共通マスタ(master.DecodeMoveEffect)で厳格に検証し、確率 100% のときだけ
// 数値の字面を保った Effect にする(ADR-0253)。確率 100% 未満は nil(キーを省く)。検証に通らなければ error
// (確率 100% 未満の行でも黙って捨てない。呼び出し側が 503 にする)。
func publicGuaranteedMoveEffect(raw json.RawMessage) (*domain.Effect, error) {
	decoded, err := master.DecodeMoveEffect(raw)
	if err != nil {
		return nil, err
	}
	if decoded.Chance != 100 {
		return nil, nil
	}
	return domain.EffectVerbatim(raw)
}

// publicMoveTarget は技の対象を公開 API の分類(single/spread)にする。NULL はキーを省く(nil)。
// 未知の値は分類できないので 503 master_unavailable(ADR-0223 §4。効果と同じ扱い)。
func publicMoveTarget(moveID string, v *string) (*string, error) {
	if v == nil {
		return nil, nil
	}
	if !master.IsMoveTarget(*v) {
		return nil, domain.Unavailable("move target: "+moveID, fmt.Errorf("未知の技の対象: %q", *v))
	}
	out := string(master.MoveTarget(*v).Engine())
	return &out, nil
}

// SearchItems は GET /api/pokedex/items の手順。
func (s *Service) SearchItems(ctx context.Context, in NameQuery) ([]PublicItem, error) {
	limit, err := domain.ResolveLimit(in.Limit)
	if err != nil {
		return nil, err
	}
	// 一覧と相性表(効果の検証用)は1つの読み取り専用トランザクションで読む(ADR-0127・ADR-0218 §3)。
	tx, err := s.store.BeginSnapshot(ctx)
	if err != nil {
		return nil, domain.Unavailable("SearchItems/BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }()
	reg, err := tx.GetDefaultRegulation(ctx)
	if err != nil {
		return nil, domain.Unavailable("SearchItems/GetDefaultRegulation", err)
	}
	rows, err := tx.SearchItems(ctx, NameSearch{
		RegulationID: reg.ID, Pattern: domain.LikePattern(in.Q), Limit: limit,
	})
	if err != nil {
		return nil, domain.Unavailable("SearchItems", err)
	}
	var chart engine.TypeChart // 効果を持つ行があるときだけ読む
	out := make([]PublicItem, 0, len(rows))
	for _, r := range rows {
		effect, decoded, err := publicEffect(ctx, tx, &chart, r.Effect, master.DecodeItemEffect)
		if err != nil {
			return nil, domain.Unavailable("SearchItems/effect:"+r.ID, err)
		}
		// ItemRoles は nil を返さない(空でも JSON で [] を出す)。isMegaStone は常にキーごと出す(ADR-0175)。
		roles := master.ItemRoles(decoded, r.IsMegaStone)
		outRoles := make([]string, 0, len(roles))
		for _, role := range roles {
			outRoles = append(outRoles, string(role))
		}
		out = append(out, PublicItem{ID: r.ID, NameJa: r.NameJa, Effect: effect, Roles: outRoles, IsMegaStone: r.IsMegaStone, FlingPower: r.FlingPower})
	}
	// 読み終えたらすぐ閉じて接続を返す(応答の書き込みを待たない)。エラー経路は defer の Rollback が閉じる。
	if err := tx.Commit(); err != nil {
		return nil, domain.Unavailable("SearchItems/Commit", err)
	}
	return out, nil
}

// GetSpecies は GET /api/pokedex/species/{key} の手順。使用可能集合の外の種族も返す(詳細はマスタの参照)。
// learnset は習得技 ∩ 既定のレギュレーションの使用可能な技(ID 昇順)。
func (s *Service) GetSpecies(ctx context.Context, key string) (SpeciesDetail, error) {
	if err := domain.CheckSpeciesKey(key); err != nil {
		return SpeciesDetail{}, err
	}
	tx, err := s.store.BeginSnapshot(ctx)
	if err != nil {
		return SpeciesDetail{}, domain.Unavailable("GetSpecies/BeginTx", err)
	}
	defer func() { _ = tx.Rollback() }()
	reg, err := tx.GetDefaultRegulation(ctx)
	if err != nil {
		return SpeciesDetail{}, domain.Unavailable("GetSpecies/GetDefaultRegulation", err)
	}
	sp, err := tx.GetSpeciesByKey(ctx, key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return SpeciesDetail{}, domain.Errorf(domain.CodeNotFound, "種族が無い: %s", key)
		}
		return SpeciesDetail{}, domain.Unavailable("GetSpeciesByKey", err)
	}
	abilityRows, err := tx.ListSpeciesAbilityNames(ctx, key)
	if err != nil {
		return SpeciesDetail{}, domain.Unavailable("ListSpeciesAbilityNames", err)
	}
	sort.Slice(abilityRows, func(i, j int) bool { return abilityRows[i].Slot < abilityRows[j].Slot })
	abilities := make([]PublicAbility, 0, len(abilityRows))
	var chart engine.TypeChart // 効果を持つ行があるときだけ読む
	for _, a := range abilityRows {
		effect, _, err := publicEffect(ctx, tx, &chart, a.Effect, master.DecodeAbilityEffect)
		if err != nil {
			return SpeciesDetail{}, domain.Unavailable("GetSpecies/effect:"+a.ID, err)
		}
		abilities = append(abilities, PublicAbility{ID: a.ID, NameJa: a.NameJa, Effect: effect})
	}
	learnset, err := tx.ListSpeciesLearnset(ctx, key, reg.ID)
	if err != nil {
		return SpeciesDetail{}, domain.Unavailable("ListSpeciesLearnset", err)
	}
	sort.Strings(learnset)
	if learnset == nil {
		learnset = []string{}
	}
	// 基本種の名前も同じ読み取り専用トランザクションで読む(1スナップショット。ADR-0127・ADR-0175 §3)。
	var baseNameJa *string
	if sp.BaseSpeciesKey != nil {
		base, err := tx.GetSpeciesByKey(ctx, *sp.BaseSpeciesKey)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return SpeciesDetail{}, domain.Unavailable("GetSpecies/GetBaseSpecies", err)
		}
		if err == nil {
			baseNameJa = &base.NameJa
		}
	}
	if err := tx.Commit(); err != nil {
		return SpeciesDetail{}, domain.Unavailable("GetSpecies/Commit", err)
	}
	return SpeciesDetail{Species: sp, Abilities: abilities, Learnset: learnset, BaseSpeciesNameJa: baseNameJa}, nil
}

// GetMove は GET /api/pokedex/moves/{key} の手順。使用可能集合の外の技も返す(絞り込みは検索の仕事)。
// 判定レーンが技の優先度(priority)を個別に引くための経路(2026-09-22 の依頼。DECISIONS.md)。
func (s *Service) GetMove(ctx context.Context, key string) (PublicMove, error) {
	row, err := s.store.GetMove(ctx, key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return PublicMove{}, domain.Errorf(domain.CodeNotFound, "技が無い: %s", key)
		}
		return PublicMove{}, domain.Unavailable("GetMove", err)
	}
	target, err := publicMoveTarget(row.ID, row.Target)
	if err != nil {
		return PublicMove{}, err
	}
	extras, err := publicMoveExtras(ctx, s.store, []string{row.ID})
	if err != nil {
		return PublicMove{}, err
	}
	return extras.move(row, target), nil
}

// GetMovesByIDs は GET /api/pokedex/moves/batch の手順。getMove の複数版(ADR-0304 §3)。
// getMove と同様に既定のレギュレーションで絞らない。見つからなかった ID は黙って省き、
// 結果は ids と同じ順にする(DB の IN 句は順序を保証しないため、ここで並べ替える)。
func (s *Service) GetMovesByIDs(ctx context.Context, ids []string) ([]PublicMove, error) {
	if err := domain.CheckBatchIDs(ids); err != nil {
		return nil, err
	}
	rows, err := s.store.GetMovesByIDs(ctx, ids)
	if err != nil {
		return nil, domain.Unavailable("GetMovesByIDs", err)
	}
	byID := make(map[string]Move, len(rows))
	found := make([]string, 0, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
		found = append(found, r.ID)
	}
	extras, err := publicMoveExtras(ctx, s.store, found)
	if err != nil {
		return nil, err
	}
	out := make([]PublicMove, 0, len(ids))
	for _, id := range ids {
		r, ok := byID[id]
		if !ok {
			continue
		}
		target, err := publicMoveTarget(r.ID, r.Target)
		if err != nil {
			return nil, err
		}
		out = append(out, extras.move(r, target))
	}
	return out, nil
}

// ListNatures は GET /api/pokedex/natures の手順。0行なら 503 master_unavailable。
func (s *Service) ListNatures(ctx context.Context) ([]Nature, error) {
	rows, err := s.store.ListNatures(ctx)
	if err != nil {
		return nil, domain.Unavailable("ListNatures", err)
	}
	if len(rows) == 0 {
		return nil, domain.Unavailable("ListNatures", domain.ErrEmpty("natures"))
	}
	return rows, nil
}

// ListMoveLearners は GET /api/pokedex/moves/{key}/learners(技を覚える種族の一覧。ADR-0251)の手順。
// 判定の順: 入力の検証 → 既定のレギュレーション(無ければ 503) → 技の存在(無ければ 404) → 逆引き。
// 技がマスタにあって使用可能集合の外なら空。
func (s *Service) ListMoveLearners(ctx context.Context, in LearnersQuery) ([]SpeciesSummary, error) {
	limit, err := domain.ResolveLimit(in.Limit)
	if err != nil {
		return nil, err
	}
	offset, err := domain.ResolveOffset(in.Offset)
	if err != nil {
		return nil, err
	}
	reg, err := s.store.GetDefaultRegulation(ctx)
	if err != nil {
		return nil, domain.Unavailable("ListMoveLearners/GetDefaultRegulation", err)
	}
	move, err := s.store.GetMove(ctx, in.MoveKey)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, domain.Errorf(domain.CodeNotFound, "技が無い: %s", in.MoveKey)
		}
		return nil, domain.Unavailable("ListMoveLearners/GetMove", err)
	}
	rows, err := s.store.ListMoveLearners(ctx, MoveLearnersQuery{
		RegulationID: reg.ID, MoveID: move.ID, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, domain.Unavailable("ListMoveLearners", err)
	}
	return rows, nil
}

// publicEffect は公開 API の effect(ADR-0218)。行が無ければ nil(キーを省く)。あれば共通マスタで
// 厳格に検証してから、数値の字面を保ったまま Effect にする。検証に通らなければ error
// (呼び出し側が 503 にする。error の文面は本文に出ない)。相性表は最初に必要になったときだけ chart に読む。
func publicEffect[T any](ctx context.Context, q Reader, chart *engine.TypeChart, rawp *json.RawMessage,
	decode func([]byte, engine.TypeChart) (*T, error)) (*domain.Effect, *T, error) {
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
	me, err := domain.EffectVerbatim(raw)
	return me, decoded, err
}

// loadTypeChart は types / type_chart から engine.TypeChart を作る(services/internal/master 経由)。
func loadTypeChart(ctx context.Context, q Reader) (engine.TypeChart, error) {
	types, err := q.ListTypes(ctx)
	if err != nil {
		return engine.TypeChart{}, err
	}
	chart, err := q.ListTypeChart(ctx)
	if err != nil {
		return engine.TypeChart{}, err
	}
	return typeChartOf(types, chart)
}

// typeChartOf は行から engine.TypeChart を組み立てる(services/internal/master 経由。ADR-0100 §6)。
func typeChartOf(types []Type, chart []TypeChartEntry) (engine.TypeChart, error) {
	typeRows := make([]master.TypeRow, 0, len(types))
	for _, t := range types {
		typeRows = append(typeRows, master.TypeRow{ID: t.ID, SortOrder: t.SortOrder, NameJa: t.NameJa})
	}
	chartRows := make([]master.TypeChartRow, 0, len(chart))
	for _, c := range chart {
		chartRows = append(chartRows, master.TypeChartRow{AttackType: c.AttackType, DefenseType: c.DefenseType, Code: c.Code})
	}
	return master.TypeChart(typeRows, chartRows)
}

// mechanismParams は move_mechanism_params の行を契約の機構の中身にする(ADR-0142 §8)。
// 無い項目(NULL)は nil。値の検証は受け取った calc-svc が行う。
func mechanismParams(p MoveMechanismParams) *MechanismParams {
	out := &MechanismParams{
		OffenseStat:    p.OffenseStat,
		OffensePokemon: p.OffensePokemon,
		DefenseStat:    p.DefenseStat,
	}
	if p.MultiHitMin != nil || p.MultiHitMax != nil {
		out.MultiHit = &MultiHit{Min: intOrZero(p.MultiHitMin), Max: intOrZero(p.MultiHitMax)}
	}
	if p.FixedDamageLevel != nil || p.FixedDamageValue != nil {
		out.FixedDamage = &FixedDamage{Level: p.FixedDamageLevel != nil && *p.FixedDamageLevel, Value: intOrZero(p.FixedDamageValue)}
	}
	if p.Ohko != nil && *p.Ohko {
		out.Ohko = &OHKO{ImmuneType: p.OhkoImmuneType}
	}
	return out
}

func intOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
