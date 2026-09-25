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
// 技は既定ではどれも優先度 0 で、優先度を変えたいテストは upstreams.movePriorities で指定する
// (JD4。ADR-0704 §2)。
const (
	attackerSpeciesKey  = "9001-000"
	defenderSpeciesKey  = "9002-000"
	defender2SpeciesKey = "9003-000"
	defender3SpeciesKey = "9004-000"
	// testMoveID は attacker が使う技(request 直下の moveId)。
	testMoveID = "test-move"
	// defenderMoveID は相手候補が撃ち返してくる既定の技(JD4。ADR-0704 §1)。
	// 順方向(testMoveID)と逆方向(候補の技)を取り違えていないか見分けるため、必ず別の ID にする。
	defenderMoveID  = "test-defender-move"
	naturePlusSpeID = "test-plus-spe"
	natureNeutralID = "test-neutral"
)

// speciesBody は pokedex-svc の SpeciesDetail を模した架空の本文。
func speciesBody(key string, baseSpeed int) string {
	return `{"key":"` + key + `","dexNo":9001,"form":0,"nameJa":"テストポケモン",
	  "types":["fire"],"baseStats":{"hp":78,"atk":84,"def":78,"spa":109,"spd":85,"spe":` +
		strconv.Itoa(baseSpeed) + `},"abilities":[{"id":"test-ability","nameJa":"テストとくせい"}]}`
}

// moveBody は pokedex-svc の GET /api/pokedex/moves/{key}(Move)を模した架空の本文
// (JD4。ADR-0704 §9)。judge が読むのは id と priority だけ。
func moveBody(key string, priority int) string {
	return `{"id":"` + key + `","nameJa":"テストわざ","type":"fire","category":"physical",` +
		`"power":90,"priority":` + strconv.Itoa(priority) + `}`
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
	moveStatus    int
	moveBody      string
	calcStatus    int
	calcBody      string
	attackerSpeed int // attacker の種族の素早さ種族値(0 なら 100)
	defenderSpeed int // defenderSpeciesKey の素早さ種族値(0 なら 100)

	// JD3: 候補ごとの差し替え(ADR-0703)。キーは speciesKey。
	baseSpeeds  map[string]int          // speciesKey → 素早さ種族値
	speciesFail map[string]stubResponse // speciesKey → その種族の取得だけを失敗させる

	// JD4: 技ごとの差し替え(ADR-0704)。calcFail / calcKO は calcRoute(「技 → 防御側の種族」)で引く。
	// 同じ候補に対して順方向(自分の技)と逆方向(候補の技)の 2 回 calc を呼ぶので、
	// defender.speciesKey だけでは向きを区別できない。
	movePriorities map[string]int          // moveId → priority(未指定は 0)
	moveFail       map[string]stubResponse // moveId → その技の取得だけを失敗させる
	calcFail       map[string]stubResponse // calcRoute → その計算だけを失敗させる
	calcKO         map[string]string       // calcRoute → 返す ko の JSON

	// 記録。
	naturesCalls int
	speciesKeys  []string
	moveKeys     []string
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

// moveCalls は GET /api/pokedex/moves/{key} に渡された技の ID を呼ばれた順に返す
// (JD4: attacker の技 → 各候補の技を index 昇順。ADR-0704 §5)。
func (u *upstreams) moveCalls() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]string(nil), u.moveKeys...)
}

// calcRoutes は calc-svc に届いた計算要求を呼ばれた順に「技 → 防御側の種族」で返す
// (ADR-0703 §4: 候補は index 昇順。ADR-0704 §5: 候補ごとに順方向 → 逆方向)。
func (u *upstreams) calcRoutes() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	routes := make([]string, 0, len(u.calcBodies))
	for _, body := range u.calcBodies {
		routes = append(routes, calcRoute(body))
	}
	return routes
}

// calcBodyAt は i 番目の計算要求の body を返す(順方向・逆方向の中身を個別に見るため)。
func (u *upstreams) calcBodyAt(t *testing.T, i int) map[string]any {
	t.Helper()
	u.mu.Lock()
	defer u.mu.Unlock()
	if i >= len(u.calcBodies) {
		t.Fatalf("calc-svc は %d 回しか呼ばれていない(%d 回目を見ようとした)", len(u.calcBodies), i+1)
	}
	return u.calcBodies[i]
}

func calcDefenderSpeciesKey(body map[string]any) string {
	return calcSideSpeciesKey(body, "defender")
}

func calcSideSpeciesKey(body map[string]any, side string) string {
	individual, ok := body[side].(map[string]any)
	if !ok {
		return ""
	}
	key, _ := individual["speciesKey"].(string)
	return key
}

// calcRoute は 1 回の計算要求を「技 → 防御側の種族」で表す(JD4)。順方向は
// 「自分の技 → 候補の種族」、逆方向は「候補の技 → 自分の種族」になるので、
// この 1 本の文字列で候補と向きの両方を指せる。
func calcRoute(body map[string]any) string {
	moveID, _ := body["moveId"].(string)
	return moveID + "->" + calcDefenderSpeciesKey(body)
}

// forwardRoute は自分 → その候補(順方向。attackerKo を求める計算)の calcRoute。
func forwardRoute(defenderSpeciesKey string) string {
	return testMoveID + "->" + defenderSpeciesKey
}

// reverseRoute はその候補 → 自分(逆方向。defenderKo を求める計算)の calcRoute。
// 逆方向の防御側は常に attacker なので、候補の技 ID で一意に指せる(ADR-0704 §4)。
func reverseRoute(candidateMoveID string) string {
	return candidateMoveID + "->" + attackerSpeciesKey
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
		case strings.HasPrefix(r.URL.Path, "/api/pokedex/moves/"):
			key := strings.TrimPrefix(r.URL.Path, "/api/pokedex/moves/")
			u.moveKeys = append(u.moveKeys, key)
			if fail, ok := u.moveFail[key]; ok {
				writeStub(w, fail.status, fail.body)
				return
			}
			if u.moveBody != "" || u.moveStatus != 0 {
				writeStub(w, u.moveStatus, u.moveBody)
				return
			}
			writeStub(w, http.StatusOK, moveBody(key, u.movePriorities[key]))
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

		route := calcRoute(body)
		if fail, ok := u.calcFail[route]; ok {
			writeStub(w, fail.status, fail.body)
			return
		}
		if ko, ok := u.calcKO[route]; ok {
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

// individual は 1 個体ぶんの request の欄(Individual)。attacker に使う。
func individual(speciesKey, natureID string, spSpe int) map[string]any {
	return map[string]any{
		"speciesKey": speciesKey,
		"natureId":   natureID,
		"sp":         sp(spSpe),
	}
}

// candidate は相手候補 1 件ぶんの request の欄(DefenderCandidate)。JD4 から候補は
// **自分が撃ち返す技 moveId を必ず持つ**(ADR-0704 §1)。
func candidate(speciesKey, natureID string, spSpe int, moveID string) map[string]any {
	c := individual(speciesKey, natureID, spSpe)
	c["moveId"] = moveID
	return c
}

// validBody は 200 になる request body(相手候補 1 件)。attacker は最速(SP32・上昇補正)= 167、
// 候補は無振り無補正 = 120 で、attacker が抜ける。双方の技は優先度 0。
func validBody() map[string]any {
	return map[string]any{
		"format":    "single",
		"attacker":  individual(attackerSpeciesKey, naturePlusSpeID, 32),
		"defenders": []any{candidate(defenderSpeciesKey, natureNeutralID, 0, defenderMoveID)},
		"moveId":    testMoveID,
	}
}

// bodyWithDefenders は attacker / moveId はそのままに、相手候補だけを差し替える(JD3)。
// moveId を持たない候補には **候補ごとに違う架空の技**を補う(JD4)。候補全員が同じ技だと
// 逆方向の計算の取り違え(どの候補の技で自分が削られたか)が緑のまま通るため、
// 既定でも候補ごとに別の ID にしておく。技を明示したいテストは candidate(...) で渡す。
func bodyWithDefenders(defenders ...map[string]any) map[string]any {
	body := validBody()
	list := make([]any, 0, len(defenders))
	for i, d := range defenders {
		if _, present := d["moveId"]; !present {
			d["moveId"] = candidateMoveID(i)
		}
		list = append(list, d)
	}
	body["defenders"] = list
	return body
}

// candidateMoveID は i 番目の候補の既定の技 ID(架空)。
func candidateMoveID(i int) string {
	return defenderMoveID + "-" + strconv.Itoa(i)
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
	moveKeys := stub.moveCalls()
	if natures+len(speciesKeys)+len(moveKeys)+calcCalls != 0 {
		t.Errorf("上流を呼んでいる(natures=%d species=%v moves=%v calc=%d)。request の検査は上流より先(ADR-0701 §5)",
			natures, speciesKeys, moveKeys, calcCalls)
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
		DefenderIndex:        0,
		Outspeeds:            true,
		SpeedTie:             false,
		AttackerSpeed:        167,
		DefenderSpeed:        120,
		AttackerMovePriority: 0,
		DefenderMovePriority: 0,
		AttackerMovesFirst:   true,
		TurnOrderTie:         false,
		AttackerKo:           api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100},
		DefenderKo:           api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100},
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
	// JD4: 技の優先度も引く(attacker → 候補を index 昇順。ADR-0704 §5)。
	wantMoves := []string{testMoveID, defenderMoveID}
	if !reflect.DeepEqual(stub.moveCalls(), wantMoves) {
		t.Errorf("moves の呼び出し = %v, want %v(attacker の技 → 候補の技)", stub.moveCalls(), wantMoves)
	}
	if calcCalls != 2 {
		t.Errorf("calc-svc を %d 回呼んでいる。候補 1 件なら順方向 + 逆方向 = 2 回(ADR-0704 §4)", calcCalls)
	}

	// 端末 ID・セッション ID は呼び出し元のものをそのまま全ての上流へ転送する(ADR-0700 §2)。
	stub.mu.Lock()
	devices, sessions := stub.deviceIDs, stub.sessionIDs
	stub.mu.Unlock()
	if len(devices) != 7 {
		t.Errorf("上流を %d 回呼んでいる。natures 1 + species 2 + moves 2 + calc 2 = 7 回にする", len(devices))
	}
	for i := range devices {
		if devices[i] != testDeviceID || sessions[i] != testSessionID {
			t.Errorf("上流 %d 回目のヘッダー = (%q, %q), want (%q, %q)", i, devices[i], sessions[i], testDeviceID, testSessionID)
		}
	}

	// 順方向(1 回目)の calc へは judge の request をそのまま組み替えて渡す(1 候補 = 1 回の 1vs1 計算)。
	body := stub.calcBodyAt(t, 0)
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
	// calc には候補の moveId をそのまま混ぜない(候補の技は逆方向の moveId として送る。ADR-0704 §4)。
	if defender, ok := body["defender"].(map[string]any); ok {
		if value, present := defender["moveId"]; present {
			t.Errorf("calc の defender に moveId = %v を送っている。calc-svc の Individual に技の欄は無い", value)
		}
	}
	// 逆方向(2 回目)は役割が入れ替わる(ADR-0704 §4。詳細は TestOutspeedAndKoReverseCalc)。
	reverse := stub.calcBodyAt(t, 1)
	if reverse["moveId"] != defenderMoveID {
		t.Errorf("逆方向の calc の moveId = %v, want %s(候補の技)", reverse["moveId"], defenderMoveID)
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
			// 見るのは順方向(1 回目)の計算要求(逆方向では attacker / defender が入れ替わる)。
			calcBody := stub.calcBodyAt(t, 0)
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

// TestOutspeedAndKoTranscribesKO: attackerKo / defenderKo は calc-svc の値をそのまま転記する
// (judge は確定数を再計算しない。ADR-0701 受け入れ条件1・ADR-0704 §3)。
// **順方向と逆方向で違う値**を返すスタブで、2 つを取り違えていないことも確かめる。
// 画面に出さない chancePercent は返さない(ADR-0010)。
func TestOutspeedAndKoTranscribesKO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		forwardKO      string
		reverseKO      string
		wantAttackerKO api.KOChance
		wantDefenderKO api.KOChance
	}{
		{
			"自分は確定2発・相手は乱数1発",
			`{"hits":2,"guaranteed":true,"chancePercent":0,"displayChancePercent":100}`,
			`{"hits":1,"guaranteed":false,"chancePercent":87.4321,"displayChancePercent":87.4}`,
			api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100},
			api.KOChance{Hits: 1, Guaranteed: false, DisplayChancePercent: 87.4},
		},
		{
			"自分は倒せない・相手は確定1発(返り討ち)",
			`{"hits":0,"guaranteed":false,"chancePercent":0,"displayChancePercent":0}`,
			`{"hits":1,"guaranteed":true,"chancePercent":0,"displayChancePercent":100}`,
			api.KOChance{Hits: 0, Guaranteed: false, DisplayChancePercent: 0},
			api.KOChance{Hits: 1, Guaranteed: true, DisplayChancePercent: 100},
		},
		{
			"自分は確定1発・相手は倒せない",
			`{"hits":1,"guaranteed":true,"chancePercent":0,"displayChancePercent":100}`,
			`{"hits":0,"guaranteed":false,"chancePercent":0,"displayChancePercent":0}`,
			api.KOChance{Hits: 1, Guaranteed: true, DisplayChancePercent: 100},
			api.KOChance{Hits: 0, Guaranteed: false, DisplayChancePercent: 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := &upstreams{calcKO: map[string]string{
				forwardRoute(defenderSpeciesKey): tt.forwardKO,
				reverseRoute(defenderMoveID):     tt.reverseKO,
			}}
			recorder := postOutspeed(newUpstreams(t, stub), validBody(), nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			got := onlyMatchup(t, recorder)
			if got.AttackerKo != tt.wantAttackerKO {
				t.Errorf("attackerKo = %+v, want %+v(自分の技 → 候補)", got.AttackerKo, tt.wantAttackerKO)
			}
			if got.DefenderKo != tt.wantDefenderKO {
				t.Errorf("defenderKo = %+v, want %+v(候補の技 → 自分)", got.DefenderKo, tt.wantDefenderKO)
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
			// JD3 までの ko という欄は残っていない(ADR-0704 §3 の改名)。
			if value, present := raw.Matchups[0]["ko"]; present {
				t.Errorf("matchups に ko = %v が残っている。attackerKo に改名した(ADR-0704 §3)", value)
			}
			for _, key := range []string{"attackerKo", "defenderKo"} {
				ko, ok := raw.Matchups[0][key].(map[string]any)
				if !ok {
					t.Fatalf("%s = %v, want an object", key, raw.Matchups[0][key])
				}
				if _, present := ko["chancePercent"]; present {
					t.Errorf("%s に chancePercent を返している。画面に出す値ではない(ADR-0010)", key)
				}
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

		// 順方向(1 回目)はそのまま転送する。逆方向の壁の入れ替えは
		// TestOutspeedAndKoReverseCalcSwapsScreens で別に確かめる(ADR-0704 §4)。
		field, ok := stub.calcBodyAt(t, 0)["field"].(map[string]any)
		if !ok {
			t.Fatalf("calc の field = %v, want an object", stub.calcBodyAt(t, 0)["field"])
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
		// 順方向・逆方向のどちらにも送らない(ADR-0704 §4 は壁の入れ替えだけを決めた。
		// 省略された field を逆方向で作り出さない)。
		for i := 0; i < 2; i++ {
			if value, present := stub.calcBodyAt(t, i)["field"]; present {
				t.Errorf("calc %d 回目に field = %v を送っている。未指定なら送らない", i, value)
			}
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
		if len(bodies) != 4 {
			t.Fatalf("calc を %d 回呼んでいる。候補 2 件なら (順方向 + 逆方向) × 2 = 4 回", len(bodies))
		}
		// 天候は場全体の状態なので、候補にも向きにもよらず同じものが届く(ADR-0703 §5・ADR-0704 §4)。
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
		{"moveId が形式に合わない", func() any {
			body := validBody()
			body["moveId"] = "test move"
			return body
		}(), nil},
		{"speciesKey が形式に合わない", withAttacker(func(a map[string]any) { a["speciesKey"] = "pikachu" }), nil},
		{"natureId が空", withAttacker(func(a map[string]any) { a["natureId"] = "" }), nil},
		{"natureId が形式に合わない", withAttacker(func(a map[string]any) { a["natureId"] = "test/nature" }), nil},
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
		{"候補の moveId が形式に合わない", func() any {
			body := validBody()
			defenderAt(body, 0)["moveId"] = "test?move=1"
			return body
		}(), nil},
		{"候補の natureId が形式に合わない", func() any {
			body := validBody()
			defenderAt(body, 0)["natureId"] = "test nature"
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

		_, speciesKeys, calcCalls := stub.counts()
		if len(speciesKeys)+len(stub.moveCalls())+calcCalls != 0 {
			t.Error("性格を解決できていないのに種族・技・calc-svc を呼んでいる")
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

		_, speciesKeys, calcCalls := stub.counts()
		if len(speciesKeys)+len(stub.moveCalls())+calcCalls != 0 {
			t.Error("性格を解決できていないのに種族・技・calc-svc を呼んでいる")
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
	// attacker の種族で止まるので、技もまだ引いていない(ADR-0704 §5 の検査順)。
	if moves := stub.moveCalls(); len(moves) != 0 {
		t.Errorf("技を %v 回引いている。attacker の種族で打ち切る", moves)
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

		// 順方向・逆方向のどちらにも送らない(JD4 で計算が 2 本になっても変わらない)。
		for i := 0; i < 2; i++ {
			calc := stub.calcBodyAt(t, i)
			if value, present := calc["speedField"]; present {
				t.Errorf("calc %d 回目に speedField = %v を送っている。calc-svc は解釈できない(ADR-0702 §1)", i, value)
			}
			if value, present := calc["field"]; present {
				t.Errorf("calc %d 回目に field = %v を送っている。request で指定していない", i, value)
			}
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

		for i := 0; i < 2; i++ {
			calc := stub.calcBodyAt(t, i)
			if value, present := calc["speedField"]; present {
				t.Errorf("calc %d 回目に speedField = %v を送っている(ADR-0702 §1)", i, value)
			}
			field, ok := calc["field"].(map[string]any)
			if !ok {
				t.Fatalf("calc %d 回目の field = %v, want an object", i, calc["field"])
			}
			if field["weather"] != "sun" {
				t.Errorf("calc %d 回目の field.weather = %v, want sun(ADR-0701 受け入れ条件6 は変わらない)", i, field["weather"])
			}
			// トリックルーム・追い風が field に紛れ込んでいないこと。
			for _, key := range []string{"trickRoom", "attackerTailwind", "defenderTailwind"} {
				if value, present := field[key]; present {
					t.Errorf("calc %d 回目の field に %s = %v が混ざっている", i, key, value)
				}
			}
		}
	})
}

// TestOutspeedAndKoSpeedFieldOmittedMatchesJD1: speedField を省略した request の判定は JD1 と同一
// (ADR-0702 受け入れ条件5)。空オブジェクト・全欄 false も同じ。
func TestOutspeedAndKoSpeedFieldOmittedMatchesJD1(t *testing.T) {
	t.Parallel()

	// JD1 と同じ条件: attacker = 167、候補 = 120 で抜ける。
	// JD4 で足した欄は、どちらの技も優先度 0 なので素早さの結果と一致する(ADR-0704 §2)。
	want := api.Matchup{
		DefenderIndex:      0,
		Outspeeds:          true,
		SpeedTie:           false,
		AttackerSpeed:      167,
		DefenderSpeed:      120,
		AttackerMovesFirst: true,
		AttackerKo:         api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100},
		DefenderKo:         api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100},
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
		// 順方向(自分 → 各候補)と逆方向(各候補 → 自分)で、候補ごとに違う値を返す。
		// すべて同じ値だと、行の取り違えも向きの取り違えも緑のまま通る(ADR-0703・ADR-0704 テストの期待値)。
		calcKO: map[string]string{
			forwardRoute(defender2SpeciesKey): `{"hits":1,"guaranteed":true,"chancePercent":0,"displayChancePercent":100}`,
			forwardRoute(defenderSpeciesKey):  `{"hits":2,"guaranteed":false,"chancePercent":50,"displayChancePercent":50}`,
			forwardRoute(defender3SpeciesKey): `{"hits":0,"guaranteed":false,"chancePercent":0,"displayChancePercent":0}`,
			reverseRoute(candidateMoveID(0)):  `{"hits":3,"guaranteed":true,"chancePercent":0,"displayChancePercent":100}`,
			reverseRoute(candidateMoveID(1)):  `{"hits":4,"guaranteed":false,"chancePercent":25,"displayChancePercent":25}`,
			reverseRoute(candidateMoveID(2)):  `{"hits":1,"guaranteed":true,"chancePercent":0,"displayChancePercent":100}`,
		},
	}
	recorder := postOutspeed(newUpstreams(t, stub), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	// どちらの技も優先度 0 なので、attackerMovesFirst / turnOrderTie は素早さの結果と一致する。
	want := []api.Matchup{
		{
			DefenderIndex: 0, Outspeeds: true, SpeedTie: false,
			AttackerSpeed: 120, DefenderSpeed: 110,
			AttackerMovesFirst: true, TurnOrderTie: false,
			AttackerKo: api.KOChance{Hits: 1, Guaranteed: true, DisplayChancePercent: 100},
			DefenderKo: api.KOChance{Hits: 3, Guaranteed: true, DisplayChancePercent: 100},
		},
		{
			DefenderIndex: 1, Outspeeds: false, SpeedTie: true,
			AttackerSpeed: 120, DefenderSpeed: 120,
			AttackerMovesFirst: false, TurnOrderTie: true,
			AttackerKo: api.KOChance{Hits: 2, Guaranteed: false, DisplayChancePercent: 50},
			DefenderKo: api.KOChance{Hits: 4, Guaranteed: false, DisplayChancePercent: 25},
		},
		{
			DefenderIndex: 2, Outspeeds: false, SpeedTie: false,
			AttackerSpeed: 120, DefenderSpeed: 150,
			AttackerMovesFirst: false, TurnOrderTie: false,
			AttackerKo: api.KOChance{Hits: 0, Guaranteed: false, DisplayChancePercent: 0},
			DefenderKo: api.KOChance{Hits: 1, Guaranteed: true, DisplayChancePercent: 100},
		},
	}
	got := decodeResponse(t, recorder).Matchups
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matchups = %+v, want %+v", got, want)
	}

	// 上流は natures 1 + 種族 (1+3) + 技 (1+3) + calc 2×3(ADR-0703 §1・ADR-0704 §5)。
	natures, speciesKeys, calcCalls := stub.counts()
	if natures != 1 {
		t.Errorf("natures を %d 回呼んでいる。候補が増えても 1 回(ADR-0703 §1)", natures)
	}
	wantSpecies := []string{attackerSpeciesKey, defender2SpeciesKey, defenderSpeciesKey, defender3SpeciesKey}
	if !reflect.DeepEqual(speciesKeys, wantSpecies) {
		t.Errorf("species の呼び出し = %v, want %v(attacker → 候補を index 昇順。ADR-0703 §4)", speciesKeys, wantSpecies)
	}
	wantMoves := []string{testMoveID, candidateMoveID(0), candidateMoveID(1), candidateMoveID(2)}
	if !reflect.DeepEqual(stub.moveCalls(), wantMoves) {
		t.Errorf("moves の呼び出し = %v, want %v(attacker の技 → 候補の技を index 昇順。ADR-0704 §5)", stub.moveCalls(), wantMoves)
	}
	if calcCalls != 6 {
		t.Errorf("calc を %d 回呼んでいる。候補 3 件なら (順方向 + 逆方向) × 3 = 6 回", calcCalls)
	}
	wantCalcOrder := []string{
		forwardRoute(defender2SpeciesKey), reverseRoute(candidateMoveID(0)),
		forwardRoute(defenderSpeciesKey), reverseRoute(candidateMoveID(1)),
		forwardRoute(defender3SpeciesKey), reverseRoute(candidateMoveID(2)),
	}
	if !reflect.DeepEqual(stub.calcRoutes(), wantCalcOrder) {
		t.Errorf("calc の呼び出し = %v, want %v(index 昇順・候補ごとに順方向 → 逆方向。ADR-0704 §5)",
			stub.calcRoutes(), wantCalcOrder)
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
	if calcCalls != 4 {
		t.Errorf("calc を %d 回呼んでいる。候補 2 件なら (順方向 + 逆方向) × 2 = 4 回(調整が違えば結果も違う)", calcCalls)
	}
	// 技も同じで、候補の数だけ素直に引く(同じ moveId でもまとめない。ADR-0704 却下した案)。
	wantMoves := []string{testMoveID, candidateMoveID(0), candidateMoveID(1)}
	if !reflect.DeepEqual(stub.moveCalls(), wantMoves) {
		t.Errorf("moves の呼び出し = %v, want %v", stub.moveCalls(), wantMoves)
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
			if moves := stub.moveCalls(); len(moves) != count+1 {
				t.Errorf("moves を %d 回呼んでいる。attacker の技 1 + 候補 %d = %d 回(ADR-0704 §5)",
					len(moves), count, count+1)
			}
			if calcCalls != 2*count {
				t.Errorf("calc を %d 回呼んでいる。候補の数 × (順方向 + 逆方向) = %d 回", calcCalls, 2*count)
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
		// 技は「その候補の種族 → その候補の技」の順なので、失敗した候補の技は引かない(ADR-0704 §5)。
		wantMoves := []string{testMoveID, candidateMoveID(0)}
		if !reflect.DeepEqual(stub.moveCalls(), wantMoves) {
			t.Errorf("moves の呼び出し = %v, want %v(種族で失敗した候補の技は引かない)", stub.moveCalls(), wantMoves)
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
				forwardRoute(defender2SpeciesKey): {http.StatusBadRequest, `{"code":"unknown_move","message":"no such move"}`},
			},
		}
		recorder := postOutspeed(newUpstreams(t, stub), threeDefenders(), nil)

		assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
		assertBlamesCandidate(t, recorder, 1)
		assertNoUpstreamDetail(t, recorder.Body.String(), stub.pokedexURL, stub.calcURL)

		// 種族も技も 4 件すべて引いた後に calc へ進む(ADR-0703 §4 の 2 段階。ADR-0704 §5)。
		_, speciesKeys, calcCalls := stub.counts()
		if len(speciesKeys) != 4 {
			t.Errorf("species を %d 回呼んでいる。attacker 1 + 候補 3 = 4 回", len(speciesKeys))
		}
		if moves := stub.moveCalls(); len(moves) != 4 {
			t.Errorf("moves を %d 回呼んでいる。attacker の技 1 + 候補 3 = 4 回", len(moves))
		}
		if calcCalls != 3 {
			t.Errorf("calc を %d 回呼んでいる。候補 0 の 2 本 + 候補 1 の順方向で打ち切るので 3 回", calcCalls)
		}
		// 候補 1 の順方向で失敗するので、その逆方向も候補 2 も呼ばない(ADR-0703 §3)。
		wantCalcOrder := []string{
			forwardRoute(defenderSpeciesKey), reverseRoute(candidateMoveID(0)),
			forwardRoute(defender2SpeciesKey),
		}
		if !reflect.DeepEqual(stub.calcRoutes(), wantCalcOrder) {
			t.Errorf("calc の呼び出し = %v, want %v", stub.calcRoutes(), wantCalcOrder)
		}
	})

	t.Run("2 番目の候補の逆方向の計算を calc が 400 で拒否する", func(t *testing.T) {
		t.Parallel()

		// 逆方向(候補の技 → 自分)だけを失敗させる。順方向は成功しているので、
		// 「順方向さえ通れば候補を成功扱いにする」実装だと落ちる(ADR-0704 §4)。
		stub := &upstreams{
			calcFail: map[string]stubResponse{
				reverseRoute(candidateMoveID(1)): {http.StatusBadRequest, `{"code":"unknown_move","message":"no such move"}`},
			},
		}
		recorder := postOutspeed(newUpstreams(t, stub), threeDefenders(), nil)

		assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
		assertBlamesCandidate(t, recorder, 1)
		assertNoUpstreamDetail(t, recorder.Body.String(), stub.pokedexURL, stub.calcURL)

		wantCalcOrder := []string{
			forwardRoute(defenderSpeciesKey), reverseRoute(candidateMoveID(0)),
			forwardRoute(defender2SpeciesKey), reverseRoute(candidateMoveID(1)),
		}
		if !reflect.DeepEqual(stub.calcRoutes(), wantCalcOrder) {
			t.Errorf("calc の呼び出し = %v, want %v(逆方向の失敗でも 3 番目の候補は計算しない)",
				stub.calcRoutes(), wantCalcOrder)
		}
	})

	t.Run("2 番目の候補の計算で calc が 503", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			calcFail: map[string]stubResponse{
				forwardRoute(defender2SpeciesKey): {http.StatusServiceUnavailable, `{"code":"master_unavailable","message":"no master"}`},
			},
		}
		recorder := postOutspeed(newUpstreams(t, stub), threeDefenders(), nil)

		assertStatusAndCode(t, recorder, http.StatusServiceUnavailable, api.UpstreamUnavailable)
		if _, _, calcCalls := stub.counts(); calcCalls != 3 {
			t.Errorf("calc を %d 回呼んでいる。候補 1 の順方向で打ち切るので 3 回", calcCalls)
		}
	})

	t.Run("エラーのときは部分的な結果を返さない", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			calcFail: map[string]stubResponse{
				forwardRoute(defender3SpeciesKey): {http.StatusBadRequest, `{"code":"unknown_move","message":"no such move"}`},
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
		// 技はすべて優先度 0 なので、行動順は素早さの結果と一致する。
		defaultKO := api.KOChance{Hits: 2, Guaranteed: true, DisplayChancePercent: 100}
		want := []api.Matchup{
			{DefenderIndex: 0, Outspeeds: true, AttackerSpeed: 240, DefenderSpeed: 220,
				AttackerMovesFirst: true, AttackerKo: defaultKO, DefenderKo: defaultKO},
			{DefenderIndex: 1, SpeedTie: true, AttackerSpeed: 240, DefenderSpeed: 240,
				TurnOrderTie: true, AttackerKo: defaultKO, DefenderKo: defaultKO},
			{DefenderIndex: 2, AttackerSpeed: 240, DefenderSpeed: 300,
				AttackerKo: defaultKO, DefenderKo: defaultKO},
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

// --- JD4: 相手の技を含めた返り討ち判定。ADR-0704 ---

// screenFlag は calc-svc に届いた field の screens の 1 欄を読む。judge のクライアントは
// false の欄を送らない(omitempty)ので、欄が無い = false として扱う。
func screenFlag(t *testing.T, field map[string]any, side, key string) bool {
	t.Helper()
	screens, ok := field[side].(map[string]any)
	if !ok {
		return false
	}
	value, _ := screens[key].(bool)
	return value
}

func calcField(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	field, ok := body["field"].(map[string]any)
	if !ok {
		t.Fatalf("calc の field = %v, want an object", body["field"])
	}
	return field
}

// TestOutspeedAndKoReverseCalcSwapsRoles: 逆方向の計算は「その候補が攻撃側・自分が防御側」で、
// 技はその候補の moveId になる(ADR-0704 §4・受け入れ条件4)。順方向と役割が入れ替わるだけで、
// 個体の中身(調整・持ち物・特性)はそのまま運ばれる。
func TestOutspeedAndKoReverseCalcSwapsRoles(t *testing.T) {
	t.Parallel()

	body := validBody()
	attacker := attackerOf(body)
	attacker["itemId"] = "test-attacker-item"
	attacker["abilityId"] = "test-attacker-ability"
	defender := defenderAt(body, 0)
	defender["itemId"] = "test-defender-item"
	defender["abilityId"] = "test-defender-ability"
	defender["ranks"] = map[string]any{"atk": 2}

	stub := &upstreams{}
	recorder := postOutspeed(newUpstreams(t, stub), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	forward := stub.calcBodyAt(t, 0)
	reverse := stub.calcBodyAt(t, 1)

	if forward["moveId"] != testMoveID {
		t.Errorf("順方向の moveId = %v, want %s(自分の技)", forward["moveId"], testMoveID)
	}
	if reverse["moveId"] != defenderMoveID {
		t.Errorf("逆方向の moveId = %v, want %s(候補の技)", reverse["moveId"], defenderMoveID)
	}
	if reverse["format"] != "single" {
		t.Errorf("逆方向の format = %v, want single(向きで変わらない)", reverse["format"])
	}

	reverseAttacker, ok := reverse["attacker"].(map[string]any)
	if !ok {
		t.Fatalf("逆方向の attacker = %v, want an object", reverse["attacker"])
	}
	if reverseAttacker["speciesKey"] != defenderSpeciesKey {
		t.Errorf("逆方向の attacker.speciesKey = %v, want %s(候補が攻撃側になる)",
			reverseAttacker["speciesKey"], defenderSpeciesKey)
	}
	if reverseAttacker["itemId"] != "test-defender-item" || reverseAttacker["abilityId"] != "test-defender-ability" {
		t.Errorf("逆方向の attacker = %v, want 候補の持ち物・特性", reverseAttacker)
	}
	ranks, ok := reverseAttacker["ranks"].(map[string]any)
	if !ok || ranks["atk"] != float64(2) {
		t.Errorf("逆方向の attacker.ranks = %v, want atk=2(候補のランクを運ぶ)", reverseAttacker["ranks"])
	}

	reverseDefender, ok := reverse["defender"].(map[string]any)
	if !ok {
		t.Fatalf("逆方向の defender = %v, want an object", reverse["defender"])
	}
	if reverseDefender["speciesKey"] != attackerSpeciesKey {
		t.Errorf("逆方向の defender.speciesKey = %v, want %s(自分が防御側になる)",
			reverseDefender["speciesKey"], attackerSpeciesKey)
	}
	if reverseDefender["itemId"] != "test-attacker-item" || reverseDefender["abilityId"] != "test-attacker-ability" {
		t.Errorf("逆方向の defender = %v, want 自分の持ち物・特性", reverseDefender)
	}
	// 候補の技は Individual の欄としては送らない(calc-svc の Individual に技の欄は無い)。
	if value, present := reverseAttacker["moveId"]; present {
		t.Errorf("逆方向の attacker に moveId = %v を送っている。技は body 直下の moveId で表す", value)
	}
}

// TestOutspeedAndKoReverseCalcSwapsScreens: 壁は「どちらの側に張られているか」を表すので、
// 逆方向の計算では attackerScreens と defenderScreens を入れ替えて送る(ADR-0704 §4・受け入れ条件4)。
// 天候・地形は場全体の状態なので入れ替えない。入れ替えを忘れると、自分の壁が相手を守る
// 計算になり、エラーも出ないまま結果だけが静かに間違う。
func TestOutspeedAndKoReverseCalcSwapsScreens(t *testing.T) {
	t.Parallel()

	t.Run("両側に違う壁があるとき入れ替わる", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		body["field"] = map[string]any{
			"weather":         "sun",
			"terrain":         "grassy",
			"attackerScreens": map[string]any{"reflect": true},
			"defenderScreens": map[string]any{"lightScreen": true, "auroraVeil": true},
		}

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}

		forward := calcField(t, stub.calcBodyAt(t, 0))
		if !screenFlag(t, forward, "attackerScreens", "reflect") {
			t.Errorf("順方向の field.attackerScreens = %v, want reflect=true(そのまま転送する)", forward["attackerScreens"])
		}
		if screenFlag(t, forward, "attackerScreens", "lightScreen") {
			t.Errorf("順方向の field.attackerScreens = %v に相手側の壁が混ざっている", forward["attackerScreens"])
		}
		if !screenFlag(t, forward, "defenderScreens", "lightScreen") || !screenFlag(t, forward, "defenderScreens", "auroraVeil") {
			t.Errorf("順方向の field.defenderScreens = %v, want lightScreen=true auroraVeil=true", forward["defenderScreens"])
		}

		reverse := calcField(t, stub.calcBodyAt(t, 1))
		if !screenFlag(t, reverse, "defenderScreens", "reflect") {
			t.Errorf("逆方向の field.defenderScreens = %v, want reflect=true(自分の側の壁は防御側に回る。ADR-0704 §4)",
				reverse["defenderScreens"])
		}
		if screenFlag(t, reverse, "attackerScreens", "reflect") {
			t.Errorf("逆方向の field.attackerScreens = %v に自分の壁が残っている。入れ替えていない(ADR-0704 §4)",
				reverse["attackerScreens"])
		}
		if !screenFlag(t, reverse, "attackerScreens", "lightScreen") || !screenFlag(t, reverse, "attackerScreens", "auroraVeil") {
			t.Errorf("逆方向の field.attackerScreens = %v, want lightScreen=true auroraVeil=true(相手側の壁が攻撃側に回る)",
				reverse["attackerScreens"])
		}
		// 天候・地形は場全体の状態なので向きで変わらない。
		if reverse["weather"] != "sun" || reverse["terrain"] != "grassy" {
			t.Errorf("逆方向の field = %v, want weather=sun terrain=grassy(入れ替えない)", reverse)
		}
	})

	t.Run("片側だけの壁も向こう側に移る", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		body["field"] = map[string]any{"attackerScreens": map[string]any{"reflect": true}}

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}

		reverse := calcField(t, stub.calcBodyAt(t, 1))
		if !screenFlag(t, reverse, "defenderScreens", "reflect") {
			t.Errorf("逆方向の field.defenderScreens = %v, want reflect=true", reverse["defenderScreens"])
		}
		if screenFlag(t, reverse, "attackerScreens", "reflect") {
			t.Errorf("逆方向の field.attackerScreens = %v。指定の無かった側に壁を作らない", reverse["attackerScreens"])
		}
	})
}

// TestOutspeedAndKoTurnOrder: 先に動く側は **優先度が違えば優先度が高い方**で、素早さも
// トリックルームも見ない。優先度が同じときだけ素早さ(outspeeds)で決まり、優先度も素早さも
// 同じなら turnOrderTie(ADR-0704 §2・受け入れ条件3)。
// attacker は種族値 100・無振り無補正 = 120 に固定し、候補の種族値で速さを作る。
func TestOutspeedAndKoTurnOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		attackerPriority  int
		defenderPriority  int
		defenderBaseSpeed int // 90 → 110(自分が速い) / 100 → 120(同速) / 130 → 150(自分が遅い)
		trickRoom         bool
		wantMovesFirst    bool
		wantTurnOrderTie  bool
		wantOutspeeds     bool
		wantSpeedTie      bool
	}{
		{"同優先度・自分が速い", 0, 0, 90, false, true, false, true, false},
		{"同優先度・自分が遅い", 0, 0, 130, false, false, false, false, false},
		{"同優先度・同速は turnOrderTie", 0, 0, 100, false, false, true, false, true},
		// JD4 の主眼: 素早さで負けていても先制技なら先に動く。
		{"遅くても先制技なら先に動く", 1, 0, 130, false, true, false, false, false},
		{"速くても相手が先制技なら後になる", 0, 1, 90, false, false, false, true, false},
		// 優先度で決まるなら同速でも tie にならない(speedTie は true のまま)。
		{"同速でも先制技なら先に動く", 1, 0, 100, false, true, false, false, true},
		{"同速でも相手が先制技なら後になる", 0, 1, 100, false, false, false, false, true},
		// トリックルームは優先度に影響しない(ADR-0704 §2)。
		{"トリックルーム中でも先制技が勝つ", 1, 0, 90, true, true, false, false, false},
		{"トリックルーム中・同優先度なら遅い方が先", 0, 0, 90, true, false, false, false, false},
		{"トリックルーム中・同優先度で自分が遅ければ先に動く", 0, 0, 130, true, true, false, true, false},
		{"トリックルーム中でも相手の先制技が勝つ", 0, 1, 130, true, false, false, true, false},
		// 優先度 0 を特別扱いしない。
		{"負の優先度どうしでも高い方が先", -1, -6, 130, false, true, false, false, false},
		{"自分だけ後攻技(優先度 -6)なら速くても後", -6, 0, 90, false, false, false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := bodyWithDefenders(candidate(defenderSpeciesKey, natureNeutralID, 0, defenderMoveID))
			attackerOf(body)["natureId"] = natureNeutralID
			attackerOf(body)["sp"] = sp(0)
			if tt.trickRoom {
				body["speedField"] = map[string]any{"trickRoom": true}
			}

			stub := &upstreams{
				baseSpeeds: map[string]int{
					attackerSpeciesKey: 100,
					defenderSpeciesKey: tt.defenderBaseSpeed,
				},
				movePriorities: map[string]int{
					testMoveID:     tt.attackerPriority,
					defenderMoveID: tt.defenderPriority,
				},
			}
			recorder := postOutspeed(newUpstreams(t, stub), body, nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}

			got := onlyMatchup(t, recorder)
			if got.AttackerMovesFirst != tt.wantMovesFirst || got.TurnOrderTie != tt.wantTurnOrderTie {
				t.Errorf("attackerMovesFirst/turnOrderTie = %v/%v, want %v/%v",
					got.AttackerMovesFirst, got.TurnOrderTie, tt.wantMovesFirst, tt.wantTurnOrderTie)
			}
			if got.AttackerMovesFirst && got.TurnOrderTie {
				t.Error("attackerMovesFirst と turnOrderTie が同時に true になっている(ADR-0704 §2)")
			}
			// 素早さの比較(JD1〜JD3 の意味)は優先度では変わらない。
			if got.Outspeeds != tt.wantOutspeeds || got.SpeedTie != tt.wantSpeedTie {
				t.Errorf("outspeeds/speedTie = %v/%v, want %v/%v(素早さの比較は優先度で変わらない)",
					got.Outspeeds, got.SpeedTie, tt.wantOutspeeds, tt.wantSpeedTie)
			}
			// 優先度はそのまま転記する(画面が「なぜ先に動くのか」を出せるように)。
			if got.AttackerMovePriority != tt.attackerPriority || got.DefenderMovePriority != tt.defenderPriority {
				t.Errorf("attackerMovePriority/defenderMovePriority = %d/%d, want %d/%d",
					got.AttackerMovePriority, got.DefenderMovePriority, tt.attackerPriority, tt.defenderPriority)
			}
		})
	}
}

// TestOutspeedAndKoTurnOrderPerCandidate: 優先度は候補ごとに違う(候補ごとに技が違う)。
// 攻撃側の技は 1 つなので attackerMovePriority は全行で同じ値になる。
func TestOutspeedAndKoTurnOrderPerCandidate(t *testing.T) {
	t.Parallel()

	body := bodyWithDefenders(
		candidate(defenderSpeciesKey, natureNeutralID, 0, "test-move-priority-plus"),
		candidate(defender2SpeciesKey, natureNeutralID, 0, "test-move-priority-zero"),
		candidate(defender3SpeciesKey, natureNeutralID, 0, "test-move-priority-minus"),
	)
	attackerOf(body)["natureId"] = natureNeutralID
	attackerOf(body)["sp"] = sp(0)

	stub := &upstreams{
		// すべて同じ種族値 100(= 120)にして、差が優先度だけから出るようにする。
		baseSpeeds: map[string]int{
			attackerSpeciesKey: 100, defenderSpeciesKey: 100,
			defender2SpeciesKey: 100, defender3SpeciesKey: 100,
		},
		movePriorities: map[string]int{
			testMoveID:                 0,
			"test-move-priority-plus":  1,
			"test-move-priority-zero":  0,
			"test-move-priority-minus": -1,
		},
	}
	recorder := postOutspeed(newUpstreams(t, stub), body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	got := decodeResponse(t, recorder).Matchups
	if len(got) != 3 {
		t.Fatalf("matchups の件数 = %d, want 3", len(got))
	}
	// 全員同速(120 対 120)なので、行動順は優先度だけで決まる。
	wantFirst := []bool{false, false, true}
	wantTie := []bool{false, true, false}
	wantDefenderPriority := []int{1, 0, -1}
	for i, matchup := range got {
		if matchup.AttackerMovePriority != 0 {
			t.Errorf("matchups[%d].attackerMovePriority = %d, want 0(攻撃側の技は 1 つ)", i, matchup.AttackerMovePriority)
		}
		if matchup.DefenderMovePriority != wantDefenderPriority[i] {
			t.Errorf("matchups[%d].defenderMovePriority = %d, want %d", i, matchup.DefenderMovePriority, wantDefenderPriority[i])
		}
		if matchup.AttackerMovesFirst != wantFirst[i] || matchup.TurnOrderTie != wantTie[i] {
			t.Errorf("matchups[%d] attackerMovesFirst/turnOrderTie = %v/%v, want %v/%v",
				i, matchup.AttackerMovesFirst, matchup.TurnOrderTie, wantFirst[i], wantTie[i])
		}
		// 素早さは全員同速なので、速さの欄は 3 行とも同じ(優先度の影響を受けない)。
		if !matchup.SpeedTie || matchup.Outspeeds {
			t.Errorf("matchups[%d] outspeeds/speedTie = %v/%v, want false/true", i, matchup.Outspeeds, matchup.SpeedTie)
		}
	}
}

// TestOutspeedAndKoRejectsMissingCandidateMove: 候補の moveId は必須(ADR-0704 §1・受け入れ条件1)。
// 無い・空・文字列でない request は上流を 1 回も呼ばずに 400 で、message は最初に不正だった候補を示す。
func TestOutspeedAndKoRejectsMissingCandidateMove(t *testing.T) {
	t.Parallel()

	threeCandidates := func() map[string]any {
		return bodyWithDefenders(
			candidate(defenderSpeciesKey, natureNeutralID, 0, candidateMoveID(0)),
			candidate(defender2SpeciesKey, natureNeutralID, 0, candidateMoveID(1)),
			candidate(defender3SpeciesKey, natureNeutralID, 0, candidateMoveID(2)),
		)
	}

	tests := []struct {
		name      string
		mutate    func(map[string]any)
		wantIndex int
	}{
		{"2 番目の候補に moveId が無い", func(body map[string]any) {
			delete(defenderAt(body, 1), "moveId")
		}, 1},
		{"3 番目の候補の moveId が空", func(body map[string]any) {
			defenderAt(body, 2)["moveId"] = ""
		}, 2},
		{"候補の moveId が文字列でない", func(body map[string]any) {
			defenderAt(body, 0)["moveId"] = 1
		}, 0},
		{"複数が不正なら最初の候補", func(body map[string]any) {
			delete(defenderAt(body, 1), "moveId")
			delete(defenderAt(body, 2), "moveId")
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := threeCandidates()
			tt.mutate(body)

			stub := &upstreams{}
			recorder := postOutspeed(newUpstreams(t, stub), body, nil)

			assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
			assertBlamesCandidate(t, recorder, tt.wantIndex)
			assertNoUpstreamCalls(t, stub)
		})
	}
}

// TestOutspeedAndKoRejectsUnknownCandidateField: 候補の欄は DefenderCandidate のものだけ。
// defenders の要素は json.RawMessage で受けるため外側の DisallowUnknownFields() は届かず、
// candidateWireKeys の allow-list(大文字小文字を厳密に区別)が綴り違いや余計な欄を 400 で弾く。
func TestOutspeedAndKoRejectsUnknownCandidateField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value any
	}{
		{"moveId の綴り違い", "moveid", "test-x"},
		{"複数の技は受け取らない", "moveIds", []any{"test-x", "test-y"}},
		{"技の優先度を呼び出し側が指定することはできない", "movePriority", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := validBody()
			defenderAt(body, 0)[tt.key] = tt.value

			stub := &upstreams{}
			recorder := postOutspeed(newUpstreams(t, stub), body, nil)

			assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
			assertNoUpstreamCalls(t, stub)
		})
	}
}

// TestOutspeedAndKoRejectsUnknownAttackerField: attacker の欄も DefenderCandidate の候補と同じ
// 厳密さで検査する(plan.md の JD4/JD5 critic 指摘の積み残し。attacker は json.RawMessage で受け、
// individualWireKeys の allow-list が綴り違いを 400 で弾く。特に "specieskey" のような大文字小文字
// 違いは encoding/json の DisallowUnknownFields() だけでは検出できず、fold match で黙って
// speciesKey を上書きしてしまう〈defenders 側で確立した対策と同じ理由〉)。
func TestOutspeedAndKoRejectsUnknownAttackerField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value any
	}{
		{"speciesKey の大文字小文字違い", "specieskey", "9999-000"},
		{"natureId の綴り違い", "natureid", "test-x"},
		{"候補にしか無い moveId は attacker には置けない", "moveId", "test-x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := validBody()
			attackerOf(body)[tt.key] = tt.value

			stub := &upstreams{}
			recorder := postOutspeed(newUpstreams(t, stub), body, nil)

			assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
			assertBlamesAttacker(t, recorder)
			assertNoUpstreamCalls(t, stub)
		})
	}
}

// TestOutspeedAndKoUnknownMove: moveId がマスタに無ければ 422 unknown_move(ADR-0704 §6)。
// **攻撃側の技も JD4 からは上流で検証される**ので、JD3 までの 400 invalid_request
// (calc-svc の 400 に畳まれていた)から変わる(ADR-0704 §7)。
func TestOutspeedAndKoUnknownMove(t *testing.T) {
	t.Parallel()

	const notFound = `{"code":"not_found","message":"no such move"}`

	t.Run("attacker の技", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{moveFail: map[string]stubResponse{
			testMoveID: {http.StatusNotFound, notFound},
		}}
		recorder := postOutspeed(newUpstreams(t, stub), validBody(), nil)

		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownMove)
		assertBlamesAttacker(t, recorder)
		assertNoUpstreamDetail(t, recorder.Body.String(), stub.pokedexURL, stub.calcURL)

		// attacker の技で打ち切るので、候補の種族も calc も呼ばない(ADR-0704 §5)。
		_, speciesKeys, calcCalls := stub.counts()
		if !reflect.DeepEqual(speciesKeys, []string{attackerSpeciesKey}) {
			t.Errorf("species の呼び出し = %v, want [%s](attacker の技で打ち切る)", speciesKeys, attackerSpeciesKey)
		}
		if calcCalls != 0 {
			t.Errorf("calc を %d 回呼んでいる。技を解決できていないのに計算しない", calcCalls)
		}
	})

	t.Run("候補の技", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{moveFail: map[string]stubResponse{
			candidateMoveID(1): {http.StatusNotFound, notFound},
			candidateMoveID(2): {http.StatusNotFound, notFound},
		}}
		body := bodyWithDefenders(
			individual(defenderSpeciesKey, natureNeutralID, 0),
			individual(defender2SpeciesKey, natureNeutralID, 0),
			individual(defender3SpeciesKey, natureNeutralID, 0),
		)
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)

		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownMove)
		assertBlamesCandidate(t, recorder, 1)
		assertNoUpstreamDetail(t, recorder.Body.String(), stub.pokedexURL, stub.calcURL)

		// 最初に失敗した候補で打ち切る(3 番目の種族・技は引かない。ADR-0703 §3・ADR-0704 §5)。
		_, speciesKeys, calcCalls := stub.counts()
		wantSpecies := []string{attackerSpeciesKey, defenderSpeciesKey, defender2SpeciesKey}
		if !reflect.DeepEqual(speciesKeys, wantSpecies) {
			t.Errorf("species の呼び出し = %v, want %v", speciesKeys, wantSpecies)
		}
		wantMoves := []string{testMoveID, candidateMoveID(0), candidateMoveID(1)}
		if !reflect.DeepEqual(stub.moveCalls(), wantMoves) {
			t.Errorf("moves の呼び出し = %v, want %v", stub.moveCalls(), wantMoves)
		}
		if calcCalls != 0 {
			t.Errorf("calc を %d 回呼んでいる。技が揃う前に計算しない(ADR-0704 §5)", calcCalls)
		}
	})
}

// TestOutspeedAndKoMoveUpstreamFailures: 技の取得の失敗も ADR-0701 §6 の対応表に従う
// (404 だけが 422 unknown_move で、ほかは 503 upstream_unavailable。ADR-0704 §6)。
func TestOutspeedAndKoMoveUpstreamFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stub       func() *upstreams
		wantStatus int
		wantCode   api.ErrorCode
	}{
		{
			"技の取得が 500",
			func() *upstreams {
				return &upstreams{moveStatus: http.StatusInternalServerError, moveBody: `{"code":"internal","message":"boom"}`}
			},
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			"技の取得が 503",
			func() *upstreams {
				return &upstreams{moveStatus: http.StatusServiceUnavailable, moveBody: `{"code":"master_unavailable","message":"no master"}`}
			},
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			// pokedex の 400 は calc-svc の 400 と違って invalid_request にしない(ADR-0701 §6・ADR-0704 §6)。
			"技の取得が 400",
			func() *upstreams {
				return &upstreams{moveStatus: http.StatusBadRequest, moveBody: `{"code":"invalid_input","message":"bad key"}`}
			},
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			"技の本文が壊れた JSON",
			func() *upstreams { return &upstreams{moveBody: `{"id":`} },
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
		{
			// priority が無い応答を 0 と読むと、先制技を普通の技として扱った判定を
			// 正しい顔で返してしまう(ADR-0704 §9)。
			"技に priority が無い",
			func() *upstreams {
				return &upstreams{moveBody: `{"id":"test-move","nameJa":"テストわざ","type":"fire","category":"physical","power":90}`}
			},
			http.StatusServiceUnavailable, api.UpstreamUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stub := tt.stub()
			recorder := postOutspeed(newUpstreams(t, stub), validBody(), nil)

			assertStatusAndCode(t, recorder, tt.wantStatus, tt.wantCode)
			assertNoUpstreamDetail(t, recorder.Body.String(), stub.pokedexURL, stub.calcURL)
			if _, _, calcCalls := stub.counts(); calcCalls != 0 {
				t.Errorf("calc を %d 回呼んでいる。技を解決できていないのに計算しない", calcCalls)
			}
		})
	}
}

// TestOutspeedAndKoMoveCheckOrder: 技の解決を挟んでも検査順は固定(ADR-0704 §5)。
// 逐次で呼ぶので、複数の原因が同時にあってもどれが返るかが入力だけから決まる。
func TestOutspeedAndKoMoveCheckOrder(t *testing.T) {
	t.Parallel()

	const notFound = `{"code":"not_found","message":"no such"}`

	t.Run("attacker の種族は attacker の技より先", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			speciesFail: map[string]stubResponse{attackerSpeciesKey: {http.StatusNotFound, notFound}},
			moveFail:    map[string]stubResponse{testMoveID: {http.StatusNotFound, notFound}},
		}
		recorder := postOutspeed(newUpstreams(t, stub), validBody(), nil)

		// どちらも解決できないが、検査順どおり unknown_species が返る。
		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownSpecies)
		assertBlamesAttacker(t, recorder)
		if moves := stub.moveCalls(); len(moves) != 0 {
			t.Errorf("技を %v 引いている。attacker の種族で打ち切る", moves)
		}
	})

	t.Run("attacker の技は候補の種族より先", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			speciesFail: map[string]stubResponse{defenderSpeciesKey: {http.StatusNotFound, notFound}},
			moveFail:    map[string]stubResponse{testMoveID: {http.StatusNotFound, notFound}},
		}
		recorder := postOutspeed(newUpstreams(t, stub), validBody(), nil)

		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownMove)
		assertBlamesAttacker(t, recorder)
		if _, speciesKeys, _ := stub.counts(); !reflect.DeepEqual(speciesKeys, []string{attackerSpeciesKey}) {
			t.Errorf("species の呼び出し = %v, want [%s](attacker の技で打ち切る)", speciesKeys, attackerSpeciesKey)
		}
	})

	t.Run("同じ候補の中では種族が技より先", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			speciesFail: map[string]stubResponse{defenderSpeciesKey: {http.StatusNotFound, notFound}},
			moveFail:    map[string]stubResponse{candidateMoveID(0): {http.StatusNotFound, notFound}},
		}
		body := bodyWithDefenders(individual(defenderSpeciesKey, natureNeutralID, 0))
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)

		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownSpecies)
		assertBlamesCandidate(t, recorder, 0)
		if moves := stub.moveCalls(); !reflect.DeepEqual(moves, []string{testMoveID}) {
			t.Errorf("moves の呼び出し = %v, want [%s](種族で失敗した候補の技は引かない)", moves, testMoveID)
		}
	})

	t.Run("前の候補の技は次の候補の種族より先", func(t *testing.T) {
		t.Parallel()

		// 候補 0 の技と候補 1 の種族がどちらも 404。候補ごとに「種族 → 技」を回すので、
		// 候補 0 の技の失敗が勝つ(ADR-0704 §5)。
		stub := &upstreams{
			speciesFail: map[string]stubResponse{defender2SpeciesKey: {http.StatusNotFound, notFound}},
			moveFail:    map[string]stubResponse{candidateMoveID(0): {http.StatusNotFound, notFound}},
		}
		body := bodyWithDefenders(
			individual(defenderSpeciesKey, natureNeutralID, 0),
			individual(defender2SpeciesKey, natureNeutralID, 0),
		)
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)

		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownMove)
		assertBlamesCandidate(t, recorder, 0)
		_, speciesKeys, _ := stub.counts()
		wantSpecies := []string{attackerSpeciesKey, defenderSpeciesKey}
		if !reflect.DeepEqual(speciesKeys, wantSpecies) {
			t.Errorf("species の呼び出し = %v, want %v(2 番目の候補の種族は引かない)", speciesKeys, wantSpecies)
		}
	})

	t.Run("全候補の技が揃うまで計算しない", func(t *testing.T) {
		t.Parallel()

		// 最後の候補の技だけが 404。それより前の候補の計算を 1 回も始めていないこと。
		stub := &upstreams{
			moveFail: map[string]stubResponse{candidateMoveID(2): {http.StatusNotFound, notFound}},
		}
		body := bodyWithDefenders(
			individual(defenderSpeciesKey, natureNeutralID, 0),
			individual(defender2SpeciesKey, natureNeutralID, 0),
			individual(defender3SpeciesKey, natureNeutralID, 0),
		)
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)

		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownMove)
		assertBlamesCandidate(t, recorder, 2)
		if routes := stub.calcRoutes(); len(routes) != 0 {
			t.Errorf("calc を %v 回呼んでいる。全候補の種族・技が揃ってから計算する(ADR-0704 §5)", routes)
		}
	})
}

// --- ADR-0706: ID(moveId / natureId)の形式検証 ------------------------------------------
// issue #234: 形式の検査が無いと、制御文字を含む moveId は上流 URL の組み立てで落ちて
// 503 upstream_unavailable + 「上流が落ちている」という誤った警告ログになり、
// "x?y=1" のような moveId は /moves/x への問い合わせに静かにすり替わる。
// クライアントの入力ミスは上流を呼ぶ前に 400 invalid_request で返す(ADR-0706 §1・§3)。

// invalidIDSamples は ID に現れてはいけない値(ADR-0706 §1・受け入れ条件1)。
// 前半は URL の構文を壊す・書き換える文字、後半は Showdown ID の綴りから外れる形。
var invalidIDSamples = []struct {
	name string
	id   string
}{
	{"制御文字(タブ)", "test\tmove"},
	{"制御文字(0x7f)", "test\u007fmove"},
	{"空白", "test move"},
	{"スラッシュ", "test/move"},
	{"クエリの開始", "test?move=1"},
	{"フラグメントの開始", "test#move"},
	{"パーセント", "test%2fmove"},
	{"大文字", "Test-Move"},
	{"先頭のハイフン", "-test-move"},
	{"末尾のハイフン", "test-move-"},
	{"ハイフンの連続", "test--move"},
}

// idFields は moveId / natureId が現れる 4 か所(ADR-0706 テストの期待値)。
// どれか 1 か所だけ直った実装を通さないために、4 つすべてを同じ表で回す。
// wantIndex が -1 なら attacker 側(message は候補の index を騙らない。ADR-0703 §3)。
var idFields = []struct {
	name      string
	mutate    func(body map[string]any, id string)
	wantIndex int
}{
	{"attacker の moveId(request 直下)", func(body map[string]any, id string) { body["moveId"] = id }, -1},
	{"attacker の natureId", func(body map[string]any, id string) { attackerOf(body)["natureId"] = id }, -1},
	{"候補の moveId", func(body map[string]any, id string) { defenderAt(body, 1)["moveId"] = id }, 1},
	{"候補の natureId", func(body map[string]any, id string) { defenderAt(body, 1)["natureId"] = id }, 1},
}

// twoCandidateBody は候補 2 件の 200 になる request。ID の異常系は **index 1** を不正にして
// 確かめる(index 0 だと、帰属ラベルを defenders[0] に固定した実装が緑のまま通る)。
func twoCandidateBody() map[string]any {
	return bodyWithDefenders(
		candidate(defenderSpeciesKey, natureNeutralID, 0, candidateMoveID(0)),
		candidate(defender2SpeciesKey, natureNeutralID, 0, candidateMoveID(1)),
	)
}

// assertBlamesSide は attacker 側(index < 0)と候補側で帰属の検査を振り分ける。
func assertBlamesSide(t *testing.T, recorder *httptest.ResponseRecorder, wantIndex int) {
	t.Helper()
	if wantIndex < 0 {
		assertBlamesAttacker(t, recorder)
		return
	}
	assertBlamesCandidate(t, recorder, wantIndex)
}

// TestOutspeedAndKoRejectsInvalidIDFormat: moveId / natureId が Showdown ID の形式
// (^[a-z0-9]+(-[a-z0-9]+)*$)に合わなければ、**上流を 1 回も呼ばずに** 400 invalid_request
// (ADR-0706 §1・§3・受け入れ条件1・2)。message はどちら側の ID かを示す(ADR-0706 §4)。
func TestOutspeedAndKoRejectsInvalidIDFormat(t *testing.T) {
	t.Parallel()

	for _, field := range idFields {
		t.Run(field.name, func(t *testing.T) {
			t.Parallel()

			for _, sample := range invalidIDSamples {
				t.Run(sample.name, func(t *testing.T) {
					t.Parallel()

					body := twoCandidateBody()
					field.mutate(body, sample.id)

					stub := &upstreams{}
					recorder := postOutspeed(newUpstreams(t, stub), body, nil)

					assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
					assertBlamesSide(t, recorder, field.wantIndex)
					assertNoUpstreamCalls(t, stub)
					assertNoUpstreamDetail(t, recorder.Body.String(), stub.pokedexURL, stub.calcURL)
				})
			}
		})
	}
}

// TestOutspeedAndKoIDLengthLimit: 長さの上限は 64 文字ちょうどまで通り、65 文字は上流を
// 1 回も呼ばずに 400(ADR-0706 §1・受け入れ条件4)。上限は「際限なく長い path 要素を
// 上流へ出さない」ためのもので、正しい形式の ID を落とすためのものではない。
func TestOutspeedAndKoIDLengthLimit(t *testing.T) {
	t.Parallel()

	atLimit := strings.Repeat("a", maxIDLength)
	overLimit := strings.Repeat("a", maxIDLength+1)

	// 上限ちょうどの natureId が「一覧に無い」(422 unknown_nature)で落ちないよう、
	// その ID を載せた架空の性格一覧を返させる(形式の検査とは別の話)。
	naturesWithLongID := `[
	  {"id":"` + naturePlusSpeID + `","nameJa":"テストようき","plus":"spe","minus":"spa"},
	  {"id":"` + natureNeutralID + `","nameJa":"テストまじめ","plus":null,"minus":null},
	  {"id":"` + atLimit + `","nameJa":"テストながいせいかく","plus":null,"minus":null}
	]`

	t.Run("64 文字ちょうどの moveId / natureId は通る", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		body["moveId"] = atLimit
		attackerOf(body)["natureId"] = atLimit
		defenderAt(body, 0)["moveId"] = atLimit
		defenderAt(body, 0)["natureId"] = atLimit

		stub := &upstreams{naturesBody: naturesWithLongID}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200(上限ちょうどは通る); body=%s", recorder.Code, recorder.Body.String())
		}
		if moves := stub.moveCalls(); len(moves) != 2 || moves[0] != atLimit || moves[1] != atLimit {
			t.Errorf("技の呼び出し = %v, want [%s %s](上限ちょうどの ID がそのまま上流に届く)", moves, atLimit, atLimit)
		}
	})

	t.Run("65 文字は 400", func(t *testing.T) {
		t.Parallel()

		for _, field := range idFields {
			t.Run(field.name, func(t *testing.T) {
				t.Parallel()

				body := twoCandidateBody()
				field.mutate(body, overLimit)

				stub := &upstreams{}
				recorder := postOutspeed(newUpstreams(t, stub), body, nil)

				assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
				assertBlamesSide(t, recorder, field.wantIndex)
				assertNoUpstreamCalls(t, stub)
			})
		}
	})
}

// TestOutspeedAndKoIDFormatCheckOrder: 形式の検査は ADR-0703 §4 の順(attacker → 候補を
// index 昇順)に従い、最初の 1 件だけを返す(ADR-0706 §4)。body の上限(8 KiB)は
// 長さの上限より**先**に見るので、巨大な moveId は 400 ではなく 413 のまま(受け入れ条件4)。
func TestOutspeedAndKoIDFormatCheckOrder(t *testing.T) {
	t.Parallel()

	t.Run("attacker と候補の両方が不正なら attacker", func(t *testing.T) {
		t.Parallel()

		body := twoCandidateBody()
		body["moveId"] = "test move"
		defenderAt(body, 1)["natureId"] = "test/nature"

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)

		assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
		assertBlamesAttacker(t, recorder)
		assertNoUpstreamCalls(t, stub)
	})

	t.Run("候補が複数不正なら index の小さい方", func(t *testing.T) {
		t.Parallel()

		body := twoCandidateBody()
		defenderAt(body, 0)["moveId"] = "test move"
		defenderAt(body, 1)["natureId"] = "test/nature"

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)

		assertStatusAndCode(t, recorder, http.StatusBadRequest, api.InvalidRequest)
		assertBlamesCandidate(t, recorder, 0)
		assertNoUpstreamCalls(t, stub)
	})

	t.Run("8 KiB を超える moveId は長さの上限より先に 413", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		body["moveId"] = strings.Repeat("a", 9*1024)

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)

		assertStatusAndCode(t, recorder, http.StatusRequestEntityTooLarge, api.RequestTooLarge)
		assertNoUpstreamCalls(t, stub)
	})
}

// TestOutspeedAndKoAcceptsValidIDFormat: 正常系は変わらない(ADR-0706 受け入れ条件5)。
// ハイフン区切りの小文字英数はそのまま通り、上流へ渡される ID も変わらない。
// 形式は合うがマスタに無い技は、今まで通り 400 ではなく 422 unknown_move(ADR-0704 §6)。
func TestOutspeedAndKoAcceptsValidIDFormat(t *testing.T) {
	t.Parallel()

	t.Run("数字だけ・ハイフン区切りの ID も通る", func(t *testing.T) {
		t.Parallel()

		body := validBody()
		body["moveId"] = "m1"
		defenderAt(body, 0)["moveId"] = "test-move-2-x9"

		stub := &upstreams{}
		recorder := postOutspeed(newUpstreams(t, stub), body, nil)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		want := []string{"m1", "test-move-2-x9"}
		if moves := stub.moveCalls(); !reflect.DeepEqual(moves, want) {
			t.Errorf("技の呼び出し = %v, want %v(ID は加工せずそのまま上流へ渡す)", moves, want)
		}
	})

	t.Run("形式は合うがマスタに無い技は 422 unknown_move のまま", func(t *testing.T) {
		t.Parallel()

		stub := &upstreams{
			moveFail: map[string]stubResponse{
				testMoveID: {http.StatusNotFound, `{"code":"not_found","message":"no such move"}`},
			},
		}
		recorder := postOutspeed(newUpstreams(t, stub), validBody(), nil)

		assertStatusAndCode(t, recorder, http.StatusUnprocessableEntity, api.UnknownMove)
		assertBlamesAttacker(t, recorder)
	})
}
