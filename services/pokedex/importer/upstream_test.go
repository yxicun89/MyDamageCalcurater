package importer_test

// 上流の最新版の検出結果の読み込みと、固定版との比較(ADR-0104 §1・§4・§10)。
// 検出結果はファイル(tools/importer/check-upstream.mjs が書く)で受け取り、Go はネットワークに触らない。

import (
	"errors"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/importer"
)

const (
	pinnedShowdown = "abad1deaabad1deaabad1deaabad1deaabad1dea"
	newerShowdown  = "0123456789abcdef0123456789abcdef01234567"
	pinnedPokeAPI  = "cafef00dcafef00dcafef00dcafef00dcafef00d"
)

const validUpstream = `{"schemaVersion":1,"checkedAt":"2026-09-26T03:00:00Z",
 "sources":{"calc":"test-calc-1","showdown":"` + newerShowdown + `"},
 "errors":{"pokeapi":"HTTP 503"}}`

func TestDecodeUpstreamLatest(t *testing.T) {
	got, err := importer.DecodeUpstreamLatest([]byte(validUpstream))
	if err != nil {
		t.Fatalf("妥当な検出結果を拒否した: %v", err)
	}
	if got.SchemaVersion != 1 || got.CheckedAt != "2026-09-26T03:00:00Z" ||
		got.Sources["showdown"] != newerShowdown || got.Sources["calc"] != "test-calc-1" || got.Errors["pokeapi"] != "HTTP 503" {
		t.Fatalf("読み取り結果が違う: %+v", got)
	}
	if _, err := importer.DecodeUpstreamLatest([]byte(`{"schemaVersion":1,"checkedAt":"2026-09-26T03:00:00Z","sources":{}}`)); err != nil {
		t.Errorf("errors の省略・sources が空(全部取れなかったときは errors に入る)を拒否した: %v", err)
	}
}

func TestDecodeUpstreamLatestIsStrict(t *testing.T) {
	bad := map[string]string{
		"JSON でない":         `not json`,
		"未知のフィールド":         strings.Replace(validUpstream, `"schemaVersion":1,`, `"schemaVersion":1,"extra":true,`, 1),
		"schemaVersion 違い": strings.Replace(validUpstream, `"schemaVersion":1`, `"schemaVersion":2`, 1),
		"checkedAt が無い":    strings.Replace(validUpstream, `"checkedAt":"2026-09-26T03:00:00Z",`, ``, 1),
		"checkedAt の形式":    strings.Replace(validUpstream, `2026-09-26T03:00:00Z`, `2026/09/26`, 1),
		"版が空":              strings.Replace(validUpstream, `"calc":"test-calc-1"`, `"calc":""`, 1),
		"版に空白":             strings.Replace(validUpstream, `"calc":"test-calc-1"`, `"calc":"test calc"`, 1),
		"source の形式":       strings.Replace(validUpstream, `"calc":`, `"Calc_JS":`, 1),
		"同じ source が sources と errors の両方": strings.Replace(validUpstream, `"pokeapi":"HTTP 503"`, `"calc":"HTTP 503"`, 1),
		"後続のデータ":                           validUpstream + `{}`,
	}
	for name, raw := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := importer.DecodeUpstreamLatest([]byte(raw)); !errors.Is(err, importer.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestCompareUpstream(t *testing.T) {
	pinned := map[string]string{"calc": "test-calc-1", "showdown": pinnedShowdown, "pokeapi": pinnedPokeAPI, "extra": "x1"}
	latest, err := importer.DecodeUpstreamLatest([]byte(validUpstream))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 26, 4, 0, 0, 0, time.UTC)
	got := importer.CompareUpstream(pinned, latest, now, 24*time.Hour)

	want := []struct {
		source, latest string
		state          importer.UpstreamState
		detail         string
	}{
		{"calc", "test-calc-1", importer.UpstreamSame, ""},
		{"extra", "", importer.UpstreamUnknown, "not-checked"}, // 検出結果に無い source
		{"pokeapi", "", importer.UpstreamUnknown, "HTTP 503"},  // errors にある source
		{"showdown", newerShowdown, importer.UpstreamDiffers, ""},
	}
	if len(got) != len(want) {
		t.Fatalf("件数 %d, want %d(pinned の source ごとに1件。昇順): %+v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Source != w.source || g.Pinned != pinned[w.source] || g.Latest != w.latest || g.State != w.state {
			t.Errorf("[%d] = %+v, want source=%s pinned=%s latest=%s state=%s", i, g, w.source, pinned[w.source], w.latest, w.state)
		}
		if w.detail != "" && !strings.Contains(g.Detail, w.detail) {
			t.Errorf("[%d] Detail = %q, %q を含むこと", i, g.Detail, w.detail)
		}
	}
}

func TestCompareUpstreamIgnoresSourcesNotPinned(t *testing.T) {
	latest := importer.UpstreamLatest{SchemaVersion: 1, CheckedAt: "2026-09-26T03:00:00Z",
		Sources: map[string]string{"calc": "v1", "unpinned": "v9"}}
	got := importer.CompareUpstream(map[string]string{"calc": "v1"}, latest, time.Date(2026, 9, 26, 3, 0, 0, 0, time.UTC), time.Hour)
	if len(got) != 1 || got[0].Source != "calc" || got[0].State != importer.UpstreamSame {
		t.Fatalf("固定していない source を比較に入れた / 結果が違う: %+v", got)
	}
}

func TestCompareUpstreamStaleResultIsUnknown(t *testing.T) {
	latest, err := importer.DecodeUpstreamLatest([]byte(validUpstream)) // checkedAt 2026-09-26T03:00:00Z
	if err != nil {
		t.Fatal(err)
	}
	pinned := map[string]string{"calc": "test-calc-1", "showdown": pinnedShowdown}
	// 前回の週の結果を使い回して「最新」と誤らない(check-upstream が失敗して古いファイルが残った場合)。
	got := importer.CompareUpstream(pinned, latest, time.Date(2026, 10, 3, 3, 0, 0, 0, time.UTC), 24*time.Hour)
	if len(got) != 2 {
		t.Fatalf("件数 %d: %+v", len(got), got)
	}
	for _, g := range got {
		if g.State != importer.UpstreamUnknown || !strings.Contains(g.Detail, "stale") {
			t.Errorf("古い検出結果が unknown(stale)になっていない: %+v", g)
		}
	}
}

func TestFormatUpstream(t *testing.T) {
	statuses := []importer.UpstreamStatus{
		{Source: "calc", Pinned: "test-calc-1", Latest: "test-calc-1", State: importer.UpstreamSame},
		{Source: "pokeapi", Pinned: pinnedPokeAPI, State: importer.UpstreamUnknown, Detail: "HTTP 503"},
		{Source: "showdown", Pinned: pinnedShowdown, Latest: newerShowdown, State: importer.UpstreamDiffers},
	}
	out := importer.FormatUpstream(statuses)
	if out != importer.FormatUpstream(statuses) {
		t.Fatal("決定的でない")
	}
	var differs string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "showdown") && strings.Contains(line, "UPSTREAM") {
			differs = line
		}
	}
	if differs == "" {
		t.Fatalf("differs の source に UPSTREAM の行が無い:\n%s", out)
	}
	for _, want := range []string{pinnedShowdown, newerShowdown, "data/importer/config.json"} {
		if !strings.Contains(differs, want) {
			t.Errorf("UPSTREAM の行に %q が無い: %q", want, differs)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "UPSTREAM") && !strings.Contains(line, "showdown") {
			t.Errorf("differs でない source に UPSTREAM の行を出した: %q", line)
		}
	}
	if !strings.Contains(out, "HTTP 503") {
		t.Errorf("unknown の理由が表示に無い:\n%s", out)
	}
}
