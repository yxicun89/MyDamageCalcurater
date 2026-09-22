package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/judge/internal/api"
	"example.com/pokecalc/services/judge/internal/client"
)

// outspeedPath は JD1 の判定 endpoint(ADR-0701 §1)。
const outspeedPath = "/api/judge/v1/outspeed-and-ko"

const (
	testDeviceID  = "test-device"
	testSessionID = "test-session"
)

// 架空の種族・性格・技(実マスタは使わない。CLAUDE.md のドメイン規約)。
// どちらの種族も素早さ種族値 100 なので、差は調整(SP・性格・ランク・持ち物)だけで決まる。
const (
	attackerSpeciesKey = "9001-000"
	defenderSpeciesKey = "9002-000"
	testMoveID         = "test-move"
	naturePlusSpeID    = "test-plus-spe"
	natureNeutralID    = "test-neutral"
)

// speciesBody は pokedex-svc の SpeciesDetail を模した架空の本文。
func speciesBody(key string, baseSpeed int) string {
	return `{"key":"` + key + `","dexNo":9001,"form":0,"nameJa":"テストポケモン",
	  "types":["fire"],"baseStats":{"hp":78,"atk":84,"def":78,"spa":109,"spd":85,"spe":` +
		strconv.Itoa(baseSpeed) + `},"abilities":[{"id":"test-ability","nameJa":"テストとくせい"}]}`
}

func mustJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}

// naturesBody は pokedex-svc の GET /api/pokedex/natures を模した架空の本文。
const naturesBody = `[
  {"id":"` + naturePlusSpeID + `","nameJa":"テストようき","plus":"spe","minus":"spa"},
  {"id":"` + natureNeutralID + `","nameJa":"テストまじめ","plus":null,"minus":null}
]`

// calcBody は calc-svc の CalcResult を模した架空の本文。
const calcBody = `{"rolls":[100,101,102,103,104,105,106,107,108,109,110,111,112,113,114,115],
  "minDamage":100,"maxDamage":115,"minPercent":58.1,"maxPercent":66.8,"defenderHP":172,
  "effectiveness":2,"stab":true,"category":"physical",
  "ko":{"hits":2,"guaranteed":true,"chancePercent":0,"displayChancePercent":100}}`

// upstreams は pokedex-svc・calc-svc のスタブ。応答を差し替えられるようにしつつ、
// 「何回・どの順で呼ばれたか」を記録する(ADR-0701 §5 の検査順と「natures は 1 回だけ」の確認用)。
type upstreams struct {
	mu sync.Mutex

	// 差し替え(0 / "" なら既定の 200 と上の本文)。
	naturesStatus int
	naturesBody   string
	speciesStatus int
	speciesBody   string
	calcStatus    int
	calcBody      string
	attackerSpeed int // attacker の種族の素早さ種族値(0 なら 100)
	defenderSpeed int // defender の種族の素早さ種族値(0 なら 100)

	// 記録。
	naturesCalls int
	speciesKeys  []string
	calcBodies   []map[string]any
	deviceIDs    []string
	sessionIDs   []string

	pokedexURL string
	calcURL    string
}

func (u *upstreams) record(r *http.Request) {
	u.deviceIDs = append(u.deviceIDs, r.Header.Get("X-Device-Id"))
	u.sessionIDs = append(u.sessionIDs, r.Header.Get("X-Session-Id"))
}

func (u *upstreams) counts() (natures int, speciesKeys []string, calcCalls int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.naturesCalls, append([]string(nil), u.speciesKeys...), len(u.calcBodies)
}

func (u *upstreams) lastCalcBody(t *testing.T) map[string]any {
	t.Helper()
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.calcBodies) == 0 {
		t.Fatal("calc-svc が呼ばれていない")
	}
	return u.calcBodies[len(u.calcBodies)-1]
}

func writeStub(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func orDefault(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

// newUpstreams は pokedex-svc と calc-svc のスタブを別々に立て、judge の Dependencies を返す。
// テスト終了時にどちらも閉じる。
func newUpstreams(t *testing.T, u *upstreams) Dependencies {
	t.Helper()

	pokedex := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		defer u.mu.Unlock()
		u.record(r)

		switch {
		case r.URL.Path == "/api/pokedex/natures":
			u.naturesCalls++
			body := u.naturesBody
			if body == "" {
				body = naturesBody
			}
			writeStub(w, u.naturesStatus, body)
		case strings.HasPrefix(r.URL.Path, "/api/pokedex/species/"):
			key := strings.TrimPrefix(r.URL.Path, "/api/pokedex/species/")
			u.speciesKeys = append(u.speciesKeys, key)
			if u.speciesBody != "" || u.speciesStatus != 0 {
				writeStub(w, u.speciesStatus, u.speciesBody)
				return
			}
			baseSpeed := orDefault(u.attackerSpeed, 100)
			if key == defenderSpeciesKey {
				baseSpeed = orDefault(u.defenderSpeed, 100)
			}
			writeStub(w, http.StatusOK, speciesBody(key, baseSpeed))
		default:
			writeStub(w, http.StatusNotFound, `{"code":"not_found","message":"no route"}`)
		}
	}))
	t.Cleanup(pokedex.Close)

	calcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		defer u.mu.Unlock()
		u.record(r)

		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		u.calcBodies = append(u.calcBodies, body)

		stub := u.calcBody
		if stub == "" {
			stub = calcBody
		}
		writeStub(w, u.calcStatus, stub)
	}))
	t.Cleanup(calcServer.Close)

	u.pokedexURL, u.calcURL = pokedex.URL, calcServer.URL

	pokedexClient, err := client.NewPokedex(client.Config{BaseURL: pokedex.URL, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewPokedex: %v", err)
	}
	calcClient, err := client.NewCalc(client.Config{BaseURL: calcServer.URL, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewCalc: %v", err)
	}
	return Dependencies{Pokedex: pokedexClient, Calc: calcClient}
}

// sp は StatBlock の 6 欄をすべて持つ(judge の契約では必須)。
func sp(spe int) map[string]any {
	return map[string]any{"hp": 0, "atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": spe}
}

// validBody は 200 になる request body。attacker は最速(SP32・上昇補正)= 167、
// defender は無振り無補正 = 120 で、attacker が抜ける。
func validBody() map[string]any {
	return map[string]any{
		"format": "single",
		"attacker": map[string]any{
			"speciesKey": attackerSpeciesKey,
			"natureId":   naturePlusSpeID,
			"sp":         sp(32),
		},
		"defender": map[string]any{
			"speciesKey": defenderSpeciesKey,
			"natureId":   natureNeutralID,
			"sp":         sp(0),
		},
		"moveId": testMoveID,
	}
}

// postOutspeed は判定 endpoint を叩く。headers が nil なら端末 ID・セッション ID を付ける。
func postOutspeed(deps Dependencies, body any, headers map[string]string) *httptest.ResponseRecorder {
	var raw []byte
	if s, ok := body.(string); ok {
		raw = []byte(s)
	} else {
		raw = mustJSON(body)
	}
	request := httptest.NewRequest(http.MethodPost, outspeedPath, bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	if headers == nil {
		headers = map[string]string{"X-Device-Id": testDeviceID, "X-Session-Id": testSessionID}
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return serve(deps, request)
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) api.OutspeedAndKoResponse {
	t.Helper()
	var response api.OutspeedAndKoResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("response をデコードできない: %v; body=%s", err, recorder.Body.String())
	}
	return response
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) api.Error {
	t.Helper()
	var apiError api.Error
	if err := json.Unmarshal(recorder.Body.Bytes(), &apiError); err != nil {
		t.Fatalf("Error をデコードできない: %v; body=%s", err, recorder.Body.String())
	}
	return apiError
}

func assertStatusAndCode(t *testing.T, recorder *httptest.ResponseRecorder, status int, code api.ErrorCode) {
	t.Helper()
	if recorder.Code != status {
		t.Errorf("status = %d, want %d; body=%s", recorder.Code, status, recorder.Body.String())
	}
	if got := decodeError(t, recorder).Code; got != code {
		t.Errorf("code = %q, want %q; body=%s", got, code, recorder.Body.String())
	}
}

// assertNoUpstreamDetail は応答の本文に上流の URL・host:port・ホスト名が現れないことを確かめる
// (ADR-0700 §3・ADR-0701 §6。internal/client の assertNoUpstreamAuthority と同じ検査を
// HTTP 応答側で行う)。
func assertNoUpstreamDetail(t *testing.T, body string, rawURLs ...string) {
	t.Helper()
	for _, rawURL := range rawURLs {
		if rawURL == "" {
			continue
		}
		if strings.Contains(body, rawURL) {
			t.Errorf("応答が上流の URL(%s)を漏らしている: %s", rawURL, body)
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			t.Fatalf("テストの前提が壊れている: url.Parse(%q): %v", rawURL, err)
		}
		if u.Host != "" && strings.Contains(body, u.Host) {
			t.Errorf("応答が上流のアドレス(%s)を漏らしている: %s", u.Host, body)
		}
		if hostname := u.Hostname(); hostname != "" && strings.Contains(body, hostname) {
			t.Errorf("応答が上流のホスト名(%s)を漏らしている: %s", hostname, body)
		}
	}
}

// TestOutspeedAndKo: 正常系(ADR-0701 受け入れ条件1・2・3)。
// attacker = (100+20+32)×1.1 = 167、defender = 100+20+0 = 120 で抜ける。
// natures は 1 リクエストにつき 1 回だけ引き、両方の natureId をそれで解決する。
func TestOutspeedAndKo(t *testing.T) {
	t.Parallel()

	stub := &upstreams{}
	recorder := postOutspeed(newUpstreams(t, stub), validBody(), nil)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	got := decodeResponse(t, recorder)
	want := api.OutspeedAndKoResponse{
		Outspeeds:     true,
		SpeedTie:      false,
		AttackerSpeed: 167,
		DefenderSpeed: 120,
		Ko:            api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100},
	}
	if got != want {
		t.Errorf("response = %+v, want %+v", got, want)
	}

	natures, speciesKeys, calcCalls := stub.counts()
	if natures != 1 {
		t.Errorf("GET /api/pokedex/natures を %d 回呼んでいる。1 リクエストにつき 1 回だけにする(ADR-0701 §4)", natures)
	}
	if len(speciesKeys) != 2 {
		t.Errorf("species を %d 回呼んでいる(%v)。attacker と defender の 2 回にする", len(speciesKeys), speciesKeys)
	}
	for _, key := range []string{attackerSpeciesKey, defenderSpeciesKey} {
		if !contains(speciesKeys, key) {
			t.Errorf("species に %q を引いていない(引いたのは %v)", key, speciesKeys)
		}
	}
	if calcCalls != 1 {
		t.Errorf("calc-svc を %d 回呼んでいる。1 回にする", calcCalls)
	}

	// 端末 ID・セッション ID は呼び出し元のものをそのまま全ての上流へ転送する(ADR-0700 §2)。
	stub.mu.Lock()
	devices, sessions := stub.deviceIDs, stub.sessionIDs
	stub.mu.Unlock()
	if len(devices) != 4 {
		t.Errorf("上流を %d 回呼んでいる。natures 1 + species 2 + calc 1 = 4 回にする", len(devices))
	}
	for i := range devices {
		if devices[i] != testDeviceID || sessions[i] != testSessionID {
			t.Errorf("上流 %d 回目のヘッダー = (%q, %q), want (%q, %q)", i, devices[i], sessions[i], testDeviceID, testSessionID)
		}
	}

	// calc-svc へは judge の request をそのまま組み替えて渡す。
	body := stub.lastCalcBody(t)
	if body["format"] != "single" {
		t.Errorf("calc の format = %v, want single", body["format"])
	}
	if body["moveId"] != testMoveID {
		t.Errorf("calc の moveId = %v, want %s", body["moveId"], testMoveID)
	}
	attacker, ok := body["attacker"].(map[string]any)
	if !ok {
		t.Fatalf("calc の attacker = %v, want an object", body["attacker"])
	}
	if attacker["speciesKey"] != attackerSpeciesKey || attacker["natureId"] != naturePlusSpeID {
		t.Errorf("calc の attacker = %v, want speciesKey=%s natureId=%s", attacker, attackerSpeciesKey, naturePlusSpeID)
	}
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// TestOutspeedAndKoSpeedTie: 同速は outspeeds=false・speedTie=true(ADR-0700 §6-1)。
// 真偽値 1 つに丸めないので、「抜けられている」(どちらも false)と区別できる。
func TestOutspeedAndKoSpeedTie(t *testing.T) {
	t.Parallel()

	// 双方とも無振り無補正(120)にすると同速になる。
	body := validBody()
	body["attacker"].(map[string]any)["natureId"] = natureNeutralID
	body["attacker"].(map[string]any)["sp"] = sp(0)

	recorder := postOutspeed(newUpstreams(t, &upstreams{}), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	got := decodeResponse(t, recorder)
	if got.Outspeeds || !got.SpeedTie {
		t.Errorf("outspeeds/speedTie = %v/%v, want false/true", got.Outspeeds, got.SpeedTie)
	}
	if got.AttackerSpeed != 120 || got.DefenderSpeed != 120 {
		t.Errorf("speeds = %d/%d, want 120/120", got.AttackerSpeed, got.DefenderSpeed)
	}
}

// TestOutspeedAndKoSlower: 遅いときは outspeeds も speedTie も false。
func TestOutspeedAndKoSlower(t *testing.T) {
	t.Parallel()

	body := validBody()
	body["attacker"].(map[string]any)["natureId"] = natureNeutralID
	body["attacker"].(map[string]any)["sp"] = sp(0)
	body["defender"].(map[string]any)["natureId"] = naturePlusSpeID
	body["defender"].(map[string]any)["sp"] = sp(32)

	recorder := postOutspeed(newUpstreams(t, &upstreams{}), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	got := decodeResponse(t, recorder)
	if got.Outspeeds || got.SpeedTie {
		t.Errorf("outspeeds/speedTie = %v/%v, want false/false", got.Outspeeds, got.SpeedTie)
	}
	if got.AttackerSpeed != 120 || got.DefenderSpeed != 167 {
		t.Errorf("speeds = %d/%d, want 120/167", got.AttackerSpeed, got.DefenderSpeed)
	}
}

// TestOutspeedAndKoRanks: ranks は「技の追加効果を適用した後のランク」として受け取り、
// そのまま素早さに乗せる(ADR-0700 §6-5)。120 × 3/2 = 180 で 167 を抜く。
func TestOutspeedAndKoRanks(t *testing.T) {
	t.Parallel()

	body := validBody()
	attacker := body["attacker"].(map[string]any)
	attacker["natureId"] = natureNeutralID
	attacker["sp"] = sp(0)
	attacker["ranks"] = map[string]any{"spe": 1}
	body["defender"].(map[string]any)["natureId"] = naturePlusSpeID
	body["defender"].(map[string]any)["sp"] = sp(32)

	recorder := postOutspeed(newUpstreams(t, &upstreams{}), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	got := decodeResponse(t, recorder)
	if got.AttackerSpeed != 180 || !got.Outspeeds {
		t.Errorf("attackerSpeed/outspeeds = %d/%v, want 180/true", got.AttackerSpeed, got.Outspeeds)
	}
}

// TestOutspeedAndKoChoiceScarf: itemId が設定のこだわりスカーフ ID と一致するときだけ ×1.5
// (ADR-0701 §3)。既定は choicescarf で、Dependencies で上書きできる。
// 120 × 1.5 = 180 で 167 を抜く。
func TestOutspeedAndKoChoiceScarf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		scarfItemID   string // Dependencies の上書き(空なら既定)
		itemID        string
		wantSpeed     int
		wantOutspeeds bool
	}{
		{"既定の ID", "", "choicescarf", 180, true},
		{"別の持ち物は効かない", "", "test-other-item", 120, false},
		{"持ち物なし", "", "", 120, false},
		{"設定で上書きした ID", "test-scarf", "test-scarf", 180, true},
		{"上書きすると既定は効かない", "test-scarf", "choicescarf", 120, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := validBody()
			attacker := body["attacker"].(map[string]any)
			attacker["natureId"] = natureNeutralID
			attacker["sp"] = sp(0)
			if tt.itemID != "" {
				attacker["itemId"] = tt.itemID
			}
			body["defender"].(map[string]any)["natureId"] = naturePlusSpeID
			body["defender"].(map[string]any)["sp"] = sp(32)

			stub := &upstreams{}
			deps := newUpstreams(t, stub)
			deps.ChoiceScarfItemID = tt.scarfItemID

			recorder := postOutspeed(deps, body, nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			got := decodeResponse(t, recorder)
			if got.AttackerSpeed != tt.wantSpeed || got.Outspeeds != tt.wantOutspeeds {
				t.Errorf("attackerSpeed/outspeeds = %d/%v, want %d/%v",
					got.AttackerSpeed, got.Outspeeds, tt.wantSpeed, tt.wantOutspeeds)
			}
			// スカーフの持ち物 ID は calc-svc にもそのまま渡す(ダメージ側の効果は calc-svc が持つ)。
			if tt.itemID != "" {
				attackerSent := stub.lastCalcBody(t)["attacker"].(map[string]any)
				if attackerSent["itemId"] != tt.itemID {
					t.Errorf("calc の attacker.itemId = %v, want %q", attackerSent["itemId"], tt.itemID)
				}
			}
		})
	}
}

// TestOutspeedAndKoTranscribesKO: ko は calc-svc の値をそのまま転記する(judge は確定数を
// 再計算しない。ADR-0701 受け入れ条件1)。画面に出さない chancePercent は返さない(ADR-0010)。
func TestOutspeedAndKoTranscribesKO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ko   string
		want api.KOChance
	}{
		{"確定2発", `{"hits":2,"guaranteed":true,"chancePercent":0,"displayChancePercent":100}`,
			api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100}},
		{"乱数1発", `{"hits":1,"guaranteed":false,"chancePercent":87.4321,"displayChancePercent":87.4}`,
			api.KOChance{Hits: 1, Guaranteed: false, DisplayChancePercent: 87.4}},
		{"倒せない", `{"hits":0,"guaranteed":false,"chancePercent":0,"displayChancePercent":0}`,
			api.KOChance{Hits: 0, Guaranteed: false, DisplayChancePercent: 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &upstreams{calcBody: `{"minDamage":1,"maxDamage":2,"defenderHP":172,"ko":` + tt.ko + `}`}
			recorder := postOutspeed(newUpstreams(t, stub), validBody(), nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := decodeResponse(t, recorder).Ko; got != tt.want {
				t.Errorf("ko = %+v, want %+v", got, tt.want)
			}

			var raw map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
				t.Fatalf("body をデコードできない: %v", err)
			}
			ko, ok := raw["ko"].(map[string]any)
			if !ok {
				t.Fatalf("ko = %v, want an object", raw["ko"])
			}
			if _, present := ko["chancePercent"]; present {
				t.Error("ko に chancePercent を返している。画面に出す値ではない(ADR-0010)")
			}
		})
	}
}

// TestOutspeedAndKoForwardsField: field は解釈せず calc-svc にそのまま転送し、
// 省略時は送らない(ADR-0701 受け入れ条件6)。
func TestOutspeedAndKoForwardsField(t *testing.T) {
	t.Parallel()

	t.Run("指定したら転送する", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		body["field"] = map[string]any{
			"weather":         "sun",
			"terrain":         "electric",
			"defenderScreens": map[string]any{"reflect": true},
		}

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}

		field, ok := stub.lastCalcBody(t)["field"].(map[string]any)
		if !ok {
			t.Fatalf("calc の field = %v, want an object", stub.lastCalcBody(t)["field"])
		}
		if field["weather"] != "sun" || field["terrain"] != "electric" {
			t.Errorf("calc の field = %v, want weather=sun terrain=electric", field)
		}
		screens, ok := field["defenderScreens"].(map[string]any)
		if !ok || screens["reflect"] != true {
			t.Errorf("calc の field.defenderScreens = %v, want reflect=true", field["defenderScreens"])
		}
	})

	t.Run("省略したら送らない", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), validBody(), nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		if value, present := stub.lastCalcBody(t)["field"]; present {
			t.Errorf("calc に field = %v を送っている。未指定なら送らない", value)
		}
	})
}

// TestOutspeedAndKoRejectsInvalidRequest: ヘッダー・body の検査は上流より先(ADR-0701 §5)。
// 400 invalid_request を返し、**上流を 1 回も呼ばない**(無駄な往復をしない。ADR-0700 §3 と同じ立場)。
func TestOutspeedAndKoRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	withAttacker := func(mutate func(map[string]any)) map[string]any {
		body := validBody()
		mutate(body["attacker"].(map[string]any))
		return body
	}

	tests := []struct {
		name    string
		body    any
		headers map[string]string
	}{
		{"ヘッダーが両方無い", validBody(), map[string]string{}},
		{"X-Device-Id が無い", validBody(), map[string]string{"X-Session-Id": testSessionID}},
		{"X-Session-Id が無い", validBody(), map[string]string{"X-Device-Id": testDeviceID}},
		{"X-Device-Id が空", validBody(), map[string]string{"X-Device-Id": "", "X-Session-Id": testSessionID}},
		{"body が JSON でない", "not json", nil},
		{"body が JSON 値を 2 つ含む", `{"format":"single"} {"format":"single"}`, nil},
		{"body が配列", `[]`, nil},
		{"未知の欄がある", func() any {
			body := validBody()
			body["criticalHit"] = true
			return body
		}(), nil},
		{"attacker が無い", func() any {
			body := validBody()
			delete(body, "attacker")
			return body
		}(), nil},
		{"moveId が無い", func() any {
			body := validBody()
			delete(body, "moveId")
			return body
		}(), nil},
		{"moveId が空", func() any {
			body := validBody()
			body["moveId"] = ""
			return body
		}(), nil},
		{"format が enum に無い", func() any {
			body := validBody()
			body["format"] = "triple"
			return body
		}(), nil},
		{"speciesKey が形式に合わない", withAttacker(func(a map[string]any) { a["speciesKey"] = "pikachu" }), nil},
		{"natureId が空", withAttacker(func(a map[string]any) { a["natureId"] = "" }), nil},
		{"sp が無い", withAttacker(func(a map[string]any) { delete(a, "sp") }), nil},
		{"sp の欄が足りない", withAttacker(func(a map[string]any) { a["sp"] = map[string]any{"spe": 32} }), nil},
		{"sp が負", withAttacker(func(a map[string]any) { a["sp"] = sp(-1) }), nil},
		{"sp が 32 超", withAttacker(func(a map[string]any) { a["sp"] = sp(33) }), nil},
		{"sp の合計が 66 超", withAttacker(func(a map[string]any) {
			a["sp"] = map[string]any{"hp": 32, "atk": 32, "def": 32, "spa": 0, "spd": 0, "spe": 0}
		}), nil},
		{"ranks が -6 未満", withAttacker(func(a map[string]any) { a["ranks"] = map[string]any{"spe": -7} }), nil},
		{"ranks が +6 超", withAttacker(func(a map[string]any) { a["ranks"] = map[string]any{"spe": 7} }), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &upstreams{}
			recorder := postOutspeed(newUpstreams(t, stub), tt.body, tt.headers)

			assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)

			natures, speciesKeys, calcCalls := stub.counts()
			if natures+len(speciesKeys)+calcCalls != 0 {
				t.Errorf("上流を呼んでいる(natures=%d species=%v calc=%d)。request の検査は上流より先(ADR-0701 §5)",
					natures, speciesKeys, calcCalls)
			}
		})
	}
}

// TestOutspeedAndKoRejectsOversizedBody: body には上限(8 KiB)があり、超えたら 413
// request_too_large(ADR-0701 §6。speed の PositionRequest と同じ形)。
func TestOutspeedAndKoRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	body := validBody()
	body["moveId"] = strings.Repeat("x", 9*1024)

	stub := &upstreams{}
	recorder := postOutspeed(newUpstreams(t, stub), body, nil)

	assertStatusAndCode(t, recorder, http.StatusRequestEntityTooLarge, api.RequestTooLarge)

	natures, speciesKeys, calcCalls := stub.counts()
	if natures+len(speciesKeys)+calcCalls != 0 {
		t.Error("上限を超えた body で上流を呼んでいる")
	}
}

// TestOutspeedAndKoUpstreamNotConfigured: 上流の base URL が未設定(client が nil)なら
// 503 upstream_unavailable(ADR-0700 §5・ADR-0701 §6)。ヘルスは 200 のままであることは
// TestHealth が別に確かめている。
func TestOutspeedAndKoUpstreamNotConfigured(t *testing.T) {
	t.Parallel()

	tests := map[string]func(Dependencies) Dependencies{
		"どちらも未設定":      func(Dependencies) Dependencies { return Dependencies{} },
		"pokedex が未設定": func(deps Dependencies) Dependencies { deps.Pokedex = nil; return deps },
		"calc が未設定":    func(deps Dependencies) Dependencies { deps.Calc = nil; return deps },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			deps := mutate(newUpstreams(t, &upstreams{}))
			recorder := postOutspeed(deps, validBody(), nil)
			assertStatusAndCode(t, recorder, http.StatusServiceUnavailable, api.UpstreamUnavailable)
		})
	}
}

// TestOutspeedAndKoUnknownNature: natureId が性格の一覧に無ければ 422 unknown_nature
// (ADR-0701 §6)。黙って無補正に倒さない。
func TestOutspeedAndKoUnknownNature(t *testing.T) {
	t.Parallel()

	for _, side := range []string{"attacker", "defender"} {
		t.Run(side, func(t *testing.T) {
			t.Parallel()

			body := validBody()
			body[side].(map[string]any)["natureId"] = "test-missing-nature"

			stub := &upstreams{}
			recorder := postOutspeed(newUpstreams(t, stub), body, nil)
			assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownNature)

			if _, _, calcCalls := stub.counts(); calcCalls != 0 {
				t.Error("性格を解決できていないのに calc-svc を呼んでいる")
			}
		})
	}
}

// TestOutspeedAndKoUnknownSpecies: pokedex-svc が 404 を返す speciesKey は 422 unknown_species
// (ADR-0700 §3 の ErrNotFound → ADR-0701 §6)。形は正しいがマスタに無い、という
// balance/speed の unknown_pokemon と同じ区別。
func TestOutspeedAndKoUnknownSpecies(t *testing.T) {
	t.Parallel()

	stub := &upstreams{
		speciesStatus: http.StatusNotFound,
		speciesBody:   `{"code":"not_found","message":"no such species"}`,
	}
	recorder := postOutspeed(newUpstreams(t, stub), validBody(), nil)

	assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownSpecies)
	if _, _, calcCalls := stub.counts(); calcCalls != 0 {
		t.Error("種族を引けていないのに calc-svc を呼んでいる(ADR-0701 §5 の検査順)")
	}
}

// TestOutspeedAndKoUpstreamFailures: 上流の失敗を ADR-0701 §6 の対応表どおりに畳む。
func TestOutspeedAndKoUpstreamFailures(t *testing.T) {
	t.Parallel()

	// stub は呼ばれるたびに新しいスタブを作る(upstreams は mutex を持つので値でコピーしない)。
	tests := []struct {
		name       string
		stub       func() *upstreams
		wantStatus int
		wantCode   api.ErrorCode
	}{
		{
			"性格の一覧が 503",
			func() *upstreams {
				return &upstreams{naturesStatus: http.StatusServiceUnavailable, naturesBody: `{"code":"master_unavailable","message":"no natures"}`}
			},
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			"性格の一覧が壊れた JSON",
			func() *upstreams { return &upstreams{naturesBody: `[{"id":`} },
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			"性格の一覧が空",
			func() *upstreams { return &upstreams{naturesBody: `[]`} },
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			"species が 500",
			func() *upstreams {
				return &upstreams{speciesStatus: http.StatusInternalServerError, speciesBody: `{"code":"internal","message":"boom"}`}
			},
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			"species の baseStats が欠けている",
			func() *upstreams {
				return &upstreams{speciesBody: `{"key":"9001-000","nameJa":"テスト","types":["fire"]}`}
			},
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			"calc が 500",
			func() *upstreams {
				return &upstreams{calcStatus: http.StatusInternalServerError, calcBody: `{"code":"internal","message":"boom"}`}
			},
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			"calc が 503",
			func() *upstreams {
				return &upstreams{calcStatus: http.StatusServiceUnavailable, calcBody: `{"code":"master_unavailable","message":"no master"}`}
			},
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			"calc の ko が欠けている",
			func() *upstreams { return &upstreams{calcBody: `{"minDamage":1,"maxDamage":2,"defenderHP":172}`} },
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			// calc-svc の 400(未知の技・持ち物・特性など)は judge からは見分けられないので
			// invalid_request に畳む(ADR-0701 §6)。
			"calc が 400",
			func() *upstreams {
				return &upstreams{calcStatus: http.StatusBadRequest, calcBody: `{"code":"unknown_move","message":"no such move"}`}
			},
			http.StatusBadRequest, api.InvalidRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := tt.stub()
			deps := newUpstreams(t, stub)
			recorder := postOutspeed(deps, validBody(), nil)

			assertStatusAndCode(t, recorder, tt.wantStatus, tt.wantCode)
			assertNoUpstreamDetail(t, recorder.Body.String(), stub.pokedexURL, stub.calcURL)
		})
	}
}

// TestOutspeedAndKoDoesNotLeakUpstreamDetail: 上流の本文・URL・アドレスを応答に出さない
// (ADR-0700 §3。原因はログに残す)。
func TestOutspeedAndKoDoesNotLeakUpstreamDetail(t *testing.T) {
	t.Parallel()

	const upstreamDetail = "dsn dbhost01 svcaccount internal-only-detail"
	stub := &upstreams{
		naturesStatus: http.StatusInternalServerError,
		naturesBody:   `{"code":"internal","message":"` + upstreamDetail + `"}`,
	}
	recorder := postOutspeed(newUpstreams(t, stub), validBody(), nil)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, upstreamDetail) {
		t.Errorf("応答が上流の本文を漏らしている: %s", body)
	}
	assertNoUpstreamDetail(t, body, stub.pokedexURL, stub.calcURL)
}

// TestOutspeedAndKoCheckOrder: 検査順は固定(ADR-0701 §5)。上流を逐次で呼ぶので、
// 複数の原因が同時にあってもどれが返るかが決まる(並列化すると実行ごとに変わる)。
func TestOutspeedAndKoCheckOrder(t *testing.T) {
	t.Parallel()

	t.Run("range 検査は性格の一覧より先", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		body["attacker"].(map[string]any)["sp"] = sp(33)
		body["attacker"].(map[string]any)["natureId"] = "test-missing-nature"

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
		if natures, _, _ := stub.counts(); natures != 0 {
			t.Error("範囲外の request で性格の一覧を引いている")
		}
	})

	t.Run("性格の一覧は種族より先", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		body["attacker"].(map[string]any)["natureId"] = "test-missing-nature"

		stub := &upstreams{
			speciesStatus: http.StatusNotFound,
			speciesBody:   `{"code":"not_found","message":"no such species"}`,
		}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		// 性格も種族も解決できないが、検査順どおり unknown_nature が返る。
		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownNature)
	})
}
