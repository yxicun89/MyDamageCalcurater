package domain

// 列挙の検証(invalid_enum)。どの値が契約の列挙に含まれるかは契約(api/openapi.yaml)が決めるので、
// その判定は adapters が生成型の Valid() で行い、結果を Enum.Known に入れて渡す(ADR-0252)。
// ここは「どの順で・どの文言で失敗にするか」だけを持つ(engine/wasmapi と同じ失敗にするため。ADR-0200 §4)。

import "example.com/pokecalc/engine"

// Enum は契約の列挙として受け取った値。Known は Value が契約の列挙に含まれるか。
type Enum struct {
	Value string
	Known bool
}

// ParseFormat は形式を検証する(必須。欠落は空文字で Known=false として届く)。
func ParseFormat(f Enum) (engine.Format, error) {
	if !f.Known {
		return "", Errorf(CodeInvalidEnum, "format に未知の値 %q", f.Value)
	}
	return engine.Format(f.Value), nil
}

// ParseStatus は状態異常を検証する(省略は状態異常なし)。label は入力の場所(例 "attacker")。
func ParseStatus(label string, s *Enum) (engine.Status, error) {
	if s == nil {
		return engine.StatusNone, nil
	}
	if !s.Known {
		return "", Errorf(CodeInvalidEnum, "%s.status に未知の値 %q", label, s.Value)
	}
	return engine.Status(s.Value), nil
}

// ParseTeraType はテラスタイプを検証する(省略はテラスタルしない)。
func ParseTeraType(label string, t *Enum) (engine.Type, error) {
	if t == nil {
		return engine.TypeNone, nil
	}
	if !t.Known {
		return "", Errorf(CodeInvalidEnum, "%s.teraType に未知の値 %q", label, t.Value)
	}
	return engine.Type(t.Value), nil
}

// Screens は壁の指定(省略は張っていない)。
type Screens struct {
	Reflect, LightScreen, AuroraVeil *bool
}

// Field は場の状態の指定(weather・terrain は列挙。省略は既定値)。
type Field struct {
	Weather, Terrain                 *Enum
	AttackerScreens, DefenderScreens *Screens
}

// ParseField は場の状態を検証して engine.Field にする(nil は既定の場)。
func ParseField(f *Field) (engine.Field, error) {
	if f == nil {
		return engine.Field{}, nil
	}
	weather := engine.WeatherNone
	if f.Weather != nil {
		if !f.Weather.Known {
			return engine.Field{}, Errorf(CodeInvalidEnum, "field.weather に未知の値 %q", f.Weather.Value)
		}
		weather = engine.Weather(f.Weather.Value)
	}
	terrain := engine.TerrainNone
	if f.Terrain != nil {
		if !f.Terrain.Known {
			return engine.Field{}, Errorf(CodeInvalidEnum, "field.terrain に未知の値 %q", f.Terrain.Value)
		}
		terrain = engine.Terrain(f.Terrain.Value)
	}
	return engine.Field{
		Weather: weather, Terrain: terrain,
		AttackerScreens: screensFrom(f.AttackerScreens), DefenderScreens: screensFrom(f.DefenderScreens),
	}, nil
}

func screensFrom(s *Screens) engine.Screens {
	if s == nil {
		return engine.Screens{}
	}
	return engine.Screens{Reflect: derefBool(s.Reflect), LightScreen: derefBool(s.LightScreen), AuroraVeil: derefBool(s.AuroraVeil)}
}

func derefBool(p *bool) bool { return p != nil && *p }
