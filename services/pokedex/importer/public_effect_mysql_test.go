//go:build mysql

package importer_test

// 実 MySQL で、公開 API の Item / Ability の effect(issue #211・ADR-0218)が内部 API(getMasterExport)の
// effect と同じ値になることを確かめる。sqlc のクエリ(SearchItems・ListSpeciesAbilityNames などに効果の列を
// 足す変更)と MySQL の JSON 列の正規化を、偽の Querier ではなく実物で通す。`make test-db` だけが実行する。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/pokedex/importer"
	"example.com/pokecalc/services/pokedex/internal/httpapi"
	"example.com/pokecalc/services/pokedex/internal/readtx"
)

// getPublic は公開 API を端末ID・セッションID付きで呼び、200 の本文を json.Number で読む。
func getPublic(t *testing.T, h http.Handler, target string) any {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("X-Device-Id", "00000000-0000-4000-8000-000000000001")
	req.Header.Set("X-Session-Id", "00000000-0000-4000-8000-000000000002")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status = %d, want 200\nbody=%s", target, rec.Code, rec.Body.String())
	}
	var v any
	dec := json.NewDecoder(strings.NewReader(rec.Body.String()))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("GET %s: 本文を読めない: %v", target, err)
	}
	return v
}

func TestPublicEffectMatchesMasterExportOnMySQL(t *testing.T) {
	conn := freshImportDB(t)
	out, versions := fixtureOutput(t)
	if err := importer.Apply(context.Background(), conn, out, versions, time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	h := httpapi.NewHandler(readtx.NewDB(conn))

	var ex map[string]any
	dec := json.NewDecoder(strings.NewReader(string(getMaster(t, h))))
	dec.UseNumber()
	if err := dec.Decode(&ex); err != nil {
		t.Fatal(err)
	}
	internal := func(kind string) map[string]any {
		m := map[string]any{}
		for _, v := range ex[kind].([]any) {
			o := v.(map[string]any)
			m[o["id"].(string)] = o["effect"]
		}
		return m
	}
	internalItems, internalAbilities := internal("items"), internal("abilities")

	// compare は公開の1件と内部の同じ ID を比べ、効果の有無を数える。
	var withEffect, withoutEffect int
	compare := func(kind string, obj map[string]any, want map[string]any) {
		id := obj["id"].(string)
		w, known := want[id]
		if !known {
			t.Errorf("%s %s が内部 API に無い", kind, id)
			return
		}
		got, has := obj["effect"]
		switch {
		case w == nil && has:
			t.Errorf("%s %s: 効果が無いのに effect キーがある(省くこと): %#v", kind, id, got)
		case w != nil && !reflect.DeepEqual(got, w):
			t.Errorf("%s %s: 公開 effect = %#v, 内部 effect = %#v", kind, id, got, w)
		case w != nil:
			withEffect++
		default:
			withoutEffect++
		}
	}

	for _, v := range getPublic(t, h, "/api/pokedex/items?limit=200").([]any) {
		compare("items", v.(map[string]any), internalItems)
	}
	for _, sp := range getPublic(t, h, "/api/pokedex/species?limit=200").([]any) {
		key := sp.(map[string]any)["key"].(string)
		detail := getPublic(t, h, "/api/pokedex/species/"+key).(map[string]any)
		for _, a := range detail["abilities"].([]any) {
			compare("abilities", a.(map[string]any), internalAbilities)
		}
	}
	// q に一致しない持ち物は返らない(実 MySQL の LIKE。偽 Querier の前方一致との一致の確認)。
	if got := getPublic(t, h, "/api/pokedex/items?q=%E3%81%9D%E3%82%93%E3%81%AA%E3%82%82%E3%81%AE%E3%81%AF%E7%84%A1%E3%81%84").([]any); len(got) != 0 {
		t.Errorf("一致しない q なのに持ち物が %d 件返った", len(got))
	}
	// 架空 fixture(testdata/fictional)は効果ありの持ち物・特性と、効果なしの行の両方を持つ。片方が 0 なら比較が空振りしている。
	if withEffect == 0 || withoutEffect == 0 {
		t.Fatalf("効果あり %d 件・効果なし %d 件(どちらも 1 件以上を比べること)", withEffect, withoutEffect)
	}
}
