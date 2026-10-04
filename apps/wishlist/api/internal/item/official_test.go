package item_test

import (
	"fmt"
	"testing"
	"time"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// フェーズ4-3 公式サイトの販売状況の純関数(docs/phase4-spec.md AC-O1・AC-O2)。

func TestOfficialState_ValidJudged(t *testing.T) {
	judged := map[item.OfficialState]bool{
		item.OfficialAvailable: true, item.OfficialPreorder: true, item.OfficialSoldOut: true, item.OfficialEnded: true,
		item.OfficialUnknown: true, item.OfficialAmbiguous: true, item.OfficialBlocked: false, item.OfficialFailed: false,
	}
	for s, want := range judged {
		if !s.Valid() || s.Judged() != want {
			t.Errorf("%s: Valid=%v Judged=%v, want true・%v", s, s.Valid(), s.Judged(), want)
		}
	}
	for _, s := range []item.OfficialState{"", "AVAILABLE", "sold_out"} {
		if s.Valid() || s.Judged() {
			t.Errorf("%q は Valid・Judged でない", s)
		}
	}
}

// officialKey は比較用の文字列。
func officialKey(s item.OfficialStatus) string {
	changed, prev := "nil", "nil"
	if s.ChangedAt != nil {
		changed = s.ChangedAt.UTC().Format(time.RFC3339)
	}
	if s.PreviousStatus != nil {
		prev = string(*s.PreviousStatus)
	}
	return fmt.Sprintf("status=%s evidence=%q checked=%s changed=%s prev=%s last=%s attempt=%s",
		s.Status, s.Evidence, s.CheckedAt.UTC().Format(time.RFC3339), changed, prev, s.LastResult, s.LastAttemptAt.UTC().Format(time.RFC3339))
}

func statep(s item.OfficialState) *item.OfficialState { return &s }
func timep(t time.Time) *time.Time                    { return &t }

// AC-O1: MergeOfficial の規則(表)。
//   - 判定済みの結果(available〜ambiguous)は status・evidence・checked_at を置き換える
//   - 判定済みの status が別の判定済みの状態に変わったときだけ changed_at・previous_status を更新する(初回・同じ状態・failed/blocked からは変えない)
//   - failed・blocked は判定済みの status を上書きしない(last_result・last_attempt_at だけ)。判定済みが無ければ status にする(evidence は空)
//   - 時刻は秒未満を切り捨てた UTC
func TestMergeOfficial(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	d1 := time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 10, 2, 3, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 10, 3, 12, 0, 0, 700_000_000, jst) // = 03:00:00.7 UTC
	d3s := time.Date(2026, 10, 3, 3, 0, 0, 0, time.UTC)

	availableD1 := &item.OfficialStatus{Status: item.OfficialAvailable, Evidence: []string{"販売中"}, CheckedAt: d1, LastResult: item.OfficialAvailable, LastAttemptAt: d1}
	changedBefore := &item.OfficialStatus{
		Status: item.OfficialPreorder, Evidence: []string{"予約受付中"}, CheckedAt: d2, ChangedAt: timep(d1), PreviousStatus: statep(item.OfficialAvailable),
		LastResult: item.OfficialPreorder, LastAttemptAt: d2,
	}
	failedOnly := &item.OfficialStatus{Status: item.OfficialFailed, Evidence: []string{}, CheckedAt: d1, LastResult: item.OfficialFailed, LastAttemptAt: d1}
	availableThenFailed := &item.OfficialStatus{Status: item.OfficialAvailable, Evidence: []string{"販売中"}, CheckedAt: d1, LastResult: item.OfficialFailed, LastAttemptAt: d2}

	cases := []struct {
		name string
		prev *item.OfficialStatus
		c    item.OfficialCheck
		want item.OfficialStatus
	}{
		{
			"初回の判定は変化にしない",
			nil, item.OfficialCheck{State: item.OfficialPreorder, Evidence: []string{"予約受付中", "予約する"}, At: d3},
			item.OfficialStatus{Status: item.OfficialPreorder, Evidence: []string{"予約受付中", "予約する"}, CheckedAt: d3s, LastResult: item.OfficialPreorder, LastAttemptAt: d3s},
		},
		{
			"初回が failed なら status も failed(evidence は空)",
			nil, item.OfficialCheck{State: item.OfficialFailed, At: d3},
			item.OfficialStatus{Status: item.OfficialFailed, Evidence: []string{}, CheckedAt: d3s, LastResult: item.OfficialFailed, LastAttemptAt: d3s},
		},
		{
			"初回が blocked なら status も blocked(渡された evidence は捨てる)",
			nil, item.OfficialCheck{State: item.OfficialBlocked, Evidence: []string{"x"}, At: d3},
			item.OfficialStatus{Status: item.OfficialBlocked, Evidence: []string{}, CheckedAt: d3s, LastResult: item.OfficialBlocked, LastAttemptAt: d3s},
		},
		{
			"同じ状態は checked_at と evidence だけ更新",
			availableD1, item.OfficialCheck{State: item.OfficialAvailable, Evidence: []string{"在庫あり"}, At: d3},
			item.OfficialStatus{Status: item.OfficialAvailable, Evidence: []string{"在庫あり"}, CheckedAt: d3s, LastResult: item.OfficialAvailable, LastAttemptAt: d3s},
		},
		{
			"判定済みから別の判定済みへ → changed_at・previous_status",
			availableD1, item.OfficialCheck{State: item.OfficialEnded, Evidence: []string{"販売終了"}, At: d3},
			item.OfficialStatus{Status: item.OfficialEnded, Evidence: []string{"販売終了"}, CheckedAt: d3s, ChangedAt: timep(d3s), PreviousStatus: statep(item.OfficialAvailable), LastResult: item.OfficialEnded, LastAttemptAt: d3s},
		},
		{
			"available → unknown も変化(判定済み同士)",
			availableD1, item.OfficialCheck{State: item.OfficialUnknown, At: d3},
			item.OfficialStatus{Status: item.OfficialUnknown, Evidence: []string{}, CheckedAt: d3s, ChangedAt: timep(d3s), PreviousStatus: statep(item.OfficialAvailable), LastResult: item.OfficialUnknown, LastAttemptAt: d3s},
		},
		{
			"同じ状態なら前の変化(changed_at・previous_status)を残す",
			changedBefore, item.OfficialCheck{State: item.OfficialPreorder, Evidence: []string{"予約受付中"}, At: d3},
			item.OfficialStatus{Status: item.OfficialPreorder, Evidence: []string{"予約受付中"}, CheckedAt: d3s, ChangedAt: timep(d1), PreviousStatus: statep(item.OfficialAvailable), LastResult: item.OfficialPreorder, LastAttemptAt: d3s},
		},
		{
			"failed は判定済みの status を上書きしない(last だけ)",
			changedBefore, item.OfficialCheck{State: item.OfficialFailed, At: d3},
			item.OfficialStatus{Status: item.OfficialPreorder, Evidence: []string{"予約受付中"}, CheckedAt: d2, ChangedAt: timep(d1), PreviousStatus: statep(item.OfficialAvailable), LastResult: item.OfficialFailed, LastAttemptAt: d3s},
		},
		{
			"blocked も判定済みの status を上書きしない",
			availableD1, item.OfficialCheck{State: item.OfficialBlocked, At: d3},
			item.OfficialStatus{Status: item.OfficialAvailable, Evidence: []string{"販売中"}, CheckedAt: d1, LastResult: item.OfficialBlocked, LastAttemptAt: d3s},
		},
		{
			"failed のあとの判定は、前の判定済みの status と比べる",
			availableThenFailed, item.OfficialCheck{State: item.OfficialSoldOut, Evidence: []string{"在庫切れ"}, At: d3},
			item.OfficialStatus{Status: item.OfficialSoldOut, Evidence: []string{"在庫切れ"}, CheckedAt: d3s, ChangedAt: timep(d3s), PreviousStatus: statep(item.OfficialAvailable), LastResult: item.OfficialSoldOut, LastAttemptAt: d3s},
		},
		{
			"failed だけだったあとの初めての判定は変化にしない",
			failedOnly, item.OfficialCheck{State: item.OfficialAvailable, Evidence: []string{"販売中"}, At: d3},
			item.OfficialStatus{Status: item.OfficialAvailable, Evidence: []string{"販売中"}, CheckedAt: d3s, LastResult: item.OfficialAvailable, LastAttemptAt: d3s},
		},
		{
			"failed のあとの blocked(判定済みが無い)は status を blocked にする・変化にしない",
			failedOnly, item.OfficialCheck{State: item.OfficialBlocked, At: d3},
			item.OfficialStatus{Status: item.OfficialBlocked, Evidence: []string{}, CheckedAt: d3s, LastResult: item.OfficialBlocked, LastAttemptAt: d3s},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := item.MergeOfficial(c.prev, c.c)
			if officialKey(got) != officialKey(c.want) {
				t.Errorf("got  %s\nwant %s", officialKey(got), officialKey(c.want))
			}
			if got.Evidence == nil {
				t.Error("Evidence は空でも nil にしない")
			}
			if got.CheckedAt.Location() != time.UTC || got.LastAttemptAt.Location() != time.UTC {
				t.Error("時刻は UTC で返す")
			}
		})
	}
}

// AC-O1: MergeOfficial は prev・c の Evidence を共有しない(返した値を変えても元が変わらない)。
func TestMergeOfficial_DoesNotAlias(t *testing.T) {
	ev := []string{"販売中"}
	got := item.MergeOfficial(nil, item.OfficialCheck{State: item.OfficialAvailable, Evidence: ev, At: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)})
	if len(got.Evidence) != 1 {
		t.Fatalf("Evidence = %q", got.Evidence)
	}
	got.Evidence[0] = "変えた"
	if ev[0] != "販売中" {
		t.Error("返した Evidence が入力と同じ配列を指している")
	}
}

// AC-O2: WatchTarget は WatchOfficial が true で source_url が http(s) の絶対 URL のときだけ対象にする。
func TestWatchTarget(t *testing.T) {
	cases := []struct {
		name  string
		watch bool
		src   *string
		want  string
		ok    bool
	}{
		{"ON・https", true, strp("https://tamashii.jp/item/1/"), "https://tamashii.jp/item/1/", true},
		{"ON・http", true, strp("http://example.com/a"), "http://example.com/a", true},
		{"ON・大文字のスキーム", true, strp("HTTPS://example.com/a"), "HTTPS://example.com/a", true},
		{"OFF", false, strp("https://tamashii.jp/item/1/"), "", false},
		{"ON・source_url なし", true, nil, "", false},
		{"ON・空", true, strp(""), "", false},
		{"ON・ftp", true, strp("ftp://example.com/a"), "", false},
		{"ON・javascript", true, strp("javascript:alert(1)"), "", false},
		{"ON・相対", true, strp("/item/1"), "", false},
		{"ON・ホストなし", true, strp("https:///a"), "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := item.WatchTarget(item.Item{ID: 1, WatchOfficial: c.watch, SourceURL: c.src})
			if got != c.want || ok != c.ok {
				t.Errorf("WatchTarget = %q, %v, want %q, %v", got, ok, c.want, c.ok)
			}
		})
	}
}
