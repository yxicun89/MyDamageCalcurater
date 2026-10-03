//go:build tidb

package store

// お気に入り(ADR-0227。P5-3c)の実 SQL 検証。tidb_test.go と同じく `RECORD_TEST_DSN` が必須で、
// `make test-db`(`make test-db-docker`)からだけ実行する(スキップしない)。
//
// favorites に snapshot_hash と UNIQUE (device_id, snapshot_hash) を足す migration(000006)と、
// CreateFavorite / ListFavorites / DeleteFavorite の実 SQL を検証する。fake(httpapi/fixture_test.go)では
// 確かめられない、一意制約による同時作成の重複排除・トランザクション内の上限判定・DATETIME(6) の往復をここで見る。
// 補助(testDB・newStore・newDeviceID・ctx・countByDevice)は tidb_test.go のものを使う。

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

// favoriteSnapshot は正規化済みの形を模した Snapshot(store は中身を解釈しない)。
func favoriteSnapshot(label string) []byte {
	return []byte(`{"label":"` + label + `","individual":{"speciesKey":"9001-000","level":50,"natureId":"n",` +
		`"sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"}}`)
}

func newFavorite(label string) Favorite {
	return Favorite{SpeciesKey: "9001-000", Snapshot: favoriteSnapshot(label)}
}

// jsonEqual は TiDB の JSON 列から読み戻した値(空白・キー順が変わりうる)を意味で比べる。
func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatalf("JSON でない: %v; %s", err, a)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatalf("JSON でない: %v; %s", err, b)
	}
	return reflect.DeepEqual(x, y)
}

func favoriteUpdatedAt(t *testing.T, db *sql.DB, id int64) time.Time {
	t.Helper()
	var ts time.Time
	if err := db.QueryRowContext(ctx(), `SELECT updated_at FROM favorites WHERE id = ?`, id).Scan(&ts); err != nil {
		t.Fatalf("favorites.updated_at を読めない: %v", err)
	}
	return ts.UTC()
}

// 作成: ID は store が発行し、CreatedAt == UpdatedAt == now(DATETIME(6) の精度)。species_key 列に入り、
// snapshot_hash は Snapshot のバイト列の SHA-256(16進小文字64文字)。一覧で同じものが返る。
func TestTiDBCreateFavoriteAndList(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	in := Favorite{ID: 999999, SpeciesKey: "9001-000", Snapshot: favoriteSnapshot("a"), CreatedAt: now.Add(-time.Hour)}
	fav, outcome, err := st.CreateFavorite(ctx(), deviceID, in, now)
	if err != nil {
		t.Fatalf("CreateFavorite: %v", err)
	}
	if outcome != FavoriteCreated {
		t.Errorf("outcome = %v, want FavoriteCreated", outcome)
	}
	if fav.ID <= 0 || fav.ID == 999999 {
		t.Errorf("ID = %d(store が発行する正の値。引数の ID は無視する)", fav.ID)
	}
	if !fav.CreatedAt.Equal(now) || !fav.UpdatedAt.Equal(now) {
		t.Errorf("CreatedAt/UpdatedAt = %v/%v, want 両方 %v(引数の CreatedAt は無視する)", fav.CreatedAt, fav.UpdatedAt, now)
	}

	var speciesKey, hash string
	if err := db.QueryRowContext(ctx(), `SELECT species_key, snapshot_hash FROM favorites WHERE id = ?`, fav.ID).Scan(&speciesKey, &hash); err != nil {
		t.Fatalf("favorites を読めない: %v", err)
	}
	sum := sha256.Sum256(favoriteSnapshot("a"))
	if speciesKey != "9001-000" || hash != hex.EncodeToString(sum[:]) {
		t.Errorf("species_key=%q snapshot_hash=%q, want 9001-000 / %x", speciesKey, hash, sum)
	}

	list, err := st.ListFavorites(ctx(), deviceID)
	if err != nil {
		t.Fatalf("ListFavorites: %v", err)
	}
	if len(list) != 1 || list[0].ID != fav.ID || list[0].SpeciesKey != "9001-000" || !jsonEqual(t, list[0].Snapshot, favoriteSnapshot("a")) {
		t.Fatalf("一覧 = %+v, want 作った1件", list)
	}
	if !list[0].CreatedAt.Equal(now) || !list[0].UpdatedAt.Equal(now) {
		t.Errorf("一覧の時刻 = %v/%v, want %v(DATETIME(6) で往復する)", list[0].CreatedAt, list[0].UpdatedAt, now)
	}

	// 1件も無い端末は長さ0(エラーにしない)。
	empty, err := st.ListFavorites(ctx(), newDeviceID(t))
	if err != nil || len(empty) != 0 {
		t.Errorf("空の端末の一覧 = %v, %v, want 0件・エラーなし", empty, err)
	}
}

// 一覧の並び: UpdatedAt の降順、同時刻は ID の降順。他端末の行は含めない(AC-D1)。
func TestTiDBListFavoritesOrderAndIsolation(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID, other := newDeviceID(t), newDeviceID(t)
	base := time.Now().UTC().Truncate(time.Microsecond)

	a, _, err := st.CreateFavorite(ctx(), deviceID, newFavorite("a"), base)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := st.CreateFavorite(ctx(), deviceID, newFavorite("b"), base) // a と同時刻
	if err != nil {
		t.Fatal(err)
	}
	c, _, err := st.CreateFavorite(ctx(), deviceID, newFavorite("c"), base.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.CreateFavorite(ctx(), other, newFavorite("a"), base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	list, err := st.ListFavorites(ctx(), deviceID)
	if err != nil {
		t.Fatal(err)
	}
	var got []int64
	for _, f := range list {
		got = append(got, f.ID)
	}
	hi, lo := a.ID, b.ID
	if lo > hi {
		hi, lo = lo, hi
	}
	want := []int64{c.ID, hi, lo}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("並び = %v, want %v(updated_at 降順・同時刻は id 降順。他端末の行を含まない)", got, want)
	}
}

// 冪等(ADR-0227 §2): 同じ端末・同じ Snapshot の2回目は FavoriteExisted で同じ ID を返し、行は増えない。
// CreatedAt は変わらず、UpdatedAt(DB の列も)が now に進む。別の端末の同じ Snapshot は別の行。
func TestTiDBCreateFavoriteIsIdempotent(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID, other := newDeviceID(t), newDeviceID(t)
	t1 := time.Now().UTC().Truncate(time.Microsecond)
	t2 := t1.Add(48 * time.Hour)

	first, _, err := st.CreateFavorite(ctx(), deviceID, newFavorite("x"), t1)
	if err != nil {
		t.Fatal(err)
	}
	again, outcome, err := st.CreateFavorite(ctx(), deviceID, newFavorite("x"), t2)
	if err != nil {
		t.Fatalf("2回目: %v", err)
	}
	if outcome != FavoriteExisted || again.ID != first.ID {
		t.Errorf("2回目 = (%d, %v), want (%d, FavoriteExisted)", again.ID, outcome, first.ID)
	}
	if !again.CreatedAt.Equal(t1) || !again.UpdatedAt.Equal(t2) {
		t.Errorf("2回目の時刻 = %v/%v, want CreatedAt %v・UpdatedAt %v", again.CreatedAt, again.UpdatedAt, t1, t2)
	}
	if got := favoriteUpdatedAt(t, db, first.ID); !got.Equal(t2) {
		t.Errorf("DB の updated_at = %v, want %v(失効の判定 max(last_seen_at, updated_at) が進む。ADR-0209 §4)", got, t2)
	}
	if n := countByDevice(t, db, "favorites", deviceID); n != 1 {
		t.Errorf("行数 = %d, want 1", n)
	}
	if _, outcome, err := st.CreateFavorite(ctx(), other, newFavorite("x"), t1); err != nil || outcome != FavoriteCreated {
		t.Errorf("別端末の同じ内容 = %v, %v, want FavoriteCreated(端末をまたいで重複判定しない)", outcome, err)
	}
}

// 冪等(同時): 同じ要求が同時に来ても行は1つで、全員が同じ ID をエラーなしで受け取る
// (UNIQUE (device_id, snapshot_hash) の一意制約違反を「既存を返す」に写す)。
func TestTiDBCreateFavoriteConcurrentDuplicates(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	const n = 8
	ids := make([]int64, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f, _, err := st.CreateFavorite(ctx(), deviceID, newFavorite("same"), now)
			ids[i], errs[i] = f.ID, err
		}(i)
	}
	wg.Wait()
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Errorf("%d 番目: %v(重複は既存を返し、エラーにしない)", i, errs[i])
		} else if ids[i] != ids[0] {
			t.Errorf("%d 番目の ID = %d, want %d(全員が同じ行を受け取る)", i, ids[i], ids[0])
		}
	}
	if c := countByDevice(t, db, "favorites", deviceID); c != 1 {
		t.Errorf("行数 = %d, want 1(同時の二重作成でも1行)", c)
	}
}

// 上限(ADR-0227 §3): MaxFavoritesPerDevice 件ある端末への新しい作成は ErrFavoriteLimitReached。
// 上限でも同じ内容の再ピン留めは FavoriteExisted。別の端末は影響を受けない。
func TestTiDBCreateFavoriteLimit(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	for i := 0; i < MaxFavoritesPerDevice; i++ {
		if _, _, err := st.CreateFavorite(ctx(), deviceID, newFavorite(fmt.Sprintf("n%d", i)), now); err != nil {
			t.Fatalf("%d 件目: %v", i+1, err)
		}
	}
	if _, _, err := st.CreateFavorite(ctx(), deviceID, newFavorite("over"), now); !errors.Is(err, ErrFavoriteLimitReached) {
		t.Errorf("上限を超える作成 = %v, want ErrFavoriteLimitReached", err)
	}
	if _, outcome, err := st.CreateFavorite(ctx(), deviceID, newFavorite("n0"), now.Add(time.Minute)); err != nil || outcome != FavoriteExisted {
		t.Errorf("上限での再ピン留め = %v, %v, want FavoriteExisted", outcome, err)
	}
	if c := countByDevice(t, db, "favorites", deviceID); c != MaxFavoritesPerDevice {
		t.Errorf("行数 = %d, want %d", c, MaxFavoritesPerDevice)
	}
	if _, _, err := st.CreateFavorite(ctx(), newDeviceID(t), newFavorite("over"), now); err != nil {
		t.Errorf("別端末の作成 = %v, want 成功", err)
	}
}

// 上限(同時): 上限の手前で別々の内容を同時に作っても、上限をすり抜けない
// (件数の確認と挿入を同じトランザクションで行い、端末ごとに直列化する)。
func TestTiDBCreateFavoriteLimitUnderConcurrency(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	const free = 3
	for i := 0; i < MaxFavoritesPerDevice-free; i++ {
		if _, _, err := st.CreateFavorite(ctx(), deviceID, newFavorite(fmt.Sprintf("p%d", i)), now); err != nil {
			t.Fatalf("%d 件目: %v", i+1, err)
		}
	}
	const n = 8
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, errs[i] = st.CreateFavorite(ctx(), deviceID, newFavorite(fmt.Sprintf("c%d", i)), now)
		}(i)
	}
	wg.Wait()
	limited := 0
	for _, err := range errs {
		switch {
		case err == nil:
		case errors.Is(err, ErrFavoriteLimitReached):
			limited++
		default:
			t.Errorf("想定外のエラー: %v(競合は上限か成功のどちらかに解決する)", err)
		}
	}
	if c := countByDevice(t, db, "favorites", deviceID); c != MaxFavoritesPerDevice {
		t.Errorf("行数 = %d, want %d(上限をすり抜けない)", c, MaxFavoritesPerDevice)
	}
	if limited != n-free {
		t.Errorf("上限で断られた数 = %d, want %d", limited, n-free)
	}
}

// 削除(AC-D2): 自分の行は消え、2回目は ErrNotFound。他端末の ID は ErrNotFound で、その行は残る。
func TestTiDBDeleteFavorite(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID, other := newDeviceID(t), newDeviceID(t)
	now := time.Now().UTC()

	mine, _, err := st.CreateFavorite(ctx(), deviceID, newFavorite("m"), now)
	if err != nil {
		t.Fatal(err)
	}
	theirs, _, err := st.CreateFavorite(ctx(), other, newFavorite("t"), now)
	if err != nil {
		t.Fatal(err)
	}

	if err := st.DeleteFavorite(ctx(), deviceID, theirs.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("他端末の ID の削除 = %v, want ErrNotFound", err)
	}
	if c := countByDevice(t, db, "favorites", other); c != 1 {
		t.Errorf("他端末の行 = %d, want 1(消えない)", c)
	}
	if err := st.DeleteFavorite(ctx(), deviceID, mine.ID); err != nil {
		t.Errorf("自分の行の削除 = %v", err)
	}
	if err := st.DeleteFavorite(ctx(), deviceID, mine.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("2回目の削除 = %v, want ErrNotFound", err)
	}
	if err := st.DeleteFavorite(ctx(), deviceID, 1<<62); !errors.Is(err, ErrNotFound) {
		t.Errorf("実在しない ID の削除 = %v, want ErrNotFound", err)
	}
	// 消した内容はもう一度作れる(一意制約が残骸で邪魔をしない)。
	if _, outcome, err := st.CreateFavorite(ctx(), deviceID, newFavorite("m"), now); err != nil || outcome != FavoriteCreated {
		t.Errorf("削除後の再作成 = %v, %v, want FavoriteCreated", outcome, err)
	}
}

// 全削除との整合(ADR-0209 §5・ADR-0227 §6): CreateFavorite で作った行は PurgeDevice で消え、
// Deleted.Favorites に数えられる。墓石が立った後でも新しいピン留めは保存できる。
func TestTiDBPurgeDeviceDeletesCreatedFavorites(t *testing.T) {
	db := testDB(t)
	st := newStore(db, hugeHalfLife, 1000)
	deviceID := newDeviceID(t)
	now := time.Now().UTC()

	for _, label := range []string{"a", "b"} {
		if _, _, err := st.CreateFavorite(ctx(), deviceID, newFavorite(label), now); err != nil {
			t.Fatal(err)
		}
	}
	res, err := st.PurgeDevice(ctx(), deviceID, now.Add(time.Second))
	if err != nil {
		t.Fatalf("PurgeDevice: %v", err)
	}
	if res.Deleted.Favorites != 2 || res.Remaining {
		t.Errorf("PurgeDevice = %+v, want favorites 2・残りなし", res)
	}
	if _, outcome, err := st.CreateFavorite(ctx(), deviceID, newFavorite("a"), now.Add(2*time.Second)); err != nil || outcome != FavoriteCreated {
		t.Errorf("全削除後の作成 = %v, %v, want FavoriteCreated(墓石は計算イベント用)", outcome, err)
	}
}

// DB に届かないときは3つとも ErrUnavailable で包む(httpapi が 503 store_unavailable に写す)。
func TestTiDBFavoritesUnavailable(t *testing.T) {
	testDB(t) // DSN と migration の前提を満たしていること
	closedDB, err := sql.Open("mysql", os.Getenv("RECORD_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	closedDB.Close()
	unreachable := newStore(closedDB, hugeHalfLife, 1000)

	if _, err := unreachable.ListFavorites(ctx(), "d"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("ListFavorites = %v, want ErrUnavailable", err)
	}
	if _, _, err := unreachable.CreateFavorite(ctx(), "d", newFavorite("x"), time.Now()); !errors.Is(err, ErrUnavailable) {
		t.Errorf("CreateFavorite = %v, want ErrUnavailable", err)
	}
	if err := unreachable.DeleteFavorite(ctx(), "d", 1); !errors.Is(err, ErrUnavailable) {
		t.Errorf("DeleteFavorite = %v, want ErrUnavailable", err)
	}
}
