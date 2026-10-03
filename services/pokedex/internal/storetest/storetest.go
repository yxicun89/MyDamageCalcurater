// Package storetest はテスト専用の偽の store.Querier と架空データ(ADR-0105 §7)。
//
// pokedex-svc の読み出し(httpapi・readmodel)のテストが、実 DB の代わりに使う。本番のコードから import しない
// (import してよいのは *_test.go だけ。readmodel / httpapi の静的検査は layout_test で固定する)。
//
// 架空データの規則は ADR-0100 §7 と同じ: 図鑑番号 9001 以降・ID は test で始まる英小文字・日本語名は「テスト」で始まる。
// タイプ ID だけは一般名(fire 等)を使う(balance の JSON Schema が 18 タイプの列挙を持つため)。
//
// Querier に埋め込んだ store.Querier は nil で、上書きしていないメソッドを呼ぶと panic する
// (テストが想定していないクエリ・書き込みを呼んだことに気づくため)。
//
// Querier は readtx.DB も満たす。BeginTx が返す Tx は同じ架空データを読み、呼び出しを Call.InTx: true で
// 記録する。読み出しが1つの読み取り専用トランザクションの中で行われたかは SnapshotViolations /
// RollbackViolations で確かめる(ADR-0127・issue #220)。
package storetest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"example.com/pokecalc/services/pokedex/internal/readtx"
	"example.com/pokecalc/services/pokedex/internal/store"
)

// DefaultRegulationID は架空データの既定のレギュレーション。
const DefaultRegulationID = "test-a"

// ErrDB は DB の失敗を模した値(接続できない等)。
var ErrDB = errors.New("storetest: DB に接続できない")

// Querier は偽の store.Querier。フィールドを書き換えて各テストの状態を作る。
// Err* が非 nil ならそのメソッドはその値を返す。
type Querier struct {
	store.Querier

	DataVersions        []store.DataVersion
	Types               []store.Type
	TypeChart           []store.TypeChart
	Species             []store.Species
	SpeciesAbilities    []store.SpeciesAbility
	Moves               []store.Move
	MoveEffects         []store.MoveEffect
	MoveMechanisms      []store.MoveMechanism
	Items               []store.Item
	ItemEffects         []store.ItemEffect
	Abilities           []store.Ability
	AbilityEffects      []store.AbilityEffect
	Natures             []store.Nature
	Learnsets           []store.Learnset
	DefaultRegulation   *store.GetDefaultRegulationRow // nil = 既定のレギュレーションが無い(sql.ErrNoRows)
	RegulationSpecies   map[string][]string            // regulation_id → species_key
	RegulationMoves     map[string][]string
	RegulationItems     map[string][]string
	RegulationAbilities map[string][]string

	// Err は全メソッドに共通の失敗(nil なら成功)。ErrByMethod はメソッド名ごとの失敗。
	Err         error
	ErrByMethod map[string]error

	mu    sync.Mutex
	Calls []Call // 呼ばれたメソッドと引数(検索のパターン・件数の確認用)

	// root・tx はトランザクションの中の Querier(BeginTx が返す Tx の中身)だけが持つ。
	// root は記録・失敗の注入の持ち主(BeginTx を呼んだ Querier)、tx はそのトランザクションの状態。
	root *Querier
	tx   *txState
	txs  []*txState // BeginTx が開いた(成功した)トランザクション。root の側だけが持つ
}

// Call は呼び出しの記録。InTx は BeginTx が返した Tx 経由の呼び出し(Commit / Rollback を含む)なら真。
// BeginTx 自身は Tx の外の呼び出しとして記録し、Arg に渡された *sql.TxOptions を持つ。
type Call struct {
	Method string
	Arg    any
	InTx   bool
}

// トランザクションの操作の記録名(Call.Method。ErrByMethod のキーにも使える)。
const (
	MethodBeginTx  = "BeginTx"
	MethodCommit   = "Commit"
	MethodRollback = "Rollback"
)

type txState struct {
	done bool
}

func (q *Querier) owner() *Querier {
	if q.root != nil {
		return q.root
	}
	return q
}

func (q *Querier) record(method string, arg any) error {
	o := q.owner()
	o.mu.Lock()
	defer o.mu.Unlock()
	if q.tx != nil && q.tx.done {
		// *sql.Tx と同じく、終わったトランザクションでは読めない(記録もしない)。
		return sql.ErrTxDone
	}
	o.Calls = append(o.Calls, Call{Method: method, Arg: arg, InTx: q.tx != nil})
	if err := o.ErrByMethod[method]; err != nil {
		return err
	}
	return o.Err
}

// BeginTx は偽のトランザクションを開く(readtx.Beginner)。opts を Call.Arg に記録する。
// ErrByMethod["BeginTx"](または Err)が非 nil なら開かずにその値を返す。
// 返す Tx は同じ架空データを読み、呼び出しを InTx: true で記録する。
func (q *Querier) BeginTx(_ context.Context, opts *sql.TxOptions) (readtx.Tx, error) {
	if err := q.record(MethodBeginTx, opts); err != nil {
		return nil, err
	}
	view := q.txView()
	q.mu.Lock()
	q.txs = append(q.txs, view.tx)
	q.mu.Unlock()
	return &Tx{Querier: view}, nil
}

// txView は q と同じ架空データ(公開フィールド。Calls を除く)を持ち、記録を q に送る Querier を作る。
// フィールドを足しても写し漏れないよう reflect で写す。
func (q *Querier) txView() *Querier {
	view := &Querier{root: q, tx: &txState{}}
	src := reflect.ValueOf(q).Elem()
	dst := reflect.ValueOf(view).Elem()
	for i := 0; i < src.NumField(); i++ {
		f := src.Type().Field(i)
		if !f.IsExported() || f.Name == "Calls" {
			continue
		}
		dst.Field(i).Set(src.Field(i))
	}
	return view
}

// Tx は偽の readtx.Tx。Commit / Rollback は1回だけ記録し、2回目以降(Commit 後の defer Rollback 等)は
// *sql.Tx と同じく sql.ErrTxDone を返して記録しない。
type Tx struct {
	*Querier
}

var _ readtx.Tx = (*Tx)(nil)

// Commit はトランザクションを終える。ErrByMethod["Commit"] が非 nil ならその値を返す(終わったことにはなる)。
func (t *Tx) Commit() error { return t.finish(MethodCommit) }

// Rollback はトランザクションを終える。
func (t *Tx) Rollback() error { return t.finish(MethodRollback) }

func (t *Tx) finish(method string) error {
	o := t.owner()
	o.mu.Lock()
	defer o.mu.Unlock()
	if t.tx.done {
		return sql.ErrTxDone
	}
	t.tx.done = true
	o.Calls = append(o.Calls, Call{Method: method, InTx: true})
	return o.ErrByMethod[method]
}

// OpenTxCount は BeginTx で開いて Commit も Rollback もしていないトランザクションの数。
func (q *Querier) OpenTxCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	open := 0
	for _, tx := range q.txs {
		if !tx.done {
			open++
		}
	}
	return open
}

// SnapshotViolations は「BeginTx(ReadOnly)をちょうど1回開き、reads の全メソッドを含む全ての読み出しを
// その Tx の中で行い、最後に Commit し(Rollback しない)、開いたままのトランザクションが無い」ことを確かめ、
// 満たさない点を文で返す(空なら満たす)。BeginTx が失敗した呼び出しも1回と数える。
func (q *Querier) SnapshotViolations(reads []string) []string {
	q.mu.Lock()
	calls := append([]Call(nil), q.Calls...)
	q.mu.Unlock()

	var problems []string
	begins, commits, rollbacks := 0, 0, 0
	called := map[string]bool{}
	for i, c := range calls {
		switch c.Method {
		case MethodBeginTx:
			begins++
			opts, _ := c.Arg.(*sql.TxOptions)
			if opts == nil || !opts.ReadOnly {
				problems = append(problems, fmt.Sprintf("BeginTx の TxOptions が ReadOnly でない: %+v", opts))
			} else if opts.Isolation == sql.LevelReadUncommitted || opts.Isolation == sql.LevelReadCommitted {
				problems = append(problems, fmt.Sprintf("BeginTx の分離レベルが文ごとのスナップショットになる: %v", opts.Isolation))
			}
		case MethodCommit:
			commits++
			if i != len(calls)-1 {
				problems = append(problems, fmt.Sprintf("Commit の後に呼び出しがある: %+v", calls[i+1:]))
			}
		case MethodRollback:
			rollbacks++
		default:
			called[c.Method] = true
			if !c.InTx {
				problems = append(problems, fmt.Sprintf("%s をトランザクションの外(autocommit)で呼んだ", c.Method))
			}
		}
	}
	if begins != 1 {
		problems = append(problems, fmt.Sprintf("BeginTx の回数 = %d, want 1", begins))
	}
	if commits != 1 {
		problems = append(problems, fmt.Sprintf("Commit の回数 = %d, want 1", commits))
	}
	if rollbacks != 0 {
		problems = append(problems, fmt.Sprintf("成功したのに Rollback した(%d 回)", rollbacks))
	}
	if len(calls) > 0 && calls[0].Method != MethodBeginTx {
		problems = append(problems, fmt.Sprintf("最初の呼び出しが BeginTx でない: %s", calls[0].Method))
	}
	for _, m := range reads {
		if !called[m] {
			problems = append(problems, fmt.Sprintf("%s を読んでいない", m))
		}
	}
	return problems
}

// RollbackViolations は「失敗した読み出しのトランザクションを Rollback で閉じ、Commit していない」ことを確かめ、
// 満たさない点を文で返す(空なら満たす)。BeginTx 自体が失敗した(Tx が無い)ときは、Tx の中の呼び出しが
// 1つも無いことだけを確かめる。
func (q *Querier) RollbackViolations() []string {
	q.mu.Lock()
	calls := append([]Call(nil), q.Calls...)
	opened := len(q.txs)
	q.mu.Unlock()

	var problems []string
	begins, rollbacks := 0, 0
	for _, c := range calls {
		switch {
		case c.Method == MethodBeginTx:
			begins++
		case c.Method == MethodCommit:
			problems = append(problems, "失敗したのに Commit した")
		case c.Method == MethodRollback:
			rollbacks++
		case !c.InTx:
			problems = append(problems, fmt.Sprintf("%s をトランザクションの外(autocommit)で呼んだ", c.Method))
		}
	}
	if begins > 1 {
		problems = append(problems, fmt.Sprintf("BeginTx の回数 = %d, want 1 以下", begins))
	}
	if opened != rollbacks {
		problems = append(problems, fmt.Sprintf("開いたトランザクション %d 個に対し Rollback %d 回", opened, rollbacks))
	}
	if open := q.OpenTxCount(); open > 0 {
		problems = append(problems, fmt.Sprintf("開いたままのトランザクションが %d 個ある", open))
	}
	return problems
}

// CallsOf は method の呼び出しの引数を順に返す。
func (q *Querier) CallsOf(method string) []any {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []any
	for _, c := range q.Calls {
		if c.Method == method {
			out = append(out, c.Arg)
		}
	}
	return out
}

func (q *Querier) ListDataVersions(context.Context) ([]store.DataVersion, error) {
	if err := q.record("ListDataVersions", nil); err != nil {
		return nil, err
	}
	return append([]store.DataVersion(nil), q.DataVersions...), nil
}

func (q *Querier) ListTypes(context.Context) ([]store.Type, error) {
	if err := q.record("ListTypes", nil); err != nil {
		return nil, err
	}
	return append([]store.Type(nil), q.Types...), nil
}

func (q *Querier) ListTypeChart(context.Context) ([]store.TypeChart, error) {
	if err := q.record("ListTypeChart", nil); err != nil {
		return nil, err
	}
	return append([]store.TypeChart(nil), q.TypeChart...), nil
}

func (q *Querier) ListSpecies(context.Context) ([]store.Species, error) {
	if err := q.record("ListSpecies", nil); err != nil {
		return nil, err
	}
	return append([]store.Species(nil), q.Species...), nil
}

func (q *Querier) ListAllSpeciesAbilities(context.Context) ([]store.SpeciesAbility, error) {
	if err := q.record("ListAllSpeciesAbilities", nil); err != nil {
		return nil, err
	}
	return append([]store.SpeciesAbility(nil), q.SpeciesAbilities...), nil
}

func (q *Querier) ListMoves(context.Context) ([]store.Move, error) {
	if err := q.record("ListMoves", nil); err != nil {
		return nil, err
	}
	return append([]store.Move(nil), q.Moves...), nil
}

func (q *Querier) ListMoveEffects(context.Context) ([]store.MoveEffect, error) {
	if err := q.record("ListMoveEffects", nil); err != nil {
		return nil, err
	}
	return append([]store.MoveEffect(nil), q.MoveEffects...), nil
}

func (q *Querier) ListMoveMechanisms(context.Context) ([]store.MoveMechanism, error) {
	if err := q.record("ListMoveMechanisms", nil); err != nil {
		return nil, err
	}
	return append([]store.MoveMechanism(nil), q.MoveMechanisms...), nil
}

func (q *Querier) ListItems(context.Context) ([]store.Item, error) {
	if err := q.record("ListItems", nil); err != nil {
		return nil, err
	}
	return append([]store.Item(nil), q.Items...), nil
}

func (q *Querier) ListItemEffects(context.Context) ([]store.ItemEffect, error) {
	if err := q.record("ListItemEffects", nil); err != nil {
		return nil, err
	}
	return append([]store.ItemEffect(nil), q.ItemEffects...), nil
}

func (q *Querier) ListAbilities(context.Context) ([]store.Ability, error) {
	if err := q.record("ListAbilities", nil); err != nil {
		return nil, err
	}
	return append([]store.Ability(nil), q.Abilities...), nil
}

func (q *Querier) ListAbilityEffects(context.Context) ([]store.AbilityEffect, error) {
	if err := q.record("ListAbilityEffects", nil); err != nil {
		return nil, err
	}
	return append([]store.AbilityEffect(nil), q.AbilityEffects...), nil
}

func (q *Querier) ListNatures(context.Context) ([]store.Nature, error) {
	if err := q.record("ListNatures", nil); err != nil {
		return nil, err
	}
	return append([]store.Nature(nil), q.Natures...), nil
}

func (q *Querier) GetDefaultRegulation(context.Context) (store.GetDefaultRegulationRow, error) {
	if err := q.record("GetDefaultRegulation", nil); err != nil {
		return store.GetDefaultRegulationRow{}, err
	}
	if q.DefaultRegulation == nil {
		return store.GetDefaultRegulationRow{}, sql.ErrNoRows
	}
	return *q.DefaultRegulation, nil
}

func (q *Querier) ListRegulationSpeciesKeys(_ context.Context, regulationID string) ([]string, error) {
	if err := q.record("ListRegulationSpeciesKeys", regulationID); err != nil {
		return nil, err
	}
	return append([]string(nil), q.RegulationSpecies[regulationID]...), nil
}

func (q *Querier) ListRegulationMoveIDs(_ context.Context, regulationID string) ([]string, error) {
	if err := q.record("ListRegulationMoveIDs", regulationID); err != nil {
		return nil, err
	}
	return append([]string(nil), q.RegulationMoves[regulationID]...), nil
}

func (q *Querier) ListRegulationAbilityIDs(_ context.Context, regulationID string) ([]string, error) {
	if err := q.record("ListRegulationAbilityIDs", regulationID); err != nil {
		return nil, err
	}
	return append([]string(nil), q.RegulationAbilities[regulationID]...), nil
}

// 検索: 偽物は「集合に含まれるものを全件」返し、パターン・件数は記録するだけ(LIKE の評価は DB の仕事。
// 実際の前方一致と照合順序は db の -tags mysql のテストが確かめる)。ただし Limit は守る。

func (q *Querier) SearchSpecies(_ context.Context, arg store.SearchSpeciesParams) ([]store.SearchSpeciesRow, error) {
	if err := q.record("SearchSpecies", arg); err != nil {
		return nil, err
	}
	in := set(q.RegulationSpecies[arg.RegulationID])
	var out []store.SearchSpeciesRow
	for _, s := range q.Species {
		if in[s.Key] && len(out) < int(arg.Limit) {
			out = append(out, store.SearchSpeciesRow{Key: s.Key, DexNo: s.DexNo, Form: s.Form, NameJa: s.NameJa, Type1: s.Type1, Type2: s.Type2})
		}
	}
	return out, nil
}

func (q *Querier) SearchMoves(_ context.Context, arg store.SearchMovesParams) ([]store.SearchMovesRow, error) {
	if err := q.record("SearchMoves", arg); err != nil {
		return nil, err
	}
	in := set(q.RegulationMoves[arg.RegulationID])
	var out []store.SearchMovesRow
	for _, m := range q.Moves {
		if in[m.ID] && len(out) < int(arg.Limit) {
			out = append(out, store.SearchMovesRow{ID: m.ID, NameJa: m.NameJa, Type: m.Type, Category: m.Category, Power: m.Power, Priority: m.Priority})
		}
	}
	return out, nil
}

func (q *Querier) SearchItems(_ context.Context, arg store.SearchItemsParams) ([]store.SearchItemsRow, error) {
	if err := q.record("SearchItems", arg); err != nil {
		return nil, err
	}
	in := set(q.RegulationItems[arg.RegulationID])
	prefix := likePrefix(arg.Pattern)
	effects := map[string]*json.RawMessage{} // 行が無ければ nil(LEFT JOIN の NULL)
	for _, e := range q.ItemEffects {
		effect := e.Effect
		effects[e.ItemID] = &effect
	}
	var out []store.SearchItemsRow
	for _, it := range q.Items {
		if in[it.ID] && strings.HasPrefix(it.NameJa, prefix) && len(out) < int(arg.Limit) {
			out = append(out, store.SearchItemsRow{ID: it.ID, NameJa: it.NameJa, Effect: effects[it.ID]})
		}
	}
	return out, nil
}

func (q *Querier) GetSpeciesByKey(_ context.Context, key string) (store.Species, error) {
	if err := q.record("GetSpeciesByKey", key); err != nil {
		return store.Species{}, err
	}
	for _, s := range q.Species {
		if s.Key == key {
			return s, nil
		}
	}
	return store.Species{}, sql.ErrNoRows
}

func (q *Querier) GetMove(_ context.Context, id string) (store.GetMoveRow, error) {
	if err := q.record("GetMove", id); err != nil {
		return store.GetMoveRow{}, err
	}
	for _, m := range q.Moves {
		if m.ID == id {
			return store.GetMoveRow{ID: m.ID, NameJa: m.NameJa, Type: m.Type, Category: m.Category, Power: m.Power, Priority: m.Priority}, nil
		}
	}
	return store.GetMoveRow{}, sql.ErrNoRows
}

func (q *Querier) GetMovesByIDs(_ context.Context, ids []string) ([]store.GetMovesByIDsRow, error) {
	if err := q.record("GetMovesByIDs", ids); err != nil {
		return nil, err
	}
	in := set(ids)
	var out []store.GetMovesByIDsRow
	for _, m := range q.Moves {
		if in[m.ID] {
			out = append(out, store.GetMovesByIDsRow{ID: m.ID, NameJa: m.NameJa, Type: m.Type, Category: m.Category, Power: m.Power, Priority: m.Priority})
		}
	}
	return out, nil
}

func (q *Querier) ListSpeciesAbilityNames(_ context.Context, speciesKey string) ([]store.ListSpeciesAbilityNamesRow, error) {
	if err := q.record("ListSpeciesAbilityNames", speciesKey); err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, a := range q.Abilities {
		names[a.ID] = a.NameJa
	}
	effects := map[string]*json.RawMessage{} // 行が無ければ nil(LEFT JOIN の NULL)
	for _, e := range q.AbilityEffects {
		effect := e.Effect
		effects[e.AbilityID] = &effect
	}
	var out []store.ListSpeciesAbilityNamesRow
	for _, sa := range q.SpeciesAbilities {
		if sa.SpeciesKey == speciesKey {
			out = append(out, store.ListSpeciesAbilityNamesRow{Slot: sa.Slot, ID: sa.AbilityID, NameJa: names[sa.AbilityID], Effect: effects[sa.AbilityID]})
		}
	}
	return out, nil
}

func (q *Querier) ListSpeciesLearnset(_ context.Context, arg store.ListSpeciesLearnsetParams) ([]string, error) {
	if err := q.record("ListSpeciesLearnset", arg); err != nil {
		return nil, err
	}
	in := set(q.RegulationMoves[arg.RegulationID])
	var out []string
	for _, l := range q.Learnsets {
		if l.SpeciesKey == arg.SpeciesKey && in[l.MoveID] {
			out = append(out, l.MoveID)
		}
	}
	return out, nil
}

// ListMoveLearners は技を覚える種族(ADR-0251)。実 DB のクエリと同じ規則で返す: learnsets のうち、
// 種族が regulation_species、技が regulation_moves(どちらも arg.RegulationID)にあるものを、
// species の (DexNo, Form) 昇順に並べ、Offset 件飛ばして Limit 件まで。
func (q *Querier) ListMoveLearners(_ context.Context, arg store.ListMoveLearnersParams) ([]store.ListMoveLearnersRow, error) {
	if err := q.record("ListMoveLearners", arg); err != nil {
		return nil, err
	}
	if !set(q.RegulationMoves[arg.RegulationID])[arg.MoveID] {
		return nil, nil
	}
	inSpecies := set(q.RegulationSpecies[arg.RegulationID])
	learns := map[string]bool{}
	for _, l := range q.Learnsets {
		if l.MoveID == arg.MoveID {
			learns[l.SpeciesKey] = true
		}
	}
	var all []store.ListMoveLearnersRow
	for _, s := range q.Species {
		if inSpecies[s.Key] && learns[s.Key] {
			all = append(all, store.ListMoveLearnersRow{Key: s.Key, DexNo: s.DexNo, Form: s.Form, NameJa: s.NameJa, Type1: s.Type1, Type2: s.Type2})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].DexNo != all[j].DexNo {
			return all[i].DexNo < all[j].DexNo
		}
		return all[i].Form < all[j].Form
	})
	start := int(arg.Offset)
	if start > len(all) {
		start = len(all)
	}
	end := start + int(arg.Limit)
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], nil
}

func set(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func ns(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

// New は架空データを入れた偽の Querier を返す(呼ぶたびに新しいコピー)。
//
//   - 種族: 9001-000 テストモン(fire。特性 slot1 testblaze・slot3 testguard)、9001-001 テストメガモン(メガ。fire/water)、
//     9002-000 テストリーフ(grass。特性 slot1〜4 の4つ)、9003-000 テストキンシ(water。既定のレギュレーションの外)
//   - 技: testflame / teststrike / testglare(使用可能)、testbanned(使用可能集合の外)
//   - 持ち物: testorb(効果あり)・teststone(メガストーン。効果なし)・testberry(効果あり)、testplain(集合の外)
//   - 特性: testblaze(攻撃側の効果だけ)・testguard(炎・水を半減)・testleafy(抜群を 3/4)・testhidden・teststance・testspecial、
//     testunused(集合の外。効果あり)
//   - 性格: testbrave(atk↑ spe↓)・testneutral(無補正)
func New() *Querier {
	imported := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	return &Querier{
		DataVersions: []store.DataVersion{
			{Source: "showdown", Version: "abad1deaabad1deaabad1deaabad1deaabad1dea", Checksum: "1111111111111111111111111111111111111111111111111111111111111111", ImportedAt: imported},
			{Source: "calc", Version: "test-calc-1", Checksum: "2222222222222222222222222222222222222222222222222222222222222222", ImportedAt: imported},
			{Source: "pokeapi", Version: "cafef00dcafef00dcafef00dcafef00dcafef00d", Checksum: "3333333333333333333333333333333333333333333333333333333333333333", ImportedAt: imported},
		},
		Types: []store.Type{
			{ID: "normal", SortOrder: 1, NameJa: "テストノーマル", NameJaSource: "pokeapi"},
			{ID: "fire", SortOrder: 2, NameJa: "テストほのお", NameJaSource: "pokeapi"},
			{ID: "water", SortOrder: 3, NameJa: "テストみず", NameJaSource: "pokeapi"},
			{ID: "grass", SortOrder: 4, NameJa: "テストくさ", NameJaSource: "override"},
		},
		TypeChart: []store.TypeChart{
			{AttackType: "fire", DefenseType: "grass", Code: 4},
			{AttackType: "fire", DefenseType: "water", Code: 1},
			{AttackType: "water", DefenseType: "fire", Code: 4},
			{AttackType: "grass", DefenseType: "water", Code: 4},
			{AttackType: "grass", DefenseType: "fire", Code: 1},
			{AttackType: "normal", DefenseType: "normal", Code: 2},
		},
		Species: []store.Species{
			{Key: "9001-000", DexNo: 9001, Form: 0, ShowdownID: "testmon", NameJa: "テストモン", NameJaSource: "pokeapi", NameEn: "Testmon",
				Type1: "fire", BaseHp: 80, BaseAtk: 90, BaseDef: 70, BaseSpa: 100, BaseSpd: 75, BaseSpe: 85},
			{Key: "9001-001", DexNo: 9001, Form: 1, ShowdownID: "testmonmega", NameJa: "テストメガモン", NameJaSource: "pokeapi", NameEn: "Testmon-Mega",
				Type1: "fire", Type2: ns("water"), BaseHp: 80, BaseAtk: 120, BaseDef: 90, BaseSpa: 130, BaseSpd: 95, BaseSpe: 105,
				IsMega: true, BaseSpeciesKey: ns("9001-000"), RequiredItemID: ns("teststone")},
			{Key: "9002-000", DexNo: 9002, Form: 0, ShowdownID: "testleaf", NameJa: "テストリーフ", NameJaSource: "pokeapi", NameEn: "Testleaf",
				Type1: "grass", BaseHp: 60, BaseAtk: 60, BaseDef: 60, BaseSpa: 60, BaseSpd: 60, BaseSpe: 60},
			{Key: "9003-000", DexNo: 9003, Form: 0, ShowdownID: "testbanned", NameJa: "テストキンシ", NameJaSource: "override", NameEn: "Testbanned",
				Type1: "water", BaseHp: 50, BaseAtk: 50, BaseDef: 50, BaseSpa: 50, BaseSpd: 50, BaseSpe: 200},
		},
		SpeciesAbilities: []store.SpeciesAbility{
			{SpeciesKey: "9001-000", Slot: 3, AbilityID: "testguard"},
			{SpeciesKey: "9001-000", Slot: 1, AbilityID: "testblaze"},
			{SpeciesKey: "9001-001", Slot: 1, AbilityID: "teststance"},
			{SpeciesKey: "9002-000", Slot: 4, AbilityID: "testspecial"},
			{SpeciesKey: "9002-000", Slot: 2, AbilityID: "testguard"},
			{SpeciesKey: "9002-000", Slot: 1, AbilityID: "testleafy"},
			{SpeciesKey: "9002-000", Slot: 3, AbilityID: "testhidden"},
			{SpeciesKey: "9003-000", Slot: 1, AbilityID: "testunused"},
		},
		Moves: []store.Move{
			{ID: "testflame", NameJa: "テストほのおわざ", NameJaSource: "pokeapi", NameEn: "Test Flame", Type: "fire", Category: "special", Power: 90, Accuracy: sql.NullInt16{Int16: 100, Valid: true}, Pp: 15, Priority: 0},
			{ID: "teststrike", NameJa: "テストうちこみ", NameJaSource: "override", NameEn: "Test Strike", Type: "normal", Category: "physical", Power: 40, Accuracy: sql.NullInt16{Int16: 100, Valid: true}, Pp: 30, Priority: 1},
			{ID: "testglare", NameJa: "テストにらみ", NameJaSource: "pokeapi", NameEn: "Test Glare", Type: "normal", Category: "status", Power: 0, Pp: 30, Priority: 0},
			{ID: "testbanned", NameJa: "テストきんじて", NameJaSource: "pokeapi", NameEn: "Test Banned", Type: "water", Category: "special", Power: 80, Accuracy: sql.NullInt16{Int16: 100, Valid: true}, Pp: 10, Priority: 0},
		},
		// testflame は複数の機構を持つ(わざと逆順に入れて、応答が昇順に並び替わることを確認する。ADR-0121)。
		// teststrike 等は機構を持たない(通常の技。応答は空配列になる)。
		MoveMechanisms: []store.MoveMechanism{
			{MoveID: "testflame", Mechanism: "variable_power"},
			{MoveID: "testflame", Mechanism: "multi_hit"},
		},
		Items: []store.Item{
			{ID: "testorb", NameJa: "テストだま", NameJaSource: "pokeapi", NameEn: "Test Orb"},
			{ID: "teststone", NameJa: "テストナイト", NameJaSource: "pokeapi", NameEn: "Teststone"},
			{ID: "testberry", NameJa: "テストのみ", NameJaSource: "override", NameEn: "Test Berry"},
			{ID: "testplain", NameJa: "テストふつう", NameJaSource: "pokeapi", NameEn: "Test Plain"},
		},
		// MySQL の JSON 列は正規化した文字列(キーの後に空白)を返す。
		ItemEffects: []store.ItemEffect{
			{ItemID: "testorb", Effect: json.RawMessage(`{"DamageMod": 5324}`)},
			{ItemID: "testberry", Effect: json.RawMessage(`{"ResistBerryType": "fire"}`)},
		},
		Abilities: []store.Ability{
			{ID: "testblaze", NameJa: "テストもうか", NameJaSource: "pokeapi", NameEn: "Test Blaze"},
			{ID: "testguard", NameJa: "テストまもり", NameJaSource: "pokeapi", NameEn: "Test Guard"},
			{ID: "testleafy", NameJa: "テストはっぱ", NameJaSource: "pokeapi", NameEn: "Test Leafy"},
			{ID: "testhidden", NameJa: "テストかくれ", NameJaSource: "override", NameEn: "Test Hidden"},
			{ID: "teststance", NameJa: "テストかまえ", NameJaSource: "pokeapi", NameEn: "Test Stance"},
			{ID: "testspecial", NameJa: "テストとくべつ", NameJaSource: "override", NameEn: "Test Special"},
			{ID: "testunused", NameJa: "テストみしよう", NameJaSource: "override", NameEn: "Test Unused"},
		},
		AbilityEffects: []store.AbilityEffect{
			{AbilityID: "testblaze", Effect: json.RawMessage(`{"OffBoostType": "fire", "OffBoostTypeMod": 6144}`)},
			{AbilityID: "testguard", Effect: json.RawMessage(`{"DefResistType": {"water": 2048, "fire": 2048}}`)},
			{AbilityID: "testleafy", Effect: json.RawMessage(`{"ReduceSuperEffective": 3072}`)},
			{AbilityID: "testunused", Effect: json.RawMessage(`{"StabMod": 8192}`)},
		},
		Natures: []store.Nature{
			{ID: "testbrave", NameJa: "テストゆうかん", NameJaSource: "pokeapi", NameEn: "Testbrave", Plus: ns("atk"), Minus: ns("spe")},
			{ID: "testneutral", NameJa: "テストむほせい", NameJaSource: "override", NameEn: "Testneutral"},
		},
		Learnsets: []store.Learnset{
			{SpeciesKey: "9001-000", MoveID: "testflame"},
			{SpeciesKey: "9001-000", MoveID: "teststrike"},
			{SpeciesKey: "9001-000", MoveID: "testbanned"},
			{SpeciesKey: "9002-000", MoveID: "testglare"},
		},
		DefaultRegulation: &store.GetDefaultRegulationRow{ID: DefaultRegulationID, NameJa: "テストレギュレーションA", IsDefault: true},
		RegulationSpecies: map[string][]string{
			DefaultRegulationID: {"9001-000", "9001-001", "9002-000"},
			"test-b":            {"9003-000"},
		},
		RegulationMoves: map[string][]string{
			DefaultRegulationID: {"testflame", "testglare", "teststrike"},
			"test-b":            {"testbanned"},
		},
		RegulationItems: map[string][]string{
			DefaultRegulationID: {"testberry", "testorb", "teststone"},
		},
		RegulationAbilities: map[string][]string{
			DefaultRegulationID: {"testblaze", "testguard", "testhidden", "testleafy", "testspecial", "teststance"},
			"test-b":            {"testunused"},
		},
	}
}

// 注意: 実 MySQL(照合順序 ja_0900_as_cs。かな種別・全角半角を区別しない)の LIKE より厳しい(バイト列の前方一致)。
// 実 MySQL との一致は services/pokedex/importer の mysql タグのテストが確かめる。
// likePrefix は前方一致の LIKE パターン("..." + "%"。\ \% \_ はエスケープ)から接頭辞を取り出す。
func likePrefix(pattern string) string {
	pattern = strings.TrimSuffix(pattern, "%")
	var b strings.Builder
	rs := []rune(pattern)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' && i+1 < len(rs) {
			i++
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}
