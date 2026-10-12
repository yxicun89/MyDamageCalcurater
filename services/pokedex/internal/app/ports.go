package app

import (
	"context"
	"encoding/json"
	"errors"

	"example.com/pokecalc/services/pokedex/internal/domain"
)

// ErrNotFound は1行を引く読み出し(GetDefaultRegulation・GetSpeciesByKey・GetMove)で行が無いこと。
// adapters は元のエラー(sql.ErrNoRows)を包んで返し、errors.Is で判定する(ログに出る文面は元のまま)。
var ErrNotFound = errors.New("行が無い")

// Reader はマスタの読み出し(ADR-0105・ADR-0127)。メソッドは sqlc のクエリと1対1で、名前も同じにする。
// 値の形は DB を知らない app の型(NULL は nil)。
type Reader interface {
	GetDefaultRegulation(ctx context.Context) (Regulation, error)
	ListDataVersions(ctx context.Context) ([]domain.DataVersion, error)
	ListTypes(ctx context.Context) ([]Type, error)
	ListTypeChart(ctx context.Context) ([]TypeChartEntry, error)

	SearchSpecies(ctx context.Context, arg SpeciesSearch) ([]SpeciesSummary, error)
	SearchMoves(ctx context.Context, arg NameSearch) ([]Move, error)
	SearchItems(ctx context.Context, arg NameSearch) ([]ItemWithEffect, error)
	GetSpeciesByKey(ctx context.Context, key string) (Species, error)
	ListSpeciesAbilityNames(ctx context.Context, speciesKey string) ([]SpeciesAbilityName, error)
	ListSpeciesLearnset(ctx context.Context, speciesKey, regulationID string) ([]string, error)
	GetMove(ctx context.Context, id string) (Move, error)
	GetMovesByIDs(ctx context.Context, ids []string) ([]Move, error)
	ListMoveLearners(ctx context.Context, arg MoveLearnersQuery) ([]SpeciesSummary, error)
	ListNatures(ctx context.Context) ([]Nature, error)

	ListMoveMechanismsByMoveIDs(ctx context.Context, ids []string) ([]MoveMechanism, error)
	ListMoveMechanismParamsByMoveIDs(ctx context.Context, ids []string) ([]MoveMechanismParams, error)
	ListMoveRulesByMoveIDs(ctx context.Context, ids []string) ([]MoveRule, error)
	ListMoveEffectsByMoveIDs(ctx context.Context, ids []string) ([]MoveEffect, error)
	HasMoveFlags(ctx context.Context) (bool, error)
	ListMoveFlagsByMoveIDs(ctx context.Context, ids []string) ([]MoveFlag, error)

	ListSpecies(ctx context.Context) ([]Species, error)
	ListAllSpeciesAbilities(ctx context.Context) ([]SpeciesAbility, error)
	ListMoves(ctx context.Context) ([]Move, error)
	ListMoveEffects(ctx context.Context) ([]MoveEffect, error)
	ListMoveMechanisms(ctx context.Context) ([]MoveMechanism, error)
	ListMoveMechanismParams(ctx context.Context) ([]MoveMechanismParams, error)
	ListMoveRules(ctx context.Context) ([]MoveRule, error)
	ListMoveFlags(ctx context.Context) ([]MoveFlag, error)
	ListItems(ctx context.Context) ([]Item, error)
	ListItemEffects(ctx context.Context) ([]ItemEffect, error)
	ListAbilities(ctx context.Context) ([]Ability, error)
	ListAbilityEffects(ctx context.Context) ([]AbilityEffect, error)

	ListRegulationSpeciesKeys(ctx context.Context, regulationID string) ([]string, error)
	ListRegulationMoveIDs(ctx context.Context, regulationID string) ([]string, error)
	ListRegulationAbilityIDs(ctx context.Context, regulationID string) ([]string, error)
}

// Snapshot は1つの読み取り専用トランザクションの中の Reader(ADR-0127)。Commit / Rollback の意味は *sql.Tx と同じ。
type Snapshot interface {
	Reader
	Commit() error
	Rollback() error
}

// SnapshotBeginner は一貫したスナップショット(読み取り専用・REPEATABLE READ)を開ける(ADR-0127・ADR-0218 §3)。
type SnapshotBeginner interface {
	BeginSnapshot(ctx context.Context) (Snapshot, error)
}

// Store は autocommit の読み出し(1回の SELECT で完結する検索)とスナップショットの両方ができる。
type Store interface {
	Reader
	SnapshotBeginner
	// BeginReadOnly は読み取り専用のトランザクション(分離レベルは DB の既定)を開ける(readiness。ADR-0129 §1)。
	BeginReadOnly(ctx context.Context) (Snapshot, error)
}

// --- 読み出しの引数と行(NULL は nil) ---------------------------------------------------

// SpeciesSearch は SearchSpecies の引数(pattern・megaPattern は domain.SpeciesSearchPatterns の値)。
type SpeciesSearch struct {
	RegulationID string
	Pattern      string
	MegaPattern  string
	Limit        int32
}

// NameSearch は SearchMoves・SearchItems の引数(Pattern は domain.LikePattern の値)。
type NameSearch struct {
	RegulationID string
	Pattern      string
	Limit        int32
}

// MoveLearnersQuery は ListMoveLearners の引数。
type MoveLearnersQuery struct {
	RegulationID string
	MoveID       string
	Limit        int32
	Offset       int32
}

// Regulation は既定のレギュレーション。
type Regulation struct {
	ID string
}

// Type はタイプ。
type Type struct {
	ID        string
	SortOrder int
	NameJa    string
}

// TypeChartEntry は相性表の1行(Code は 0/1/2/4)。
type TypeChartEntry struct {
	AttackType  string
	DefenseType string
	Code        int
}

// StatBlock は種族値。
type StatBlock struct {
	Hp, Atk, Def, Spa, Spd, Spe int
}

// SpeciesSummary は検索・逆引きの種族の行。
type SpeciesSummary struct {
	Key    string
	DexNo  int
	Form   int
	NameJa string
	Type1  string
	Type2  *string
}

// Species は種族の全項目。
type Species struct {
	Key            string
	DexNo          int
	Form           int
	ShowdownID     string
	NameJa         string
	Type1          string
	Type2          *string
	BaseStats      StatBlock
	IsMega         bool
	BaseSpeciesKey *string
	RequiredItemID *string
	WeightHg       *int
}

// SpeciesAbility は種族の特性の枠。
type SpeciesAbility struct {
	SpeciesKey string
	Slot       int
	AbilityID  string
}

// SpeciesAbilityName は getSpecies の特性(Effect は効果の行が無ければ nil)。
type SpeciesAbilityName struct {
	Slot   int
	ID     string
	NameJa string
	Effect *json.RawMessage
}

// Move は技の行。
type Move struct {
	ID       string
	NameJa   string
	Type     string
	Category string
	Power    int
	Priority int
	Target   *string
}

// MoveMechanism は技の機構の1行。
type MoveMechanism struct {
	MoveID    string
	Mechanism string
}

// MoveMechanismParams は技の機構の中身の行(ADR-0142 §8)。
type MoveMechanismParams struct {
	MoveID           string
	MultiHitMin      *int
	MultiHitMax      *int
	FixedDamageLevel *bool
	FixedDamageValue *int
	Ohko             *bool
	OhkoImmuneType   *string
	OffenseStat      *string
	OffensePokemon   *string
	DefenseStat      *string
}

// MoveRule は技の処理の定義の行(ADR-0143 §6)。
type MoveRule struct {
	MoveID string
	Rule   json.RawMessage
}

// MoveEffect は技の追加効果の行。
type MoveEffect struct {
	MoveID string
	Effect json.RawMessage
}

// MoveFlag は技のフラグの1行(ADR-0178)。
type MoveFlag struct {
	MoveID string
	Flag   string
}

// Item は持ち物の行。
type Item struct {
	ID         string
	NameJa     string
	FlingPower *int
}

// ItemWithEffect は searchItems の行(Effect は効果の行が無ければ nil)。
type ItemWithEffect struct {
	ID          string
	NameJa      string
	Effect      *json.RawMessage
	IsMegaStone bool
	FlingPower  *int
}

// ItemEffect は持ち物の効果の行。
type ItemEffect struct {
	ItemID string
	Effect json.RawMessage
}

// Ability は特性の行。
type Ability struct {
	ID     string
	NameJa string
}

// AbilityEffect は特性の効果の行。
type AbilityEffect struct {
	AbilityID string
	Effect    json.RawMessage
}

// Nature は性格の行(無補正は Plus・Minus が nil)。
type Nature struct {
	ID     string
	NameJa string
	Plus   *string
	Minus  *string
}
