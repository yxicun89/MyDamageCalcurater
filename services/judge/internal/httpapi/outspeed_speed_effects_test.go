package httpapi

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/pokecalc/services/judge/internal/api"
	"example.com/pokecalc/services/judge/internal/client"
	"example.com/pokecalc/services/judge/internal/speedeffects"
)

// 素早さに効く特性・持ち物を、pokedex-svc の内部 API(/internal/pokedex/master)の効果データ
// (ADR-0139 の SpeedMods・IgnoresParalysisSpeedDrop)で反映する(issue 235 第2段・ADR-0714)。
// 特性・持ち物は架空の ID と架空の効果(実データは使わない)。
//
// 実装への契約(ADR-0714):
//   - Dependencies.SpeedEffects(*speedeffects.Cache)。nil なら「データなし」で第1段と同じ応答。
//   - 特性・持ち物(スカーフ以外)が 1 つも指定されていないリクエストではマスタを取りに行かない。
//   - マスタの取得失敗は判定を失敗させない(200。その特性・持ち物は *SpeedIgnored に残り、素早さに掛けない)。

const (
	fxRainAbility     = "test-rain-ability"     // 雨で ×2
	fxElecAbility     = "test-elec-ability"     // エレキフィールドで ×2
	fxStatusAbility   = "test-status-ability"   // 状態異常で ×1.5、まひの半減を受けない
	fxItemLostAbility = "test-itemlost-ability" // 持ち物を失った後に ×2(judge は評価できない)
	fxFogAbility      = "test-fog-ability"      // 語彙に無い条件(評価できない)
	fxBrokenAbility   = "test-broken-ability"   // 素早さの項目の形が不正
	fxPlainAbility    = "test-plain-ability"    // 素早さ効果なし(効果 null)
	fxDamageAbility   = "test-damage-ability"   // ダメージの効果だけ(素早さ効果なし)
	fxHeavyItem       = "test-heavy-item"       // 常に ×0.5
	fxPlainItem       = "test-plain-item"       // 素早さ効果なし
	fxUnknownID       = "test-not-in-master"    // マスタに無い ID
)

const fxMasterBody = `{
  "schemaVersion": 1, "dataVersion": "test", "types": [], "typeChart": [], "species": [], "moves": [], "natures": [],
  "items": [
    {"id":"` + fxHeavyItem + `","nameJa":"x","effect":{"SpeedMods":[{"Condition":"always","Modifier":2048}]}},
    {"id":"` + fxPlainItem + `","nameJa":"x","effect":null},
    {"id":"choicescarf","nameJa":"x","effect":null}
  ],
  "abilities": [
    {"id":"` + fxRainAbility + `","nameJa":"x","effect":{"SpeedMods":[{"Condition":"weather_rain","Modifier":8192}]}},
    {"id":"` + fxElecAbility + `","nameJa":"x","effect":{"SpeedMods":[{"Condition":"terrain_electric","Modifier":8192}]}},
    {"id":"` + fxStatusAbility + `","nameJa":"x","effect":{"SpeedMods":[{"Condition":"has_status","Modifier":6144}],"IgnoresParalysisSpeedDrop":true}},
    {"id":"` + fxItemLostAbility + `","nameJa":"x","effect":{"SpeedMods":[{"Condition":"item_lost","Modifier":8192}]}},
    {"id":"` + fxFogAbility + `","nameJa":"x","effect":{"SpeedMods":[{"Condition":"weather_fog","Modifier":8192}]}},
    {"id":"` + fxBrokenAbility + `","nameJa":"x","effect":{"SpeedMods":[{"Condition":"always","Modifier":4096}]}},
    {"id":"` + fxPlainAbility + `","nameJa":"x","effect":null},
    {"id":"` + fxDamageAbility + `","nameJa":"x","effect":{"DamageMod":{"Kind":"x","Modifier":5325}}}
  ]
}`

// masterStub は内部 API のスタブ。呼ばれた回数を数え、status/body を差し替えられる。
type masterStub struct {
	mu     sync.Mutex
	calls  int
	status int
	body   string
}

func (m *masterStub) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// withSpeedEffects は newUpstreams の Dependencies に、内部 API のスタブを引く SpeedEffects を足す。
func withSpeedEffects(t *testing.T, deps Dependencies, stub *masterStub) Dependencies {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.mu.Lock()
		defer stub.mu.Unlock()
		if r.URL.Path != "/internal/pokedex/master" {
			writeStub(w, http.StatusNotFound, `{"code":"not_found","message":"no route"}`)
			return
		}
		stub.calls++
		body := stub.body
		if body == "" {
			body = fxMasterBody
		}
		writeStub(w, stub.status, body)
	}))
	t.Cleanup(server.Close)

	pokedex, err := client.NewPokedex(client.Config{BaseURL: server.URL, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewPokedex: %v", err)
	}
	cache, err := speedeffects.NewCache(pokedex.MasterEffects, speedeffects.Config{
		TTL: 10 * time.Minute, RetryAfterFailure: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	deps.SpeedEffects = cache
	return deps
}

// effectsCase は 1 件の判定を流して、自分・相手の素早さと Applied/Ignored を見る。
type effectsCase struct {
	name          string
	attackerBase  int // 0 なら 100
	defenderBase  int // 0 なら 100
	mutate        func(body map[string]any)
	wantAttacker  int
	wantDefender  int
	wantAApplied  []string
	wantDApplied  []string
	wantAIgnored  []string
	wantDIgnored  []string
	wantOutspeeds *bool
}

func boolPtr(b bool) *bool { return &b }

func runEffectsCase(t *testing.T, tt effectsCase, stub *masterStub) api.Matchup {
	t.Helper()
	body := neutralBody()
	tt.mutate(body)
	deps := withSpeedEffects(t, newUpstreams(t, &upstreams{attackerSpeed: tt.attackerBase, defenderSpeed: tt.defenderBase}), stub)
	recorder := postOutspeed(deps, body, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	got := onlyMatchup(t, recorder)
	if got.AttackerSpeed != tt.wantAttacker || got.DefenderSpeed != tt.wantDefender {
		t.Errorf("素早さ = %d / %d, want %d / %d", got.AttackerSpeed, got.DefenderSpeed, tt.wantAttacker, tt.wantDefender)
	}
	if !reflect.DeepEqual(speedNames(got.AttackerSpeedApplied), orEmpty(tt.wantAApplied)) ||
		!reflect.DeepEqual(speedNames(got.DefenderSpeedApplied), orEmpty(tt.wantDApplied)) {
		t.Errorf("speedApplied = %v / %v, want %v / %v", got.AttackerSpeedApplied, got.DefenderSpeedApplied, tt.wantAApplied, tt.wantDApplied)
	}
	if !reflect.DeepEqual(speedNames(got.AttackerSpeedIgnored), orEmpty(tt.wantAIgnored)) ||
		!reflect.DeepEqual(speedNames(got.DefenderSpeedIgnored), orEmpty(tt.wantDIgnored)) {
		t.Errorf("speedIgnored = %v / %v, want %v / %v", got.AttackerSpeedIgnored, got.DefenderSpeedIgnored, tt.wantAIgnored, tt.wantDIgnored)
	}
	if tt.wantOutspeeds != nil && got.Outspeeds != *tt.wantOutspeeds {
		t.Errorf("outspeeds = %v, want %v", got.Outspeeds, *tt.wantOutspeeds)
	}
	return got
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// TestOutspeedAndKoSpeedEffectsFromMaster: 各条件の反映と Applied/Ignored の出し分け。
// 素早さは自分 = 種族値 73(実数値 93)、相手 = 種族値 71(実数値 91)など奇数にして丸めの順を区別する
// (期待値は @smogon/calc 0.12.0 の getFinalSpeed と一致。ADR-0714「テストの期待値」)。
func TestOutspeedAndKoSpeedEffectsFromMaster(t *testing.T) {
	t.Parallel()

	tests := []effectsCase{
		{
			name:         "雨 + 雨で ×2 の特性: 反映して Ignored に出さない(fieldWeather も出さない)",
			attackerBase: 71, defenderBase: 100,
			mutate: func(b map[string]any) {
				attackerOf(b)["abilityId"] = fxRainAbility
				b["field"] = map[string]any{"weather": "rain"}
			},
			wantAttacker: 182, wantDefender: 120, wantAApplied: []string{"ability"},
			wantOutspeeds: boolPtr(true),
		},
		{
			name:         "天候なし + 雨で ×2 の特性: 不成立と確定したので Applied にも Ignored にも出さない",
			attackerBase: 71,
			mutate:       func(b map[string]any) { attackerOf(b)["abilityId"] = fxRainAbility },
			wantAttacker: 91, wantDefender: 120,
		},
		{
			name:         "晴れ + 雨で ×2 の特性: 不成立(fieldWeather も出さない)",
			attackerBase: 71,
			mutate: func(b map[string]any) {
				attackerOf(b)["abilityId"] = fxRainAbility
				b["field"] = map[string]any{"weather": "sun"}
			},
			wantAttacker: 91, wantDefender: 120,
		},
		{
			name:         "エレキフィールド: 相手の特性だけ ×2(場は共通の field.terrain)",
			defenderBase: 71,
			mutate: func(b map[string]any) {
				defenderAt(b, 0)["abilityId"] = fxElecAbility
				b["field"] = map[string]any{"terrain": "electric"}
			},
			wantAttacker: 120, wantDefender: 182, wantDApplied: []string{"ability"},
			wantOutspeeds: boolPtr(false),
		},
		{
			name:         "状態異常(やけど)で ×1.5: 91 → 136",
			attackerBase: 71,
			mutate: func(b map[string]any) {
				attackerOf(b)["abilityId"] = fxStatusAbility
				attackerOf(b)["status"] = "burn"
			},
			wantAttacker: 136, wantDefender: 120, wantAApplied: []string{"ability"},
		},
		{
			name:         "まひ + まひの半減を受けない特性: ×1.5 だけで半減しない(paralysis は Applied に入らない)",
			attackerBase: 71,
			mutate: func(b map[string]any) {
				attackerOf(b)["abilityId"] = fxStatusAbility
				attackerOf(b)["status"] = "paralysis"
			},
			wantAttacker: 136, wantDefender: 120, wantAApplied: []string{"ability"},
		},
		{
			name:         "状態異常なし(none)+ 状態異常で ×1.5 の特性: 不成立と確定",
			attackerBase: 71,
			mutate: func(b map[string]any) {
				attackerOf(b)["abilityId"] = fxStatusAbility
				attackerOf(b)["status"] = "none"
			},
			wantAttacker: 91, wantDefender: 120,
		},
		{
			name:         "持ち物を失った後の特性: 評価できないので掛けず、abilityId を Ignored に残す",
			attackerBase: 71,
			mutate:       func(b map[string]any) { attackerOf(b)["abilityId"] = fxItemLostAbility },
			wantAttacker: 91, wantDefender: 120, wantAIgnored: []string{"abilityId"},
		},
		{
			name:         "評価できない特性 + 天候: abilityId と fieldWeather を Ignored に残す",
			attackerBase: 71,
			mutate: func(b map[string]any) {
				attackerOf(b)["abilityId"] = fxItemLostAbility
				b["field"] = map[string]any{"weather": "sand"}
			},
			wantAttacker: 91, wantDefender: 120, wantAIgnored: []string{"abilityId", "fieldWeather"},
		},
		{
			name:         "語彙に無い条件の特性: 掛けず abilityId を Ignored に残す(安全側)",
			attackerBase: 71,
			mutate: func(b map[string]any) {
				attackerOf(b)["abilityId"] = fxFogAbility
				b["field"] = map[string]any{"weather": "rain"}
			},
			wantAttacker: 91, wantDefender: 120, wantAIgnored: []string{"abilityId", "fieldWeather"},
		},
		{
			name:         "素早さの項目が不正な特性: その ID だけ確定できない(他の ID は使える)",
			attackerBase: 71,
			mutate: func(b map[string]any) {
				attackerOf(b)["abilityId"] = fxBrokenAbility
				defenderAt(b, 0)["itemId"] = fxHeavyItem
			},
			wantAttacker: 91, wantDefender: 60, wantAIgnored: []string{"abilityId"}, wantDApplied: []string{"item"},
		},
		{
			name: "素早さ効果の無い特性・持ち物: 影響しないと確定したので Ignored に出さない(第1段からの仕様変更)",
			mutate: func(b map[string]any) {
				attackerOf(b)["abilityId"] = fxPlainAbility
				attackerOf(b)["itemId"] = fxPlainItem
				defenderAt(b, 0)["abilityId"] = fxDamageAbility
				b["field"] = map[string]any{"weather": "rain"}
			},
			wantAttacker: 120, wantDefender: 120,
		},
		{
			name: "マスタに無い ID: 確定できないので従来どおり Ignored(abilityId → itemId → fieldWeather)",
			mutate: func(b map[string]any) {
				defenderAt(b, 0)["abilityId"] = fxUnknownID
				defenderAt(b, 0)["itemId"] = fxUnknownID
				b["field"] = map[string]any{"weather": "snow"}
			},
			wantAttacker: 120, wantDefender: 120, wantDIgnored: []string{"abilityId", "itemId", "fieldWeather"},
		},
		{
			name:         "常に ×0.5 の持ち物: 93 → 46.5 → 46",
			attackerBase: 73,
			mutate:       func(b map[string]any) { attackerOf(b)["itemId"] = fxHeavyItem },
			wantAttacker: 46, wantDefender: 120, wantAApplied: []string{"item"},
		},
		{
			name:         "状態異常 ×1.5 + 持ち物 ×0.5 は連結してから 1 回丸める: 93 → 70(補正ごとに丸めると 69)",
			attackerBase: 73,
			mutate: func(b map[string]any) {
				attackerOf(b)["abilityId"] = fxStatusAbility
				attackerOf(b)["itemId"] = fxHeavyItem
				attackerOf(b)["status"] = "poison"
			},
			wantAttacker: 70, wantDefender: 120, wantAApplied: []string{"ability", "item"},
		},
		{
			name:         "まひ + 持ち物 ×0.5: 93 → 46 → 23(rank → … → item → paralysis)",
			attackerBase: 73,
			mutate: func(b map[string]any) {
				attackerOf(b)["itemId"] = fxHeavyItem
				attackerOf(b)["status"] = "paralysis"
			},
			wantAttacker: 23, wantDefender: 120, wantAApplied: []string{"item", "paralysis"},
		},
		{
			name:         "雨 ×2 の特性 + スカーフ: chain で ×3、93 → 279(スカーフで先に丸めると 278)",
			attackerBase: 73,
			mutate: func(b map[string]any) {
				attackerOf(b)["abilityId"] = fxRainAbility
				attackerOf(b)["itemId"] = "choicescarf"
				b["field"] = map[string]any{"weather": "rain"}
			},
			wantAttacker: 279, wantDefender: 120, wantAApplied: []string{"ability", "choiceScarf"},
		},
		{
			name:         "全部: ランク +1・追い風・状態異常 ×1.5・持ち物 ×0.5 → 139 × 1.5 = 208.5 → 208",
			attackerBase: 73,
			mutate: func(b map[string]any) {
				attackerOf(b)["ranks"] = map[string]any{"spe": 1}
				attackerOf(b)["abilityId"] = fxStatusAbility
				attackerOf(b)["itemId"] = fxHeavyItem
				attackerOf(b)["status"] = "sleep"
				b["speedField"] = map[string]any{"attackerTailwind": true}
			},
			wantAttacker: 208, wantDefender: 120, wantAApplied: []string{"rank", "tailwind", "ability", "item"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runEffectsCase(t, tt, &masterStub{})
		})
	}
}

// TestOutspeedAndKoSpeedEffectsFailSoft: マスタの取得に失敗しても判定は 200 で、素早さには掛けず、
// 第1段と同じ Ignored を返す(ADR-0714 §1。ADR-0700 §3 の 4 区分の 503 にしない)。
func TestOutspeedAndKoSpeedEffectsFailSoft(t *testing.T) {
	t.Parallel()

	failures := map[string]*masterStub{
		"503":      {status: http.StatusServiceUnavailable, body: `{"code":"master_unavailable","message":"x"}`},
		"500":      {status: http.StatusInternalServerError, body: `{}`},
		"404(古い版)": {status: http.StatusNotFound, body: `{"code":"not_found","message":"x"}`},
		"壊れた JSON": {body: `{"items":[`},
		"形が違う":     {body: `{"items":{},"abilities":[]}`},
	}
	for name, stub := range failures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			body := neutralBody()
			attackerOf(body)["abilityId"] = fxRainAbility
			attackerOf(body)["itemId"] = fxHeavyItem
			b := body
			b["field"] = map[string]any{"weather": "rain"}

			deps := withSpeedEffects(t, newUpstreams(t, &upstreams{}), stub)
			recorder := postOutspeed(deps, body, nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200(素早さ効果のデータが無いだけで判定を落とさない); body=%s",
					recorder.Code, recorder.Body.String())
			}
			got := onlyMatchup(t, recorder)
			if got.AttackerSpeed != 120 {
				t.Errorf("attackerSpeed = %d, want 120(データが無ければ掛けない)", got.AttackerSpeed)
			}
			if len(got.AttackerSpeedApplied) != 0 {
				t.Errorf("AttackerSpeedApplied = %v, want 空", got.AttackerSpeedApplied)
			}
			if want := []string{"abilityId", "itemId", "fieldWeather"}; !reflect.DeepEqual(speedNames(got.AttackerSpeedIgnored), want) {
				t.Errorf("AttackerSpeedIgnored = %v, want %v(第1段と同じ)", got.AttackerSpeedIgnored, want)
			}
			if strings.Contains(recorder.Body.String(), "master_unavailable") {
				t.Errorf("上流の事情を応答に漏らしている: %s", recorder.Body.String())
			}
		})
	}
}

// TestOutspeedAndKoSpeedEffectsNilMatchesStage1: SpeedEffects が未設定(nil)なら第1段の応答と同じ。
// 既存の TestOutspeedAndKoSpeedAppliedAndIgnored・TestOutspeedAndKoStatusParalysis がこの経路を固定する。
func TestOutspeedAndKoSpeedEffectsNilMatchesStage1(t *testing.T) {
	t.Parallel()

	body := neutralBody()
	attackerOf(body)["abilityId"] = fxRainAbility
	b := body
	b["field"] = map[string]any{"weather": "rain"}
	deps := newUpstreams(t, &upstreams{})
	if deps.SpeedEffects != nil {
		t.Fatal("newUpstreams は SpeedEffects を設定しない(第1段の経路を保つ)")
	}
	got := onlyMatchup(t, postOutspeed(deps, body, nil))
	if got.AttackerSpeed != 120 {
		t.Errorf("attackerSpeed = %d, want 120", got.AttackerSpeed)
	}
	if want := []string{"abilityId", "fieldWeather"}; !reflect.DeepEqual(speedNames(got.AttackerSpeedIgnored), want) {
		t.Errorf("AttackerSpeedIgnored = %v, want %v", got.AttackerSpeedIgnored, want)
	}
}

// TestOutspeedAndKoSpeedEffectsFetchOnlyWhenNeeded: 特性・スカーフ以外の持ち物が 1 つも無ければ
// マスタを取りに行かない。必要なら 1 回取り、TTL の間は次のリクエストでも取り直さない(候補が複数でも 1 回)。
func TestOutspeedAndKoSpeedEffectsFetchOnlyWhenNeeded(t *testing.T) {
	t.Parallel()

	stub := &masterStub{}
	deps := withSpeedEffects(t, newUpstreams(t, &upstreams{}), stub)

	plain := neutralBody()
	attackerOf(plain)["itemId"] = "choicescarf" // スカーフは設定の ID 一致で判定するのでマスタは要らない
	if recorder := postOutspeed(deps, plain, nil); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if n := stub.count(); n != 0 {
		t.Errorf("特性・持ち物の無いリクエストでマスタを %d 回取った, want 0", n)
	}

	withAbility := bodyWithDefenders(
		candidate(defenderSpeciesKey, natureNeutralID, 0, defenderMoveID),
		candidate(defender2SpeciesKey, natureNeutralID, 0, candidateMoveID(1)),
	)
	attackerOf(withAbility)["natureId"] = natureNeutralID
	attackerOf(withAbility)["sp"] = sp(0)
	defenderAt(withAbility, 0)["abilityId"] = fxRainAbility
	defenderAt(withAbility, 1)["itemId"] = fxHeavyItem
	for range 3 {
		if recorder := postOutspeed(deps, withAbility, nil); recorder.Code != http.StatusOK {
			t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
		}
	}
	if n := stub.count(); n != 1 {
		t.Errorf("マスタの取得回数 = %d, want 1(TTL 内は取り直さない・候補ごとに取らない)", n)
	}
}

// TestOutspeedAndKoSpeedEffectsDoNotChangeCalcRequest: 特性・持ち物は従来どおり calc-svc へそのまま送る
// (素早さの反映はダメージ計算の要求を変えない)。
func TestOutspeedAndKoSpeedEffectsDoNotChangeCalcRequest(t *testing.T) {
	t.Parallel()

	up := &upstreams{}
	deps := withSpeedEffects(t, newUpstreams(t, up), &masterStub{})
	body := neutralBody()
	attackerOf(body)["abilityId"] = fxRainAbility
	attackerOf(body)["itemId"] = fxHeavyItem
	body["field"] = map[string]any{"weather": "rain"}
	if recorder := postOutspeed(deps, body, nil); recorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	forward := up.calcBodyAt(t, 0)
	attacker, _ := forward["attacker"].(map[string]any)
	if attacker["abilityId"] != fxRainAbility || attacker["itemId"] != fxHeavyItem {
		t.Errorf("calc-svc への attacker = %v, want abilityId/itemId をそのまま", attacker)
	}
	if _, has := forward["speedEffects"]; has {
		t.Errorf("calc-svc への要求に judge の素早さの情報を足している: %v", forward)
	}
}
