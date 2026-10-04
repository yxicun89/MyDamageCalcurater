//go:build tidb

package store

// 計算の入力全体(calc)を持つお気に入り(ADR-0228)の実 SQL 検証。`make test-db`(`make test-db-docker`)から実行する。
//
// store は snapshot の中身を解釈しない(正規化は httpapi の担当)ので、migration も store のコードも変えない見込み。
// ここでは「httpapi が許す最大(4096 バイト)の snapshot が JSON 列にそのまま入り、意味が変わらずに読み戻せる」
// 「calc の有無だけが違う snapshot は別の行になり、同じバイト列は従来どおり重複判定される」ことを実 TiDB で固定する
// (fake では JSON 列の正規化〈キー順・空白の書き換え〉と行の大きさを確かめられない)。

import (
	"strings"
	"testing"
	"time"
)

// calcSnapshotOfSize は calc を持つ正規形を模した snapshot を、moveId の長さで size バイトちょうどにする。
func calcSnapshotOfSize(t *testing.T, size int) []byte {
	t.Helper()
	ind := `{"speciesKey":"9001-000","level":50,"natureId":"n","sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},` +
		`"ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"}`
	head := `{"label":"あ","individual":` + ind + `,"calc":{"format":"double","attacker":` + ind + `,"defender":` + ind + `,"moveId":"`
	tail := `","field":{"weather":"rain","terrain":"none","attackerScreens":{"reflect":true,"lightScreen":false,"auroraVeil":false},` +
		`"defenderScreens":{"reflect":false,"lightScreen":false,"auroraVeil":true}},"options":{"critical":true}}}`
	pad := size - len(head) - len(tail)
	if pad < 1 {
		t.Fatalf("size %d が小さすぎる", size)
	}
	return []byte(head + strings.Repeat("m", pad) + tail)
}

// 上限ちょうど(4096 バイト)の calc 付き snapshot が保存・読み戻し・重複判定できる。
func TestTiDBCreateFavoriteWithMaxSizeCalcSnapshot(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	snap := calcSnapshotOfSize(t, 4096)
	if len(snap) != 4096 {
		t.Fatalf("テストの snapshot = %d バイト, want 4096", len(snap))
	}
	fav, outcome, err := st.CreateFavorite(ctx(), deviceID, Favorite{SpeciesKey: "9001-000", Snapshot: snap}, now)
	if err != nil || outcome != FavoriteCreated {
		t.Fatalf("CreateFavorite = %v, %v, want FavoriteCreated", outcome, err)
	}
	list, err := st.ListFavorites(ctx(), deviceID)
	if err != nil {
		t.Fatalf("ListFavorites: %v", err)
	}
	if len(list) != 1 || list[0].ID != fav.ID || !jsonEqual(t, list[0].Snapshot, snap) {
		t.Fatalf("読み戻した snapshot が意味で一致しない: %+v", list)
	}

	again, outcome, err := st.CreateFavorite(ctx(), deviceID, Favorite{SpeciesKey: "9001-000", Snapshot: snap}, now.Add(time.Second))
	if err != nil || outcome != FavoriteExisted || again.ID != fav.ID {
		t.Errorf("同じ snapshot の2回目 = (%d, %v, %v), want (%d, FavoriteExisted)", again.ID, outcome, err, fav.ID)
	}
}

// calc の有無だけが違う snapshot は別の行(snapshot_hash が違う)。calc の無い snapshot は従来どおり重複判定される。
func TestTiDBFavoriteCalcPresenceIsDistinct(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	legacy := favoriteSnapshot("same")
	withCalc := []byte(strings.TrimSuffix(string(legacy), "}") +
		`,"calc":{"format":"single","attacker":{"speciesKey":"9001-000","level":50,"natureId":"n",` +
		`"sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"},` +
		`"defender":{"speciesKey":"9002-000","level":50,"natureId":"n","sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},` +
		`"ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"},"moveId":"m",` +
		`"field":{"weather":"none","terrain":"none","attackerScreens":{"reflect":false,"lightScreen":false,"auroraVeil":false},` +
		`"defenderScreens":{"reflect":false,"lightScreen":false,"auroraVeil":false}},"options":{"critical":false}}}`)

	a, oa, err := st.CreateFavorite(ctx(), deviceID, Favorite{SpeciesKey: "9001-000", Snapshot: legacy}, now)
	if err != nil || oa != FavoriteCreated {
		t.Fatalf("calc 無し = %v, %v", oa, err)
	}
	b, ob, err := st.CreateFavorite(ctx(), deviceID, Favorite{SpeciesKey: "9001-000", Snapshot: withCalc}, now)
	if err != nil || ob != FavoriteCreated || b.ID == a.ID {
		t.Fatalf("calc あり = (%d, %v, %v), want 別の行の FavoriteCreated", b.ID, ob, err)
	}
	if _, oc, err := st.CreateFavorite(ctx(), deviceID, Favorite{SpeciesKey: "9001-000", Snapshot: legacy}, now.Add(time.Second)); err != nil || oc != FavoriteExisted {
		t.Errorf("calc 無しの2回目 = %v, %v, want FavoriteExisted(従来の重複判定のまま)", oc, err)
	}
	if n := countByDevice(t, db, "favorites", deviceID); n != 2 {
		t.Errorf("行数 = %d, want 2", n)
	}
}
