package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/judge/internal/api"
	"example.com/pokecalc/services/judge/internal/client"
)

// outspeedPath は判定 endpoint(ADR-0701 §1。JD3 で request/response の形が
// defenders / matchups に変わったが、path は変わらない)。
const outspeedPath = "/api/judge/v1/outspeed-and-ko"

const (
	testDeviceID  = "test-device"
	testSessionID = "test-session"
)

// 架空の種族・性格・技(実マスタは使わない。CLAUDE.md のドメイン規約)。
// 既定ではどの種族も素早さ種族値 100 で、差は調整(SP・性格・ランク・持ち物)だけで決まる。
// 候補ごとに種族値を変えたいテストは upstreams.baseSpeeds で指定する。
const (
	attackerSpeciesKey  = "9001-000"
	defenderSpeciesKey  = "9002-000"
	defender2SpeciesKey = "9003-000"
	defender3SpeciesKey = "9004-000"
	testMoveID          = "test-move"
	naturePlusSpeID     = "test-plus-spe"
	natureNeutralID     = "test-neutral"
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

// stubResponse は 1 つの上流呼び出しだけを差し替えるための応答(JD3: 候補ごとに
// 成否を変え、「最初に失敗した候補で打ち切る」ことを確かめるのに使う)。
type stubResponse struct {
	status int
	body   string
}

// upstreams は pokedex-svc・calc-svc のスタブ。応答を差し替えられるようにしつつ、
// 「何回・どの順で呼ばれたか」を記録する(ADR-0701 §5 の検査順、「natures は 1 回だけ」、
// ADR-0703 §4 の「index 昇順・最初の失敗で打ち切り」の確認用)。
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
	defenderSpeed int // defenderSpeciesKey の素早さ種族値(0 なら 100)

	// JD3: 候補ごとの差し替え(ADR-0703)。キーは speciesKey で、
	// calcFail / calcKO は「その計算要求の defender.speciesKey」で引く。
	baseSpeeds  map[string]int          // speciesKey → 素早さ種族値
	speciesFail map[string]stubResponse // speciesKey → その種族の取得だけを失敗させる
	calcFail    map[string]stubResponse // defender の speciesKey → その計算だけを失敗させる
	calcKO      map[string]string       // defender の speciesKey → 返す ko の JSON

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

// calcDefenderKeys は calc-svc に送られた defender.speciesKey を呼ばれた順に返す
// (ADR-0703 §4: 候補は index 昇順に計算する)。
func (u *upstreams) calcDefenderKeys() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	keys := make([]string, 0, len(u.calcBodies))
	for _, body := range u.calcBodies {
		keys = append(keys, calcDefenderSpeciesKey(body))
	}
	return keys
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

func calcDefenderSpeciesKey(body map[string]any) string {
	defender, ok := body["defender"].(map[string]any)
	if !ok {
		return ""
	}
	key, _ := defender["speciesKey"].(string)
	return key
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
			if fail, ok := u.speciesFail[key]; ok {
				writeStub(w, fail.status, fail.body)
				return
			}
			if u.speciesBody != "" || u.speciesStatus != 0 {
				writeStub(w, u.speciesStatus, u.speciesBody)
				return
			}
			baseSpeed := orDefault(u.attackerSpeed, 100)
			if key == defenderSpeciesKey {
				baseSpeed = orDefault(u.defenderSpeed, 100)
			}
			if explicit, ok := u.baseSpeeds[key]; ok {
				baseSpeed = explicit
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

		defenderKey := calcDefenderSpeciesKey(body)
		if fail, ok := u.calcFail[defenderKey]; ok {
			writeStub(w, fail.status, fail.body)
			return
		}
		if ko, ok := u.calcKO[defenderKey]; ok {
			writeStub(w, http.StatusOK, `{"minDamage":1,"maxDamage":2,"defenderHP":172,"ko":`+ko+`}`)
			return
		}

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

// individual は 1 個体ぶんの request の欄。候補を並べるときに使う。
func individual(speciesKey, natureID string, spSpe int) map[string]any {
	return map[string]any{
		"speciesKey": speciesKey,
		"natureId":   natureID,
		"sp":         sp(spSpe),
	}
}

// validBody は 200 になる request body(相手候補 1 件)。attacker は最速(SP32・上昇補正)= 167、
// 候補は無振り無補正 = 120 で、attacker が抜ける。
func validBody() map[string]any {
	return map[string]any{
		"format":    "single",
		"attacker":  individual(attackerSpeciesKey, naturePlusSpeID, 32),
		"defenders": []any{individual(defenderSpeciesKey, natureNeutralID, 0)},
		"moveId":    testMoveID,
	}
}

// bodyWithDefenders は attacker / moveId はそのままに、相手候補だけを差し替える(JD3)。
func bodyWithDefenders(defenders ...map[string]any) map[string]any {
	body := validBody()
	list := make([]any, 0, len(defenders))
	for _, d := range defenders {
		list = append(list, d)
	}
	body["defenders"] = list
	return body
}

func attackerOf(body map[string]any) map[string]any {
	return body["attacker"].(map[string]any)
}

// defenderAt は body の i 番目の相手候補を返す。
func defenderAt(body map[string]any, i int) map[string]any {
	return body["defenders"].([]any)[i].(map[string]any)
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

// onlyMatchup は相手候補 1 件の request の唯一の判定結果を返す。候補が 1 件なら
// matchups も 1 件でなければならない(ADR-0703 受け入れ条件1: 同じ件数)。
func onlyMatchup(t *testing.T, recorder *httptest.ResponseRecorder) api.Matchup {
	t.Helper()
	response := decodeResponse(t, recorder)
	if len(response.Matchups) != 1 {
		t.Fatalf("matchups の件数 = %d, want 1; body=%s", len(response.Matchups), recorder.Body.String())
	}
	if response.Matchups[0].DefenderIndex != 0 {
		t.Errorf("matchups[0].defenderIndex = %d, want 0", response.Matchups[0].DefenderIndex)
	}
	return response.Matchups[0]
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

// assertBlamesCandidate はエラーの message が「どの候補で失敗したか」を defenders[<index>] の
// 形で示していることを確かめる(ADR-0703 §3。上流の URL・本文は含めないので ADR-0700 §3 は保たれる)。
func assertBlamesCandidate(t *testing.T, recorder *httptest.ResponseRecorder, index int) {
	t.Helper()
	want := "defenders[" + strconv.Itoa(index) + "]"
	message := decodeError(t, recorder).Message
	if !strings.Contains(message, want) {
		t.Errorf("message = %q に %q が無い。どの候補で失敗したかを示す(ADR-0703 §3)", message, want)
	}
}

// assertBlamesAttacker は attacker 側の失敗で候補の index を騙らないことを確かめる。
func assertBlamesAttacker(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	message := decodeError(t, recorder).Message
	if strings.Contains(message, "defenders[") {
		t.Errorf("message = %q。attacker 側の失敗なのに候補の index を示している", message)
	}
	if !strings.Contains(message, "attacker") {
		t.Errorf("message = %q に attacker が無い。どちら側の失敗かを示す(ADR-0703 §3)", message)
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

// assertNoUpstreamCalls は上流を 1 回も呼んでいないことを確かめる(ADR-0701 §5:
// request の検査は上流より先)。
func assertNoUpstreamCalls(t *testing.T, stub *upstreams) {
	t.Helper()
	natures, speciesKeys, calcCalls := stub.counts()
	if natures+len(speciesKeys)+calcCalls != 0 {
		t.Errorf("上流を呼んでいる(natures=%d species=%v calc=%d)。request の検査は上流より先(ADR-0701 §5)",
			natures, speciesKeys, calcCalls)
	}
}

// TestOutspeedAndKo: 正常系(ADR-0701 受け入れ条件1・2・3、ADR-0703 受け入れ条件1)。
// attacker = (100+20+32)×1.1 = 167、候補は無振り無補正 = 120 で抜ける。
// natures は 1 リクエストにつき 1 回だけ引き、attacker と全候補をそれで解決する。
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

	got := onlyMatchup(t, recorder)
	want := api.Matchup{
		DefenderIndex: 0,
		Outspeeds:     true,
		SpeedTie:      false,
		AttackerSpeed: 167,
		DefenderSpeed: 120,
		Ko:            api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100},
	}
	if got != want {
		t.Errorf("matchups[0] = %+v, want %+v", got, want)
	}

	natures, speciesKeys, calcCalls := stub.counts()
	if natures != 1 {
		t.Errorf("GET /api/pokedex/natures を %d 回呼んでいる。1 リクエストにつき 1 回だけにする(ADR-0701 §4)", natures)
	}
	wantKeys := []string{attackerSpeciesKey, defenderSpeciesKey}
	if !reflect.DeepEqual(speciesKeys, wantKeys) {
		t.Errorf("species の呼び出し = %v, want %v(attacker → 候補を index 昇順)", speciesKeys, wantKeys)
	}
	if calcCalls != 1 {
		t.Errorf("calc-svc を %d 回呼んでいる。候補 1 件なら 1 回にする", calcCalls)
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

	// calc-svc へは judge の request をそのまま組み替えて渡す(1 候補 = 1 回の 1vs1 計算)。
	body := stub.lastCalcBody(t)
	if body["format"] != "single" {
		t.Errorf("calc の format = %v, want single", body["format"])
	}
	if body["moveId"] != testMoveID {
		t.Errorf("calc の moveId = %v, want %s", body["moveId"], testMoveID)
	}
	if value, present := body["defenders"]; present {
		t.Errorf("calc に defenders = %v を送っている。calc-svc は 1vs1 の defender しか知らない", value)
	}
	attacker, ok := body["attacker"].(map[string]any)
	if !ok {
		t.Fatalf("calc の attacker = %v, want an object", body["attacker"])
	}
	if attacker["speciesKey"] != attackerSpeciesKey || attacker["natureId"] != naturePlusSpeID {
		t.Errorf("calc の attacker = %v, want speciesKey=%s natureId=%s", attacker, attackerSpeciesKey, naturePlusSpeID)
	}
}

// TestOutspeedAndKoSpeedTie: 同速は outspeeds=false・speedTie=true(ADR-0700 §6-1)。
// 真偽値 1 つに丸めないので、「抜けられている」(どちらも false)と区別できる。
func TestOutspeedAndKoSpeedTie(t *testing.T) {
	t.Parallel()

	// 双方とも無振り無補正(120)にすると同速になる。
	body := validBody()
	attackerOf(body)["natureId"] = natureNeutralID
	attackerOf(body)["sp"] = sp(0)

	recorder := postOutspeed(newUpstreams(t, &upstreams{}), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	got := onlyMatchup(t, recorder)
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
	attackerOf(body)["natureId"] = natureNeutralID
	attackerOf(body)["sp"] = sp(0)
	defenderAt(body, 0)["natureId"] = naturePlusSpeID
	defenderAt(body, 0)["sp"] = sp(32)

	recorder := postOutspeed(newUpstreams(t, &upstreams{}), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	got := onlyMatchup(t, recorder)
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
	attacker := attackerOf(body)
	attacker["natureId"] = natureNeutralID
	attacker["sp"] = sp(0)
	attacker["ranks"] = map[string]any{"spe": 1}
	defenderAt(body, 0)["natureId"] = naturePlusSpeID
	defenderAt(body, 0)["sp"] = sp(32)

	recorder := postOutspeed(newUpstreams(t, &upstreams{}), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	got := onlyMatchup(t, recorder)
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
		name           string
		scarfItemID    string // Dependencies の上書き(空なら既定)
		itemID         string
		defenderItemID string
		wantSpeed      int
		wantDefSpeed   int
		wantOutspeeds  bool
		wantSpeedTie   bool
	}{
		// 候補は natureId=naturePlusSpeID・sp=32(下のループの前で固定)なので、スカーフ無しの
		// 素早さは (100+20+32)×1.1 = floor(167.2) = 167 になる。
		{"既定の ID", "", "choicescarf", "", 180, 167, true, false},
		{"別の持ち物は効かない", "", "test-other-item", "", 120, 167, false, false},
		{"持ち物なし", "", "", "", 120, 167, false, false},
		{"設定で上書きした ID", "test-scarf", "test-scarf", "", 180, 167, true, false},
		{"上書きすると既定は効かない", "test-scarf", "choicescarf", "", 120, 167, false, false},
		// 候補側の itemId でもスカーフが乗ることを確認する(critic 指摘: attacker 側しか
		// 検査していなかった。ADR-0701 受け入れ条件2は自分・相手どちらの持ち物も対象)。
		// 167 × 1.5 の五捨五超入 = floor((167*6144+2047)/4096) = 250。
		{"候補のスカーフで抜き返される", "", "", "choicescarf", 120, 250, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := validBody()
			attacker := attackerOf(body)
			attacker["natureId"] = natureNeutralID
			attacker["sp"] = sp(0)
			if tt.itemID != "" {
				attacker["itemId"] = tt.itemID
			}
			defender := defenderAt(body, 0)
			defender["natureId"] = naturePlusSpeID
			defender["sp"] = sp(32)
			if tt.defenderItemID != "" {
				defender["itemId"] = tt.defenderItemID
			}

			stub := &upstreams{}
			deps := newUpstreams(t, stub)
			deps.ChoiceScarfItemID = tt.scarfItemID

			recorder := postOutspeed(deps, body, nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			got := onlyMatchup(t, recorder)
			if got.AttackerSpeed != tt.wantSpeed || got.DefenderSpeed != tt.wantDefSpeed ||
				got.Outspeeds != tt.wantOutspeeds || got.SpeedTie != tt.wantSpeedTie {
				t.Errorf("attackerSpeed/defenderSpeed/outspeeds/speedTie = %d/%d/%v/%v, want %d/%d/%v/%v",
					got.AttackerSpeed, got.DefenderSpeed, got.Outspeeds, got.SpeedTie,
					tt.wantSpeed, tt.wantDefSpeed, tt.wantOutspeeds, tt.wantSpeedTie)
			}
			// スカーフの持ち物 ID は calc-svc にもそのまま渡す(ダメージ側の効果は calc-svc が持つ)。
			calcBody := stub.lastCalcBody(t)
			if tt.itemID != "" {
				attackerSent := calcBody["attacker"].(map[string]any)
				if attackerSent["itemId"] != tt.itemID {
					t.Errorf("calc の attacker.itemId = %v, want %q", attackerSent["itemId"], tt.itemID)
				}
			}
			if tt.defenderItemID != "" {
				defenderSent := calcBody["defender"].(map[string]any)
				if defenderSent["itemId"] != tt.defenderItemID {
					t.Errorf("calc の defender.itemId = %v, want %q", defenderSent["itemId"], tt.defenderItemID)
				}
			}
		})
	}
}

// TestOutspeedAndKoAsymmetricBaseSpeed: attacker/候補の種族値そのものが違う(調整では
// なく種族差)場合でも、それぞれの Species を正しく紐づけて素早さを計算する(critic 指摘:
// attackerSpeed/defenderSpeed のフィクスチャがどのテストからも設定されておらず、種族の
// 取り違えが緑のまま通っていた)。
func TestOutspeedAndKoAsymmetricBaseSpeed(t *testing.T) {
	t.Parallel()

	body := validBody()
	attackerOf(body)["natureId"] = natureNeutralID
	attackerOf(body)["sp"] = sp(0)

	stub := &upstreams{attackerSpeed: 100, defenderSpeed: 130}
	recorder := postOutspeed(newUpstreams(t, stub), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	got := onlyMatchup(t, recorder)
	// 100 + 20 + 0 = 120 / 130 + 20 + 0 = 150(どちらも無補正・無振り)。
	if got.AttackerSpeed != 120 {
		t.Errorf("attackerSpeed = %d, want 120(種族値100の方)", got.AttackerSpeed)
	}
	if got.DefenderSpeed != 150 {
		t.Errorf("defenderSpeed = %d, want 150(種族値130の方)", got.DefenderSpeed)
	}
	if got.Outspeeds || got.SpeedTie {
		t.Errorf("outspeeds/speedTie = %v/%v, want false/false(候補の方が速い)", got.Outspeeds, got.SpeedTie)
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
			if got := onlyMatchup(t, recorder).Ko; got != tt.want {
				t.Errorf("ko = %+v, want %+v", got, tt.want)
			}

			var raw struct {
				Matchups []map[string]any `json:"matchups"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
				t.Fatalf("body をデコードできない: %v", err)
			}
			if len(raw.Matchups) != 1 {
				t.Fatalf("matchups の件数 = %d, want 1", len(raw.Matchups))
			}
			ko, ok := raw.Matchups[0]["ko"].(map[string]any)
			if !ok {
				t.Fatalf("ko = %v, want an object", raw.Matchups[0]["ko"])
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

	t.Run("候補が複数でも同じ field が全員に転送される", func(t *testing.T) {
		t.Parallel()

		body := bodyWithDefenders(
			individual(defenderSpeciesKey, natureNeutralID, 0),
			individual(defender2SpeciesKey, natureNeutralID, 0),
		)
		body["field"] = map[string]any{"weather": "rain"}

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}

		stub.mu.Lock()
		bodies := append([]map[string]any(nil), stub.calcBodies...)
		stub.mu.Unlock()
		if len(bodies) != 2 {
			t.Fatalf("calc を %d 回呼んでいる。候補 2 件なら 2 回", len(bodies))
		}
		for i, sent := range bodies {
			field, ok := sent["field"].(map[string]any)
			if !ok || field["weather"] != "rain" {
				t.Errorf("calc %d 回目の field = %v, want weather=rain(ADR-0703 §5)", i, sent["field"])
			}
		}
	})
}

// TestOutspeedAndKoRejectsInvalidRequest: ヘッダー・body の検査は上流より先(ADR-0701 §5)。
// 400 invalid_request を返し、**上流を 1 回も呼ばない**(無駄な往復をしない。ADR-0700 §3 と同じ立場)。
func TestOutspeedAndKoRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	withAttacker := func(mutate func(map[string]any)) map[string]any {
		body := validBody()
		mutate(attackerOf(body))
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
		// 候補側の欄も同じ検査を受ける。
		{"候補の speciesKey が形式に合わない", func() any {
			body := validBody()
			defenderAt(body, 0)["speciesKey"] = "pikachu"
			return body
		}(), nil},
		{"候補の sp が無い", func() any {
			body := validBody()
			delete(defenderAt(body, 0), "sp")
			return body
		}(), nil},
		{"候補が個体ではなく文字列", func() any {
			body := validBody()
			body["defenders"] = []any{"9002-000"}
			return body
		}(), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &upstreams{}
			recorder := postOutspeed(newUpstreams(t, stub), tt.body, tt.headers)

			assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
			assertNoUpstreamCalls(t, stub)
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
	assertNoUpstreamCalls(t, stub)
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
// (ADR-0701 §6)。黙って無補正に倒さない。どの候補だったかは message に出す(ADR-0703 §3)。
func TestOutspeedAndKoUnknownNature(t *testing.T) {
	t.Parallel()

	t.Run("attacker", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		attackerOf(body)["natureId"] = "test-missing-nature"

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownNature)
		assertBlamesAttacker(t, recorder)

		if _, speciesKeys, calcCalls := stub.counts(); len(speciesKeys)+calcCalls != 0 {
			t.Error("性格を解決できていないのに種族・calc-svc を呼んでいる")
		}
	})

	t.Run("候補", func(t *testing.T) {
		t.Parallel()

		body := bodyWithDefenders(
			individual(defenderSpeciesKey, natureNeutralID, 0),
			individual(defender2SpeciesKey, "test-missing-nature", 0),
		)

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownNature)
		assertBlamesCandidate(t, recorder, 1)

		if _, speciesKeys, calcCalls := stub.counts(); len(speciesKeys)+calcCalls != 0 {
			t.Error("性格を解決できていないのに種族・calc-svc を呼んでいる")
		}
	})
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
		{
			// pokedex の 400 は calc-svc の 400 と違って invalid_request にしない(ADR-0701 §6追記)。
			// judge は speciesKey を事前検査してから呼ぶため、それでも pokedex が 400 を返すのは
			// 呼び出し側の入力の非ではなく上流との契約ズレを意味する。
			"natures が 400",
			func() *upstreams {
				return &upstreams{naturesStatus: http.StatusBadRequest, naturesBody: `{"code":"invalid_input","message":"bad request"}`}
			},
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			"species が 400",
			func() *upstreams {
				return &upstreams{speciesStatus: http.StatusBadRequest, speciesBody: `{"code":"invalid_input","message":"bad key"}`}
			},
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
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
// (ADR-0700 §3。原因はログに残す)。候補の index を message に足しても、この性質は変わらない
// (ADR-0703 §3)。
func TestOutspeedAndKoDoesNotLeakUpstreamDetail(t *testing.T) {
	t.Parallel()

	const upstreamDetail = "dsn dbhost01 svcaccount internal-only-detail"

	t.Run("性格の一覧の失敗", func(t *testing.T) {
		t.Parallel()

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
	})

	t.Run("候補の種族の失敗", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			speciesFail: map[string]stubResponse{
				defender2SpeciesKey: {http.StatusNotFound, `{"code":"not_found","message":"` + upstreamDetail + `"}`},
			},
		}
		body := bodyWithDefenders(
			individual(defenderSpeciesKey, natureNeutralID, 0),
			individual(defender2SpeciesKey, natureNeutralID, 0),
		)
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)

		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownSpecies)
		responseBody := recorder.Body.String()
		if strings.Contains(responseBody, upstreamDetail) {
			t.Errorf("応答が上流の本文を漏らしている: %s", responseBody)
		}
		assertNoUpstreamDetail(t, responseBody, stub.pokedexURL, stub.calcURL)
	})
}

// TestOutspeedAndKoCheckOrder: 検査順は固定(ADR-0701 §5・ADR-0703 §4)。上流を逐次で呼ぶので、
// 複数の原因が同時にあってもどれが返るかが決まる(並列化すると実行ごとに変わる)。
func TestOutspeedAndKoCheckOrder(t *testing.T) {
	t.Parallel()

	t.Run("range 検査は性格の一覧より先", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		attackerOf(body)["sp"] = sp(33)
		attackerOf(body)["natureId"] = "test-missing-nature"

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
		attackerOf(body)["natureId"] = "test-missing-nature"

		stub := &upstreams{
			speciesStatus: http.StatusNotFound,
			speciesBody:   `{"code":"not_found","message":"no such species"}`,
		}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		// 性格も種族も解決できないが、検査順どおり unknown_nature が返る。
		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownNature)
	})

	t.Run("候補の件数の検査は候補の中身の範囲検査より先", func(t *testing.T) {
		t.Parallel()

		// 7 件目があり、かつ 1 件目の sp も範囲外。件数の方が先に返る(ADR-0703 §4)。
		defenders := make([]map[string]any, 0, 7)
		for i := 0; i < 7; i++ {
			defenders = append(defenders, individual(defenderSpeciesKey, natureNeutralID, 0))
		}
		body := bodyWithDefenders(defenders...)
		defenderAt(body, 0)["sp"] = sp(33)

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
		assertNoUpstreamCalls(t, stub)
	})

	t.Run("attacker の検査は候補の検査より先", func(t *testing.T) {
		t.Parallel()

		body := bodyWithDefenders(
			individual(defenderSpeciesKey, natureNeutralID, 33),
			individual(defender2SpeciesKey, natureNeutralID, 0),
		)
		attackerOf(body)["sp"] = sp(33)

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
		assertBlamesAttacker(t, recorder)
		assertNoUpstreamCalls(t, stub)
	})

	t.Run("attacker の種族は候補の種族より先", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			speciesFail: map[string]stubResponse{
				attackerSpeciesKey:  {http.StatusNotFound, `{"code":"not_found","message":"no such species"}`},
				defenderSpeciesKey:  {http.StatusNotFound, `{"code":"not_found","message":"no such species"}`},
				defender2SpeciesKey: {http.StatusNotFound, `{"code":"not_found","message":"no such species"}`},
			},
		}
		body := bodyWithDefenders(
			individual(defenderSpeciesKey, natureNeutralID, 0),
			individual(defender2SpeciesKey, natureNeutralID, 0),
		)
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)

		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownSpecies)
		assertBlamesAttacker(t, recorder)
		_, speciesKeys, _ := stub.counts()
		if !reflect.DeepEqual(speciesKeys, []string{attackerSpeciesKey}) {
			t.Errorf("species の呼び出し = %v, want [%s](attacker で打ち切る)", speciesKeys, attackerSpeciesKey)
		}
	})
}

// --- JD2: 場の効果(トリックルーム・追い風)。ADR-0702 ---

// neutralBody は attacker / 候補をどちらも無振り・無補正(= 実数値 120)にした body。
// JD2 のテストは場の効果だけを動かして差を見たいので、調整の差を先に消しておく。
func neutralBody() map[string]any {
	body := validBody()
	attacker := attackerOf(body)
	attacker["natureId"] = natureNeutralID
	attacker["sp"] = sp(0)
	defender := defenderAt(body, 0)
	defender["natureId"] = natureNeutralID
	defender["sp"] = sp(0)
	return body
}

// TestOutspeedAndKoTrickRoom: トリックルームは実数値を変えず、outspeeds(自分が先に動くか)の
// 向きだけを反転する(ADR-0702 §3・受け入れ条件4)。speedTie は反転しない。
func TestOutspeedAndKoTrickRoom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		trickRoom     bool
		attackerSpeed int // 種族の素早さ種族値(0 なら 100)
		defenderSpeed int
		wantAttacker  int
		wantDefender  int
		wantOutspeeds bool
		wantSpeedTie  bool
	}{
		// 種族値 100 → 120、種族値 130 → 150(どちらも無振り・無補正)。
		{"トリックルーム無し: 速い方が先に動く", false, 130, 100, 150, 120, true, false},
		{"トリックルーム中: 速い方が後になる", true, 130, 100, 150, 120, false, false},
		{"トリックルーム無し: 遅いと抜けない", false, 100, 130, 120, 150, false, false},
		{"トリックルーム中: 遅い方が先に動く", true, 100, 130, 120, 150, true, false},
		{"同速はトリックルーム無しで speedTie", false, 100, 100, 120, 120, false, true},
		// 同速はゲームでもトリックルームの有無に関わらず行動順が決まらない(ADR-0702 §3)。
		{"同速はトリックルーム中でも反転しない", true, 100, 100, 120, 120, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := neutralBody()
			body["speedField"] = map[string]any{"trickRoom": tt.trickRoom}

			stub := &upstreams{attackerSpeed: tt.attackerSpeed, defenderSpeed: tt.defenderSpeed}
			recorder := postOutspeed(newUpstreams(t, stub), body, nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}

			got := onlyMatchup(t, recorder)
			// トリックルームは実数値を変えない(ADR-0702 §2)。
			if got.AttackerSpeed != tt.wantAttacker || got.DefenderSpeed != tt.wantDefender {
				t.Errorf("attackerSpeed/defenderSpeed = %d/%d, want %d/%d(トリックルームは実数値を変えない)",
					got.AttackerSpeed, got.DefenderSpeed, tt.wantAttacker, tt.wantDefender)
			}
			if got.Outspeeds != tt.wantOutspeeds || got.SpeedTie != tt.wantSpeedTie {
				t.Errorf("outspeeds/speedTie = %v/%v, want %v/%v",
					got.Outspeeds, got.SpeedTie, tt.wantOutspeeds, tt.wantSpeedTie)
			}
			if got.Outspeeds && got.SpeedTie {
				t.Error("outspeeds と speedTie が同時に true になっている")
			}
		})
	}
}

// TestOutspeedAndKoTailwind: 追い風は指定した側の実数値を ×2 し、attackerSpeed / defenderSpeed に
// 現れる(ADR-0702 §2・受け入れ条件2)。attackerTailwind は自分だけ、defenderTailwind は相手だけ。
func TestOutspeedAndKoTailwind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		attackerTailwind bool
		defenderTailwind bool
		wantAttacker     int
		wantDefender     int
		wantOutspeeds    bool
		wantSpeedTie     bool
	}{
		// attacker の種族値 100 → 120、候補の種族値 130 → 150。
		{"追い風なし", false, false, 120, 150, false, false},
		{"自分だけ追い風", true, false, 240, 150, true, false},
		{"相手だけ追い風", false, true, 120, 300, false, false},
		{"両方追い風(大小関係は変わらない)", true, true, 240, 300, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := neutralBody()
			body["speedField"] = map[string]any{
				"attackerTailwind": tt.attackerTailwind,
				"defenderTailwind": tt.defenderTailwind,
			}

			stub := &upstreams{attackerSpeed: 100, defenderSpeed: 130}
			recorder := postOutspeed(newUpstreams(t, stub), body, nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}

			got := onlyMatchup(t, recorder)
			if got.AttackerSpeed != tt.wantAttacker || got.DefenderSpeed != tt.wantDefender ||
				got.Outspeeds != tt.wantOutspeeds || got.SpeedTie != tt.wantSpeedTie {
				t.Errorf("attackerSpeed/defenderSpeed/outspeeds/speedTie = %d/%d/%v/%v, want %d/%d/%v/%v",
					got.AttackerSpeed, got.DefenderSpeed, got.Outspeeds, got.SpeedTie,
					tt.wantAttacker, tt.wantDefender, tt.wantOutspeeds, tt.wantSpeedTie)
			}
		})
	}
}

// TestOutspeedAndKoTailwindAndScarfRounding: 追い風とこだわりスカーフが同時に乗るときは、
// 4096 基準で 1 つに連結してから 1 回だけ五捨五超入する(ADR-0702 §2・受け入れ条件3)。
// 出典は @smogon/calc 0.12.0 の getFinalSpeed(chainMods → pokeRound を 1 回)。
//
// 種族値 71・無振り・無補正 = 実数値 91(奇数)。
// 連結: chainMods([8192, 6144]) = 12288 = ×3 → 91 × 3 = 273。
// 各補正ごとに丸める実装: 91 × 1.5 = 136.5 → 136 → × 2 = 272(1 ずれる)。
func TestOutspeedAndKoTailwindAndScarfRounding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		baseSpeed int
		scarf     bool
		tailwind  bool
		want      int
		naive     int // 補正ごとに丸めた場合の誤った値(want と同じなら差が出ないケース)
	}{
		{"実数値 91・スカーフのみ(JD1 から変わらない)", 71, true, false, 136, 136},
		{"実数値 91・追い風のみ", 71, false, true, 182, 182},
		{"実数値 91・スカーフ + 追い風", 71, true, true, 273, 272},
		{"実数値 93・スカーフ + 追い風", 73, true, true, 279, 278},
		{"実数値 120・スカーフ + 追い風(偶数なので差は出ない)", 100, true, true, 360, 360},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := neutralBody()
			if tt.scarf {
				attackerOf(body)["itemId"] = "choicescarf"
			}
			body["speedField"] = map[string]any{"attackerTailwind": tt.tailwind}

			stub := &upstreams{attackerSpeed: tt.baseSpeed, defenderSpeed: 100}
			recorder := postOutspeed(newUpstreams(t, stub), body, nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}

			got := onlyMatchup(t, recorder).AttackerSpeed
			if got != tt.want {
				t.Errorf("attackerSpeed = %d, want %d", got, tt.want)
			}
			if tt.naive != tt.want && got == tt.naive {
				t.Errorf("attackerSpeed = %d は補正ごとに丸めた値。連結してから 1 回だけ五捨五超入する(ADR-0702 §2)", got)
			}
		})
	}
}

// TestOutspeedAndKoDoesNotForwardSpeedField: speedField は judge だけが解釈し、calc-svc には
// 送らない(ADR-0702 §1・受け入れ条件6)。calc-svc が理解するのは weather/terrain/screens だけで、
// トリックルーム・追い風はダメージに関与しない。既存の field の転送は変わらない。
func TestOutspeedAndKoDoesNotForwardSpeedField(t *testing.T) {
	t.Parallel()

	t.Run("speedField だけを指定しても calc には送らない", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		body["speedField"] = map[string]any{
			"trickRoom":        true,
			"attackerTailwind": true,
			"defenderTailwind": true,
		}

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}

		calc := stub.lastCalcBody(t)
		if value, present := calc["speedField"]; present {
			t.Errorf("calc に speedField = %v を送っている。calc-svc は解釈できない(ADR-0702 §1)", value)
		}
		if value, present := calc["field"]; present {
			t.Errorf("calc に field = %v を送っている。request で指定していない", value)
		}
	})

	t.Run("field と併用しても field だけが転送される", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		body["field"] = map[string]any{"weather": "sun"}
		body["speedField"] = map[string]any{"trickRoom": true, "attackerTailwind": true}

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}

		calc := stub.lastCalcBody(t)
		if value, present := calc["speedField"]; present {
			t.Errorf("calc に speedField = %v を送っている(ADR-0702 §1)", value)
		}
		field, ok := calc["field"].(map[string]any)
		if !ok {
			t.Fatalf("calc の field = %v, want an object", calc["field"])
		}
		if field["weather"] != "sun" {
			t.Errorf("calc の field.weather = %v, want sun(ADR-0701 受け入れ条件6 は変わらない)", field["weather"])
		}
		// トリックルーム・追い風が field に紛れ込んでいないこと。
		for _, key := range []string{"trickRoom", "attackerTailwind", "defenderTailwind"} {
			if value, present := field[key]; present {
				t.Errorf("calc の field に %s = %v が混ざっている", key, value)
			}
		}
	})
}

// TestOutspeedAndKoSpeedFieldOmittedMatchesJD1: speedField を省略した request の判定は JD1 と同一
// (ADR-0702 受け入れ条件5)。空オブジェクト・全欄 false も同じ。
func TestOutspeedAndKoSpeedFieldOmittedMatchesJD1(t *testing.T) {
	t.Parallel()

	// JD1 と同じ条件: attacker = 167、候補 = 120 で抜ける。
	want := api.Matchup{
		DefenderIndex: 0,
		Outspeeds:     true,
		SpeedTie:      false,
		AttackerSpeed: 167,
		DefenderSpeed: 120,
		Ko:            api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100},
	}

	tests := []struct {
		name       string
		speedField any // nil なら欄そのものを送らない
	}{
		{"speedField を送らない", nil},
		{"speedField が空オブジェクト", map[string]any{}},
		{"speedField の全欄が false", map[string]any{
			"trickRoom": false, "attackerTailwind": false, "defenderTailwind": false,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := validBody()
			if tt.speedField != nil {
				body["speedField"] = tt.speedField
			}

			stub := &upstreams{}
			recorder := postOutspeed(newUpstreams(t, stub), body, nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := onlyMatchup(t, recorder); got != want {
				t.Errorf("matchups[0] = %+v, want %+v(JD1 と同じ)", got, want)
			}
		})
	}
}

// TestOutspeedAndKoRejectsInvalidSpeedField: speedField の未知の欄・真偽値でない値は
// 400 invalid_request で、上流を 1 回も呼ばない(ADR-0702 受け入れ条件7。検査順は ADR-0701 §5 のまま)。
func TestOutspeedAndKoRejectsInvalidSpeedField(t *testing.T) {
	t.Parallel()

	withSpeedField := func(value any) map[string]any {
		body := validBody()
		body["speedField"] = value
		return body
	}

	tests := []struct {
		name string
		body any
	}{
		{"未知の欄がある", withSpeedField(map[string]any{"gravity": true})},
		{"トリックルームの綴り違い", withSpeedField(map[string]any{"trickroom": true})},
		{"trickRoom が真偽値でない", withSpeedField(map[string]any{"trickRoom": "true"})},
		{"attackerTailwind が数値", withSpeedField(map[string]any{"attackerTailwind": 1})},
		{"speedField がオブジェクトでない", withSpeedField(true)},
		// 追い風は side ごとの欄で受け取る。単一の tailwind という欄は契約に無い。
		{"tailwind という欄は無い", withSpeedField(map[string]any{"tailwind": true})},
		// 候補ごとに追い風を変える機能は JD3 に無い(ADR-0703 §5)。
		{"候補ごとの追い風という欄は無い", withSpeedField(map[string]any{"defenderTailwinds": []any{true}})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &upstreams{}
			recorder := postOutspeed(newUpstreams(t, stub), tt.body, nil)

			assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
			assertNoUpstreamCalls(t, stub)
		})
	}
}

// --- JD3: 複数の相手候補を一度に判定。ADR-0703 ---

// TestOutspeedAndKoMultipleDefenders: 攻撃側と技は 1 つに固定したまま、相手候補ごとの判定を
// matchups の配列で返す(ADR-0703 §1・§2・受け入れ条件1・2)。
// attacker は無振り無補正・種族値 100 = 120。候補は種族値 90 / 100 / 130 = 110 / 120 / 150 で、
// 「抜ける」「同速」「抜かれる」が 1 リクエストで同時に出る。
func TestOutspeedAndKoMultipleDefenders(t *testing.T) {
	t.Parallel()

	body := bodyWithDefenders(
		individual(defender2SpeciesKey, natureNeutralID, 0), // 種族値 90 → 110
		individual(defenderSpeciesKey, natureNeutralID, 0),  // 種族値 100 → 120(同速)
		individual(defender3SpeciesKey, natureNeutralID, 0), // 種族値 130 → 150
	)
	attackerOf(body)["natureId"] = natureNeutralID
	attackerOf(body)["sp"] = sp(0)

	stub := &upstreams{
		baseSpeeds: map[string]int{
			attackerSpeciesKey:  100,
			defender2SpeciesKey: 90,
			defenderSpeciesKey:  100,
			defender3SpeciesKey: 130,
		},
		calcKO: map[string]string{
			defender2SpeciesKey: `{"hits":1,"guaranteed":true,"chancePercent":0,"displayChancePercent":100}`,
			defenderSpeciesKey:  `{"hits":2,"guaranteed":false,"chancePercent":50,"displayChancePercent":50}`,
			defender3SpeciesKey: `{"hits":0,"guaranteed":false,"chancePercent":0,"displayChancePercent":0}`,
		},
	}
	recorder := postOutspeed(newUpstreams(t, stub), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	want := []api.Matchup{
		{
			DefenderIndex: 0, Outspeeds: true, SpeedTie: false,
			AttackerSpeed: 120, DefenderSpeed: 110,
			Ko: api.KOChance{Hits: 1, Guaranteed: true, DisplayChancePercent: 100},
		},
		{
			DefenderIndex: 1, Outspeeds: false, SpeedTie: true,
			AttackerSpeed: 120, DefenderSpeed: 120,
			Ko: api.KOChance{Hits: 2, Guaranteed: false, DisplayChancePercent: 50},
		},
		{
			DefenderIndex: 2, Outspeeds: false, SpeedTie: false,
			AttackerSpeed: 120, DefenderSpeed: 150,
			Ko: api.KOChance{Hits: 0, Guaranteed: false, DisplayChancePercent: 0},
		},
	}
	got := decodeResponse(t, recorder).Matchups
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matchups = %+v, want %+v", got, want)
	}

	// 上流は natures 1 + attacker の種族 1 + 候補の種族 3 + calc 3(ADR-0703 §1)。
	natures, speciesKeys, calcCalls := stub.counts()
	if natures != 1 {
		t.Errorf("natures を %d 回呼んでいる。候補が増えても 1 回(ADR-0703 §1)", natures)
	}
	wantSpecies := []string{attackerSpeciesKey, defender2SpeciesKey, defenderSpeciesKey, defender3SpeciesKey}
	if !reflect.DeepEqual(speciesKeys, wantSpecies) {
		t.Errorf("species の呼び出し = %v, want %v(attacker → 候補を index 昇順。ADR-0703 §4)", speciesKeys, wantSpecies)
	}
	if calcCalls != 3 {
		t.Errorf("calc を %d 回呼んでいる。候補 3 件なら 3 回", calcCalls)
	}
	wantCalcOrder := []string{defender2SpeciesKey, defenderSpeciesKey, defender3SpeciesKey}
	if !reflect.DeepEqual(stub.calcDefenderKeys(), wantCalcOrder) {
		t.Errorf("calc の defender = %v, want %v(index 昇順)", stub.calcDefenderKeys(), wantCalcOrder)
	}
}

// TestOutspeedAndKoMatchupOrderFollowsDefenders: matchups は defenders と同じ順序・同じ件数で、
// i 番目の defenderIndex は i(ADR-0703 §2・受け入れ条件1)。同じ speciesKey が重複していても
// 取りまとめず、候補の数だけ素直に上流を引く(ADR-0703 §6)。
func TestOutspeedAndKoMatchupOrderFollowsDefenders(t *testing.T) {
	t.Parallel()

	// 同じ種族・違う調整の 2 候補。重複除去をすると 2 行目が 1 行目の値になって落ちる。
	body := bodyWithDefenders(
		individual(defenderSpeciesKey, natureNeutralID, 0),  // 120
		individual(defenderSpeciesKey, naturePlusSpeID, 32), // 167
	)
	attackerOf(body)["natureId"] = natureNeutralID
	attackerOf(body)["sp"] = sp(0) // 120

	stub := &upstreams{}
	recorder := postOutspeed(newUpstreams(t, stub), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	got := decodeResponse(t, recorder).Matchups
	if len(got) != 2 {
		t.Fatalf("matchups の件数 = %d, want 2(defenders と同じ件数)", len(got))
	}
	if got[0].DefenderIndex != 0 || got[1].DefenderIndex != 1 {
		t.Errorf("defenderIndex = %d/%d, want 0/1", got[0].DefenderIndex, got[1].DefenderIndex)
	}
	if got[0].DefenderSpeed != 120 || !got[0].SpeedTie {
		t.Errorf("matchups[0] = %+v, want defenderSpeed=120 speedTie=true", got[0])
	}
	if got[1].DefenderSpeed != 167 || got[1].Outspeeds || got[1].SpeedTie {
		t.Errorf("matchups[1] = %+v, want defenderSpeed=167 outspeeds=false speedTie=false", got[1])
	}

	// 重複除去はしない(ADR-0703 §6)。同じ speciesKey を候補の数だけ引く。
	_, speciesKeys, calcCalls := stub.counts()
	wantSpecies := []string{attackerSpeciesKey, defenderSpeciesKey, defenderSpeciesKey}
	if !reflect.DeepEqual(speciesKeys, wantSpecies) {
		t.Errorf("species の呼び出し = %v, want %v(重複除去はしない)", speciesKeys, wantSpecies)
	}
	if calcCalls != 2 {
		t.Errorf("calc を %d 回呼んでいる。候補 2 件なら 2 回(調整が違えば結果も違う)", calcCalls)
	}
}

// TestOutspeedAndKoNaturesCalledOnceForEveryCandidate: natures は候補が増えても
// 1 リクエストにつき 1 回だけ(ADR-0701 §4・ADR-0703 §1・受け入れ条件4)。
// 上限の 6 件でも 200 になる。
func TestOutspeedAndKoNaturesCalledOnceForEveryCandidate(t *testing.T) {
	t.Parallel()

	for _, count := range []int{1, 2, 6} {
		t.Run(strconv.Itoa(count)+"件", func(t *testing.T) {
			t.Parallel()

			defenders := make([]map[string]any, 0, count)
			for i := 0; i < count; i++ {
				defenders = append(defenders, individual(defenderSpeciesKey, natureNeutralID, 0))
			}

			stub := &upstreams{}
			recorder := postOutspeed(newUpstreams(t, stub), bodyWithDefenders(defenders...), nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := len(decodeResponse(t, recorder).Matchups); got != count {
				t.Errorf("matchups の件数 = %d, want %d", got, count)
			}

			natures, speciesKeys, calcCalls := stub.counts()
			if natures != 1 {
				t.Errorf("natures を %d 回呼んでいる。候補が %d 件でも 1 回(ADR-0703 §1)", natures, count)
			}
			if len(speciesKeys) != count+1 {
				t.Errorf("species を %d 回呼んでいる。attacker 1 + 候補 %d = %d 回", len(speciesKeys), count, count+1)
			}
			if calcCalls != count {
				t.Errorf("calc を %d 回呼んでいる。候補の数だけ %d 回", calcCalls, count)
			}
		})
	}
}

// TestOutspeedAndKoRejectsDefenderCount: defenders は 1〜6 件(ADR-0703 §1・受け入れ条件3)。
// 0 件・7 件・配列でない・欄そのものが無い、はすべて 400 invalid_request で、上流を 1 回も呼ばない。
// JD2 までの単数の defender は契約から消えたので、未知の欄として弾かれる(破壊的変更。ADR-0703 §7)。
func TestOutspeedAndKoRejectsDefenderCount(t *testing.T) {
	t.Parallel()

	manyDefenders := func(count int) []any {
		list := make([]any, 0, count)
		for i := 0; i < count; i++ {
			list = append(list, individual(defenderSpeciesKey, natureNeutralID, 0))
		}
		return list
	}

	tests := []struct {
		name string
		body any
	}{
		{"defenders が空配列", func() any {
			body := validBody()
			body["defenders"] = []any{}
			return body
		}()},
		{"defenders が無い", func() any {
			body := validBody()
			delete(body, "defenders")
			return body
		}()},
		{"defenders が null", func() any {
			body := validBody()
			body["defenders"] = nil
			return body
		}()},
		{"defenders が 7 件", func() any {
			body := validBody()
			body["defenders"] = manyDefenders(7)
			return body
		}()},
		{"defenders が配列でない", func() any {
			body := validBody()
			body["defenders"] = individual(defenderSpeciesKey, natureNeutralID, 0)
			return body
		}()},
		{"JD2 までの単数の defender は受け付けない", func() any {
			body := validBody()
			delete(body, "defenders")
			body["defender"] = individual(defenderSpeciesKey, natureNeutralID, 0)
			return body
		}()},
		{"defenders と defender の両方", func() any {
			body := validBody()
			body["defender"] = individual(defenderSpeciesKey, natureNeutralID, 0)
			return body
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &upstreams{}
			recorder := postOutspeed(newUpstreams(t, stub), tt.body, nil)

			assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
			assertNoUpstreamCalls(t, stub)
		})
	}
}

// TestOutspeedAndKoRejectsOutOfRangeCandidate: 候補の sp / ranks の範囲検査も上流より先で、
// **最初に範囲外だった候補の index** でエラーになる(ADR-0703 §4・受け入れ条件3)。
func TestOutspeedAndKoRejectsOutOfRangeCandidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(map[string]any)
		wantIndex int
	}{
		{"2 番目の候補の sp が 32 超", func(body map[string]any) {
			defenderAt(body, 1)["sp"] = sp(33)
		}, 1},
		{"3 番目の候補の ranks が範囲外", func(body map[string]any) {
			defenderAt(body, 2)["ranks"] = map[string]any{"spe": 7}
		}, 2},
		{"複数が範囲外なら最初の候補", func(body map[string]any) {
			defenderAt(body, 1)["sp"] = sp(33)
			defenderAt(body, 2)["sp"] = sp(33)
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := bodyWithDefenders(
				individual(defenderSpeciesKey, natureNeutralID, 0),
				individual(defender2SpeciesKey, natureNeutralID, 0),
				individual(defender3SpeciesKey, natureNeutralID, 0),
			)
			tt.mutate(body)

			stub := &upstreams{}
			recorder := postOutspeed(newUpstreams(t, stub), body, nil)

			assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
			assertBlamesCandidate(t, recorder, tt.wantIndex)
			assertNoUpstreamCalls(t, stub)
		})
	}
}

// TestOutspeedAndKoStopsAtFirstFailingCandidate: 1 つの候補で失敗したら request 全体を
// 打ち切り、部分的な成功は返さない(ADR-0703 §3・受け入れ条件5)。
// それ以降の候補の種族・calc は呼ばない。
func TestOutspeedAndKoStopsAtFirstFailingCandidate(t *testing.T) {
	t.Parallel()

	threeDefenders := func() map[string]any {
		return bodyWithDefenders(
			individual(defenderSpeciesKey, natureNeutralID, 0),
			individual(defender2SpeciesKey, natureNeutralID, 0),
			individual(defender3SpeciesKey, natureNeutralID, 0),
		)
	}

	t.Run("2 番目の候補の種族がマスタに無い", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			speciesFail: map[string]stubResponse{
				defender2SpeciesKey: {http.StatusNotFound, `{"code":"not_found","message":"no such species"}`},
				defender3SpeciesKey: {http.StatusNotFound, `{"code":"not_found","message":"no such species"}`},
			},
		}
		recorder := postOutspeed(newUpstreams(t, stub), threeDefenders(), nil)

		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownSpecies)
		assertBlamesCandidate(t, recorder, 1)

		_, speciesKeys, calcCalls := stub.counts()
		wantSpecies := []string{attackerSpeciesKey, defenderSpeciesKey, defender2SpeciesKey}
		if !reflect.DeepEqual(speciesKeys, wantSpecies) {
			t.Errorf("species の呼び出し = %v, want %v(3 番目の候補は引かない)", speciesKeys, wantSpecies)
		}
		if calcCalls != 0 {
			t.Errorf("calc を %d 回呼んでいる。種族が揃う前に計算しない(ADR-0703 §4)", calcCalls)
		}
	})

	t.Run("2 番目の候補の種族が 500", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			speciesFail: map[string]stubResponse{
				defender2SpeciesKey: {http.StatusInternalServerError, `{"code":"internal","message":"boom"}`},
			},
		}
		recorder := postOutspeed(newUpstreams(t, stub), threeDefenders(), nil)

		assertStatusAndCode(t, recorder, http.StatusServiceUnavailable, api.UpstreamUnavailable)
		assertNoUpstreamDetail(t, recorder.Body.String(), stub.pokedexURL, stub.calcURL)

		_, speciesKeys, calcCalls := stub.counts()
		if len(speciesKeys) != 3 || calcCalls != 0 {
			t.Errorf("species=%v calc=%d。最初に失敗した候補で打ち切る(ADR-0703 §3)", speciesKeys, calcCalls)
		}
	})

	t.Run("2 番目の候補の計算を calc が 400 で拒否する", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			calcFail: map[string]stubResponse{
				defender2SpeciesKey: {http.StatusBadRequest, `{"code":"unknown_move","message":"no such move"}`},
			},
		}
		recorder := postOutspeed(newUpstreams(t, stub), threeDefenders(), nil)

		assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
		assertBlamesCandidate(t, recorder, 1)
		assertNoUpstreamDetail(t, recorder.Body.String(), stub.pokedexURL, stub.calcURL)

		// 種族は 4 件すべて引いた後に calc へ進む(ADR-0703 §4 の 2 段階)。
		_, speciesKeys, calcCalls := stub.counts()
		if len(speciesKeys) != 4 {
			t.Errorf("species を %d 回呼んでいる。attacker 1 + 候補 3 = 4 回", len(speciesKeys))
		}
		if calcCalls != 2 {
			t.Errorf("calc を %d 回呼んでいる。2 番目で打ち切るので 2 回(3 番目は呼ばない)", calcCalls)
		}
		wantCalcOrder := []string{defenderSpeciesKey, defender2SpeciesKey}
		if !reflect.DeepEqual(stub.calcDefenderKeys(), wantCalcOrder) {
			t.Errorf("calc の defender = %v, want %v", stub.calcDefenderKeys(), wantCalcOrder)
		}
	})

	t.Run("2 番目の候補の計算で calc が 503", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			calcFail: map[string]stubResponse{
				defender2SpeciesKey: {http.StatusServiceUnavailable, `{"code":"master_unavailable","message":"no master"}`},
			},
		}
		recorder := postOutspeed(newUpstreams(t, stub), threeDefenders(), nil)

		assertStatusAndCode(t, recorder, http.StatusServiceUnavailable, api.UpstreamUnavailable)
		if _, _, calcCalls := stub.counts(); calcCalls != 2 {
			t.Errorf("calc を %d 回呼んでいる。2 番目で打ち切るので 2 回", calcCalls)
		}
	})

	t.Run("エラーのときは部分的な結果を返さない", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			calcFail: map[string]stubResponse{
				defender3SpeciesKey: {http.StatusBadRequest, `{"code":"unknown_move","message":"no such move"}`},
			},
		}
		recorder := postOutspeed(newUpstreams(t, stub), threeDefenders(), nil)

		assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
		assertBlamesCandidate(t, recorder, 2)

		var raw map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &raw); err != nil {
			t.Fatalf("body をデコードできない: %v", err)
		}
		if value, present := raw["matchups"]; present {
			t.Errorf("エラー応答に matchups = %v を返している。部分的な成功は返さない(ADR-0703 §3)", value)
		}
	})
}

// TestOutspeedAndKoSpeedFieldAppliesToEveryCandidate: speedField は 1 リクエストに 1 つで、
// すべての候補に同じように適用される(ADR-0703 §5・受け入れ条件6)。
// attackerSpeed はどの matchup でも同じ値になる(ADR-0703 §2)。
func TestOutspeedAndKoSpeedFieldAppliesToEveryCandidate(t *testing.T) {
	t.Parallel()

	newBody := func() map[string]any {
		body := bodyWithDefenders(
			individual(defender2SpeciesKey, natureNeutralID, 0), // 種族値 90 → 110
			individual(defenderSpeciesKey, natureNeutralID, 0),  // 種族値 100 → 120
			individual(defender3SpeciesKey, natureNeutralID, 0), // 種族値 130 → 150
		)
		attackerOf(body)["natureId"] = natureNeutralID
		attackerOf(body)["sp"] = sp(0) // 種族値 100 → 120
		return body
	}
	baseSpeeds := map[string]int{
		attackerSpeciesKey:  100,
		defender2SpeciesKey: 90,
		defenderSpeciesKey:  100,
		defender3SpeciesKey: 130,
	}

	t.Run("追い風は両側とも全候補に効く", func(t *testing.T) {
		t.Parallel()

		body := newBody()
		body["speedField"] = map[string]any{"attackerTailwind": true, "defenderTailwind": true}

		recorder := postOutspeed(newUpstreams(t, &upstreams{baseSpeeds: baseSpeeds}), body, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}

		// attacker 120 → 240。候補 110 / 120 / 150 → 220 / 240 / 300。
		want := []api.Matchup{
			{DefenderIndex: 0, Outspeeds: true, AttackerSpeed: 240, DefenderSpeed: 220,
				Ko: api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100}},
			{DefenderIndex: 1, SpeedTie: true, AttackerSpeed: 240, DefenderSpeed: 240,
				Ko: api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100}},
			{DefenderIndex: 2, AttackerSpeed: 240, DefenderSpeed: 300,
				Ko: api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100}},
		}
		if got := decodeResponse(t, recorder).Matchups; !reflect.DeepEqual(got, want) {
			t.Errorf("matchups = %+v, want %+v", got, want)
		}
	})

	t.Run("トリックルームは全候補の比較の向きを反転する", func(t *testing.T) {
		t.Parallel()

		body := newBody()
		body["speedField"] = map[string]any{"trickRoom": true}

		recorder := postOutspeed(newUpstreams(t, &upstreams{baseSpeeds: baseSpeeds}), body, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}

		got := decodeResponse(t, recorder).Matchups
		if len(got) != 3 {
			t.Fatalf("matchups の件数 = %d, want 3", len(got))
		}
		// 実数値は変わらず(120 対 110 / 120 / 150)、向きだけ反転する。
		wantOutspeeds := []bool{false, false, true}
		wantSpeedTie := []bool{false, true, false}
		for i, matchup := range got {
			if matchup.AttackerSpeed != 120 {
				t.Errorf("matchups[%d].attackerSpeed = %d, want 120(全行で同じ・トリックルームで変わらない)", i, matchup.AttackerSpeed)
			}
			if matchup.Outspeeds != wantOutspeeds[i] || matchup.SpeedTie != wantSpeedTie[i] {
				t.Errorf("matchups[%d] outspeeds/speedTie = %v/%v, want %v/%v",
					i, matchup.Outspeeds, matchup.SpeedTie, wantOutspeeds[i], wantSpeedTie[i])
			}
		}
	})

	t.Run("attacker のスカーフは全候補に同じだけ乗る", func(t *testing.T) {
		t.Parallel()

		body := newBody()
		attackerOf(body)["itemId"] = "choicescarf"

		recorder := postOutspeed(newUpstreams(t, &upstreams{baseSpeeds: baseSpeeds}), body, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}

		got := decodeResponse(t, recorder).Matchups
		if len(got) != 3 {
			t.Fatalf("matchups の件数 = %d, want 3", len(got))
		}
		for i, matchup := range got {
			// 120 × 1.5 = 180(五捨五超入)。
			if matchup.AttackerSpeed != 180 {
				t.Errorf("matchups[%d].attackerSpeed = %d, want 180", i, matchup.AttackerSpeed)
			}
			if !matchup.Outspeeds || matchup.SpeedTie {
				t.Errorf("matchups[%d] outspeeds/speedTie = %v/%v, want true/false(180 は 110/120/150 をすべて抜く)",
					i, matchup.Outspeeds, matchup.SpeedTie)
			}
		}
	})
}
