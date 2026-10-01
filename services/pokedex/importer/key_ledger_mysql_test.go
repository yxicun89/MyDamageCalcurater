//go:build mysql

package importer_test

// 種族 key の台帳(species_key_ledger)と、ID の消滅・消滅後の再利用の検出を実 MySQL で確かめる
// (issue #277・ADR-0131)。`make test-db` だけが実行する。
//
// issue の再現手順(架空データを投入 → 9002-002 の行を除いて投入 → 9002-002 を別の showdown_id で投入)
// を2段階のシナリオとして固定する。どの段階でも、止めた投入は DB(台帳を含む)を変えない。

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/importer"
)

const (
	removedSpeciesKey        = "9002-002"
	removedSpeciesShowdownID = "testleafrain"
)

var ledgerT0 = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

type ledgerRow struct {
	Key, ShowdownID string
	FirstSeenAt     time.Time
}

// ledgerRows は species_key_ledger の中身を key 順に返す。
func ledgerRows(t *testing.T, conn *sql.DB) []ledgerRow {
	t.Helper()
	rows, err := conn.Query("SELECT species_key, showdown_id, first_seen_at FROM species_key_ledger ORDER BY species_key")
	if err != nil {
		t.Fatalf("species_key_ledger を読めない: %v", err)
	}
	defer rows.Close()
	var out []ledgerRow
	for rows.Next() {
		var r ledgerRow
		if err := rows.Scan(&r.Key, &r.ShowdownID, &r.FirstSeenAt); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func ledgerHas(rows []ledgerRow, key, showdownID string) bool {
	for _, r := range rows {
		if r.Key == key && r.ShowdownID == showdownID {
			return true
		}
	}
	return false
}

func speciesKeysInDB(t *testing.T, conn *sql.DB) []string {
	t.Helper()
	rows, err := conn.Query("SELECT `key` FROM species ORDER BY `key`")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, k)
	}
	return keys
}

// withoutSpecies は key の種族と、それを指す行(習得技・レギュレーション)を除いた Output を返す。
func withoutSpecies(t *testing.T, out importer.Output, key string) importer.Output {
	t.Helper()
	next := out
	next.Species = nil
	found := false
	for _, sp := range out.Species {
		if sp.BaseSpeciesKey == key {
			t.Fatalf("%s はメガ %s の元の種族(このシナリオには使えない)", key, sp.Key)
		}
		if sp.Key == key {
			found = true
			continue
		}
		next.Species = append(next.Species, sp)
	}
	if !found {
		t.Fatalf("架空データに種族 %s が無い", key)
	}
	next.Learnsets = nil
	for _, l := range out.Learnsets {
		if l.SpeciesKey != key {
			next.Learnsets = append(next.Learnsets, l)
		}
	}
	next.RegulationSpecies = nil
	for _, m := range out.RegulationSpecies {
		if m.MemberID != key {
			next.RegulationSpecies = append(next.RegulationSpecies, m)
		}
	}
	return next
}

// withSpeciesShowdownID は key の種族の showdown_id を差し替えた Output を返す(参照は key なので変えない)。
func withSpeciesShowdownID(t *testing.T, out importer.Output, key, showdownID string) importer.Output {
	t.Helper()
	next := out
	next.Species = append([]importer.SpeciesRow(nil), out.Species...)
	for i := range next.Species {
		if next.Species[i].Key == key {
			next.Species[i].ShowdownID = showdownID
			return next
		}
	}
	t.Fatalf("架空データに種族 %s が無い", key)
	return next
}

// withoutMove は技 id と、それを指す行(効果・機構・習得技・レギュレーション)を除いた Output を返す。
func withoutMove(out importer.Output, id string) importer.Output {
	next := out
	next.Moves = nil
	for _, m := range out.Moves {
		if m.ID != id {
			next.Moves = append(next.Moves, m)
		}
	}
	next.MoveEffects = nil
	for _, e := range out.MoveEffects {
		if e.ID != id {
			next.MoveEffects = append(next.MoveEffects, e)
		}
	}
	next.MoveMechanisms = nil
	for _, m := range out.MoveMechanisms {
		if m.MoveID != id {
			next.MoveMechanisms = append(next.MoveMechanisms, m)
		}
	}
	next.Learnsets = nil
	for _, l := range out.Learnsets {
		if l.MoveID != id {
			next.Learnsets = append(next.Learnsets, l)
		}
	}
	next.RegulationMoves = nil
	for _, m := range out.RegulationMoves {
		if m.MemberID != id {
			next.RegulationMoves = append(next.RegulationMoves, m)
		}
	}
	return next
}

func allowSpecies(key string) importer.ApplyOptions {
	return importer.ApplyOptions{AllowRemoved: []importer.RemovedID{{Kind: importer.IDKindSpecies, ID: key}}}
}

// 1回目の投入で台帳ができる(新しい出力の全種族。first_seen_at は投入の時刻)。
// 2回目以降の投入は first_seen_at を変えない(台帳は追記だけ)。
func TestApplyCreatesSpeciesKeyLedger(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	if err := importer.Apply(ctx, conn, out, versions, ledgerT0); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := ledgerRows(t, conn)
	var want []ledgerRow
	for _, sp := range out.Species {
		want = append(want, ledgerRow{Key: sp.Key, ShowdownID: sp.ShowdownID, FirstSeenAt: ledgerT0})
	}
	sort.Slice(want, func(i, j int) bool { return want[i].Key < want[j].Key })
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("台帳 = %+v,\nwant %+v", got, want)
	}

	if _, err := importer.Run(ctx, conn, out, versions, ledgerT0.Add(24*time.Hour), true); err != nil {
		t.Fatalf("2回目(force): %v", err)
	}
	if again := ledgerRows(t, conn); !reflect.DeepEqual(again, want) {
		t.Errorf("再投入で台帳が変わった(first_seen_at は最初の投入のまま)\n got %+v\nwant %+v", again, want)
	}
}

// 段階1: 既存 DB にある種族 key が新しい出力から消える投入は、既定では ErrKeyRemoved で止め、DB を変えない。
func TestApplyRejectsSpeciesRemovalByDefault(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	if err := importer.Apply(ctx, conn, out, versions, ledgerT0); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	before := dumpTables(t, conn)

	err := importer.Apply(ctx, conn, withoutSpecies(t, out, removedSpeciesKey), versions, ledgerT0.Add(time.Hour))
	if !errors.Is(err, importer.ErrKeyRemoved) {
		t.Fatalf("err = %v, want ErrKeyRemoved", err)
	}
	var re *importer.RemovedIDsError
	if !errors.As(err, &re) {
		t.Fatalf("err = %T, want *RemovedIDsError を包む", err)
	}
	if want := []importer.RemovedID{{Kind: importer.IDKindSpecies, ID: removedSpeciesKey}}; !reflect.DeepEqual(re.IDs, want) {
		t.Errorf("消えた ID = %+v, want %+v", re.IDs, want)
	}
	if after := dumpTables(t, conn); !reflect.DeepEqual(before, after) {
		t.Errorf("止めたのに DB が変わった")
	}

	// Run(CronJob の経路)も既定で止める。
	changed := append([]importer.SourceVersion(nil), versions...)
	changed[0].Version += "-next"
	if _, err := importer.Run(ctx, conn, withoutSpecies(t, out, removedSpeciesKey), changed, ledgerT0.Add(time.Hour), false); !errors.Is(err, importer.ErrKeyRemoved) {
		t.Fatalf("Run: err = %v, want ErrKeyRemoved", err)
	}
	if after := dumpTables(t, conn); !reflect.DeepEqual(before, after) {
		t.Errorf("Run で止めたのに DB が変わった")
	}
}

// 段階1(承認あり): 人が -allow-removed で消滅を許せば投入でき、消えた key も台帳には残る。
func TestApplyAllowsApprovedSpeciesRemoval(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	if err := importer.Apply(ctx, conn, out, versions, ledgerT0); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	removed := withoutSpecies(t, out, removedSpeciesKey)
	if err := importer.ApplyWithOptions(ctx, conn, removed, versions, ledgerT0.Add(time.Hour), allowSpecies(removedSpeciesKey)); err != nil {
		t.Fatalf("消滅を許した投入: %v", err)
	}
	for _, k := range speciesKeysInDB(t, conn) {
		if k == removedSpeciesKey {
			t.Fatalf("許した消滅なのに species に %s が残っている", k)
		}
	}
	if !ledgerHas(ledgerRows(t, conn), removedSpeciesKey, removedSpeciesShowdownID) {
		t.Errorf("消えた key %s が台帳から消えた(台帳は削除しない)", removedSpeciesKey)
	}

	// 消えた後の次の投入(同じ出力)は、もう消滅ではない(DB の species に無いので)。
	if err := importer.Apply(ctx, conn, removed, versions, ledgerT0.Add(2*time.Hour)); err != nil {
		t.Errorf("消滅を承認した後の同じ出力の再投入: %v", err)
	}
}

// 段階2: 消えた key を別の showdown_id で再利用する投入は、-allow-removed があっても常に止める
// (issue #277 の再現手順 3)。DB を変えない。
func TestApplyRejectsReuseOfRemovedSpeciesKey(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	if err := importer.Apply(ctx, conn, out, versions, ledgerT0); err != nil {
		t.Fatalf("段階0: %v", err)
	}
	if err := importer.ApplyWithOptions(ctx, conn, withoutSpecies(t, out, removedSpeciesKey), versions, ledgerT0.Add(time.Hour), allowSpecies(removedSpeciesKey)); err != nil {
		t.Fatalf("段階1(消滅を許す): %v", err)
	}
	before := dumpTables(t, conn)

	reused := withSpeciesShowdownID(t, out, removedSpeciesKey, "testleafnew")
	for _, opts := range []importer.ApplyOptions{{}, allowSpecies(removedSpeciesKey)} {
		err := importer.ApplyWithOptions(ctx, conn, reused, versions, ledgerT0.Add(2*time.Hour), opts)
		if !errors.Is(err, importer.ErrKeyChanged) {
			t.Fatalf("段階2(opts=%+v): err = %v, want ErrKeyChanged", opts, err)
		}
		if after := dumpTables(t, conn); !reflect.DeepEqual(before, after) {
			t.Errorf("段階2(opts=%+v): 止めたのに DB が変わった", opts)
		}
	}
}

// 消えた種族が同じ key・同じ showdown_id で戻ってくる(上流の一時的な欠落など)のは通す。
func TestApplyAllowsRestoringRemovedSpecies(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	if err := importer.Apply(ctx, conn, out, versions, ledgerT0); err != nil {
		t.Fatalf("段階0: %v", err)
	}
	if err := importer.ApplyWithOptions(ctx, conn, withoutSpecies(t, out, removedSpeciesKey), versions, ledgerT0.Add(time.Hour), allowSpecies(removedSpeciesKey)); err != nil {
		t.Fatalf("段階1(消滅を許す): %v", err)
	}
	if err := importer.Apply(ctx, conn, out, versions, ledgerT0.Add(2*time.Hour)); err != nil {
		t.Fatalf("同じ組での復活: %v", err)
	}
	for _, r := range ledgerRows(t, conn) {
		if r.Key == removedSpeciesKey && !r.FirstSeenAt.Equal(ledgerT0) {
			t.Errorf("復活で first_seen_at が変わった: %v, want %v", r.FirstSeenAt, ledgerT0)
		}
	}
}

// 消えた showdown_id が別の key で戻ってくる投入も止める(保存済みの古い key が指す先を失う)。
func TestApplyRejectsRemovedShowdownIDUnderNewKey(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	if err := importer.Apply(ctx, conn, out, versions, ledgerT0); err != nil {
		t.Fatalf("段階0: %v", err)
	}
	if err := importer.ApplyWithOptions(ctx, conn, withoutSpecies(t, out, removedSpeciesKey), versions, ledgerT0.Add(time.Hour), allowSpecies(removedSpeciesKey)); err != nil {
		t.Fatalf("段階1(消滅を許す): %v", err)
	}
	before := dumpTables(t, conn)

	rekeyed := rekey(out, removedSpeciesKey, "9002-009", 9)
	err := importer.Apply(ctx, conn, rekeyed, versions, ledgerT0.Add(2*time.Hour))
	if !errors.Is(err, importer.ErrKeyChanged) {
		t.Fatalf("err = %v, want ErrKeyChanged", err)
	}
	if after := dumpTables(t, conn); !reflect.DeepEqual(before, after) {
		t.Errorf("止めたのに DB が変わった")
	}
}

// 技・持ち物・特性の ID の消滅も、既定では ErrKeyRemoved(終了コード 3)で人に知らせて止める。
// 許せば投入できる。
func TestApplyRejectsMoveRemovalByDefault(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	if err := importer.Apply(ctx, conn, out, versions, ledgerT0); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(out.Moves) == 0 {
		t.Fatal("架空データに技が無い")
	}
	moveID := out.Moves[len(out.Moves)-1].ID
	before := dumpTables(t, conn)

	removed := withoutMove(out, moveID)
	err := importer.Apply(ctx, conn, removed, versions, ledgerT0.Add(time.Hour))
	var re *importer.RemovedIDsError
	if !errors.Is(err, importer.ErrKeyRemoved) || !errors.As(err, &re) {
		t.Fatalf("err = %v, want ErrKeyRemoved(*RemovedIDsError)", err)
	}
	if want := []importer.RemovedID{{Kind: importer.IDKindMove, ID: moveID}}; !reflect.DeepEqual(re.IDs, want) {
		t.Errorf("消えた ID = %+v, want %+v", re.IDs, want)
	}
	if after := dumpTables(t, conn); !reflect.DeepEqual(before, after) {
		t.Errorf("止めたのに DB が変わった")
	}

	opts := importer.ApplyOptions{AllowRemoved: []importer.RemovedID{{Kind: importer.IDKindMove, ID: moveID}}}
	if err := importer.ApplyWithOptions(ctx, conn, removed, versions, ledgerT0.Add(time.Hour), opts); err != nil {
		t.Fatalf("消滅を許した投入: %v", err)
	}
}

// 許した ID が実際には消えていない(打ち間違い)なら投入せず ErrInvalidInput。DB を変えない。
func TestApplyRejectsUnusedAllowance(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	if err := importer.Apply(ctx, conn, out, versions, ledgerT0); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	before := dumpTables(t, conn)
	err := importer.ApplyWithOptions(ctx, conn, out, versions, ledgerT0.Add(time.Hour), allowSpecies("9999-999"))
	if !errors.Is(err, importer.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if after := dumpTables(t, conn); !reflect.DeepEqual(before, after) {
		t.Errorf("止めたのに DB が変わった")
	}
}

// 既存の DB への移行: 台帳ができる前(migration 000009 より前)に投入した DB は、species に行があって
// 台帳が空。次の投入は、検査の前に今の species から台帳を作る(そうしないと、その投入で消えた key を
// 台帳が知らず、後の再利用を止められない)。
func TestApplySeedsLedgerFromExistingSpecies(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	ctx := context.Background()
	if err := importer.Apply(ctx, conn, out, versions, ledgerT0); err != nil {
		t.Fatalf("段階0: %v", err)
	}
	// 台帳の無かった頃の投入を再現する(テスト用の接続は全権限。importer の経路は台帳を消さない)。
	if _, err := conn.Exec("DELETE FROM species_key_ledger"); err != nil {
		t.Fatal(err)
	}

	if err := importer.ApplyWithOptions(ctx, conn, withoutSpecies(t, out, removedSpeciesKey), versions, ledgerT0.Add(time.Hour), allowSpecies(removedSpeciesKey)); err != nil {
		t.Fatalf("段階1(台帳が空の DB で消滅を許す): %v", err)
	}
	ledger := ledgerRows(t, conn)
	for _, sp := range out.Species {
		if !ledgerHas(ledger, sp.Key, sp.ShowdownID) {
			t.Errorf("投入前の species %s=%s が台帳に入っていない", sp.Key, sp.ShowdownID)
		}
	}

	before := dumpTables(t, conn)
	err := importer.Apply(ctx, conn, withSpeciesShowdownID(t, out, removedSpeciesKey, "testleafnew"), versions, ledgerT0.Add(2*time.Hour))
	if !errors.Is(err, importer.ErrKeyChanged) {
		t.Fatalf("段階2: err = %v, want ErrKeyChanged", err)
	}
	if after := dumpTables(t, conn); !reflect.DeepEqual(before, after) {
		t.Errorf("段階2: 止めたのに DB が変わった")
	}
}
