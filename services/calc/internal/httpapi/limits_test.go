package httpapi

// 候補・観測の件数上限(ADR-0208。issue #110)。
//
// /api/calc/bulk と /api/calc/reverse は、本文が 1MiB 未満でも配列の件数の積で計算量
// (CPU・メモリ・応答サイズ)を増幅できる。契約に maxItems / uniqueItems / maximum を置き、
// calc-svc は ID の解決・engine の呼び出しより前にそれを自前で検証する
// (生成物 api.ServerInterfaceWrapper はヘッダしか検証しない。ADR-0208 §3)。
//
// ここで固定するのは次の4つ。
//  1. 契約そのものに上限が書いてあること(TestContractDefinesRequestLimits)
//  2. 契約の検証器(kin-openapi)で上限ちょうどが通り、上限+1 と重複が落ちること(TestContractRejectsOverLimitRequests)
//  3. HTTP の実際の応答(上限ちょうど=200 / 超過=400 invalid_input)
//  4. 超過時はマスタ参照も engine 呼び出しも起きないこと(TestRequestLimitsRunBeforeStoreLookup)

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/master"
	"example.com/pokecalc/services/internal/api"
)

// 契約(ADR-0208 §2)で決めた上限。テストの期待値はこの定数だけを見る。
const (
	limitPresets       = 8   // DefenderPreset の enum は 8 値。実質「全種類」
	limitItemVariants  = 64  // 一括計算の持ち物差し替え候補
	limitItemCandidate = 64  // 逆算の持ち物候補
	limitObservations  = 16  // 逆算の観測
	limitMaxCandidates = 128 // 逆算の maxCandidates(2 性格クラス × 64 持ち物)
)

// allPresets は DefenderPreset の全 8 値(上限ちょうど)。
var allPresets = []any{"none", "hp", "hb_boost", "hb", "hb_full", "hd_boost", "hd", "hd_full"}

// fillItemID は上限テスト用に量産する架空の持ち物 ID。
func fillItemID(i int) string { return fmt.Sprintf("test-fill-%03d", i) }

// addFillItems は fakeStore に効果なしの持ち物を n 件足し、その ID を返す
// (架空マスタの持ち物は 3 件しか無く、64 件の「重複しない候補」を作れないため)。
func addFillItems(f *fakeStore, n int) []string {
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := fillItemID(i)
		f.items[id] = engine.Item{ID: id, NameJa: fmt.Sprintf("テストのうめ%03d", i)}
		ids = append(ids, id)
	}
	return ids
}

// itemList は n 件の持ち物候補(JSON の配列)。withNull が true なら先頭を null(持ち物なし)にし、
// 残りを架空 ID で埋める(全体で n 件・重複なし)。
func itemList(n int, withNull bool) []any {
	out := make([]any, 0, n)
	start := 0
	if withNull {
		out = append(out, nil)
		start = 1
	}
	for i := start; i < n; i++ {
		out = append(out, fillItemID(i))
	}
	return out
}

// repeatObservations は同じ観測を n 件並べたもの(件数だけを見るテスト用)。
func repeatObservations(n int, percent int) []any {
	out := make([]any, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, map[string]any{"percent": percent})
	}
	return out
}

// --- 1. 契約に上限が書いてあること ------------------------------------------

func TestContractDefinesRequestLimits(t *testing.T) {
	doc, _ := loadContract(t)
	schema := func(name string) *openapi3.Schema {
		t.Helper()
		ref, ok := doc.Components.Schemas[name]
		if !ok || ref.Value == nil {
			t.Fatalf("契約に %s が無い", name)
		}
		return ref.Value
	}
	prop := func(schemaName, propName string) *openapi3.Schema {
		t.Helper()
		ref, ok := schema(schemaName).Properties[propName]
		if !ok || ref.Value == nil {
			t.Fatalf("契約の %s に %s が無い", schemaName, propName)
		}
		return ref.Value
	}

	arrays := []struct {
		schema, prop string
		maxItems     uint64
		unique       bool
	}{
		{"BulkCalcRequest", "presets", limitPresets, true},
		{"BulkCalcRequest", "itemVariants", limitItemVariants, true},
		{"ReverseRequest", "itemCandidates", limitItemCandidate, true},
		// observations は Observation オブジェクトなので uniqueItems は付けない(件数だけ制限する)。
		{"ReverseRequest", "observations", limitObservations, false},
	}
	for _, tt := range arrays {
		t.Run(tt.schema+"."+tt.prop, func(t *testing.T) {
			s := prop(tt.schema, tt.prop)
			if s.MaxItems == nil {
				t.Fatalf("maxItems が無い(want %d)", tt.maxItems)
			}
			if *s.MaxItems != tt.maxItems {
				t.Errorf("maxItems = %d, want %d", *s.MaxItems, tt.maxItems)
			}
			if s.UniqueItems != tt.unique {
				t.Errorf("uniqueItems = %v, want %v", s.UniqueItems, tt.unique)
			}
		})
	}

	t.Run("ReverseRequest.maxCandidates", func(t *testing.T) {
		s := prop("ReverseRequest", "maxCandidates")
		if s.Max == nil {
			t.Fatalf("maximum が無い(want %d)", limitMaxCandidates)
		}
		if *s.Max != float64(limitMaxCandidates) {
			t.Errorf("maximum = %v, want %d", *s.Max, limitMaxCandidates)
		}
		// 0 は「全件」の意味のまま残す(issue #110 の受け入れ条件)。
		if s.Min == nil || *s.Min != 0 {
			t.Errorf("minimum = %v, want 0", s.Min)
		}
	})

	t.Run("ReverseRequest.observations の minItems は 1 のまま", func(t *testing.T) {
		if got := prop("ReverseRequest", "observations").MinItems; got != 1 {
			t.Errorf("minItems = %d, want 1", got)
		}
	})
}

// --- 2. 契約の検証器で上限ちょうどが通り、超過・重複が落ちること ---------------

// validateRequestAgainstContract は本文を api/openapi.yaml のリクエストスキーマに照らす。
func validateRequestAgainstContract(t *testing.T, path string, body []byte) error {
	t.Helper()
	in, ok := contractRequest(t, http.MethodPost, path, validHeaders(), body)
	if !ok {
		t.Fatalf("契約に %s が無い", path)
	}
	return openapi3filter.ValidateRequest(context.Background(), in)
}

func TestContractRejectsOverLimitRequests(t *testing.T) {
	store := newFakeStore(t)
	addFillItems(store, limitItemVariants+1)
	withReverse := func(mutate func(b map[string]any)) []byte {
		b := reverseCases(t, store)[0].httpBody()
		mutate(b)
		return mustJSON(t, b)
	}
	percent := reverseCases(t, store)[0].httpBody()["observations"].([]any)[0].(map[string]any)["percent"].(int)

	tests := []struct {
		name    string
		path    string
		body    []byte
		wantErr bool
	}{
		{"presets 8 件(上限ちょうど)", "/api/calc/bulk",
			mustJSON(t, bulkBody(movePhysical, allPresets, nil)), false},
		{"presets 9 件", "/api/calc/bulk",
			mustJSON(t, bulkBody(movePhysical, append(append([]any{}, allPresets...), "none"), nil)), true},
		{"presets の重複", "/api/calc/bulk",
			mustJSON(t, bulkBody(movePhysical, []any{"none", "hp", "none"}, nil)), true},
		{"itemVariants 64 件(上限ちょうど)", "/api/calc/bulk",
			mustJSON(t, bulkBody(movePhysical, nil, itemList(limitItemVariants, true))), false},
		{"itemVariants 65 件", "/api/calc/bulk",
			mustJSON(t, bulkBody(movePhysical, nil, itemList(limitItemVariants+1, true))), true},
		{"itemVariants に同じ ID の重複", "/api/calc/bulk",
			mustJSON(t, bulkBody(movePhysical, nil, []any{itemPlain, itemPlain})), true},
		{"itemVariants に null の重複", "/api/calc/bulk",
			mustJSON(t, bulkBody(movePhysical, nil, []any{nil, nil})), true},

		{"itemCandidates 64 件(上限ちょうど)", "/api/calc/reverse",
			withReverse(func(b map[string]any) { b["itemCandidates"] = itemList(limitItemCandidate, true) }), false},
		{"itemCandidates 65 件", "/api/calc/reverse",
			withReverse(func(b map[string]any) { b["itemCandidates"] = itemList(limitItemCandidate+1, true) }), true},
		{"itemCandidates に同じ ID の重複", "/api/calc/reverse",
			withReverse(func(b map[string]any) { b["itemCandidates"] = []any{itemPlain, itemPlain} }), true},
		{"itemCandidates に null の重複", "/api/calc/reverse",
			withReverse(func(b map[string]any) { b["itemCandidates"] = []any{nil, nil} }), true},
		{"observations 16 件(上限ちょうど)", "/api/calc/reverse",
			withReverse(func(b map[string]any) { b["observations"] = repeatObservations(limitObservations, percent) }), false},
		{"observations 17 件", "/api/calc/reverse",
			withReverse(func(b map[string]any) { b["observations"] = repeatObservations(limitObservations+1, percent) }), true},
		{"maxCandidates 128(上限ちょうど)", "/api/calc/reverse",
			withReverse(func(b map[string]any) { b["maxCandidates"] = limitMaxCandidates }), false},
		{"maxCandidates 129", "/api/calc/reverse",
			withReverse(func(b map[string]any) { b["maxCandidates"] = limitMaxCandidates + 1 }), true},
		{"maxCandidates が負", "/api/calc/reverse",
			withReverse(func(b map[string]any) { b["maxCandidates"] = -1 }), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequestAgainstContract(t, tt.path, tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("契約の検証 = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- 3. HTTP の応答(上限ちょうどは 200、超過・重複は 400 invalid_input) -------

func TestCalcBulkRequestLimits(t *testing.T) {
	store := newFakeStore(t)
	addFillItems(store, limitItemVariants+1)
	h := NewHandler(store)

	t.Run("presets 8 件・itemVariants 64 件(上限ちょうど)は受理し 512 行", func(t *testing.T) {
		body := bulkBody(movePhysical, allPresets, itemList(limitItemVariants, true))
		rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), true)
		var got api.BulkCalcResult
		decodeInto(t, rec, &got)
		if want := limitPresets * limitItemVariants; len(got.Rows) != want {
			t.Errorf("行数 = %d, want %d(上限内の最大組合せ数)", len(got.Rows), want)
		}
	})

	tests := []struct {
		name     string
		body     map[string]any
		wantCode string
	}{
		{"presets 9 件", bulkBody(movePhysical, append(append([]any{}, allPresets...), "none"), nil), "invalid_input"},
		// 8 件以内の重複は従来どおり engine の duplicate_preset(ADR-0208 §4。既存のテストを変えない)。
		{"presets の重複(8 件以内)", bulkBody(movePhysical, []any{"hp", "none", "hp"}, nil), "duplicate_preset"},
		{"itemVariants 65 件", bulkBody(movePhysical, nil, itemList(limitItemVariants+1, true)), "invalid_input"},
		{"itemVariants に同じ ID の重複", bulkBody(movePhysical, nil, []any{itemPlain, itemPlain}), "invalid_input"},
		{"itemVariants に null の重複", bulkBody(movePhysical, nil, []any{nil, nil}), "invalid_input"},
		{"itemVariants に null と同じ ID の重複", bulkBody(movePhysical, nil, []any{nil, itemPlain, nil}), "invalid_input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, h, "/api/calc/bulk", mustJSON(t, tt.body), false)
			assertError(t, rec, http.StatusBadRequest, tt.wantCode)
		})
	}
}

func TestCalcReverseRequestLimits(t *testing.T) {
	store := newFakeStore(t)
	addFillItems(store, limitItemCandidate+1)
	h := NewHandler(store)
	base := func() map[string]any { return reverseCases(t, store)[0].httpBody() }
	percent := base()["observations"].([]any)[0].(map[string]any)["percent"].(int)
	with := func(mutate func(b map[string]any)) []byte {
		b := base()
		mutate(b)
		return mustJSON(t, b)
	}

	t.Run("itemCandidates 64 件・observations 16 件(上限ちょうど)は受理し候補 128 件以下", func(t *testing.T) {
		body := with(func(b map[string]any) {
			b["itemCandidates"] = itemList(limitItemCandidate, true)
			b["observations"] = repeatObservations(limitObservations, percent)
		})
		rec := post(t, h, "/api/calc/reverse", body, true)
		var got api.ReverseResult
		decodeInto(t, rec, &got)
		if len(got.Candidates) != limitMaxCandidates {
			t.Errorf("候補数 = %d, want %d(2 性格クラス × %d 持ち物 = 上限内の最大組合せ数)",
				len(got.Candidates), limitMaxCandidates, limitItemCandidate)
		}
	})

	t.Run("maxCandidates 128(上限ちょうど)は受理", func(t *testing.T) {
		rec := post(t, h, "/api/calc/reverse", with(func(b map[string]any) {
			b["maxCandidates"] = limitMaxCandidates
		}), true)
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
	})

	tests := []struct {
		name     string
		body     []byte
		wantCode string
	}{
		{"itemCandidates 65 件", with(func(b map[string]any) {
			b["itemCandidates"] = itemList(limitItemCandidate+1, true)
		}), "invalid_input"},
		{"itemCandidates に同じ ID の重複", with(func(b map[string]any) {
			b["itemCandidates"] = []any{itemPlain, itemPlain}
		}), "invalid_input"},
		{"itemCandidates に null の重複", with(func(b map[string]any) {
			b["itemCandidates"] = []any{nil, nil}
		}), "invalid_input"},
		{"itemCandidates に null と同じ ID の重複", with(func(b map[string]any) {
			b["itemCandidates"] = []any{nil, itemPlain, nil}
		}), "invalid_input"},
		{"observations 17 件", with(func(b map[string]any) {
			b["observations"] = repeatObservations(limitObservations+1, percent)
		}), "invalid_input"},
		{"maxCandidates 129", with(func(b map[string]any) { b["maxCandidates"] = limitMaxCandidates + 1 }), "invalid_input"},
		// 既存の挙動(ADR-0200)。0 は「全件」のまま。
		{"maxCandidates が負", with(func(b map[string]any) { b["maxCandidates"] = -1 }), "invalid_input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, h, "/api/calc/reverse", tt.body, false)
			assertError(t, rec, http.StatusBadRequest, tt.wantCode)
		})
	}
}

// --- 4. 超過時はマスタ参照も engine 呼び出しも起きないこと ---------------------

// countingStore は master.Store の参照回数を数える(壁時計時間に依存せずに
// 「上限超過は ID 解決・engine 呼び出しより前に落ちる」ことを見るため)。
// TypeChart は engine.CalcBulk / CalcReverse へ渡す入力の一部なので、
// その参照回数が 0 なら engine は呼ばれていない。
type countingStore struct {
	inner    *fakeStore
	lookups  int // Species / Move / Item / Ability / Nature / NatureID
	chartGet int // TypeChart
}

var _ master.Store = (*countingStore)(nil)

func (c *countingStore) Species(key string) (engine.Species, bool) {
	c.lookups++
	return c.inner.Species(key)
}

func (c *countingStore) Move(id string) (engine.Move, bool) {
	c.lookups++
	return c.inner.Move(id)
}

func (c *countingStore) Item(id string) (engine.Item, bool) {
	c.lookups++
	return c.inner.Item(id)
}

func (c *countingStore) Ability(id string) (engine.Ability, bool) {
	c.lookups++
	return c.inner.Ability(id)
}

func (c *countingStore) Nature(id string) (engine.Nature, bool) {
	c.lookups++
	return c.inner.Nature(id)
}

func (c *countingStore) NatureID(n engine.Nature) (string, bool) {
	c.lookups++
	return c.inner.NatureID(n)
}

func (c *countingStore) TypeChart() engine.TypeChart {
	c.chartGet++
	return c.inner.TypeChart()
}

// 上限を超えた要求は、マスタの参照(ID 解決)も engine の呼び出しも起こさずに落ちること。
// ID はすべてマスタに無いものを混ぜてあるので、先に ID 解決が走れば unknown_* になって落ちる。
func TestRequestLimitsRunBeforeStoreLookup(t *testing.T) {
	inner := newFakeStore(t)
	base := reverseCases(t, inner)[0].httpBody()

	overLimitBulk := bulkBody("test-nothing", nil, itemList(limitItemVariants+1, true))
	overLimitBulk["defenderSpeciesKey"] = speciesUnknown

	overLimitReverse := reverseCases(t, inner)[0].httpBody()
	overLimitReverse["moveId"] = "test-nothing"
	overLimitReverse["unknownSpeciesKey"] = speciesUnknown
	overLimitReverse["itemCandidates"] = itemList(limitItemCandidate+1, true)

	tooManyObservations := reverseCases(t, inner)[0].httpBody()
	tooManyObservations["unknownSpeciesKey"] = speciesUnknown
	tooManyObservations["observations"] = repeatObservations(limitObservations+1,
		base["observations"].([]any)[0].(map[string]any)["percent"].(int))

	tooManyPresets := bulkBody("test-nothing", append(append([]any{}, allPresets...), "none"), nil)

	tests := []struct {
		name string
		path string
		body map[string]any
	}{
		{"bulk の itemVariants 65 件(未知の種族・技を含む)", "/api/calc/bulk", overLimitBulk},
		{"bulk の presets 9 件(未知の技を含む)", "/api/calc/bulk", tooManyPresets},
		{"reverse の itemCandidates 65 件(未知の種族・技を含む)", "/api/calc/reverse", overLimitReverse},
		{"reverse の observations 17 件(未知の種族を含む)", "/api/calc/reverse", tooManyObservations},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &countingStore{inner: newFakeStore(t)}
			h := NewHandler(store)
			rec := post(t, h, tt.path, mustJSON(t, tt.body), false)
			assertError(t, rec, http.StatusBadRequest, "invalid_input")
			if store.lookups != 0 {
				t.Errorf("マスタの参照が %d 回(want 0。上限の検証は ID 解決より前)", store.lookups)
			}
			if store.chartGet != 0 {
				t.Errorf("TypeChart の参照が %d 回(want 0。engine を呼んでいる)", store.chartGet)
			}
		})
	}
}

// --- ベンチマーク(上限内の最大組合せでの退行を見る。壁時計の閾値は判定しない) -----

func BenchmarkCalcBulkAtLimit(b *testing.B) {
	store := newFakeStore(b)
	addFillItems(store, limitItemVariants)
	h := NewHandler(store)
	body := mustJSON(b, bulkBody(movePhysical, allPresets, itemList(limitItemVariants, true)))
	header := validHeaders()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := serve(b, h, http.MethodPost, "/api/calc/bulk", header, body)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
		}
	}
}

func BenchmarkCalcReverseAtLimit(b *testing.B) {
	store := newFakeStore(b)
	addFillItems(store, limitItemCandidate)
	h := NewHandler(store)
	body := reverseCases(b, store)[0].httpBody()
	percent := body["observations"].([]any)[0].(map[string]any)["percent"].(int)
	body["itemCandidates"] = itemList(limitItemCandidate, true)
	body["observations"] = repeatObservations(limitObservations, percent)
	raw := mustJSON(b, body)
	header := validHeaders()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := serve(b, h, http.MethodPost, "/api/calc/reverse", header, raw)
		if rec.Code != http.StatusOK {
			b.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
		}
	}
}
