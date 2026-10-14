package httpapi

// 生成型 ⇄ app の写しの検査(ADR-0811・ADR-0252)。

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"example.com/pokecalc/services/calc/internal/app"
	"example.com/pokecalc/services/calc/internal/domain"
	"example.com/pokecalc/services/internal/api"
)

// httpErrorFrom の写し: domain の失敗は code と文言のまま、締め切り超過は 503 + Retry-After、
// どちらでもない失敗は固定文の internal(500)にして元の文言を応答に出さない(ADR-0200 AC-7)。
func TestHTTPErrorFromMapping(t *testing.T) {
	if httpErrorFrom(nil) != nil {
		t.Errorf("nil が nil にならない")
	}
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    api.ErrorCode
		wantMessage string
	}{
		{"domain の失敗は code・文言のまま", domain.Errorf(domain.CodeUnknownMove, "技 %q が無い", "x"),
			http.StatusBadRequest, api.UnknownMove, `技 "x" が無い`},
		{"engine の想定外は固定文", domain.FromEngine(errors.New("内部の詳細")),
			http.StatusInternalServerError, api.Internal, messageInternal},
		{"語彙に無い失敗は固定文", errors.New("内部の詳細"),
			http.StatusInternalServerError, api.Internal, messageInternal},
		{"包まれた語彙に無い失敗も固定文", fmt.Errorf("包む: %w", errors.New("内部の詳細")),
			http.StatusInternalServerError, api.Internal, messageInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := httpErrorFrom(tt.err)
			var he *httpError
			if !errors.As(mapped, &he) {
				t.Fatalf("httpError でない: %v(%T)", mapped, mapped)
			}
			status, body := errorBodyFor(mapped)
			if status != tt.wantStatus || body.Code != tt.wantCode || body.Message != tt.wantMessage {
				t.Errorf("= (%d, %q, %q), want (%d, %q, %q)", status, body.Code, body.Message, tt.wantStatus, tt.wantCode, tt.wantMessage)
			}
		})
	}

	var he *httpError
	if !errors.As(httpErrorFrom(fmt.Errorf("包む: %w", app.ErrDeadlineExceeded)), &he) ||
		he.status != http.StatusServiceUnavailable || he.code != api.UpstreamUnavailable || !he.retryAfter {
		t.Errorf("締め切り超過 = %+v, want 503 upstream_unavailable + Retry-After", he)
	}
}

// domain.Code は契約の ErrorCode と同じ文字列(httpErrorFrom が文字列のまま写すため)。
func TestDomainCodesAreContractErrorCodes(t *testing.T) {
	for _, c := range domain.AllCodes() {
		if !api.ErrorCode(c).Valid() {
			t.Errorf("domain.Code %q が契約の ErrorCode に無い", c)
		}
	}
}

// 計算イベントにはリクエストのままを載せる(ADR-0212 §7)。生成型 → app → 生成型で省略の有無も含めて変わらない。
func TestCalcRequestRoundTripsIntoEvent(t *testing.T) {
	i := func(v int) *int { return &v }
	b := func(v bool) *bool { return &v }
	s := func(v string) *string { return &v }
	status := api.StatusCondition("burn")
	tera := api.PokeType("fire")
	weather := api.Weather("rain")
	terrain := api.Terrain("not-a-terrain") // 列挙に無い値も、そのまま写る(検証は app・domain)
	full := api.CalcRequest{
		Format: api.Format("single"),
		Attacker: api.Individual{
			SpeciesKey: "0001-000", NatureId: "adamant", Level: i(50), AbilityId: s("a"), ItemId: s("b"),
			Sp:     api.StatBlock{Hp: 1, Atk: 2, Def: 3, Spa: 4, Spd: 5, Spe: 6},
			Ranks:  &api.RankBlock{Atk: i(1), Spe: i(-1)},
			Status: &status, TeraType: &tera,
		},
		Defender: api.Individual{SpeciesKey: "0002-000", NatureId: "bold", Ranks: &api.RankBlock{}},
		MoveId:   "m",
		Field: &api.FieldState{
			Weather: &weather, Terrain: &terrain,
			AttackerScreens: &api.Screens{Reflect: b(true)}, DefenderScreens: &api.Screens{},
		},
		Options:     &api.CalcOptions{Critical: b(false)},
		BattleState: &api.CalcBattleState{Hits: i(2)},
	}
	for _, req := range []api.CalcRequest{full, {Format: "double"}, {Options: &api.CalcOptions{}, Field: &api.FieldState{}}} {
		r := calcRequestFrom(req)
		got := api.CalcRequest{
			Format: api.Format(r.Format.Value), Attacker: individualToAPI(r.Attacker), Defender: individualToAPI(r.Defender),
			MoveId: r.MoveID, Field: fieldToAPI(r.Field), Options: optionsToAPI(r.Options), BattleState: battleStateToAPI(r.BattleState),
		}
		if !reflect.DeepEqual(got, req) {
			t.Errorf("往復で変わった:\n got  %+v\n want %+v", got, req)
		}
	}
}
