//go:build tidb

package store

// ListCalcHistory(計算履歴の一覧。ADR-0230)の実 SQL 検証。`make test-db`(`make test-db-docker`)からだけ実行する。
// httpapi の fake はここまで実行しないので、並び(occurred_at DESC, event_id DESC)・keyset のカーソル・
// operation の絞り込み・保持期間の境界・墓石・端末分離・読み取り専用であることを本物の SQL で確かめる。
//
// **test-first(ADR-0003)**: 実装前に書いた。型とメソッドの形は httpapi/calc_history_fixture_test.go の冒頭と ADR-0230 §8。

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// historyEvent は operation と payload を指定できる calcEvent(payload は行の見分けに使う)。
func historyEvent(deviceID, suffix, operation string, occurredAt time.Time) CalcEvent {
	ev := calcEvent(deviceID, suffix, "9001-000", occurredAt)
	ev.Operation = operation
	if operation != "calc" {
		ev.DefenderSpeciesKey = ""
	}
	ev.Payload = []byte(fmt.Sprintf(`{"row":%q}`, suffix))
	return ev
}

func mustSave(t *testing.T, st *TiDBStore, ev CalcEvent) {
	t.Helper()
	outcome, err := st.SaveCalcEvent(ctx(), ev)
	if err != nil || outcome != Stored {
		t.Fatalf("SaveCalcEvent(%s) = %v, %v; want Stored", ev.EventID, outcome, err)
	}
}

// suffixes は行の event_id から deviceID の接頭辞を外した並び(期待との比較用)。
func suffixes(deviceID string, rows []CalcHistoryRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, strings.TrimPrefix(r.EventID, deviceID+"-"))
	}
	return out
}

func listAll(t *testing.T, st *TiDBStore, deviceID string, q CalcHistoryQuery) []CalcHistoryRow {
	t.Helper()
	rows, err := st.ListCalcHistory(ctx(), deviceID, q)
	if err != nil {
		t.Fatalf("ListCalcHistory: %v", err)
	}
	if rows == nil {
		t.Fatal("ListCalcHistory が nil を返した(0件でも長さ0のスライス)")
	}
	return rows
}

// 並びは occurred_at の降順、同時刻は event_id の降順。calc 以外(calcBulk / calcReverse)は返さない。
// payload は保存した JSON(意味が同じ)、OccurredAt は保存した時刻(マイクロ秒)をそのまま返す。
func TestTiDBListCalcHistoryOrderAndOperationFilter(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	base := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)

	mustSave(t, st, historyEvent(deviceID, "a1", "calc", base))
	mustSave(t, st, historyEvent(deviceID, "b1", "calc", base.Add(time.Minute)))
	mustSave(t, st, historyEvent(deviceID, "b2", "calc", base.Add(time.Minute))) // b1 と同時刻
	mustSave(t, st, historyEvent(deviceID, "c1", "calc", base.Add(2*time.Minute)))
	mustSave(t, st, historyEvent(deviceID, "bulk", "calcBulk", base.Add(3*time.Minute)))
	mustSave(t, st, historyEvent(deviceID, "rev", "calcReverse", base.Add(4*time.Minute)))

	rows := listAll(t, st, deviceID, CalcHistoryQuery{Limit: 10})
	want := []string{"c1", "b2", "b1", "a1"}
	if got := suffixes(deviceID, rows); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("並び = %v, want %v(新しい順・同時刻は event_id の降順・calc だけ)", got, want)
	}
	if !rows[0].OccurredAt.Equal(base.Add(2 * time.Minute)) {
		t.Errorf("OccurredAt = %v, want %v", rows[0].OccurredAt, base.Add(2*time.Minute))
	}
	if !jsonEqual(t, rows[0].Payload, []byte(`{"row":"c1"}`)) {
		t.Errorf("Payload = %s, want {\"row\":\"c1\"}", rows[0].Payload)
	}
}

// keyset: Before より「後ろ」(古い・同時刻なら event_id が小さい)の行だけを返し、Limit で切る。
// 同時刻の行をまたいでも、ページをたどると重複・欠落が無い。
func TestTiDBListCalcHistoryKeyset(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	base := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)
	for i, s := range []string{"a", "b", "c", "d", "e"} {
		// a,b が同時刻・c,d が同時刻・e が最新。
		mustSave(t, st, historyEvent(deviceID, s, "calc", base.Add(time.Duration(i/2)*time.Minute)))
	}

	var got []string
	var before *CalcHistoryCursor
	for pages := 0; ; pages++ {
		if pages > 5 {
			t.Fatal("ページが終わらない")
		}
		rows := listAll(t, st, deviceID, CalcHistoryQuery{Before: before, Limit: 2})
		got = append(got, suffixes(deviceID, rows)...)
		if len(rows) < 2 {
			break
		}
		last := rows[len(rows)-1]
		before = &CalcHistoryCursor{OccurredAt: last.OccurredAt, EventID: last.EventID}
	}
	if want := "e,d,c,b,a"; strings.Join(got, ",") != want {
		t.Errorf("たどった行 = %v, want %s(重複・欠落なし)", got, want)
	}

	// 同時刻の行の途中(d の位置)から: c(同時刻で event_id が小さい)以降を返す。
	var d CalcHistoryRow
	for _, r := range listAll(t, st, deviceID, CalcHistoryQuery{Limit: 10}) {
		if strings.HasSuffix(r.EventID, "-d") {
			d = r
		}
	}
	rows := listAll(t, st, deviceID, CalcHistoryQuery{Before: &CalcHistoryCursor{OccurredAt: d.OccurredAt, EventID: d.EventID}, Limit: 10})
	if g := strings.Join(suffixes(deviceID, rows), ","); g != "c,b,a" {
		t.Errorf("d より後ろ = %s, want c,b,a", g)
	}
}

// 保持期間の下限: occurred_at >= Since の行だけ(ちょうど Since の行は返し、1マイクロ秒古い行は返さない。
// 失効ジョブの `occurred_at < cutoff` で消える行と、返す行がちょうど補集合になる)。Since がゼロ値なら下限なし。
func TestTiDBListCalcHistorySinceBoundary(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	since := time.Now().UTC().Truncate(time.Microsecond).Add(-90 * 24 * time.Hour)

	mustSave(t, st, historyEvent(deviceID, "older", "calc", since.Add(-time.Microsecond)))
	mustSave(t, st, historyEvent(deviceID, "edge", "calc", since))
	mustSave(t, st, historyEvent(deviceID, "newer", "calc", since.Add(time.Microsecond)))

	if g := strings.Join(suffixes(deviceID, listAll(t, st, deviceID, CalcHistoryQuery{Since: since, Limit: 10})), ","); g != "newer,edge" {
		t.Errorf("Since つき = %s, want newer,edge", g)
	}
	if g := strings.Join(suffixes(deviceID, listAll(t, st, deviceID, CalcHistoryQuery{Limit: 10})), ","); g != "newer,edge,older" {
		t.Errorf("Since なし = %s, want newer,edge,older", g)
	}
}

// 墓石: 全削除(PurgeDevice)が partial で行が残っていても、purged_at 以前の行は返さない。
// 削除の後に発生した計算は返す(ADR-0209 §7)。
func TestTiDBListCalcHistoryHidesRowsBeforeTombstone(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1) // 1回の削除で1行だけ消す(partial を作る)
	deviceID := newDeviceID(t)
	purgedAt := time.Now().UTC().Truncate(time.Microsecond)
	for i := 1; i <= 3; i++ {
		mustSave(t, st, historyEvent(deviceID, fmt.Sprintf("before-%d", i), "calc", purgedAt.Add(-time.Duration(i)*time.Minute)))
	}

	res, err := st.PurgeDevice(ctx(), deviceID, purgedAt)
	if err != nil || !res.Remaining {
		t.Fatalf("PurgeDevice = %+v, %v; want partial(Remaining)", res, err)
	}
	if n := countByDevice(t, db, "calc_events", deviceID); n == 0 {
		t.Fatal("partial のはずが calc_events が空(テストの前提が崩れている)")
	}
	if rows := listAll(t, st, deviceID, CalcHistoryQuery{Limit: 10}); len(rows) != 0 {
		t.Errorf("partial の途中の履歴 = %v, want 空(墓石より前の行を返さない)", suffixes(deviceID, rows))
	}

	mustSave(t, st, historyEvent(deviceID, "after", "calc", purgedAt.Add(time.Minute)))
	if g := strings.Join(suffixes(deviceID, listAll(t, st, deviceID, CalcHistoryQuery{Limit: 10})), ","); g != "after" {
		t.Errorf("削除後に計算した行 = %s, want after", g)
	}

	// 墓石との境界: ちょうど同時刻は返さず、1マイクロ秒後は返し、1マイクロ秒前は返さない。
	// SaveCalcEvent は墓石以前を捨てるので、DB に直接入れて境界の行を作る。
	insert := func(suffix string, at time.Time) {
		t.Helper()
		if _, err := db.ExecContext(ctx(), `INSERT INTO calc_events
			(event_id, device_id, session_id, operation, occurred_at, defender_species_key, payload, created_at)
			VALUES (?, ?, 's-1', 'calc', ?, '', '{}', ?)`,
			deviceID+"-"+suffix, deviceID, at, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
	insert("tomb-equal", purgedAt)
	insert("tomb-plus1us", purgedAt.Add(time.Microsecond))
	insert("tomb-minus1us", purgedAt.Add(-time.Microsecond))
	got := strings.Join(suffixes(deviceID, listAll(t, st, deviceID, CalcHistoryQuery{Limit: 10})), ",")
	if got != "after,tomb-plus1us" {
		t.Errorf("墓石の境界 = %s, want after,tomb-plus1us(同時刻・1us 前は返さない)", got)
	}
}

// 端末分離: 他端末の行は返さない(同じ時刻・カーソルでも)。記録の無い端末は長さ0。
func TestTiDBListCalcHistoryIsolation(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	devA, devB, devC := newDeviceID(t), newDeviceID(t), newDeviceID(t)
	at := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Minute)
	mustSave(t, st, historyEvent(devA, "a", "calc", at))
	mustSave(t, st, historyEvent(devB, "b", "calc", at))

	rows := listAll(t, st, devB, CalcHistoryQuery{Limit: 10})
	if len(rows) != 1 || !strings.HasPrefix(rows[0].EventID, devB) {
		t.Errorf("端末 B の履歴 = %+v, want B の1行だけ", rows)
	}
	// A の行の位置をカーソルにしても、B の問い合わせに A の行は出ない。
	far := &CalcHistoryCursor{OccurredAt: at.Add(time.Hour), EventID: "zzzz"}
	for _, r := range listAll(t, st, devB, CalcHistoryQuery{Before: far, Limit: 10}) {
		if strings.HasPrefix(r.EventID, devA) {
			t.Errorf("端末 B の問い合わせに A の行 %s が出た(ADR-0209 §6)", r.EventID)
		}
	}
	if rows := listAll(t, st, devC, CalcHistoryQuery{Limit: 10}); len(rows) != 0 {
		t.Errorf("記録の無い端末 = %d 行, want 0", len(rows))
	}
}

// 読むだけで何も書かない: calc_events の行数・created_at、devices.last_seen_at、集計が変わらない
// (失効の判定に効く値を進めない。ADR-0209 §4)。
func TestTiDBListCalcHistoryIsReadOnly(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	at := time.Now().UTC().Truncate(time.Microsecond).Add(-48 * time.Hour)
	mustSave(t, st, historyEvent(deviceID, "x", "calc", at))

	snapshot := func() string {
		var n int
		var created time.Time
		if err := db.QueryRowContext(ctx(), `SELECT COUNT(*), MAX(created_at) FROM calc_events WHERE device_id = ?`, deviceID).Scan(&n, &created); err != nil {
			t.Fatal(err)
		}
		var score float64
		var count int
		if err := db.QueryRowContext(ctx(), `SELECT score, count FROM frequent_opponents WHERE device_id = ?`, deviceID).Scan(&score, &count); err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%d|%s|%s|%v|%d", n, created.UTC(), deviceLastSeenAt(t, db, deviceID).UTC(), score, count)
	}
	before := snapshot()
	listAll(t, st, deviceID, CalcHistoryQuery{Limit: 10})
	listAll(t, st, deviceID, CalcHistoryQuery{Since: at, Limit: 1})
	if after := snapshot(); after != before {
		t.Errorf("読み取りで記録が変わった: before=%s after=%s", before, after)
	}
}

// migration を当てた実 DB に、一覧の並びを索引で閉じるための (device_id, operation, occurred_at, event_id) の
// 索引がある(端末の全行を読んで並べ替えない。ADR-0230 §7)。
func TestTiDBCalcEventsHasHistoryIndex(t *testing.T) {
	db := testDB(t)
	rows, err := db.QueryContext(ctx(), `SHOW INDEX FROM calc_events`)
	if err != nil {
		t.Fatalf("SHOW INDEX: %v", err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	seq := map[string][]string{}
	for rows.Next() {
		vals := make([]sql.RawBytes, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		var keyName, column string
		for i, c := range cols {
			switch c {
			case "Key_name":
				keyName = string(vals[i])
			case "Column_name":
				column = string(vals[i])
			}
		}
		seq[keyName] = append(seq[keyName], column)
	}
	found := false
	for _, c := range seq {
		if strings.Join(c, ",") == "device_id,operation,occurred_at,event_id" {
			found = true
		}
	}
	if !found {
		t.Errorf("calc_events に (device_id, operation, occurred_at, event_id) の索引が無い: %v", seq)
	}
}

// 一覧の SQL(実装と同じもの)の実行計画が history 索引を使う。プランの形は版で変わりうるので、索引名が含まれることだけを見る。
func TestTiDBListCalcHistoryUsesHistoryIndex(t *testing.T) {
	db := testDB(t)
	query, args := calcHistorySQL(newDeviceID(t), CalcHistoryQuery{
		Since:  time.Now().UTC().Add(-time.Hour),
		Before: &CalcHistoryCursor{OccurredAt: time.Now().UTC(), EventID: "x"},
		Limit:  21,
	})
	rows, err := db.QueryContext(ctx(), "EXPLAIN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var plan strings.Builder
	for rows.Next() {
		vals := make([]sql.RawBytes, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		for _, v := range vals {
			plan.Write(v)
			plan.WriteByte(' ')
		}
		plan.WriteByte('\n')
	}
	if !strings.Contains(plan.String(), "idx_calc_events_device_history") {
		t.Errorf("実行計画が idx_calc_events_device_history を使っていない:\n%s", plan.String())
	}
}

// DB に届かなければ ErrUnavailable で包む(httpapi が 503 store_unavailable に写す)。
func TestTiDBListCalcHistoryUnavailable(t *testing.T) {
	testDB(t)
	closedDB, err := sql.Open("mysql", os.Getenv("RECORD_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	closedDB.Close()
	unreachable := newStore(closedDB, hugeHalfLife, 1000)
	if _, err := unreachable.ListCalcHistory(ctx(), "d", CalcHistoryQuery{Limit: 1}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("ListCalcHistory = %v, want ErrUnavailable", err)
	}
}
