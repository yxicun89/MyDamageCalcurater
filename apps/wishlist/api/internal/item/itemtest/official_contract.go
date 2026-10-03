package itemtest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/oapi-codegen/nullable"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
)

// フェーズ4-3 公式サイトの販売状況の契約テスト(docs/phase4-spec.md AC-O4〜O8)。
// メモリ実装(TestMemoryOfficialRepositoryContract)と MySQL 実装(TestMySQLOfficialRepositoryContract、-tags mysql)に流す。

// OfficialFullRepository は item.Repository と item.OfficialRepository の両方。
type OfficialFullRepository interface {
	item.Repository
	item.OfficialRepository
}

func officialString(s *item.OfficialStatus) string {
	if s == nil {
		return "nil"
	}
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

// RunOfficialRepositoryContract は OfficialRepository と items.watch_official の契約テストを流す。
func RunOfficialRepositoryContract(t *testing.T, newRepo func(t *testing.T) OfficialFullRepository) {
	ctx := context.Background()
	jst := time.FixedZone("JST", 9*60*60)
	t1 := time.Date(2026, 10, 3, 3, 0, 0, 700_000_000, jst) // 秒未満は切り捨てて保存する
	t2 := t1.Add(24 * time.Hour)
	t3 := t2.Add(24 * time.Hour)
	src := "https://tamashii.jp/item/1/"

	type ofix struct {
		fixture
		repo OfficialFullRepository
		it   item.Item
	}
	osetup := func(t *testing.T) ofix {
		t.Helper()
		var r OfficialFullRepository
		f := setup(t, func(t *testing.T) item.Repository {
			r = newRepo(t)
			return r
		})
		it, err := r.CreateItem(ctx, item.NewItem{GenreID: f.genreA.ID, Name: "グリス", ImagePath: img1, SourceURL: strp(src), WatchOfficial: true})
		if err != nil {
			t.Fatalf("CreateItem: %v", err)
		}
		return ofix{fixture: f, repo: r, it: it}
	}
	save := func(t *testing.T, r OfficialFullRepository, itemID int64, c item.OfficialCheck) item.OfficialStatus {
		t.Helper()
		s, err := r.SaveOfficialCheck(ctx, itemID, c)
		if err != nil {
			t.Fatalf("SaveOfficialCheck(%+v): %v", c, err)
		}
		return s
	}
	get := func(t *testing.T, r OfficialFullRepository, id int64) item.Item {
		t.Helper()
		it, err := r.GetItem(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return it
	}
	listed := func(t *testing.T, r OfficialFullRepository, id int64) item.Item {
		t.Helper()
		all, err := r.ListItems(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range all {
			if it.ID == id {
				return it
			}
		}
		t.Fatalf("一覧に商品 %d が無い", id)
		return item.Item{}
	}

	// AC-O4: watch_official は作成・更新で保存し、GetItem・ListItems・UpdateItem の結果に出る。更新で省略(nil)なら変えない。
	t.Run("WatchOfficialPersisted", func(t *testing.T) {
		f := osetup(t)
		if !f.it.WatchOfficial {
			t.Error("作成結果の WatchOfficial が false")
		}
		if !get(t, f.repo, f.it.ID).WatchOfficial || !listed(t, f.repo, f.it.ID).WatchOfficial {
			t.Error("読み直すと WatchOfficial が false")
		}
		other := f.newItem(t, f.genreA.ID, "ビルド", 0)
		if other.WatchOfficial || get(t, f.repo, other.ID).WatchOfficial {
			t.Error("既定が true になっている")
		}
		up, err := f.repo.UpdateItem(ctx, f.it.ID, item.ItemPatch{Name: strp("グリス改")})
		if err != nil || !up.WatchOfficial {
			t.Errorf("省略で変わった: %+v %v", up, err)
		}
		up, err = f.repo.UpdateItem(ctx, f.it.ID, item.ItemPatch{WatchOfficial: boolp(false)})
		if err != nil || up.WatchOfficial || get(t, f.repo, f.it.ID).WatchOfficial {
			t.Errorf("false にできない: %+v %v", up, err)
		}
		up, err = f.repo.UpdateItem(ctx, other.ID, item.ItemPatch{WatchOfficial: boolp(true), SourceURL: nullable.NewNullableWithValue(src)})
		if err != nil || !up.WatchOfficial || !listed(t, f.repo, other.ID).WatchOfficial {
			t.Errorf("true にできない: %+v %v", up, err)
		}
	})

	// AC-O5: 保存した状態は GetItem・ListItems の Official に出る(時刻は秒未満切り捨ての UTC)。他の商品は nil。
	// 2 回目以降は MergeOfficial の規則で重ねる。戻り値は保存した状態と同じ。
	t.Run("SaveAndRead", func(t *testing.T) {
		f := osetup(t)
		other := f.newItem(t, f.genreA.ID, "ビルド", 0)
		if f.it.Official != nil || get(t, f.repo, f.it.ID).Official != nil {
			t.Fatal("保存前に Official がある")
		}
		steps := []item.OfficialCheck{
			{State: item.OfficialPreorder, Evidence: []string{"予約受付中", "予約する"}, At: t1},
			{State: item.OfficialFailed, At: t2},
			{State: item.OfficialEnded, Evidence: []string{"予約受付終了"}, At: t3},
		}
		var prev *item.OfficialStatus
		for i, c := range steps {
			want := item.MergeOfficial(prev, c)
			got := save(t, f.repo, f.it.ID, c)
			if officialString(&got) != officialString(&want) {
				t.Errorf("%d 回目の戻り値 = %s\nwant %s", i+1, officialString(&got), officialString(&want))
			}
			for name, it := range map[string]item.Item{"GetItem": get(t, f.repo, f.it.ID), "ListItems": listed(t, f.repo, f.it.ID)} {
				if officialString(it.Official) != officialString(&want) {
					t.Errorf("%d 回目の %s = %s\nwant %s", i+1, name, officialString(it.Official), officialString(&want))
				}
				if it.Official != nil && (it.Official.CheckedAt.Location() != time.UTC || it.Official.CheckedAt.Nanosecond() != 0) {
					t.Errorf("%s の CheckedAt は秒未満切り捨ての UTC: %v", name, it.Official.CheckedAt)
				}
			}
			prev = &want
		}
		if prev.Status != item.OfficialEnded || prev.PreviousStatus == nil || *prev.PreviousStatus != item.OfficialPreorder {
			t.Errorf("最後の状態 = %s(preorder → ended の変化のはず)", officialString(prev))
		}
		if o := get(t, f.repo, other.ID).Official; o != nil {
			t.Errorf("他の商品に Official = %s", officialString(o))
		}
		// 返した値を変えても保存した値は変わらない
		got := get(t, f.repo, f.it.ID)
		if len(got.Official.Evidence) > 0 {
			got.Official.Evidence[0] = "変えた"
		}
		if again := get(t, f.repo, f.it.ID); len(again.Official.Evidence) > 0 && again.Official.Evidence[0] == "変えた" {
			t.Error("返した Evidence が保存した値と同じ配列を指している")
		}
	})

	// AC-O6: 商品が無ければ ErrNotFound。状態・根拠が規則に合わなければ ErrInvalid で何も変えない。
	t.Run("SaveErrors", func(t *testing.T) {
		f := osetup(t)
		if _, err := f.repo.SaveOfficialCheck(ctx, 99_999_999, item.OfficialCheck{State: item.OfficialAvailable, At: t1}); !errors.Is(err, item.ErrNotFound) {
			t.Errorf("存在しない商品: err = %v, want ErrNotFound", err)
		}
		base := save(t, f.repo, f.it.ID, item.OfficialCheck{State: item.OfficialAvailable, Evidence: []string{"販売中"}, At: t1})
		long := strings.Repeat("あ", item.MaxOfficialEvidenceLen+1)
		bad := []item.OfficialCheck{
			{State: "", At: t2},
			{State: "sold_out", At: t2},
			{State: item.OfficialSoldOut, Evidence: []string{"在庫切れ", "売り切れ", "SOLD OUT", "在庫なし"}, At: t2},
			{State: item.OfficialSoldOut, Evidence: []string{long}, At: t2},
			{State: item.OfficialSoldOut, Evidence: []string{""}, At: t2},
		}
		for _, c := range bad {
			if _, err := f.repo.SaveOfficialCheck(ctx, f.it.ID, c); !errors.Is(err, item.ErrInvalid) {
				t.Errorf("SaveOfficialCheck(%q, %q): err = %v, want ErrInvalid", c.State, c.Evidence, err)
			}
		}
		if got := get(t, f.repo, f.it.ID).Official; officialString(got) != officialString(&base) {
			t.Errorf("失敗したのに変わった: %s", officialString(got))
		}
		ok := strings.Repeat("あ", item.MaxOfficialEvidenceLen)
		if _, err := f.repo.SaveOfficialCheck(ctx, f.it.ID, item.OfficialCheck{State: item.OfficialSoldOut, Evidence: []string{ok, "在庫切れ", "売り切れ"}, At: t2}); err != nil {
			t.Errorf("3 語・%d 文字ちょうどは通る: %v", item.MaxOfficialEvidenceLen, err)
		}
	})

	// AC-O7: source_url を別の値・null に変えると保存済みの状態を消す。同じ値・他の項目・watch_official の切り替えでは残す。
	t.Run("SourceURLChangeClearsStatus", func(t *testing.T) {
		f := osetup(t)
		save(t, f.repo, f.it.ID, item.OfficialCheck{State: item.OfficialAvailable, Evidence: []string{"販売中"}, At: t1})
		keep := []item.ItemPatch{
			{Name: strp("グリス改")},
			{SourceURL: nullable.NewNullableWithValue(src)},
			{WatchOfficial: boolp(false)},
			{WatchOfficial: boolp(true)},
		}
		for i, p := range keep {
			up, err := f.repo.UpdateItem(ctx, f.it.ID, p)
			if err != nil {
				t.Fatal(err)
			}
			if up.Official == nil || get(t, f.repo, f.it.ID).Official == nil {
				t.Errorf("更新 %d で状態が消えた", i)
			}
		}
		up, err := f.repo.UpdateItem(ctx, f.it.ID, item.ItemPatch{SourceURL: nullable.NewNullableWithValue("https://tamashii.jp/item/2/")})
		if err != nil {
			t.Fatal(err)
		}
		if up.Official != nil || get(t, f.repo, f.it.ID).Official != nil {
			t.Error("source_url を変えても状態が残っている(別のページの状態になる)")
		}
		save(t, f.repo, f.it.ID, item.OfficialCheck{State: item.OfficialAvailable, Evidence: []string{"販売中"}, At: t2})
		if _, err := f.repo.UpdateItem(ctx, f.it.ID, item.ItemPatch{SourceURL: nullable.NewNullNullable[string](), WatchOfficial: boolp(false)}); err != nil {
			t.Fatal(err)
		}
		if got := get(t, f.repo, f.it.ID); got.Official != nil {
			t.Errorf("source_url を null にしても状態が残っている: %s", officialString(got.Official))
		}
	})

	// AC-O8: 商品を消すと状態も消える(同じ ID には保存できない)。
	t.Run("DeleteItemCascades", func(t *testing.T) {
		f := osetup(t)
		save(t, f.repo, f.it.ID, item.OfficialCheck{State: item.OfficialAvailable, Evidence: []string{"販売中"}, At: t1})
		if _, err := f.repo.DeleteItem(ctx, f.it.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repo.SaveOfficialCheck(ctx, f.it.ID, item.OfficialCheck{State: item.OfficialAvailable, At: t2}); !errors.Is(err, item.ErrNotFound) {
			t.Errorf("消した商品への保存: err = %v, want ErrNotFound", err)
		}
		all, err := f.repo.ListItems(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range all {
			if it.Official != nil && it.ID == f.it.ID {
				t.Error("消した商品の状態が残っている")
			}
		}
	})
}

func boolp(b bool) *bool { return &b }
