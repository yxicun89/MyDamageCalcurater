package httpapi

// 端末 ID によるデータ分離(ADR-0209 §6)の受け入れテスト。AC-D1 / AC-D2 / AC-D3。
//
// AC-D4(ヘッダの欠落・不正 → gateway で missing_header / invalid_header)は gateway の担当で、
// `services/gateway/internal/httpapi/headers_test.go` がすでに固定している。team-svc 側は
// 「下流は UUID 形式を検証しない」(openapi.yaml の DeviceId の description)に従い、
// ヘッダの**欠落**だけを生成ラッパ経由で 400 missing_header にする。
//
// AC-D2 は record-svc には試せる操作が無かった(集計と全削除だけ)。team-svc は
// `/api/team/teams/{teamId}` を持つので、ここが AC-D2 の実テストになる(ADR-0209 §6-2 の追記)。

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/api"
)

// AC-D1: 端末 A の構築は端末 B の一覧に出ない。B の一覧は B のものだけを反映する。
func TestListTeamsIsScopedToTheDevice(t *testing.T) {
	st := newFakeStore()
	st.seed(deviceA, 3, 2)
	st.seed(deviceB, 1, 1)
	h := NewHandler(st)

	gotB := decodeTeams(t, serve(t, h, http.MethodGet, pathTeams, headers(deviceB), nil))
	if len(gotB) != 1 {
		t.Fatalf("端末 B の一覧 = %d 件, want 1(A の3件が混ざらない)", len(gotB))
	}
	for _, id := range st.deviceIDsTouched() {
		if id != deviceB {
			t.Errorf("store が端末 %q で呼ばれた, want %q だけ(ADR-0209 §6-1・§6-3)", id, deviceB)
		}
	}
}

// AC-D2: 端末 B が端末 A の構築 ID を指して GET / PUT / DELETE すると 404 not_found(403 にしない)。
// 応答本文に A の情報(構築名・存在の有無)を含めない。A のデータは変わらない。
func TestOtherDevicesTeamIsNotFound(t *testing.T) {
	tests := []struct {
		method string
		hasBdy bool
	}{
		{http.MethodGet, false},
		{http.MethodPut, true},
		{http.MethodDelete, false},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			st := newFakeStore()
			ids := st.seed(deviceA, 1, 3)
			before := st.rowsLeft(deviceA)
			h := NewHandler(st)

			var b []byte
			if tt.hasBdy {
				b = body(t, teamJSON("乗っ取り"))
			}
			rec := serve(t, h, tt.method, teamPath(ids[0]), headers(deviceB), b)
			assertErrorBody(t, rec, http.StatusNotFound, api.NotFound)

			if rec.Code == http.StatusForbidden {
				t.Error("403 を返している(所有権の概念を作らない。ADR-0209 §6-2)")
			}
			if s := rec.Body.String(); strings.Contains(s, "構築1") || strings.Contains(s, ids[0]) {
				t.Errorf("応答に他端末の情報が漏れている: %s", s)
			}
			if after := st.rowsLeft(deviceA); after != before {
				t.Errorf("端末 A の行が %d → %d に変わった(他端末の要求で変えない)", before, after)
			}
			for _, id := range st.deviceIDsTouched() {
				if id != deviceB {
					t.Errorf("store が端末 %q で呼ばれた, want %q だけ", id, deviceB)
				}
			}
		})
	}
}

// AC-D3: 端末 ID をクエリ・ボディで受け取らない(ADR-0209 §2・§6-4)。
// 送られたら 400 unknown_field で、ヘッダの端末 ID を上書きできない。
func TestDeviceIDInQueryOrBodyIsRejected(t *testing.T) {
	withDeviceID := teamJSON("乗っ取り", memberJSON())
	withDeviceID["deviceId"] = deviceB

	tests := []struct {
		name   string
		method string
		target string
		reqBdy []byte
	}{
		{"一覧のクエリに deviceId", http.MethodGet, pathTeams + "?deviceId=" + deviceB, nil},
		{"一覧のクエリに device_id", http.MethodGet, pathTeams + "?device_id=" + deviceB, nil},
		{"作成のボディに deviceId", http.MethodPost, pathTeams, mustJSON(withDeviceID)},
		{"作成のクエリに deviceId", http.MethodPost, pathTeams + "?deviceId=" + deviceB, mustJSON(teamJSON("x"))},
		{"更新のボディに deviceId", http.MethodPut, pathTeams + "/" + fakeTeamID(1), mustJSON(withDeviceID)},
		{"削除のクエリに deviceId", http.MethodDelete, pathTeams + "/" + fakeTeamID(1) + "?deviceId=" + deviceB, nil},
		{"全削除のクエリに deviceId", http.MethodDelete, pathDeviceData + "?deviceId=" + deviceB, nil},
		{"全削除のボディに deviceId", http.MethodDelete, pathDeviceData, []byte(`{"deviceId":"` + deviceB + `"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			st.seed(deviceB, 2, 2)
			before := st.rowsLeft(deviceB)
			h := NewHandler(st)

			rec := serve(t, h, tt.method, tt.target, headers(deviceA), tt.reqBdy)
			assertErrorBody(t, rec, http.StatusBadRequest, api.UnknownField)

			for _, id := range st.deviceIDsTouched() {
				if id == deviceB {
					t.Errorf("ヘッダ外の deviceId で端末 %q が参照された(ADR-0209 §6-4)", id)
				}
			}
			if after := st.rowsLeft(deviceB); after != before {
				t.Errorf("端末 B の行が %d → %d に変わった", before, after)
			}
		})
	}
}

// AC-D2 の補強: 担当外の操作(calc / pokedex / record / internal)は team-svc では 404 not_found。
// ヘッダが無くても 404(担当外の判定がヘッダ検証より先)。
func TestOperationsOfOtherServicesAreNotFound(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	tests := []struct{ method, path string }{
		{http.MethodPost, "/api/calc"},
		{http.MethodPost, "/api/calc/bulk"},
		{http.MethodPost, "/api/calc/reverse"},
		{http.MethodGet, "/api/pokedex/species"},
		{http.MethodGet, "/api/pokedex/natures"},
		{http.MethodGet, "/api/record/frequent-opponents"},
		{http.MethodDelete, "/api/record/device-data"},
		{http.MethodGet, "/internal/pokedex/master"},
		{http.MethodGet, "/api/team/unknown"},
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
	ids := st.seed(deviceA, 1, 1)
	h := NewHandler(st)

	serve(t, h, http.MethodGet, pathTeams, headers(deviceA), nil)
	serve(t, h, http.MethodGet, teamPath(ids[0]), headers(deviceA), nil)
	serve(t, h, http.MethodPost, pathTeams, headers(deviceA), body(t, teamJSON("新規")))
	serve(t, h, http.MethodPut, teamPath(ids[0]), headers(deviceA), body(t, teamJSON("置換")))
	serve(t, h, http.MethodDelete, teamPath(ids[0]), headers(deviceA), nil)
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
	if touched != 6 {
		t.Errorf("TouchDevice の呼び出し = %d 回, want 6(要求ごとに1回)", touched)
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
			rec := serve(t, h, http.MethodGet, pathTeams, tt.header, nil)
			assertErrorBody(t, rec, http.StatusBadRequest, api.MissingHeader)
		})
	}
}

// mustJSON はテーブル定義の中で使う JSON 化(失敗は panic。テストデータの組み立てミス)。
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
