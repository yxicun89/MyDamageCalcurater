package httpapi

// 端末単位の全削除 API(DELETE /api/record/device-data。ADR-0209 §5)の受け入れテスト。
// AC-P1 / AC-P1b / AC-P2 / AC-P5 / AC-P6 / AC-P7。

import (
	"net/http"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/record/internal/store"
)

const pathDeviceData = "/api/record/device-data"

// AC-P1: 冪等。削除を2回呼ぶと2回目も 200 completed で deleted は全て 0。
// あわせて「すでに何も無い端末への削除も 200 completed(404 にしない)」(ADR-0209 §5.2)。
func TestDeleteDeviceDataIsIdempotent(t *testing.T) {
	st := newFakeStore()
	st.seed(deviceA, 3, 2, 1)
	h := NewHandler(st)

	first := decodeDeletion(t, serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil))
	if first.Status != api.Completed {
		t.Fatalf("1回目の status = %q, want completed", first.Status)
	}
	if first.Deleted.CalcEvents != 3 || first.Deleted.Aggregates != 2 || first.Deleted.Favorites != 1 {
		t.Errorf("1回目の deleted = %+v, want {3 2 1}", first.Deleted)
	}

	second := decodeDeletion(t, serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil))
	if second.Status != api.Completed {
		t.Errorf("2回目の status = %q, want completed", second.Status)
	}
	if second.Deleted.CalcEvents != 0 || second.Deleted.Aggregates != 0 || second.Deleted.Favorites != 0 {
		t.Errorf("2回目の deleted = %+v, want 全て 0(冪等)", second.Deleted)
	}

	// データが1件も無い端末への1回目も completed(404 にしない)。
	empty := decodeDeletion(t, serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceB), nil))
	if empty.Status != api.Completed {
		t.Errorf("空の端末の status = %q, want completed", empty.Status)
	}
}

// AC-P1b: purged_at の再発行。completed になった端末にもう一度 DELETE を呼ぶと
// purgedAt は1回目より新しい時刻になる(ADR-0209 §5.2)。
func TestDeleteDeviceDataReissuesPurgedAt(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	first := decodeDeletion(t, serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil))
	second := decodeDeletion(t, serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil))

	if !second.PurgedAt.After(first.PurgedAt) {
		t.Errorf("2回目の purgedAt = %v は1回目 %v より後でなければならない", second.PurgedAt, first.PurgedAt)
	}
}

// AC-P2: 繰り返し。1回の上限を小さくすると1回目が partial、繰り返すと completed になり行が残らない。
func TestDeleteDeviceDataPartialThenCompleted(t *testing.T) {
	st := newFakeStore()
	st.purgeLimit = 2
	st.seed(deviceA, 3, 2, 1) // 合計6行 → 2行ずつで3回
	h := NewHandler(st)

	var statuses []api.DeletionStatus
	for i := 0; i < 3; i++ {
		got := decodeDeletion(t, serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil))
		statuses = append(statuses, got.Status)
	}
	want := []api.DeletionStatus{api.Partial, api.Partial, api.Completed}
	for i := range want {
		if statuses[i] != want[i] {
			t.Errorf("%d 回目の status = %q, want %q(全体 %v)", i+1, statuses[i], want[i], statuses)
		}
	}
	if left := st.rowsLeft(deviceA); left != 0 {
		t.Errorf("completed の後に %d 行残っている, want 0", left)
	}
}

// AC-P5 / AC-P7 / ADR-0209 §6-1: 削除は他端末のデータを消さず、store には自分の端末 ID だけを渡す。
// (AC-P7 の「team DB を消さない」は、record-svc が team DB へのアクセス手段を一切持たないこと
// — store.Store のメソッドが record DB の操作しか持たず、他サービスの DB もこのプロセスから
// 触れないこと — で満たす。CLAUDE.md 絶対ルール4。)
func TestDeleteDeviceDataDoesNotTouchOtherDevices(t *testing.T) {
	st := newFakeStore()
	st.seed(deviceA, 3, 2, 1)
	st.seed(deviceB, 4, 1, 2)
	h := NewHandler(st)

	decodeDeletion(t, serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil))

	if left := st.rowsLeft(deviceB); left != 7 {
		t.Errorf("端末 B の残り = %d 行, want 7(他端末は消えない)", left)
	}
	for _, id := range st.deviceIDsTouched() {
		if id != deviceA {
			t.Errorf("store が端末 %q で呼ばれた, want %q だけ(ADR-0209 §6-1)", id, deviceA)
		}
	}
}

// AC-P6: 部分障害。削除の途中で DB が落ちたら 503 store_unavailable。墓石は立っているので、
// 復旧後に再送すると残りが消えて completed になる。
func TestDeleteDeviceDataStoreUnavailable(t *testing.T) {
	st := newFakeStore()
	st.seed(deviceA, 3, 2, 1)
	st.unavailableAfterTombstone = true
	h := NewHandler(st)

	rec := serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil)
	assertErrorBody(t, rec, http.StatusServiceUnavailable, api.StoreUnavailable)

	st.mu.Lock()
	tombstoned := !st.state(deviceA).purgedAt.IsZero()
	st.mu.Unlock()
	if !tombstoned {
		t.Error("503 のときも墓石(purged_at)は立っていなければならない(ADR-0209 §5.2 の削除順序)")
	}

	st.unavailableAfterTombstone = false
	got := decodeDeletion(t, serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil))
	if got.Status != api.Completed {
		t.Errorf("再送後の status = %q, want completed", got.Status)
	}
	if left := st.rowsLeft(deviceA); left != 0 {
		t.Errorf("再送後に %d 行残っている, want 0", left)
	}
}

// DB に全く届かないとき(墓石も立てられない)も 503 store_unavailable。DB のエラー文は外に出さない。
func TestDeleteDeviceDataWhenStoreIsCompletelyDown(t *testing.T) {
	st := newFakeStore()
	st.unavailable = true
	h := NewHandler(st)

	rec := serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil)
	assertErrorBody(t, rec, http.StatusServiceUnavailable, api.StoreUnavailable)
	if body := rec.Body.String(); strings.Contains(body, "fake:") || strings.Contains(body, store.ErrUnavailable.Error()) {
		t.Errorf("応答に内部のエラー文が漏れている: %s", body)
	}
}
