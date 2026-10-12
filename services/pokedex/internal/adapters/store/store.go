// Package store は app のポート(Reader・Snapshot・Store・SnapshotBeginner)を、sqlc の生成物
// (services/pokedex/internal/store)と readtx(読み取り専用トランザクション)で実装する(ADR-0811・ADR-0180)。
//
// 呼び出す sqlc のメソッド・引数・順番は app の呼び出しと1対1で、ここでは足さない・まとめない
// (テストの偽の DB〈storetest〉が呼び出しの列を確かめている)。行は app の型へ写す(NULL は nil)。
// 1行を引く読み出しの sql.ErrNoRows は app.ErrNotFound としても判定できるように包む(文面は元のまま)。
package store

import (
	"context"
	"database/sql"
	"errors"

	"example.com/pokecalc/services/pokedex/internal/adapters/readtx"
	"example.com/pokecalc/services/pokedex/internal/app"
	"example.com/pokecalc/services/pokedex/internal/domain"
	sqlc "example.com/pokecalc/services/pokedex/internal/store"
)

// snapshotOptions は一貫したスナップショットの読み出し(ADR-0127・ADR-0218 §3)。
var snapshotOptions = sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}

// readOnlyOptions は readiness の読み出し(ADR-0129 §1。分離レベルは DB の既定)。
var readOnlyOptions = sql.TxOptions{ReadOnly: true}

// DB は app.Store の実装(autocommit の読み出しとトランザクションの開始)。
type DB struct {
	Reader
	db readtx.DB
}

var _ app.Store = (*DB)(nil)

// New は readtx.DB(本番は readtx.NewDB(*sql.DB)、テストは storetest.Querier)を app.Store にする。
func New(db readtx.DB) *DB {
	return &DB{Reader: Reader{q: db}, db: db}
}

// BeginSnapshot は読み取り専用・REPEATABLE READ のトランザクションを開ける。
func (d *DB) BeginSnapshot(ctx context.Context) (app.Snapshot, error) {
	return begin(ctx, d.db, &snapshotOptions)
}

// BeginReadOnly は読み取り専用のトランザクション(分離レベルは DB の既定)を開ける。
func (d *DB) BeginReadOnly(ctx context.Context) (app.Snapshot, error) {
	return begin(ctx, d.db, &readOnlyOptions)
}

// Beginner は app.SnapshotBeginner の実装(pokedex export。トランザクションの開始だけを持つ)。
type Beginner struct {
	b readtx.Beginner
}

var _ app.SnapshotBeginner = Beginner{}

// NewBeginner は readtx.Beginner を app.SnapshotBeginner にする。
func NewBeginner(b readtx.Beginner) Beginner {
	return Beginner{b: b}
}

// BeginSnapshot は読み取り専用・REPEATABLE READ のトランザクションを開ける。
func (b Beginner) BeginSnapshot(ctx context.Context) (app.Snapshot, error) {
	return begin(ctx, b.b, &snapshotOptions)
}

func begin(ctx context.Context, b readtx.Beginner, opts *sql.TxOptions) (app.Snapshot, error) {
	o := *opts // 呼び出し側(偽の DB)が記録しても共有の値を書き換えられないように複製する
	tx, err := b.BeginTx(ctx, &o)
	if err != nil {
		return nil, err
	}
	return &snapshot{Reader: Reader{q: tx}, tx: tx}, nil
}

type snapshot struct {
	Reader
	tx readtx.Tx
}

func (s *snapshot) Commit() error   { return s.tx.Commit() }
func (s *snapshot) Rollback() error { return s.tx.Rollback() }

// Reader は app.Reader の実装(sqlc の Querier を包む)。
type Reader struct {
	q sqlc.Querier
}

var _ app.Reader = Reader{}

// notFound は sql.ErrNoRows を app.ErrNotFound としても判定できるように包む。文面と Unwrap は元のまま
// (ログに出る文面を変えない。errors.Is(err, sql.ErrNoRows) も今までどおり真)。
type notFound struct{ err error }

func (e notFound) Error() string        { return e.err.Error() }
func (e notFound) Unwrap() error        { return e.err }
func (e notFound) Is(target error) bool { return target == app.ErrNotFound }

func wrapNoRows(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return notFound{err: err}
	}
	return err
}

// mapRows は行を写す。nil は nil のまま(空の結果の nil・空の区別を変えない)。
func mapRows[S, T any](in []S, f func(S) T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}

func strPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func int16Ptr(v sql.NullInt16) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int16)
	return &n
}

func boolPtr(v sql.NullBool) *bool {
	if !v.Valid {
		return nil
	}
	b := v.Bool
	return &b
}

func (r Reader) GetDefaultRegulation(ctx context.Context) (app.Regulation, error) {
	row, err := r.q.GetDefaultRegulation(ctx)
	if err != nil {
		return app.Regulation{}, wrapNoRows(err)
	}
	return app.Regulation{ID: row.ID}, nil
}

func (r Reader) ListDataVersions(ctx context.Context) ([]domain.DataVersion, error) {
	rows, err := r.q.ListDataVersions(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(v sqlc.DataVersion) domain.DataVersion {
		return domain.DataVersion{Source: v.Source, Version: v.Version, Checksum: v.Checksum}
	}), nil
}

func (r Reader) ListTypes(ctx context.Context) ([]app.Type, error) {
	rows, err := r.q.ListTypes(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(t sqlc.Type) app.Type {
		return app.Type{ID: t.ID, SortOrder: int(t.SortOrder), NameJa: t.NameJa}
	}), nil
}

func (r Reader) ListTypeChart(ctx context.Context) ([]app.TypeChartEntry, error) {
	rows, err := r.q.ListTypeChart(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(c sqlc.TypeChart) app.TypeChartEntry {
		return app.TypeChartEntry{AttackType: c.AttackType, DefenseType: c.DefenseType, Code: int(c.Code)}
	}), nil
}

func (r Reader) SearchSpecies(ctx context.Context, arg app.SpeciesSearch) ([]app.SpeciesSummary, error) {
	rows, err := r.q.SearchSpecies(ctx, sqlc.SearchSpeciesParams{
		RegulationID: arg.RegulationID, Pattern: arg.Pattern, MegaPattern: arg.MegaPattern, Limit: arg.Limit,
	})
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(s sqlc.SearchSpeciesRow) app.SpeciesSummary {
		return app.SpeciesSummary{Key: s.Key, DexNo: int(s.DexNo), Form: int(s.Form), NameJa: s.NameJa, Type1: s.Type1, Type2: strPtr(s.Type2)}
	}), nil
}

func (r Reader) SearchMoves(ctx context.Context, arg app.NameSearch) ([]app.Move, error) {
	rows, err := r.q.SearchMoves(ctx, sqlc.SearchMovesParams{RegulationID: arg.RegulationID, Pattern: arg.Pattern, Limit: arg.Limit})
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(m sqlc.SearchMovesRow) app.Move {
		return moveOf(m.ID, m.NameJa, m.Type, m.Category, m.Power, m.Priority, m.Target)
	}), nil
}

func (r Reader) SearchItems(ctx context.Context, arg app.NameSearch) ([]app.ItemWithEffect, error) {
	rows, err := r.q.SearchItems(ctx, sqlc.SearchItemsParams{RegulationID: arg.RegulationID, Pattern: arg.Pattern, Limit: arg.Limit})
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(it sqlc.SearchItemsRow) app.ItemWithEffect {
		return app.ItemWithEffect{ID: it.ID, NameJa: it.NameJa, Effect: it.Effect, IsMegaStone: it.IsMegaStone, FlingPower: int16Ptr(it.FlingPower)}
	}), nil
}

func (r Reader) GetSpeciesByKey(ctx context.Context, key string) (app.Species, error) {
	row, err := r.q.GetSpeciesByKey(ctx, key)
	if err != nil {
		return app.Species{}, wrapNoRows(err)
	}
	return speciesOf(row), nil
}

func (r Reader) ListSpeciesAbilityNames(ctx context.Context, speciesKey string) ([]app.SpeciesAbilityName, error) {
	rows, err := r.q.ListSpeciesAbilityNames(ctx, speciesKey)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(a sqlc.ListSpeciesAbilityNamesRow) app.SpeciesAbilityName {
		return app.SpeciesAbilityName{Slot: int(a.Slot), ID: a.ID, NameJa: a.NameJa, Effect: a.Effect}
	}), nil
}

func (r Reader) ListSpeciesLearnset(ctx context.Context, speciesKey, regulationID string) ([]string, error) {
	return r.q.ListSpeciesLearnset(ctx, sqlc.ListSpeciesLearnsetParams{SpeciesKey: speciesKey, RegulationID: regulationID})
}

func (r Reader) GetMove(ctx context.Context, id string) (app.Move, error) {
	m, err := r.q.GetMove(ctx, id)
	if err != nil {
		return app.Move{}, wrapNoRows(err)
	}
	return moveOf(m.ID, m.NameJa, m.Type, m.Category, m.Power, m.Priority, m.Target), nil
}

func (r Reader) GetMovesByIDs(ctx context.Context, ids []string) ([]app.Move, error) {
	rows, err := r.q.GetMovesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(m sqlc.GetMovesByIDsRow) app.Move {
		return moveOf(m.ID, m.NameJa, m.Type, m.Category, m.Power, m.Priority, m.Target)
	}), nil
}

func (r Reader) ListMoveLearners(ctx context.Context, arg app.MoveLearnersQuery) ([]app.SpeciesSummary, error) {
	rows, err := r.q.ListMoveLearners(ctx, sqlc.ListMoveLearnersParams{
		RegulationID: arg.RegulationID, MoveID: arg.MoveID, Limit: arg.Limit, Offset: arg.Offset,
	})
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(s sqlc.ListMoveLearnersRow) app.SpeciesSummary {
		return app.SpeciesSummary{Key: s.Key, DexNo: int(s.DexNo), Form: int(s.Form), NameJa: s.NameJa, Type1: s.Type1, Type2: strPtr(s.Type2)}
	}), nil
}

func (r Reader) ListNatures(ctx context.Context) ([]app.Nature, error) {
	rows, err := r.q.ListNatures(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(n sqlc.Nature) app.Nature {
		return app.Nature{ID: n.ID, NameJa: n.NameJa, Plus: strPtr(n.Plus), Minus: strPtr(n.Minus)}
	}), nil
}

func (r Reader) ListMoveMechanismsByMoveIDs(ctx context.Context, ids []string) ([]app.MoveMechanism, error) {
	rows, err := r.q.ListMoveMechanismsByMoveIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, mechanismOf), nil
}

func (r Reader) ListMoveMechanismParamsByMoveIDs(ctx context.Context, ids []string) ([]app.MoveMechanismParams, error) {
	rows, err := r.q.ListMoveMechanismParamsByMoveIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, mechanismParamsOf), nil
}

func (r Reader) ListMoveRulesByMoveIDs(ctx context.Context, ids []string) ([]app.MoveRule, error) {
	rows, err := r.q.ListMoveRulesByMoveIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, ruleOf), nil
}

func (r Reader) ListMoveEffectsByMoveIDs(ctx context.Context, ids []string) ([]app.MoveEffect, error) {
	rows, err := r.q.ListMoveEffectsByMoveIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, moveEffectOf), nil
}

func (r Reader) HasMoveFlags(ctx context.Context) (bool, error) {
	return r.q.HasMoveFlags(ctx)
}

func (r Reader) ListMoveFlagsByMoveIDs(ctx context.Context, ids []string) ([]app.MoveFlag, error) {
	rows, err := r.q.ListMoveFlagsByMoveIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, flagOf), nil
}

func (r Reader) ListSpecies(ctx context.Context) ([]app.Species, error) {
	rows, err := r.q.ListSpecies(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, speciesOf), nil
}

func (r Reader) ListAllSpeciesAbilities(ctx context.Context) ([]app.SpeciesAbility, error) {
	rows, err := r.q.ListAllSpeciesAbilities(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(sa sqlc.SpeciesAbility) app.SpeciesAbility {
		return app.SpeciesAbility{SpeciesKey: sa.SpeciesKey, Slot: int(sa.Slot), AbilityID: sa.AbilityID}
	}), nil
}

func (r Reader) ListMoves(ctx context.Context) ([]app.Move, error) {
	rows, err := r.q.ListMoves(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(m sqlc.Move) app.Move {
		return moveOf(m.ID, m.NameJa, m.Type, m.Category, m.Power, m.Priority, m.Target)
	}), nil
}

func (r Reader) ListMoveEffects(ctx context.Context) ([]app.MoveEffect, error) {
	rows, err := r.q.ListMoveEffects(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, moveEffectOf), nil
}

func (r Reader) ListMoveMechanisms(ctx context.Context) ([]app.MoveMechanism, error) {
	rows, err := r.q.ListMoveMechanisms(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, mechanismOf), nil
}

func (r Reader) ListMoveMechanismParams(ctx context.Context) ([]app.MoveMechanismParams, error) {
	rows, err := r.q.ListMoveMechanismParams(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, mechanismParamsOf), nil
}

func (r Reader) ListMoveRules(ctx context.Context) ([]app.MoveRule, error) {
	rows, err := r.q.ListMoveRules(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, ruleOf), nil
}

func (r Reader) ListMoveFlags(ctx context.Context) ([]app.MoveFlag, error) {
	rows, err := r.q.ListMoveFlags(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, flagOf), nil
}

func (r Reader) ListItems(ctx context.Context) ([]app.Item, error) {
	rows, err := r.q.ListItems(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(it sqlc.Item) app.Item {
		return app.Item{ID: it.ID, NameJa: it.NameJa, FlingPower: int16Ptr(it.FlingPower)}
	}), nil
}

func (r Reader) ListItemEffects(ctx context.Context) ([]app.ItemEffect, error) {
	rows, err := r.q.ListItemEffects(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(e sqlc.ItemEffect) app.ItemEffect {
		return app.ItemEffect{ItemID: e.ItemID, Effect: e.Effect}
	}), nil
}

func (r Reader) ListAbilities(ctx context.Context) ([]app.Ability, error) {
	rows, err := r.q.ListAbilities(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(a sqlc.Ability) app.Ability {
		return app.Ability{ID: a.ID, NameJa: a.NameJa}
	}), nil
}

func (r Reader) ListAbilityEffects(ctx context.Context) ([]app.AbilityEffect, error) {
	rows, err := r.q.ListAbilityEffects(ctx)
	if err != nil {
		return nil, err
	}
	return mapRows(rows, func(e sqlc.AbilityEffect) app.AbilityEffect {
		return app.AbilityEffect{AbilityID: e.AbilityID, Effect: e.Effect}
	}), nil
}

func (r Reader) ListRegulationSpeciesKeys(ctx context.Context, regulationID string) ([]string, error) {
	return r.q.ListRegulationSpeciesKeys(ctx, regulationID)
}

func (r Reader) ListRegulationMoveIDs(ctx context.Context, regulationID string) ([]string, error) {
	return r.q.ListRegulationMoveIDs(ctx, regulationID)
}

func (r Reader) ListRegulationAbilityIDs(ctx context.Context, regulationID string) ([]string, error) {
	return r.q.ListRegulationAbilityIDs(ctx, regulationID)
}

// --- 行の写し ---------------------------------------------------------------------------

func moveOf(id, nameJa, typ, category string, power uint16, priority int8, target sql.NullString) app.Move {
	return app.Move{ID: id, NameJa: nameJa, Type: typ, Category: category, Power: int(power), Priority: int(priority), Target: strPtr(target)}
}

func speciesOf(s sqlc.Species) app.Species {
	return app.Species{
		Key: s.Key, DexNo: int(s.DexNo), Form: int(s.Form), ShowdownID: s.ShowdownID, NameJa: s.NameJa,
		Type1: s.Type1, Type2: strPtr(s.Type2),
		BaseStats: app.StatBlock{Hp: int(s.BaseHp), Atk: int(s.BaseAtk), Def: int(s.BaseDef), Spa: int(s.BaseSpa), Spd: int(s.BaseSpd), Spe: int(s.BaseSpe)},
		IsMega:    s.IsMega, BaseSpeciesKey: strPtr(s.BaseSpeciesKey), RequiredItemID: strPtr(s.RequiredItemID),
		WeightHg: int16Ptr(s.WeightHg),
	}
}

func mechanismOf(m sqlc.MoveMechanism) app.MoveMechanism {
	return app.MoveMechanism{MoveID: m.MoveID, Mechanism: m.Mechanism}
}

func mechanismParamsOf(p sqlc.MoveMechanismParam) app.MoveMechanismParams {
	return app.MoveMechanismParams{
		MoveID:      p.MoveID,
		MultiHitMin: int16Ptr(p.MultiHitMin), MultiHitMax: int16Ptr(p.MultiHitMax),
		FixedDamageLevel: boolPtr(p.FixedDamageLevel), FixedDamageValue: int16Ptr(p.FixedDamageValue),
		Ohko: boolPtr(p.Ohko), OhkoImmuneType: strPtr(p.OhkoImmuneType),
		OffenseStat: strPtr(p.OffenseStat), OffensePokemon: strPtr(p.OffensePokemon), DefenseStat: strPtr(p.DefenseStat),
	}
}

func ruleOf(r sqlc.MoveRule) app.MoveRule {
	return app.MoveRule{MoveID: r.MoveID, Rule: r.Rule}
}

func moveEffectOf(e sqlc.MoveEffect) app.MoveEffect {
	return app.MoveEffect{MoveID: e.MoveID, Effect: e.Effect}
}

func flagOf(f sqlc.MoveFlag) app.MoveFlag {
	return app.MoveFlag{MoveID: f.MoveID, Flag: f.Flag}
}
