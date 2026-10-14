package domain

// 列挙の検証(invalid_enum)の単体テスト。契約に含まれるかは adapters が Known に入れて渡す。

import (
	"testing"

	"example.com/pokecalc/engine"
)

func TestParseFormat(t *testing.T) {
	if got, err := ParseFormat(Enum{Value: "double", Known: true}); err != nil || got != engine.Format("double") {
		t.Errorf("既知 = (%q, %v)", got, err)
	}
	for _, e := range []Enum{{Value: "triple"}, {}} {
		if _, err := ParseFormat(e); codeOf(t, err) != CodeInvalidEnum {
			t.Errorf("%+v の code = %v", e, err)
		}
	}
}

// 省略できる列挙は nil が既定値、未知は invalid_enum、既知はその値。
func TestOptionalEnums(t *testing.T) {
	known := func(v string) *Enum { return &Enum{Value: v, Known: true} }
	unknown := &Enum{Value: "???"}
	tests := []struct {
		name      string
		parse     func(*Enum) (string, error)
		knownVal  string
		wantOnNil string
	}{
		{"status", func(e *Enum) (string, error) { v, err := ParseStatus("attacker", e); return string(v), err }, "burn", string(engine.StatusNone)},
		{"teraType", func(e *Enum) (string, error) { v, err := ParseTeraType("attacker", e); return string(v), err }, "fire", string(engine.TypeNone)},
		{"opponentSpeed", func(e *Enum) (string, error) { v, err := ParseOpponentSpeed(e); return string(v), err }, "slower", string(engine.OpponentSpeedUnknown)},
		{"focus", func(e *Enum) (string, error) { v, err := ParseBulkFocus(e); return string(v), err }, "physical", ""},
		{"offenseCategory", func(e *Enum) (string, error) { v, err := ParseOffenseCategory(e); return string(v), err }, "special", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := tt.parse(nil); err != nil || got != tt.wantOnNil {
				t.Errorf("nil = (%q, %v), want (%q, nil)", got, err, tt.wantOnNil)
			}
			if got, err := tt.parse(known(tt.knownVal)); err != nil || got != tt.knownVal {
				t.Errorf("既知 = (%q, %v)", got, err)
			}
			if _, err := tt.parse(unknown); codeOf(t, err) != CodeInvalidEnum {
				t.Errorf("未知の code = %v", err)
			}
		})
	}
}

// 必須の列挙(mode・goals[i].kind)は欠落(空・Known=false)も invalid_enum。
func TestRequiredEnums(t *testing.T) {
	if got, err := ParseAllocMode(Enum{Value: "defense", Known: true}); err != nil || got != engine.AllocMode("defense") {
		t.Errorf("mode 既知 = (%q, %v)", got, err)
	}
	if _, err := ParseAllocMode(Enum{}); codeOf(t, err) != CodeInvalidEnum {
		t.Errorf("mode 欠落 = %v", err)
	}
	if got, err := ParseSPGoalKind("goals[0]", Enum{Value: "outspeed", Known: true}); err != nil || got != engine.SPGoalOutspeed {
		t.Errorf("kind 既知 = (%q, %v)", got, err)
	}
	_, err := ParseSPGoalKind("goals[1]", Enum{Value: "x"})
	if codeOf(t, err) != CodeInvalidEnum || err.Error() != `goals[1].kind に未知の値 "x"` {
		t.Errorf("kind 未知 = %v", err)
	}
}

// 場は天候 → フィールドの順に見る。壁は指定した true だけが張られる。
func TestParseField(t *testing.T) {
	if got, err := ParseField(nil); err != nil || got != (engine.Field{}) {
		t.Errorf("nil = (%+v, %v)", got, err)
	}
	both := &Field{Weather: &Enum{Value: "w"}, Terrain: &Enum{Value: "t"}}
	if _, err := ParseField(both); err == nil || err.Error() != `field.weather に未知の値 "w"` {
		t.Errorf("天候が先でない: %v", err)
	}
	if _, err := ParseField(&Field{Terrain: &Enum{Value: "t"}}); codeOf(t, err) != CodeInvalidEnum {
		t.Errorf("フィールド未知 = %v", err)
	}
	yes, no := true, false
	got, err := ParseField(&Field{
		Weather: &Enum{Value: "rain", Known: true}, Terrain: &Enum{Value: "grassy", Known: true},
		AttackerScreens: &Screens{Reflect: &yes, LightScreen: &no},
	})
	want := engine.Field{
		Weather: engine.Weather("rain"), Terrain: engine.Terrain("grassy"),
		AttackerScreens: engine.Screens{Reflect: true},
	}
	if err != nil || got != want {
		t.Errorf("= (%+v, %v), want %+v", got, err, want)
	}
}
