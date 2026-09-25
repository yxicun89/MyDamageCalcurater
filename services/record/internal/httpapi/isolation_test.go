package httpapi

// 端末 ID によるデータ分離(ADR-0209 §6)の受け入れテスト。AC-D1 / AC-D2 / AC-D3 / AC-D5。
//
// AC-D4(ヘッダの欠落・不正 → gateway で missing_header / invalid_header)は gateway の担当で、
// `services/gateway/internal/httpapi/headers_test.go` がすでに固定している。record-svc 側は
// 「下流は UUID 形式を検証しない」(openapi.yaml の DeviceId の description)に従い、
// ヘッダの**欠落**だけを生成ラッパ経由で 400 missing_header にする。

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/api"
)

const pathFrequent = "/api/record/frequent-opponents"

// AC-D1 / AC-D5: 端末 A のイベントから作った集計は、端末 B の集計に出ない。
// B の集計は B のイベントだけを反映する。
func TestFrequentOpponentsAreScopedToTheDevice(t *testing.T) {
	st := newFakeStore()
	st.seed(deviceA, 0, 3, 0) // A に集計3件
	st.seed(deviceB, 0, 1, 0) // B に集計1件
	h := NewHandler(st)

	gotB := decodeFrequent(t, serve(t, h, http.MethodGet, pathFrequent, headers(deviceB), nil))
	if len(gotB) != 1 {
		t.Fatalf("端末 B の集計 = %d 件, want 1(A の3件が混ざらない)", len(gotB))
	}
	for _, id := range st.deviceIDsTouched() {
		if id != deviceB {
			t.Errorf("store が端末 %q で呼ばれた, want %q だけ(ADR-0209 §6-1・§6-3)", id, deviceB)
		}
	}

	// 記録が1件も無い端末は空配列(404 にしない・他端末の存在を漏らさない。§6-5)。
	st2 := newFakeStore()
	st2.seed(deviceA, 0, 3, 0)
	empty := decodeFrequent(t, serve(t, NewHandler(st2), http.MethodGet, pathFrequent, headers(deviceB), nil))
	if len(empty) != 0 {
		t.Errorf("記録の無い端末の集計 = %d 件, want 0", len(empty))
	}
}

// AC-D3: 端末 ID をクエリ・ボディで受け取らない(ADR-0209 §2・§6-4)。
// 送られたら 400 unknown_field で、ヘッダの端末 ID を上書きできない。
func TestDeviceIDInQueryOrBodyIsRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{"集計のクエリに deviceId", http.MethodGet, pathFrequent + "?deviceId=" + deviceB, nil},
		{"集計のクエリに device_id", http.MethodGet, pathFrequent + "?device_id=" + deviceB, nil},
		{"集計のボディに deviceId", http.MethodGet, pathFrequent, []byte(`{"deviceId":"` + deviceB + `"}`)},
		{"削除のクエリに deviceId", http.MethodDelete, pathDeviceData + "?deviceId=" + deviceB, nil},
		{"削除のボディに deviceId", http.MethodDelete, pathDeviceData, []byte(`{"deviceId":"` + deviceB + `"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			st.seed(deviceB, 2, 2, 2)
			h := NewHandler(st)

			rec := serve(t, h, tt.method, tt.target, headers(deviceA), tt.body)
			assertErrorBody(t, rec, http.StatusBadRequest, api.UnknownField)

			// ヘッダの端末を上書きできていないこと(B のデータに一切触れていない)。
			for _, id := range st.deviceIDsTouched() {
				if id == deviceB {
					t.Errorf("ヘッダ外の deviceId で端末 %q が参照された(ADR-0209 §6-4)", id)
				}
			}
			if left := st.rowsLeft(deviceB); left != 6 {
				t.Errorf("端末 B の残り = %d 行, want 6", left)
			}
		})
	}
}

// AC-D2 の対応: record-svc の契約にはリソース ID をパスに受ける操作がまだ無い
// (お気に入りの CRUD は P5-3 の API 範囲外。集計と全削除だけ)。よって「他端末のリソース ID を
// 指すと 404 not_found」を直接試せる操作が無い。代わりに、**将来そういう操作が増えたときに
// 気づける形**として「record の操作にパスパラメータが無い」ことを契約から固定する。
// 追加するときはこのテストが落ちるので、そのとき AC-D2 の実テストを一緒に足すこと。
func TestRecordOperationsHaveNoPathParameters(t *testing.T) {
	doc, err := api.GetSwagger()
	if err != nil {
		t.Fatalf("契約を読めない: %v", err)
	}
	for path, item := range doc.Paths.Map() {
		if !strings.HasPrefix(path, "/api/record/") {
			continue
		}
		for method, op := range item.Operations() {
			for _, p := range op.Parameters {
				if p.Value != nil && p.Value.In == "path" {
					t.Errorf("%s %s にパスパラメータ %q がある。AC-D2(他端末のリソース ID は 404 not_found)の"+
						"テストを追加してからこのテストを更新すること", method, path, p.Value.Name)
				}
			}
		}
	}
}

// AC-D2 の補強: 担当外の操作(calc / pokedex / internal)は record-svc では 404 not_found。
// ヘッダが無くても 404(担当外の判定がヘッダ検証より先。calc-svc の registerPokedexNotFoundRoutes と同じ)。
func TestOperationsOfOtherServicesAreNotFound(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	tests := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/calc"},
		{http.MethodPost, "/api/calc/bulk"},
		{http.MethodPost, "/api/calc/reverse"},
		{http.MethodGet, "/api/pokedex/species"},
		{http.MethodGet, "/api/pokedex/natures"},
		{http.MethodGet, "/internal/pokedex/master"},
		{http.MethodGet, "/api/record/unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := serve(t, h, tt.method, tt.path, http.Header{}, nil)
			assertErrorBody(t, rec, http.StatusNotFound, api.NotFound)
		})
	}
	if ids := st.deviceIDsTouched(); len(ids) != 0 {
		t.Errorf("担当外の操作で store が呼ばれた(端末 %v)", ids)
	}
}

// ADR-0209 §4: 端末 ID を含む HTTP 要求を受けたら devices.last_seen_at を更新する
// (更新の抑止〈24時間規則。AC-R5〉は store の責務で、ハンドラは毎回呼ぶ)。
func TestRequestsTouchLastSeenAtForTheirDeviceOnly(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	serve(t, h, http.MethodGet, pathFrequent, headers(deviceA), nil)
	serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil)

	st.mu.Lock()
	defer st.mu.Unlock()
	var touched int
	for _, c := range st.calls {
		if c.method == "TouchDevice" {
			touched++
			if c.deviceID != deviceA {
				t.Errorf("TouchDevice が端末 %q で呼ばれた, want %q", c.deviceID, deviceA)
			}
		}
	}
	if touched != 2 {
		t.Errorf("TouchDevice の呼び出し = %d 回, want 2(要求ごとに1回)", touched)
	}
}

// 欠落したヘッダは 400 missing_header(生成ラッパの検証。ADR-0200)。
func TestMissingClientIDHeaders(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	only := func(name, value string) http.Header {
		hd := http.Header{}
		hd.Set(name, value)
		return hd
	}
	tests := []struct {
		name   string
		header http.Header
	}{
		{"両方無い", http.Header{}},
		{"端末 ID が無い", only("X-Session-Id", sessionID)},
		{"セッション ID が無い", only("X-Device-Id", deviceA)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, h, http.MethodGet, pathFrequent, tt.header, nil)
			assertErrorBody(t, rec, http.StatusBadRequest, api.MissingHeader)
		})
	}
}

func decodeFrequent(t *testing.T, rec *httptest.ResponseRecorder) []api.FrequentOpponent {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got []api.FrequentOpponent
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("FrequentOpponent の配列として読めない: %v; body=%s", err, rec.Body.String())
	}
	return got
}
