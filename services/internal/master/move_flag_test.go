package master_test

// 技のフラグ(move_flags。ADR-0178)の検証と、特性の段階2の項目の効果定義(厳格デコード/正準エンコード)。
//
// 受け入れ条件(共通マスタ):
//   - AC-M1 語彙: master.AllMoveFlags は engine.AllMoveFlags と同じ 9 種(昇順・コピー)。IsMoveFlag は大文字小文字を区別する。
//   - AC-M2 master.Move: MoveRow.Flags を検証し、昇順にして engine.Move.Flags に、FlagsKnown を engine.Move.FlagsKnown に写す。
//     未知の値・重複・FlagsKnown が偽なのに値がある は ErrInvalidRow。フラグは他のフィールドに漏れない。
//   - AC-M3 DecodeAbilityEffect: 段階2の項目(PostAuraPowerMods・PowerMods の move_flag・FlagTypeConvert・DefImmuneFlags・
//     DefFinalModsByFlag・DefFinalModsByType・NoContact)を検証つきで読み、不正な値は ErrInvalidEffect。持ち物の効果には使えない。
//   - AC-M4 EncodeAbilityEffect: struct 定義順(… Breakable → PostAuraPowerMods → FlagTypeConvert → DefImmuneFlags →
//     DefFinalModsByFlag → DefFinalModsByType → NoContact → 未対応の印)で書き、往復で情報を落とさない。
//     入れ子のキーも定義順(Condition → MaxPower → MoveType → Flag → Modifier / Flag → To。ゼロ値は省く)。map はキーの昇順。
// データはすべて架空(タイプはテスト用の表の4種)。

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

// moveFlagVocabulary は ADR-0178 §1 の語彙(昇順)。実装の写しではなく ADR から独立に書く。
var moveFlagVocabulary = []string{"bite", "bullet", "contact", "pulse", "punch", "recoil", "secondary", "slicing", "sound"}

// --- AC-M1 語彙 -------------------------------------------------------------------

func TestAllMoveFlagsMatchesVocabulary(t *testing.T) {
	all := master.AllMoveFlags()
	got := make([]string, 0, len(all))
	for _, f := range all {
		got = append(got, string(f))
	}
	if !reflect.DeepEqual(got, moveFlagVocabulary) {
		t.Fatalf("AllMoveFlags = %v, want %v", got, moveFlagVocabulary)
	}
	if !reflect.DeepEqual(all, engine.AllMoveFlags()) {
		t.Fatalf("master と engine の一覧が違う: %v / %v", all, engine.AllMoveFlags())
	}
	for _, f := range moveFlagVocabulary {
		if !master.IsMoveFlag(f) {
			t.Errorf("IsMoveFlag(%q) = false", f)
		}
	}
	for _, f := range []string{"", "Contact", "protect", "wind"} {
		if master.IsMoveFlag(f) {
			t.Errorf("IsMoveFlag(%q) = true", f)
		}
	}
}

// --- AC-M2 master.Move -------------------------------------------------------------

func TestMoveMapsFlagsToEngine(t *testing.T) {
	c := testChart(t)
	base := master.MoveRow{ID: "testpunch", NameJa: "テストパンチ", Type: "fire", Category: "physical", Power: 75, Priority: 0}
	unknown, err := master.Move(base, c)
	if err != nil {
		t.Fatalf("Move(フラグ不明): %v", err)
	}
	if unknown.FlagsKnown || len(unknown.Flags) != 0 {
		t.Fatalf("フラグ不明の行: Flags=%v FlagsKnown=%v, want 空・偽", unknown.Flags, unknown.FlagsKnown)
	}

	cases := []struct {
		name  string
		flags []string
		want  []engine.MoveFlag
	}{
		{"既知のフラグなし", nil, nil},
		{"空配列", []string{}, nil},
		{"昇順に並べ替える", []string{"secondary", "punch", "contact"}, []engine.MoveFlag{"contact", "punch", "secondary"}},
		{"語彙のすべて", moveFlagVocabulary, []engine.MoveFlag{"bite", "bullet", "contact", "pulse", "punch", "recoil", "secondary", "slicing", "sound"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := base
			row.Flags = tc.flags
			row.FlagsKnown = true
			got, err := master.Move(row, c)
			if err != nil {
				t.Fatalf("Move: %v", err)
			}
			if !got.FlagsKnown {
				t.Error("FlagsKnown = false, want true")
			}
			if len(got.Flags) != len(tc.want) || (len(tc.want) > 0 && !reflect.DeepEqual(got.Flags, tc.want)) {
				t.Errorf("Flags = %v, want %v", got.Flags, tc.want)
			}
			// フラグ以外は不明の写像と同じ(他のフィールドに漏れない)。
			got.Flags, got.FlagsKnown = nil, false
			if !reflect.DeepEqual(got, unknown) {
				t.Errorf("フラグ以外のフィールドが変わった: %+v, want %+v", got, unknown)
			}
		})
	}

	t.Run("変化技もフラグを持てる(音の変化技)", func(t *testing.T) {
		row := master.MoveRow{ID: "testsing", NameJa: "テストうた", Type: "normal", Category: "status", Flags: []string{"sound"}, FlagsKnown: true}
		got, err := master.Move(row, c)
		if err != nil {
			t.Fatalf("Move: %v", err)
		}
		if !reflect.DeepEqual(got.Flags, []engine.MoveFlag{"sound"}) {
			t.Errorf("Flags = %v, want [sound]", got.Flags)
		}
	})

	rejects := []struct {
		name  string
		flags []string
		known bool
	}{
		{"未知の値", []string{"protect"}, true},
		{"大文字違い", []string{"Contact"}, true},
		{"空文字", []string{""}, true},
		{"重複", []string{"contact", "contact"}, true},
		{"不明なのに値がある", []string{"contact"}, false},
	}
	for _, tc := range rejects {
		t.Run("拒否/"+tc.name, func(t *testing.T) {
			row := base
			row.Flags = tc.flags
			row.FlagsKnown = tc.known
			if _, err := master.Move(row, c); !errors.Is(err, master.ErrInvalidRow) {
				t.Errorf("err = %v, want ErrInvalidRow", err)
			}
		})
	}
}

// master.Move の結果は呼び出し側の MoveRow.Flags と別のスライス(書き換えが漏れない)。
func TestMoveFlagsAreCopied(t *testing.T) {
	c := testChart(t)
	flags := []string{"contact", "punch"}
	row := master.MoveRow{ID: "testpunch", NameJa: "テストパンチ", Type: "fire", Category: "physical", Power: 75, Flags: flags, FlagsKnown: true}
	got, err := master.Move(row, c)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}
	flags[0] = "sound"
	if len(got.Flags) == 0 || got.Flags[0] != "contact" {
		t.Errorf("Flags = %v, want [contact punch](行のスライスを共有している)", got.Flags)
	}
}

// --- AC-M3 デコード --------------------------------------------------------------

func flagMod(f engine.MoveFlag, mod int) engine.ConditionalPowerMod {
	return engine.ConditionalPowerMod{Condition: engine.PowerConditionMoveFlag, Flag: f, Modifier: mod}
}

func TestDecodeAbilityEffectStage2(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		name string
		raw  string
		want engine.AbilityEffect
	}{
		{"威力の条件(フラグ・オーラの前)", `{"PowerMods":[{"Condition":"move_flag","Flag":"bite","Modifier":6144}]}`,
			engine.AbilityEffect{PowerMods: []engine.ConditionalPowerMod{flagMod("bite", 6144)}}},
		{"威力の条件(オーラの後)", `{"PostAuraPowerMods":[{"Condition":"move_flag","Flag":"contact","Modifier":5325}]}`,
			engine.AbilityEffect{PostAuraPowerMods: []engine.ConditionalPowerMod{flagMod("contact", 5325)}}},
		{"オーラの後に既存の条件も書ける", `{"PostAuraPowerMods":[{"Condition":"move_type","MoveType":"fire","Modifier":5325}]}`,
			engine.AbilityEffect{PostAuraPowerMods: []engine.ConditionalPowerMod{{Condition: engine.PowerConditionMoveType, MoveType: "fire", Modifier: 5325}}}},
		{"両側(パンクロック相当)", `{"PostAuraPowerMods":[{"Condition":"move_flag","Flag":"sound","Modifier":5325}],"DefFinalModsByFlag":{"sound":2048},"Breakable":true}`,
			engine.AbilityEffect{PostAuraPowerMods: []engine.ConditionalPowerMod{flagMod("sound", 5325)}, DefFinalModsByFlag: map[engine.MoveFlag]int{"sound": 2048}, Breakable: true}},
		{"フラグでタイプ変換", `{"FlagTypeConvert":{"Flag":"sound","To":"water"}}`,
			engine.AbilityEffect{FlagTypeConvert: &engine.FlagTypeConvert{Flag: "sound", To: "water"}}},
		{"フラグで無効", `{"DefImmuneFlags":["bullet"],"Breakable":true}`,
			engine.AbilityEffect{DefImmuneFlags: []engine.MoveFlag{"bullet"}, Breakable: true}},
		{"フラグ・タイプの最終補正", `{"DefFinalModsByFlag":{"contact":2048},"DefFinalModsByType":{"fire":8192},"Breakable":true}`,
			engine.AbilityEffect{DefFinalModsByFlag: map[engine.MoveFlag]int{"contact": 2048}, DefFinalModsByType: map[engine.Type]int{"fire": 8192}, Breakable: true}},
		{"接触しない扱い", `{"NoContact":true}`, engine.AbilityEffect{NoContact: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := master.DecodeAbilityEffect([]byte(tc.raw), c)
			if err != nil {
				t.Fatalf("DecodeAbilityEffect = %v", err)
			}
			if !reflect.DeepEqual(*got, tc.want) {
				t.Errorf("got %+v, want %+v", *got, tc.want)
			}
		})
	}
}

func TestDecodeAbilityEffectStage2Rejects(t *testing.T) {
	c := testChart(t)
	over := engine.MaxEffectModifier + 1
	cases := []struct{ name, raw string }{
		{"move_flag に Flag が無い", `{"PowerMods":[{"Condition":"move_flag","Modifier":6144}]}`},
		{"move_flag の Flag が語彙に無い", `{"PowerMods":[{"Condition":"move_flag","Flag":"protect","Modifier":6144}]}`},
		{"move_flag の Flag の大文字小文字違い", `{"PowerMods":[{"Condition":"move_flag","Flag":"Bite","Modifier":6144}]}`},
		{"move_flag に MaxPower がある", `{"PowerMods":[{"Condition":"move_flag","Flag":"bite","MaxPower":60,"Modifier":6144}]}`},
		{"move_flag に MoveType がある", `{"PowerMods":[{"Condition":"move_flag","Flag":"bite","MoveType":"fire","Modifier":6144}]}`},
		{"max_base_power に Flag がある", `{"PowerMods":[{"Condition":"max_base_power","MaxPower":60,"Flag":"bite","Modifier":6144}]}`},
		{"PostAuraPowerMods が空", `{"PostAuraPowerMods":[]}`},
		{"PostAuraPowerMods が配列でない", `{"PostAuraPowerMods":{"Condition":"move_flag","Flag":"contact","Modifier":5325}}`},
		{"PostAuraPowerMods の Modifier が中立", `{"PostAuraPowerMods":[{"Condition":"move_flag","Flag":"contact","Modifier":4096}]}`},
		{"PostAuraPowerMods の Modifier が上限超え", fmt.Sprintf(`{"PostAuraPowerMods":[{"Condition":"move_flag","Flag":"contact","Modifier":%d}]}`, over)},
		{"PostAuraPowerMods の同じ要素が2つ", `{"PostAuraPowerMods":[{"Condition":"move_flag","Flag":"contact","Modifier":5325},{"Condition":"move_flag","Flag":"contact","Modifier":5325}]}`},
		{"PostAuraPowerMods の要素に未知のキー", `{"PostAuraPowerMods":[{"Condition":"move_flag","Flag":"contact","Modifier":5325,"Note":"x"}]}`},
		{"FlagTypeConvert が null", `{"FlagTypeConvert":null}`},
		{"FlagTypeConvert の Flag が無い", `{"FlagTypeConvert":{"To":"water"}}`},
		{"FlagTypeConvert の To が無い", `{"FlagTypeConvert":{"Flag":"sound"}}`},
		{"FlagTypeConvert の To が表に無い", `{"FlagTypeConvert":{"Flag":"sound","To":"fairy"}}`},
		{"FlagTypeConvert の Flag が語彙に無い", `{"FlagTypeConvert":{"Flag":"wind","To":"water"}}`},
		{"FlagTypeConvert に未知のキー", `{"FlagTypeConvert":{"Flag":"sound","To":"water","PowerMod":4915}}`},
		{"DefImmuneFlags が空", `{"DefImmuneFlags":[]}`},
		{"DefImmuneFlags が配列でない", `{"DefImmuneFlags":"sound"}`},
		{"DefImmuneFlags の値が語彙に無い", `{"DefImmuneFlags":["wind"]}`},
		{"DefImmuneFlags の重複", `{"DefImmuneFlags":["sound","sound"]}`},
		{"DefFinalModsByFlag が空", `{"DefFinalModsByFlag":{}}`},
		{"DefFinalModsByFlag のキーが語彙に無い", `{"DefFinalModsByFlag":{"wind":2048}}`},
		{"DefFinalModsByFlag の値が 0", `{"DefFinalModsByFlag":{"contact":0}}`},
		{"DefFinalModsByFlag の値が上限超え", fmt.Sprintf(`{"DefFinalModsByFlag":{"contact":%d}}`, over)},
		{"DefFinalModsByType が空", `{"DefFinalModsByType":{}}`},
		{"DefFinalModsByType のタイプが表に無い", `{"DefFinalModsByType":{"fairy":8192}}`},
		{"DefFinalModsByType の値が 0", `{"DefFinalModsByType":{"fire":0}}`},
		{"NoContact が false", `{"NoContact":false}`},
		{"項目名の大文字小文字違い", `{"noContact":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := master.DecodeAbilityEffect([]byte(tc.raw), c); !errors.Is(err, master.ErrInvalidEffect) {
				t.Errorf("err = %v, want ErrInvalidEffect", err)
			}
		})
	}
}

// 段階2の項目は特性だけのもの(持ち物の効果には使えない)。
func TestDecodeItemEffectRejectsStage2AbilityFields(t *testing.T) {
	c := testChart(t)
	for _, raw := range []string{
		`{"PostAuraPowerMods":[{"Condition":"move_flag","Flag":"contact","Modifier":5325}]}`,
		`{"FlagTypeConvert":{"Flag":"sound","To":"water"}}`,
		`{"DefImmuneFlags":["sound"]}`,
		`{"DefFinalModsByFlag":{"contact":2048}}`,
		`{"DefFinalModsByType":{"fire":8192}}`,
		`{"NoContact":true}`,
	} {
		if _, err := master.DecodeItemEffect([]byte(raw), c); !errors.Is(err, master.ErrInvalidEffect) {
			t.Errorf("%s: err = %v, want ErrInvalidEffect", raw, err)
		}
	}
}

// --- AC-M4 エンコード --------------------------------------------------------------

func TestEncodeAbilityEffectStage2Canonical(t *testing.T) {
	c := testChart(t)
	e := engine.AbilityEffect{
		PowerMods:           []engine.ConditionalPowerMod{flagMod("bite", 6144)},
		Breakable:           true,
		PostAuraPowerMods:   []engine.ConditionalPowerMod{flagMod("sound", 5325)},
		FlagTypeConvert:     &engine.FlagTypeConvert{Flag: "sound", To: "water"},
		DefImmuneFlags:      []engine.MoveFlag{"bullet", "sound"},
		DefFinalModsByFlag:  map[engine.MoveFlag]int{"sound": 2048, "contact": 2048},
		DefFinalModsByType:  map[engine.Type]int{"fire": 8192},
		NoContact:           true,
		UnsupportedAttacker: true,
	}
	got, err := master.EncodeAbilityEffect(e)
	if err != nil {
		t.Fatalf("EncodeAbilityEffect: %v", err)
	}
	want := `{"PowerMods":[{"Condition":"move_flag","Flag":"bite","Modifier":6144}],"Breakable":true,` +
		`"PostAuraPowerMods":[{"Condition":"move_flag","Flag":"sound","Modifier":5325}],` +
		`"FlagTypeConvert":{"Flag":"sound","To":"water"},"DefImmuneFlags":["bullet","sound"],` +
		`"DefFinalModsByFlag":{"contact":2048,"sound":2048},"DefFinalModsByType":{"fire":8192},"NoContact":true,` +
		`"UnsupportedAttacker":true}`
	if string(got) != want {
		t.Errorf("Encode =\n%s\nwant\n%s", got, want)
	}
	back, err := master.DecodeAbilityEffect(got, c)
	if err != nil {
		t.Fatalf("Decode(Encode(e)) = %v", err)
	}
	if !reflect.DeepEqual(*back, e) {
		t.Errorf("Decode(Encode(e)) = %+v, want %+v(情報が落ちた)", *back, e)
	}
}

func TestAbilityEffectStage2RoundTripEachField(t *testing.T) {
	c := testChart(t)
	for _, e := range []engine.AbilityEffect{
		{PowerMods: []engine.ConditionalPowerMod{flagMod("pulse", 6144)}},
		{PostAuraPowerMods: []engine.ConditionalPowerMod{flagMod("recoil", 4915)}},
		{FlagTypeConvert: &engine.FlagTypeConvert{Flag: "sound", To: "water"}},
		{DefImmuneFlags: []engine.MoveFlag{"sound"}, Breakable: true},
		{DefFinalModsByFlag: map[engine.MoveFlag]int{"contact": 2048}, Breakable: true},
		{DefFinalModsByType: map[engine.Type]int{"fire": 8192}},
		{NoContact: true},
	} {
		raw, err := master.EncodeAbilityEffect(e)
		if err != nil {
			t.Errorf("%+v: Encode = %v", e, err)
			continue
		}
		back, err := master.DecodeAbilityEffect(raw, c)
		if err != nil {
			t.Errorf("%s: Decode = %v", raw, err)
			continue
		}
		if !reflect.DeepEqual(*back, e) {
			t.Errorf("%s: 往復 = %+v, want %+v", raw, *back, e)
		}
	}
}
