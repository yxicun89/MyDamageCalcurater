package httpapi

// エラーの共通語彙・HTTP ステータス・ヘッダ・担当外ルート・panic 回復・/healthz の受け入れテスト
// (ADR-0018 AC-5〜AC-7)。すべてのエラーは本文が {"code","message"} ちょうどで、code は
// api/openapi.yaml の ErrorCode(契約テストでも照合する)。

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
)

// calcBody は妥当な /api/calc の本文(書き換え用)。
func calcBody() map[string]any { return calcCases()[0].httpBody() }

func calcWith(t *testing.T, mutate func(b map[string]any)) []byte {
	t.Helper()
	b := calcBody()
	mutate(b)
	return mustJSON(t, b)
}

func attackerOf(b map[string]any) map[string]any { return b["attacker"].(map[string]any) }

// AC-5: 入力の不正と ID 不明はすべて 400。code は WASM 境界と共通の語彙 + HTTP だけの unknown_*。
func TestCalcDamageErrorVocabulary(t *testing.T) {
	h := NewHandler(newFakeStore(t))
	tests := []struct {
		name     string
		body     []byte
		wantCode string
	}{
		// 構文(厳格デコード。engine/wasmapi の decodeStrict と同じ振る舞い)
		{"壊れた JSON", []byte(`{"format":`), "invalid_json"},
		{"オブジェクトでない", []byte(`[]`), "invalid_json"},
		{"空の本文", []byte(``), "invalid_json"},
		{"型が合わない(SP に文字列)", calcWith(t, func(b map[string]any) {
			attackerOf(b)["sp"] = map[string]any{"hp": "a", "atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": 0}
		}), "invalid_json"},
		{"SP に小数", calcWith(t, func(b map[string]any) {
			attackerOf(b)["sp"] = map[string]any{"hp": 1.5, "atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": 0}
		}), "invalid_json"},
		{"JSON の後ろに余計なデータ", append(calcWith(t, func(map[string]any) {}), []byte(` {}`)...), "invalid_json"},
		{"未知のトップレベルフィールド", calcWith(t, func(b map[string]any) { b["attackerr"] = true }), "unknown_field"},
		{"未知の入れ子フィールド", calcWith(t, func(b map[string]any) { attackerOf(b)["hiddenPower"] = "fire" }), "unknown_field"},
		// 列挙
		{"未知の形式", calcWith(t, func(b map[string]any) { b["format"] = "triple" }), "invalid_enum"},
		{"形式の欠落(必須)", calcWith(t, func(b map[string]any) { delete(b, "format") }), "invalid_enum"},
		{"未知の天候", calcWith(t, func(b map[string]any) { b["field"] = map[string]any{"weather": "sunny"} }), "invalid_enum"},
		{"未知のフィールド効果", calcWith(t, func(b map[string]any) { b["field"] = map[string]any{"terrain": "swamp"} }), "invalid_enum"},
		{"未知の状態異常", calcWith(t, func(b map[string]any) { attackerOf(b)["status"] = "confused" }), "invalid_enum"},
		{"未知のテラスタイプ", calcWith(t, func(b map[string]any) { attackerOf(b)["teraType"] = "cosmic" }), "invalid_enum"},
		// engine の入力検証(Individual.Validate)
		{"SP 合計 67", calcWith(t, func(b map[string]any) {
			attackerOf(b)["sp"] = statsMap(engine.Stats{Atk: 32, Def: 32, Spe: 3})
		}), "invalid_input"},
		{"SP 33", calcWith(t, func(b map[string]any) { attackerOf(b)["sp"] = statsMap(engine.Stats{Atk: 33}) }), "invalid_input"},
		{"SP が負", calcWith(t, func(b map[string]any) { attackerOf(b)["sp"] = statsMap(engine.Stats{Atk: -1}) }), "invalid_input"},
		{"ランク 7", calcWith(t, func(b map[string]any) { attackerOf(b)["ranks"] = ranksMap(engine.Ranks{Atk: 7}) }), "invalid_input"},
		{"レベル 51", calcWith(t, func(b map[string]any) { attackerOf(b)["level"] = 51 }), "invalid_input"},
		// ID 不明(HTTP だけの語彙)
		{"未知の種族", calcWith(t, func(b map[string]any) { attackerOf(b)["speciesKey"] = speciesUnknown }), "unknown_species"},
		{"未知の技", calcWith(t, func(b map[string]any) { b["moveId"] = "test-nothing" }), "unknown_move"},
		{"技の欠落(空の ID)", calcWith(t, func(b map[string]any) { delete(b, "moveId") }), "unknown_move"},
		{"未知の持ち物", calcWith(t, func(b map[string]any) { attackerOf(b)["itemId"] = "test-nothing" }), "unknown_item"},
		{"未知の特性", calcWith(t, func(b map[string]any) { attackerOf(b)["abilityId"] = "test-nothing" }), "unknown_ability"},
		{"未知の性格", calcWith(t, func(b map[string]any) { attackerOf(b)["natureId"] = "test-nothing" }), "unknown_nature"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, h, "/api/calc", tt.body, false)
			assertError(t, rec, http.StatusBadRequest, tt.wantCode)
		})
	}
}

// AC-5: 検証の順序(wasmapi と同じ): 構文 → 列挙 → ID 解決 → 入力検証。複数の不正があるとき先の段の code になる。
func TestCalcDamageValidationOrder(t *testing.T) {
	h := NewHandler(newFakeStore(t))
	tests := []struct {
		name     string
		body     []byte
		wantCode string
	}{
		{"未知フィールドと未知の列挙 → unknown_field", calcWith(t, func(b map[string]any) {
			b["bogus"] = 1
			b["format"] = "triple"
		}), "unknown_field"},
		{"未知の列挙と未知の ID → invalid_enum", calcWith(t, func(b map[string]any) {
			b["format"] = "triple"
			b["moveId"] = "test-nothing"
		}), "invalid_enum"},
		{"未知の ID と SP 超過 → unknown_species", calcWith(t, func(b map[string]any) {
			attackerOf(b)["speciesKey"] = speciesUnknown
			attackerOf(b)["sp"] = statsMap(engine.Stats{Atk: 33})
		}), "unknown_species"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, h, "/api/calc", tt.body, false)
			assertError(t, rec, http.StatusBadRequest, tt.wantCode)
		})
	}
}

// AC-6: X-Device-Id / X-Session-Id の欠落・空は 400 missing_header(3操作とも)。UUID 形式は見ない(gateway の仕事)。
func TestMissingHeaders(t *testing.T) {
	h := NewHandler(newFakeStore(t))
	bodies := map[string][]byte{
		"/api/calc":         mustJSON(t, calcBody()),
		"/api/calc/bulk":    mustJSON(t, bulkBody(movePhysical, nil, nil)),
		"/api/calc/reverse": mustJSON(t, reverseCases(t, newFakeStore(t))[0].httpBody()),
	}
	headerCases := []struct {
		name   string
		header func() http.Header
		wantOK bool
	}{
		{"X-Device-Id なし", func() http.Header { h := validHeaders(); h.Del("X-Device-Id"); return h }, false},
		{"X-Session-Id なし", func() http.Header { h := validHeaders(); h.Del("X-Session-Id"); return h }, false},
		{"両方なし", func() http.Header { h := validHeaders(); h.Del("X-Device-Id"); h.Del("X-Session-Id"); return h }, false},
		{"X-Device-Id が空", func() http.Header { h := validHeaders(); h.Set("X-Device-Id", ""); return h }, false},
		{"X-Session-Id が空", func() http.Header { h := validHeaders(); h.Set("X-Session-Id", ""); return h }, false},
		{"UUID でない値は calc-svc では通す", func() http.Header {
			h := validHeaders()
			h.Set("X-Device-Id", "not-a-uuid")
			return h
		}, true},
	}
	for path, body := range bodies {
		for _, hc := range headerCases {
			t.Run(path+"/"+hc.name, func(t *testing.T) {
				header := hc.header()
				rec := serve(t, h, http.MethodPost, path, header, body)
				assertContract(t, http.MethodPost, path, header, body, rec, false)
				if hc.wantOK {
					if rec.Code != http.StatusOK {
						t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
					}
					return
				}
				assertError(t, rec, http.StatusBadRequest, "missing_header")
			})
		}
	}
}

// R1: ヘッダの重複(同名ヘッダを複数個)は missing_header ではなく invalid_header にする
// (ADR-0018 §1.6: missing_header はヘッダ欠落・空に限定する)。当初は invalid_input だったが、
// gateway と語彙を揃えるため ADR-0020 で invalid_header に変更した(期待値の変更。テストは弱めていない)。
func TestDuplicateHeaderIsInvalidHeader(t *testing.T) {
	h := NewHandler(newFakeStore(t))
	header := validHeaders()
	header.Add("X-Device-Id", testDeviceID) // 同じ名前のヘッダをもう1つ足す
	rec := serve(t, h, http.MethodPost, "/api/calc", header, mustJSON(t, calcBody()))
	assertContract(t, http.MethodPost, "/api/calc", header, mustJSON(t, calcBody()), rec, false)
	assertError(t, rec, http.StatusBadRequest, "invalid_header")
}

// AC-7: pokedex の操作は calc-svc の担当外なので 404 not_found(Error 形式・契約どおり)。
func TestPokedexRoutesAreNotFound(t *testing.T) {
	h := NewHandler(newFakeStore(t))
	for _, path := range []string{
		"/api/pokedex/species", "/api/pokedex/species?q=テ", "/api/pokedex/species/9001-000",
		"/api/pokedex/moves", "/api/pokedex/items", "/api/pokedex/natures",
	} {
		t.Run(path, func(t *testing.T) {
			header := validHeaders()
			rec := serve(t, h, http.MethodGet, path, header, nil)
			assertContract(t, http.MethodGet, path, header, nil, rec, false)
			assertError(t, rec, http.StatusNotFound, "not_found")
		})
	}

	// R1: pokedex は生成ラッパ(api.ServerInterfaceWrapper)を経由しない(NewHandler が直接
	// 404 を返す)。ヘッダが無くても・クエリの型が不正でも・ヘッダが重複していても、
	// missing_header / invalid_json / invalid_input 等に化けず常に not_found であること。
	t.Run("ヘッダなしでも not_found", func(t *testing.T) {
		rec := serve(t, h, http.MethodGet, "/api/pokedex/species", http.Header{}, nil)
		assertError(t, rec, http.StatusNotFound, "not_found")
	})
	t.Run("limit=abc でも not_found", func(t *testing.T) {
		header := validHeaders()
		path := "/api/pokedex/species?limit=abc"
		rec := serve(t, h, http.MethodGet, path, header, nil)
		assertContract(t, http.MethodGet, path, header, nil, rec, false)
		assertError(t, rec, http.StatusNotFound, "not_found")
	})
	t.Run("ヘッダが重複していても not_found", func(t *testing.T) {
		header := validHeaders()
		header.Add("X-Device-Id", testDeviceID) // 同じ名前のヘッダをもう1つ足す(値の個数が1でなくなる)
		rec := serve(t, h, http.MethodGet, "/api/pokedex/species", header, nil)
		assertError(t, rec, http.StatusNotFound, "not_found")
	})
}

// AC-7: echo の既定エラーも Error 形式にそろえる。ルートが無いときも、メソッドが違うときも 404 not_found
// (ErrorCode にメソッド違いの語彙を持たない。ADR-0018)。
func TestUnknownRoutesAreNotFound(t *testing.T) {
	h := NewHandler(newFakeStore(t))
	tests := []struct{ method, path string }{
		{http.MethodGet, "/api/nothing"},
		{http.MethodPost, "/api/calc/nothing"},
		{http.MethodGet, "/"},
		{http.MethodGet, "/api/calc"},
		{http.MethodPut, "/api/calc/bulk"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := serve(t, h, tt.method, tt.path, validHeaders(), nil)
			assertError(t, rec, http.StatusNotFound, "not_found")
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
		})
	}
}

// AC-7: panic は回復して 500 internal。message に Go の内部情報を出さない。
func TestPanicIsRecoveredAsInternal(t *testing.T) {
	store := newFakeStore(t)
	store.panicOn = true
	h := NewHandler(store)
	for path, body := range map[string][]byte{
		"/api/calc":         mustJSON(t, calcBody()),
		"/api/calc/bulk":    mustJSON(t, bulkBody(movePhysical, nil, nil)),
		"/api/calc/reverse": mustJSON(t, reverseCases(t, newFakeStore(t))[0].httpBody()),
	} {
		t.Run(path, func(t *testing.T) {
			rec := post(t, h, path, body, false)
			assertError(t, rec, http.StatusInternalServerError, "internal")
			msg := decodeErrorBody(t, rec).Message
			for _, leak := range []string{"panic", "goroutine", ".go:", "fake store", "runtime"} {
				if strings.Contains(msg, leak) {
					t.Errorf("message に内部情報 %q が漏れた: %q", leak, msg)
				}
			}
		})
	}
	// panic の後も同じハンドラで計算できる(状態を壊さない)。
	store.panicOn = false
	rec := post(t, h, "/api/calc", mustJSON(t, calcBody()), true)
	if rec.Code != http.StatusOK {
		t.Fatalf("panic の後の計算 status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

// AC-8: GET /healthz は 200 {"status":"ok"}(openapi に載せない運用エンドポイント。ヘッダ不要)。
func TestHealthz(t *testing.T) {
	h := NewHandler(newFakeStore(t))
	rec := serve(t, h, http.MethodGet, "/healthz", http.Header{}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("本文が JSON でない: %v; body=%s", err, rec.Body.String())
	}
	if len(body) != 1 || body["status"] != "ok" {
		t.Errorf("本文 = %v, want {\"status\":\"ok\"}", body)
	}
}

// R7: 本文の上限(1MiB)を超えるリクエストは invalid_json にする(無制限に読み込まない)。
// note を巨大化するだけで、それ以外は成功するはずのリクエストにする(上限を超えたことだけを見る)。
func TestRequestBodyTooLarge(t *testing.T) {
	store := newFakeStore(t)
	h := NewHandler(store)
	c := reverseCases(t, store)[0]
	body := c.httpBody()
	obs := body["observations"].([]any)
	o := obs[0].(map[string]any)
	o["note"] = strings.Repeat("a", 2<<20) // 2MiB(上限 1MiB を超える)
	rec := post(t, h, "/api/calc/reverse", mustJSON(t, body), false)
	assertError(t, rec, http.StatusBadRequest, "invalid_json")
}
