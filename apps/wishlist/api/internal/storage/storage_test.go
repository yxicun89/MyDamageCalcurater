package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/testimg"
)

var nameRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\.(jpg|png|webp|gif)$`)

func newStorage(t *testing.T) (*FileStorage, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// AC-S1: jpeg/png/webp/gif を内容で判定して保存し、名前は UUID v4 + 拡張子、Open で同じ内容と Content-Type が返る。
func TestSaveOpen_Accepted(t *testing.T) {
	cases := []struct {
		name, ext, ctype string
		data             []byte
	}{
		{"png", ".png", "image/png", testimg.PNG()},
		{"jpeg", ".jpg", "image/jpeg", testimg.JPEG()},
		{"gif", ".gif", "image/gif", testimg.GIF()},
		{"webp", ".webp", "image/webp", testimg.WebP()},
		{"ちょうど上限", ".png", "image/png", testimg.PaddedPNG(MaxImageBytes)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := newStorage(t)
			name, err := s.Save(context.Background(), bytes.NewReader(c.data))
			if err != nil {
				t.Fatalf("Save: %v", err)
			}
			if !nameRe.MatchString(name) || !strings.HasSuffix(name, c.ext) {
				t.Fatalf("name = %q, want UUID v4 + %s", name, c.ext)
			}
			if !ValidName(name) {
				t.Errorf("ValidName(%q) = false", name)
			}
			rc, ct, err := s.Open(name)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer rc.Close()
			got, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, c.data) {
				t.Errorf("内容が一致しない(len %d, want %d)", len(got), len(c.data))
			}
			if ct != c.ctype {
				t.Errorf("content type = %q, want %q", ct, c.ctype)
			}
		})
	}
}

// AC-S1: 保存のたびに名前が変わる(同じ内容でも)。
func TestSave_UniqueNames(t *testing.T) {
	s, _ := newStorage(t)
	a, err := s.Save(context.Background(), bytes.NewReader(testimg.PNG()))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Save(context.Background(), bytes.NewReader(testimg.PNG()))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Errorf("同じ名前 %q", a)
	}
}

// AC-S2/S3: 対応外の形式・上限超えは拒否し、ファイルを残さない。
func TestSave_Rejected(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"svg", testimg.SVG(), ErrUnsupportedImage},
		{"html", []byte("<!doctype html><html><body>x</body></html>"), ErrUnsupportedImage},
		{"text", []byte("hello"), ErrUnsupportedImage},
		{"空", nil, ErrUnsupportedImage},
		{"上限 + 1 バイト", testimg.PaddedPNG(MaxImageBytes + 1), ErrTooLarge},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, dir := newStorage(t)
			_, err := s.Save(context.Background(), bytes.NewReader(c.data))
			if !errors.Is(err, c.want) {
				t.Fatalf("Save err = %v, want %v", err, c.want)
			}
			if n := countFiles(t, dir); n != 0 {
				t.Errorf("拒否したのにファイルが %d 個残っている", n)
			}
		})
	}
}

// AC-S4: 形式外の名前(パストラバーサルを含む)は ErrNotFound。保存先の外のファイルを読まない・消さない。
func TestOpenDelete_InvalidNames(t *testing.T) {
	s, dir := newStorage(t)
	outside := filepath.Join(filepath.Dir(dir), "outside.png")
	if err := os.WriteFile(outside, testimg.PNG(), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(outside) })
	names := []string{
		"",
		"../outside.png",
		"..%2Foutside.png",
		"/etc/passwd",
		"x.png",
		"0f8fad5b-d9cb-469f-a165-70867728950e.svg",
		"0F8FAD5B-D9CB-469F-A165-70867728950E.png", // 大文字は形式外
		"0f8fad5b-d9cb-169f-a165-70867728950e.png", // v1
		"0f8fad5b-d9cb-469f-a165-70867728950e.png/../../outside.png",
		"0f8fad5b-d9cb-469f-a165-70867728950e.png\x00",
	}
	for _, n := range names {
		t.Run(n, func(t *testing.T) {
			if ValidName(n) {
				t.Errorf("ValidName(%q) = true", n)
			}
			if _, _, err := s.Open(n); !errors.Is(err, ErrNotFound) {
				t.Errorf("Open(%q) err = %v, want ErrNotFound", n, err)
			}
			if err := s.Delete(n); !errors.Is(err, ErrNotFound) {
				t.Errorf("Delete(%q) err = %v, want ErrNotFound", n, err)
			}
		})
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("保存先の外のファイルが消えた: %v", err)
	}
}

// AC-S4: 形式は正しいが存在しない名前は Open で ErrNotFound、Delete は nil(冪等)。
func TestOpenDelete_Missing(t *testing.T) {
	s, _ := newStorage(t)
	const n = "0f8fad5b-d9cb-469f-a165-70867728950e.png"
	if _, _, err := s.Open(n); !errors.Is(err, ErrNotFound) {
		t.Errorf("Open err = %v, want ErrNotFound", err)
	}
	if err := s.Delete(n); err != nil {
		t.Errorf("Delete err = %v, want nil", err)
	}
}

// AC-S5: Delete で消え、以後 Open は ErrNotFound。
func TestDelete(t *testing.T) {
	s, dir := newStorage(t)
	name, err := s.Save(context.Background(), bytes.NewReader(testimg.PNG()))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(name); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Open(name); !errors.Is(err, ErrNotFound) {
		t.Errorf("Open after Delete err = %v, want ErrNotFound", err)
	}
	if n := countFiles(t, dir); n != 0 {
		t.Errorf("ファイルが %d 個残っている", n)
	}
}

// AC-S6: NewFileStorage は無いディレクトリを作る。
func TestNewFileStorage_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b")
	s, err := NewFileStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(context.Background(), bytes.NewReader(testimg.PNG())); err != nil {
		t.Fatal(err)
	}
}

// AC-S2: ctx が取り消されていれば保存しない。
func TestSave_Canceled(t *testing.T) {
	s, dir := newStorage(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Save(ctx, bytes.NewReader(testimg.PNG())); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if n := countFiles(t, dir); n != 0 {
		t.Errorf("ファイルが %d 個残っている", n)
	}
}
