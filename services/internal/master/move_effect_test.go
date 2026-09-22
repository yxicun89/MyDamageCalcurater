package master_test

// move_effects.effect の厳格デコード / 正準エンコードと、MoveRow からの写像(ADR-0107 決定4・決定5)。
// DB は使わない。作法は effects.go(item_effects / ability_effects)と同じものを踏襲する。

import (
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

// --- DecodeMoveEffect(受け付けるもの) ----------------------------------------

func TestDecodeMoveEffectAcceptsValidDefinitions(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want engine.MoveEffect
	}{
		{
			"必ず発動する自分の素早さ+1",
			`{"Chance":100,"Target":"self","Stages":{"spe":1}}`,
			engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatSpe: 1}},
		},
		{
			"必ず発動する自分の防御・特防-1",
			`{"Chance":100,"Target":"self","Stages":{"def":-1,"spd":-1}}`,
			engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatDef: -1, engine.StatSpD: -1}},
		},
		{
			"10%で自分の全能力+1",
			`{"Chance":10,"Target":"self","Stages":{"atk":1,"def":1,"spa":1,"spd":1,"spe":1}}`,
			engine.MoveEffect{Chance: 10, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{
				engine.StatAtk: 1, engine.StatDef: 1, engine.StatSpA: 1, engine.StatSpD: 1, engine.StatSpe: 1,
			}},
		},
		{
			"20%で相手の防御-1",
			`{"Chance":20,"Target":"target","Stages":{"def":-1}}`,
			engine.MoveEffect{Chance: 20, Target: engine.RankTargetOpponent, Stages: map[engine.StatKey]int{engine.StatDef: -1}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := master.DecodeMoveEffect([]byte(c.raw))
			if err != nil {
				t.Fatalf("DecodeMoveEffect: %v", err)
			}
			if got == nil {
				t.Fatal("DecodeMoveEffect = nil, want 効果")
			}
			if !reflect.DeepEqual(*got, c.want) {
				t.Fatalf("got = %+v, want %+v", *got, c.want)
			}
		})
	}
}

// --- DecodeMoveEffect(拒否するもの) ------------------------------------------

func TestDecodeMoveEffectRejectsInvalidDefinitions(t *testing.T) {
	cases := []struct{ name, raw string }{
		{"空のオブジェクト", `{}`},
		{"null", `null`},
		{"オブジェクトでない(配列)", `[{"Chance":100}]`},
		{"オブジェクトでない(数値)", `100`},
		{"壊れた JSON", `{"Chance":100,`},
		{"後続のデータがある", `{"Chance":100,"Target":"self","Stages":{"spe":1}} {"Chance":50}`},
		{"未知のフィールド", `{"Chance":100,"Target":"self","Stages":{"spe":1},"Boosts":{"spe":1}}`},
		{"大文字小文字違い(chance)", `{"chance":100,"Target":"self","Stages":{"spe":1}}`},
		{"大文字小文字違い(target)", `{"Chance":100,"target":"self","Stages":{"spe":1}}`},
		{"大文字小文字違い(stages)", `{"Chance":100,"Target":"self","stages":{"spe":1}}`},
		{"Chance が無い", `{"Target":"self","Stages":{"spe":1}}`},
		{"Target が無い", `{"Chance":100,"Stages":{"spe":1}}`},
		{"Stages が無い", `{"Chance":100,"Target":"self"}`},
		{"Chance が 0", `{"Chance":0,"Target":"self","Stages":{"spe":1}}`},
		{"Chance が負", `{"Chance":-10,"Target":"self","Stages":{"spe":1}}`},
		{"Chance が 101", `{"Chance":101,"Target":"self","Stages":{"spe":1}}`},
		{"Chance が小数", `{"Chance":12.5,"Target":"self","Stages":{"spe":1}}`},
		{"Chance が文字列", `{"Chance":"100","Target":"self","Stages":{"spe":1}}`},
		{"Target が未知", `{"Chance":100,"Target":"ally","Stages":{"spe":1}}`},
		{"Target が空文字", `{"Chance":100,"Target":"","Stages":{"spe":1}}`},
		{"Target が文字列でない", `{"Chance":100,"Target":1,"Stages":{"spe":1}}`},
		{"Stages が空", `{"Chance":100,"Target":"self","Stages":{}}`},
		{"Stages がオブジェクトでない", `{"Chance":100,"Target":"self","Stages":[1]}`},
		{"Stages のキーが hp", `{"Chance":100,"Target":"self","Stages":{"hp":1}}`},
		{"Stages のキーが accuracy(engine の Ranks に無い)", `{"Chance":100,"Target":"target","Stages":{"accuracy":-1}}`},
		{"Stages のキーが evasion(engine の Ranks に無い)", `{"Chance":100,"Target":"self","Stages":{"evasion":1}}`},
		{"Stages のキーが大文字", `{"Chance":100,"Target":"self","Stages":{"Spe":1}}`},
		{"変化量が 0", `{"Chance":100,"Target":"self","Stages":{"spe":0}}`},
		{"変化量が +7", `{"Chance":100,"Target":"self","Stages":{"spe":7}}`},
		{"変化量が -7", `{"Chance":100,"Target":"self","Stages":{"spe":-7}}`},
		{"変化量が小数", `{"Chance":100,"Target":"self","Stages":{"spe":1.5}}`},
		{"変化量が文字列", `{"Chance":100,"Target":"self","Stages":{"spe":"1"}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := master.DecodeMoveEffect([]byte(c.raw))
			if err == nil {
				t.Fatalf("DecodeMoveEffect(%s) = %+v, want error", c.raw, got)
			}
			if !errors.Is(err, master.ErrInvalidEffect) {
				t.Fatalf("err = %v, want ErrInvalidEffect でラップされた誤り", err)
			}
		})
	}
}

// --- EncodeMoveEffect(正準形) -------------------------------------------------

func TestEncodeMoveEffectIsCanonical(t *testing.T) {
	cases := []struct {
		name string
		e    engine.MoveEffect
		want string
	}{
		{
			"struct の定義順(Chance → Target → Stages)",
			engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatSpe: 1}},
			`{"Chance":100,"Target":"self","Stages":{"spe":1}}`,
		},
		{
			"Stages はキー昇順(map の反復順に依らない)",
			engine.MoveEffect{Chance: 10, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{
				engine.StatSpe: 1, engine.StatAtk: 1, engine.StatSpD: 1, engine.StatDef: 1, engine.StatSpA: 1,
			}},
			`{"Chance":10,"Target":"self","Stages":{"atk":1,"def":1,"spa":1,"spd":1,"spe":1}}`,
		},
		{
			"負の変化量",
			engine.MoveEffect{Chance: 100, Target: engine.RankTargetOpponent, Stages: map[engine.StatKey]int{engine.StatDef: -1}},
			`{"Chance":100,"Target":"target","Stages":{"def":-1}}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 何度呼んでも同じバイト列(map の反復順に左右されない)。
			for i := 0; i < 20; i++ {
				got, err := master.EncodeMoveEffect(c.e)
				if err != nil {
					t.Fatalf("EncodeMoveEffect: %v", err)
				}
				if string(got) != c.want {
					t.Fatalf("got = %s, want %s", got, c.want)
				}
			}
		})
	}
}

func TestEncodeMoveEffectRejectsInvalidValue(t *testing.T) {
	cases := []struct {
		name string
		e    engine.MoveEffect
	}{
		{"ゼロ値(何も持たない)", engine.MoveEffect{}},
		{"Stages が空", engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{}}},
		{"Chance が範囲外", engine.MoveEffect{Chance: 0, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatSpe: 1}}},
		{"Target が未知", engine.MoveEffect{Chance: 100, Target: engine.RankTarget("ally"), Stages: map[engine.StatKey]int{engine.StatSpe: 1}}},
		{"変化量が範囲外", engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatSpe: 7}}},
		{"HP", engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatHP: 1}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := master.EncodeMoveEffect(c.e)
			if err == nil {
				t.Fatalf("EncodeMoveEffect = %s, want error", got)
			}
			if !errors.Is(err, master.ErrInvalidEffect) {
				t.Fatalf("err = %v, want ErrInvalidEffect でラップされた誤り", err)
			}
		})
	}
}

// TestMoveEffectRoundTripsLosesNothing は Decode(Encode(e)) == e(情報が落ちない)。
func TestMoveEffectRoundTripsLosesNothing(t *testing.T) {
	values := []engine.MoveEffect{
		{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatSpe: 1}},
		{Chance: 1, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatSpA: -2}},
		{Chance: 70, Target: engine.RankTargetOpponent, Stages: map[engine.StatKey]int{engine.StatDef: -1, engine.StatSpD: -1}},
		{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{
			engine.StatAtk: 6, engine.StatDef: -6, engine.StatSpA: 2, engine.StatSpD: -2, engine.StatSpe: 1,
		}},
	}
	for _, want := range values {
		raw, err := master.EncodeMoveEffect(want)
		if err != nil {
			t.Fatalf("EncodeMoveEffect(%+v): %v", want, err)
		}
		got, err := master.DecodeMoveEffect(raw)
		if err != nil {
			t.Fatalf("DecodeMoveEffect(%s): %v", raw, err)
		}
		if !reflect.DeepEqual(*got, want) {
			t.Fatalf("往復で変わった: %s → %+v, want %+v", raw, *got, want)
		}
		// 正準形は冪等(Encode(Decode(Encode(e))) が同じバイト列)。
		again, err := master.EncodeMoveEffect(*got)
		if err != nil {
			t.Fatalf("EncodeMoveEffect(往復後): %v", err)
		}
		if string(again) != string(raw) {
			t.Fatalf("正準形が冪等でない: %s → %s", raw, again)
		}
	}
}

// --- MoveRow → engine.Move ----------------------------------------------------

// testMoveRow は move.go の既存テストと同じ最小の正しい行(効果なし)。
func testMoveRow() master.MoveRow {
	return master.MoveRow{ID: "testflame", NameJa: "テストかえん", Type: "fire", Category: "special", Power: 90, Priority: 0}
}

func TestMoveMapsEffectFromRow(t *testing.T) {
	chart := testChart(t)

	row := testMoveRow()
	row.Effect = []byte(`{"Chance":100,"Target":"self","Stages":{"spe":1}}`)
	got, err := master.Move(row, chart)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	if got.Effect == nil {
		t.Fatal("Move().Effect = nil, want 追加効果")
	}
	want := engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatSpe: 1}}
	if !reflect.DeepEqual(*got.Effect, want) {
		t.Fatalf("Effect = %+v, want %+v", *got.Effect, want)
	}
	// 既存の列の写像は変わっていない。
	if got.ID != row.ID || got.NameJa != row.NameJa || got.Power != row.Power || got.Priority != row.Priority {
		t.Fatalf("追加効果以外の写像が変わった: %+v", got)
	}
}

func TestMoveWithoutEffectHasNilEffect(t *testing.T) {
	chart := testChart(t)
	for _, raw := range [][]byte{nil, {}} {
		row := testMoveRow()
		row.Effect = raw
		got, err := master.Move(row, chart)
		if err != nil {
			t.Fatalf("Move(Effect=%q): %v", raw, err)
		}
		if got.Effect != nil {
			t.Fatalf("Effect = %+v, want nil(効果の行が無い技)", *got.Effect)
		}
	}
}

func TestMoveRejectsInvalidEffect(t *testing.T) {
	chart := testChart(t)
	row := testMoveRow()
	row.Effect = []byte(`{"Chance":100,"Target":"self","Stages":{"accuracy":-1}}`)
	if got, err := master.Move(row, chart); err == nil {
		t.Fatalf("Move = %+v, want error(不正な効果定義は行ごと不正)", got)
	} else if !errors.Is(err, master.ErrInvalidEffect) {
		t.Fatalf("err = %v, want ErrInvalidEffect でラップされた誤り", err)
	}
}
