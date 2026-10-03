package item_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/oapi-codegen/nullable"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/testimg"
)

// フェーズ4-3 監視の切り替えの検査(docs/phase4-spec.md AC-O3)。

func boolp(b bool) *bool { return &b }

// AC-O3: 作成で watch_official を true にするには source_url が要る(無ければ ErrInvalid・画像も残さない)。
func TestService_CreateWatchOfficial(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	if _, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "グリス", WatchOfficial: true}, bytes.NewReader(testimg.PNG())); !errors.Is(err, item.ErrInvalid) {
		t.Errorf("source_url なしで ON: err = %v, want ErrInvalid", err)
	}
	if n := len(files(t, e.dir)); n != 0 {
		t.Errorf("失敗したのに画像が %d 個残った", n)
	}
	it, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "グリス", SourceURL: strp("https://tamashii.jp/item/1/"), WatchOfficial: true}, bytes.NewReader(testimg.PNG()))
	if err != nil {
		t.Fatal(err)
	}
	if !it.WatchOfficial || it.Official != nil {
		t.Errorf("作成結果 WatchOfficial=%v Official=%v, want true・nil", it.WatchOfficial, it.Official)
	}
	off, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "ビルド"}, bytes.NewReader(testimg.PNG()))
	if err != nil || off.WatchOfficial {
		t.Errorf("既定は OFF: %+v %v", off, err)
	}
}

// AC-O3: 更新は「更新後に ON で source_url が無い」なら ErrInvalid(何も変えない)。
func TestService_UpdateWatchOfficial(t *testing.T) {
	ctx := context.Background()
	src := "https://tamashii.jp/item/1/"
	cases := []struct {
		name    string
		initSrc *string
		initOn  bool
		patch   item.ItemPatch
		wantErr bool
		wantOn  bool
	}{
		{"source_url なしで ON", nil, false, item.ItemPatch{WatchOfficial: boolp(true)}, true, false},
		{"source_url と ON を同時に", nil, false, item.ItemPatch{WatchOfficial: boolp(true), SourceURL: nullable.NewNullableWithValue(src)}, false, true},
		{"source_url ありで ON", &src, false, item.ItemPatch{WatchOfficial: boolp(true)}, false, true},
		{"ON のまま source_url を null", &src, true, item.ItemPatch{SourceURL: nullable.NewNullNullable[string]()}, true, true},
		{"ON のまま source_url を空文字", &src, true, item.ItemPatch{SourceURL: nullable.NewNullableWithValue("")}, true, true},
		{"source_url を null にして OFF", &src, true, item.ItemPatch{SourceURL: nullable.NewNullNullable[string](), WatchOfficial: boolp(false)}, false, false},
		{"OFF にする", &src, true, item.ItemPatch{WatchOfficial: boolp(false)}, false, false},
		{"監視に関係ない更新", &src, true, item.ItemPatch{Name: strp("グリス2")}, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			it, err := e.svc.CreateItem(ctx, item.NewItem{GenreID: e.genre.ID, Name: "グリス", SourceURL: c.initSrc, WatchOfficial: c.initOn}, bytes.NewReader(testimg.PNG()))
			if err != nil {
				t.Fatal(err)
			}
			got, err := e.svc.UpdateItem(ctx, it.ID, c.patch)
			if c.wantErr {
				if !errors.Is(err, item.ErrInvalid) {
					t.Fatalf("err = %v, want ErrInvalid", err)
				}
				after, _ := e.svc.GetItem(ctx, it.ID)
				if after.WatchOfficial != c.initOn || !eqStr(after.SourceURL, c.initSrc) || after.Name != "グリス" {
					t.Errorf("失敗したのに変わった: %+v", after)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.WatchOfficial != c.wantOn {
				t.Errorf("WatchOfficial = %v, want %v", got.WatchOfficial, c.wantOn)
			}
		})
	}
}

func eqStr(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
