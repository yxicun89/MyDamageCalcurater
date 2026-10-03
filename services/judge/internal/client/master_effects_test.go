package client

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

// GET {base}/internal/pokedex/master(getMasterExport。ルートの api/openapi.yaml)から、judge が使う
// 特性・持ち物の ID と効果(自由形式の JSON のまま)だけを取り出す(issue 235 第2段・ADR-0714 §1)。
// 種族・技・相性表などは読まない(保持しない)。効果の中身の検証は internal/speedeffects が行う。
// 内部 API は端末 ID・セッション ID を要らない(1 回の取得を全リクエストで共有するため、呼び出し元の ID を送らない)。

// masterExportBody は内部 API の MasterExport を模した架空の本文(実マスタは使わない)。
// judge が読まない欄(species・moves など)も入れ、読まずに無視できることを確かめる。
const masterExportBody = `{
  "schemaVersion": 1,
  "dataVersion": "test-version",
  "types": [{"id":"fire","sortOrder":1,"nameJa":"ほのお"}],
  "typeChart": [],
  "species": [{"key":"9001-000","dexNo":9001,"form":0,"showdownId":"testmon","nameJa":"テスト",
    "type1":"fire","type2":null,"baseStats":{"hp":1,"atk":1,"def":1,"spa":1,"spd":1,"spe":1},
    "isMega":false,"baseSpeciesKey":null,"requiredItemId":null,"abilities":[{"slot":1,"abilityId":"test-rain-ability"}]}],
  "moves": [{"id":"test-move","nameJa":"テスト","type":"fire","category":"physical","power":90,"priority":0,
    "effect":null,"mechanisms":[],"target":null}],
  "items": [
    {"id":"test-heavy-item","nameJa":"テストおもり","effect":{"SpeedMods":[{"Condition":"always","Modifier":2048}]}},
    {"id":"test-plain-item","nameJa":"テストどうぐ","effect":null}
  ],
  "abilities": [
    {"id":"test-rain-ability","nameJa":"テストあめ","effect":{"SpeedMods":[{"Condition":"weather_rain","Modifier":8192}]}},
    {"id":"test-plain-ability","nameJa":"テストふつう","effect":null}
  ],
  "natures": []
}`

func TestMasterEffectsDecodesUpstreamResponse(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath, gotDevice, gotSession string
	server := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotDevice, gotSession = r.Header.Get("X-Device-Id"), r.Header.Get("X-Session-Id")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(masterExportBody))
	})

	got, err := newPokedex(t, server.URL, testTimeout).MasterEffects(t.Context())
	if err != nil {
		t.Fatalf("MasterEffects: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/internal/pokedex/master" {
		t.Errorf("request = %s %s, want GET /internal/pokedex/master", gotMethod, gotPath)
	}
	if gotDevice != "" || gotSession != "" {
		t.Errorf("内部 API に端末 ID・セッション ID を送っている: (%q, %q)", gotDevice, gotSession)
	}

	if len(got.Items) != 2 || got.Items[0].ID != "test-heavy-item" || got.Items[1].ID != "test-plain-item" {
		t.Fatalf("Items = %+v", got.Items)
	}
	if !strings.Contains(string(got.Items[0].Effect), `"SpeedMods"`) {
		t.Errorf("Items[0].Effect = %s, want 効果の JSON をそのまま", got.Items[0].Effect)
	}
	// effect が null の行は Effect を nil(または JSON の null)で返す。どちらも「効果なし」。
	if e := got.Items[1].Effect; e != nil && string(e) != "null" {
		t.Errorf("Items[1].Effect = %s, want nil か null", e)
	}
	if len(got.Abilities) != 2 || got.Abilities[0].ID != "test-rain-ability" || got.Abilities[1].ID != "test-plain-ability" {
		t.Fatalf("Abilities = %+v", got.Abilities)
	}
}

// TestMasterEffectsNormalizesUpstreamStatus: 状態の畳み方は他の呼び出しと同じ(ADR-0700 §3)。
// 呼び出し側(internal/speedeffects)はどれも「取得失敗」として同じに扱う(フェイルソフト)。
func TestMasterEffectsNormalizesUpstreamStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		want   error
	}{
		{http.StatusServiceUnavailable, ErrUpstreamUnavailable},
		{http.StatusInternalServerError, ErrUpstreamUnavailable},
		{http.StatusNotFound, ErrNotFound},
		{http.StatusNoContent, ErrUpstreamInvalidResponse},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			t.Parallel()
			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"code":"master_unavailable","message":"secret upstream detail"}`))
			})
			_, err := newPokedex(t, server.URL, testTimeout).MasterEffects(t.Context())
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			if err != nil && strings.Contains(err.Error(), "secret upstream detail") {
				t.Errorf("上流の本文を漏らしている: %s", err)
			}
		})
	}
}

// TestMasterEffectsRejectsInvalidBody: 形が契約に合わない本文は ErrUpstreamInvalidResponse
// (表を部分的に作らない。文書全体の形の不正は「データなし」に倒す)。個々の effect の中身は検証しない
// (internal/speedeffects がその ID だけを確定できない扱いにする)。
func TestMasterEffectsRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"JSON でない":      `not json`,
		"配列":            `[]`,
		"items が無い":     `{"abilities":[]}`,
		"abilities が無い": `{"items":[]}`,
		"items が配列でない":  `{"items":{},"abilities":[]}`,
		"items が null":  `{"items":null,"abilities":[]}`,
		"要素がオブジェクトでない":  `{"items":[1],"abilities":[]}`,
		"id が無い":        `{"items":[{"nameJa":"x","effect":null}],"abilities":[]}`,
		"id が空":         `{"items":[{"id":"","effect":null}],"abilities":[]}`,
		"id が文字列でない":    `{"items":[{"id":1,"effect":null}],"abilities":[]}`,
		"特性の ID が重複":    `{"items":[],"abilities":[{"id":"a","effect":null},{"id":"a","effect":null}]}`,
		"持ち物の ID が重複":   `{"items":[{"id":"a","effect":null},{"id":"a","effect":null}],"abilities":[]}`,
		"後ろにゴミ":         `{"items":[],"abilities":[]} {}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			})
			if _, err := newPokedex(t, server.URL, testTimeout).MasterEffects(t.Context()); !errors.Is(err, ErrUpstreamInvalidResponse) {
				t.Errorf("err = %v, want ErrUpstreamInvalidResponse", err)
			}
		})
	}
}

// TestMasterEffectsBodyLimit: マスタ一式は 1 件の種族より大きいので、上限は種族・技と別
// (MaxMasterExportBytes。calc-svc の MaxExportBytes と同じ 4 MiB)。1 MiB を超えても上限内なら読み、
// 上限を超えたら ErrUpstreamInvalidResponse(メモリを使い切らない。Pod の memory limit は 64Mi)。
func TestMasterEffectsBodyLimit(t *testing.T) {
	t.Parallel()

	if MaxMasterExportBytes != 4<<20 {
		t.Errorf("MaxMasterExportBytes = %d, want %d(calc-svc の MaxExportBytes と同じ)", MaxMasterExportBytes, 4<<20)
	}

	// species の名前を伸ばして 1 MiB 超・上限未満にする(judge は species を読まない)。
	pad := strings.Repeat("x", 2<<20)
	withinLimit := `{"species":[{"nameJa":"` + pad + `"}],"items":[],"abilities":[{"id":"a","effect":null}]}`
	server := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(withinLimit))
	})
	got, err := newPokedex(t, server.URL, testTimeout).MasterEffects(t.Context())
	if err != nil {
		t.Fatalf("上限内(約 2 MiB)を拒否した: %v", err)
	}
	if len(got.Abilities) != 1 {
		t.Errorf("Abilities = %+v", got.Abilities)
	}

	over := `{"species":[{"nameJa":"` + strings.Repeat("x", MaxMasterExportBytes) + `"}],"items":[],"abilities":[]}`
	overServer := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(over))
	})
	if _, err := newPokedex(t, overServer.URL, testTimeout).MasterEffects(t.Context()); !errors.Is(err, ErrUpstreamInvalidResponse) {
		t.Errorf("上限超え: err = %v, want ErrUpstreamInvalidResponse", err)
	}
}

// TestMasterEffectsOnConnectionErrorAndTimeout: 届かない・答えない上流は ErrUpstreamUnavailable で、
// 文面に URL・アドレスを含まない(ADR-0700 §3)。
func TestMasterEffectsOnConnectionErrorAndTimeout(t *testing.T) {
	t.Parallel()

	_, err := newPokedex(t, deadBaseURL, testTimeout).MasterEffects(t.Context())
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Fatalf("接続できない: err = %v, want ErrUpstreamUnavailable", err)
	}
	assertNoUpstreamAuthority(t, err.Error(), deadBaseURL)

	server := blockingServer(t)
	_, err = newPokedex(t, server.URL, shortTimeout).MasterEffects(t.Context())
	if !errors.Is(err, ErrUpstreamUnavailable) {
		t.Fatalf("タイムアウト: err = %v, want ErrUpstreamUnavailable", err)
	}
	assertNoUpstreamAuthority(t, err.Error(), server.URL)
}
