package query

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type buildCase struct {
	Note          string  `json:"note"`
	Template      string  `json:"template"`
	Name          string  `json:"name"`
	Option        string  `json:"option"`
	QueryOverride *string `json:"query_override"`
	SiteQuery     *string `json:"site_query"`
	Want          string  `json:"want"`
}

func loadBuildCases(t *testing.T) []buildCase {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "query-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Build []buildCase `json:"build"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Build) == 0 {
		t.Fatal("query-cases.json の build が空")
	}
	return v.Build
}

// AC-Q1〜Q4: 共通テストベクタ(仕様 §4 の表の 3 例・優先順位・空白の詰め・空 override の扱い)。
func TestBuild_SharedVectors(t *testing.T) {
	for _, c := range loadBuildCases(t) {
		t.Run(c.Note, func(t *testing.T) {
			got := Build(c.Template, c.Name, c.Option, c.QueryOverride, c.SiteQuery)
			if got != c.Want {
				t.Errorf("Build(%q, %q, %q, %v, %v) = %q, want %q", c.Template, c.Name, c.Option, deref(c.QueryOverride), deref(c.SiteQuery), got, c.Want)
			}
		})
	}
}

// 仕様 §4 の表の 3 例がベクタに必ず含まれていること(ベクタの削除で検査が弱まらないように)。
func TestBuild_SpecTableIsInVectors(t *testing.T) {
	want := map[string]bool{"S.H.Figuarts グリス": false, "ボルシャック 銀トレジャー": false, "HG ガンダムエアリアル": false}
	for _, c := range loadBuildCases(t) {
		if _, ok := want[c.Want]; ok && c.QueryOverride == nil && c.SiteQuery == nil {
			want[c.Want] = true
		}
	}
	for k, ok := range want {
		if !ok {
			t.Errorf("仕様 §4 の例 %q がベクタに無い", k)
		}
	}
}

func deref(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// AC-Q5: 正規化(NFKC → 小文字 → 空白・記号除去)。
func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"S.H.Figuarts グリス", "shfiguartsグリス"},
		{"ＳＨ．Ｆｉｇｕａｒｔｓ", "shfiguarts"}, // 全角英数・記号(NFKC)
		{"S.H.Figuarts　仮面ライダーグリス", "shfiguarts仮面ライダーグリス"},
		{"ボルシャック・ドラゴン", "ボルシャックドラゴン"},
		{"HG 1/144 ガンダム-エアリアル", "hg1144ガンダムエアリアル"},
		{"ｶﾞﾝﾀﾞﾑ", "ガンダム"},              // 半角カナ(NFKC)
		{"ポケモンカード【未開封】★", "ポケモンカード未開封"}, // 括弧・記号
		{"ラーメン", "ラーメン"},                // 長音符は記号ではない(残す)
		{"Ⅱ", "ii"},
		{"", ""},
		{" \t\n", ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got := Normalize(c.in); got != c.want {
				t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
