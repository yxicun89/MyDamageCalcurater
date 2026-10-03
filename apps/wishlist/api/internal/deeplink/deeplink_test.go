package deeplink

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type deeplinkCase struct {
	Note     string `json:"note"`
	Template string `json:"template"`
	Query    string `json:"query"`
	Want     string `json:"want"`
}

// AC-D1: 共通テストベクタ(encodeURIComponent と同じエスケープ。空白は %20、!'()*-_.~ はそのまま)。
func TestBuild_SharedVectors(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "query-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Deeplink []deeplinkCase `json:"deeplink"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Deeplink) == 0 {
		t.Fatal("query-cases.json の deeplink が空")
	}
	for _, c := range v.Deeplink {
		t.Run(c.Note+"/"+c.Query, func(t *testing.T) {
			if got := Build(c.Template, c.Query); got != c.Want {
				t.Errorf("Build(%q, %q) = %q, want %q", c.Template, c.Query, got, c.Want)
			}
		})
	}
}

// AC-D2: 仕様 §5 の確認済み URL での例(空白は %20)。
func TestBuild_SpecExamples(t *testing.T) {
	got := Build("https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc", "S.H.Figuarts グリス")
	want := "https://jp.mercari.com/search?keyword=S.H.Figuarts%20%E3%82%B0%E3%83%AA%E3%82%B9&status=on_sale&sort=price&order=asc"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// AC-D3: テンプレートの検査。
func TestValidateTemplate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"メルカリ", "https://jp.mercari.com/search?keyword={q}&status=on_sale&sort=price&order=asc", true},
		{"http も可", "http://example.com/s?q={q}", true},
		{"パスに {q}", "https://example.com/search/{q}", true},
		{"{q} が無い", "https://example.com/s?q=", false},
		{"相対 URL", "/search?q={q}", false},
		{"スキーム無し", "example.com/s?q={q}", false},
		{"javascript", "javascript:alert({q})", false},
		{"ftp", "ftp://example.com/{q}", false},
		{"ホスト無し", "https:///s?q={q}", false},
		{"空", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateTemplate(c.in)
			if c.ok && err != nil {
				t.Errorf("ValidateTemplate(%q) = %v, want nil", c.in, err)
			}
			if !c.ok && !errors.Is(err, ErrInvalidTemplate) {
				t.Errorf("ValidateTemplate(%q) = %v, want ErrInvalidTemplate", c.in, err)
			}
		})
	}
}
