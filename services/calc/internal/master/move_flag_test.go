package master

// ADR-0178: 技のフラグ(MasterMove.flags)と、特性の段階2の項目の取り込み。
//
// 受け入れ条件(calc-svc):
//   - AC-C1 FromExport は MasterMove.flags を共通マスタ経由で engine.Move.Flags(昇順)に写し、キーがあれば FlagsKnown を真にする。
//     キーが無い(古い pokedex-svc・取り込み前で省いた)ときは FlagsKnown 偽(不明)。空配列は既知のフラグなし。
//   - AC-C2 未知の値は ErrInvalidMaster(黙って捨てない)。
//   - AC-C3 Lookup(Move・Ability)は Flags と段階2の項目(スライス・map・ポインタ)のコピーを返す(呼び出し側の書き換えが漏れない)。
// データはすべて架空。

import (
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

func flagsPtr(fs ...string) *[]string {
	out := append([]string{}, fs...)
	return &out
}

// exportWithFlags は baseExport の技にフラグを足したもの。testbeam = 接触・パンチ(わざと逆順)、testwave = 既知のフラグなし、
// testunknown = フラグが不明(キーなし)。
func exportWithFlags(t *testing.T) api.MasterExport {
	t.Helper()
	ex := baseExport(t)
	for i := range ex.Moves {
		switch ex.Moves[i].Id {
		case "testbeam":
			ex.Moves[i].Flags = flagsPtr("punch", "contact")
		case "testwave":
			ex.Moves[i].Flags = flagsPtr()
		}
	}
	ex.Moves = append(ex.Moves, api.MasterMove{
		Id: "testunknown", NameJa: "テストふめい", Type: api.PokeTypeNormal, Category: api.Physical, Power: 80, Mechanisms: []string{},
	})
	return ex
}

func TestFromExportMapsMoveFlags(t *testing.T) {
	store := newStore(t, exportWithFlags(t))
	cases := []struct {
		id    string
		flags []engine.MoveFlag
		known bool
	}{
		{"testbeam", []engine.MoveFlag{engine.MoveFlagContact, engine.MoveFlagPunch}, true},
		{"testwave", nil, true},
		{"testunknown", nil, false},
	}
	for _, tc := range cases {
		mv, ok := store.Move(tc.id)
		if !ok {
			t.Fatalf("Move(%s) が見つからない", tc.id)
		}
		if mv.FlagsKnown != tc.known {
			t.Errorf("Move(%s).FlagsKnown = %v, want %v", tc.id, mv.FlagsKnown, tc.known)
		}
		if len(mv.Flags) != len(tc.flags) || (len(tc.flags) > 0 && !reflect.DeepEqual(mv.Flags, tc.flags)) {
			t.Errorf("Move(%s).Flags = %v, want %v", tc.id, mv.Flags, tc.flags)
		}
	}
}

func TestFromExportRejectsUnknownMoveFlag(t *testing.T) {
	ex := baseExport(t)
	ex.Moves[0].Flags = flagsPtr("protect")
	if _, err := FromExport(ex); !errors.Is(err, ErrInvalidMaster) {
		t.Fatalf("err = %v, want ErrInvalidMaster", err)
	}
}

func TestLookupReturnsCopiesOfMoveFlags(t *testing.T) {
	store := newStore(t, exportWithFlags(t))
	mv, _ := store.Move("testbeam")
	if len(mv.Flags) == 0 {
		t.Fatalf("testbeam のフラグが引けない: %+v", mv)
	}
	mv.Flags[0] = engine.MoveFlagSound
	again, _ := store.Move("testbeam")
	if again.Flags[0] != engine.MoveFlagContact {
		t.Errorf("Flags の書き換えが Store に漏れた: %v", again.Flags)
	}
}

func stage2Export(t *testing.T) api.MasterExport {
	t.Helper()
	export := baseExport(t)
	export.Abilities = append(export.Abilities,
		api.MasterAbility{Id: "teststage2", NameJa: "テスト段階2", Effect: effect(map[string]any{
			"PowerMods":          []any{map[string]any{"Condition": "move_flag", "Flag": "bite", "Modifier": 6144}},
			"PostAuraPowerMods":  []any{map[string]any{"Condition": "move_flag", "Flag": "sound", "Modifier": 5325}},
			"FlagTypeConvert":    map[string]any{"Flag": "sound", "To": "water"},
			"DefImmuneFlags":     []any{"bullet"},
			"DefFinalModsByFlag": map[string]any{"contact": 2048},
			"DefFinalModsByType": map[string]any{"fire": 8192},
			"NoContact":          true,
			"Breakable":          true,
		})},
	)
	return export
}

func TestFromExportCarriesStage2AbilityEffects(t *testing.T) {
	store := newStore(t, stage2Export(t))
	a, ok := store.Ability("teststage2")
	if !ok {
		t.Fatal("teststage2 が引けない(ロードで落ちたか)")
	}
	want := &engine.AbilityEffect{
		PowerMods:          []engine.ConditionalPowerMod{{Condition: engine.PowerConditionMoveFlag, Flag: engine.MoveFlagBite, Modifier: 6144}},
		PostAuraPowerMods:  []engine.ConditionalPowerMod{{Condition: engine.PowerConditionMoveFlag, Flag: engine.MoveFlagSound, Modifier: 5325}},
		FlagTypeConvert:    &engine.FlagTypeConvert{Flag: engine.MoveFlagSound, To: "water"},
		DefImmuneFlags:     []engine.MoveFlag{engine.MoveFlagBullet},
		DefFinalModsByFlag: map[engine.MoveFlag]int{engine.MoveFlagContact: 2048},
		DefFinalModsByType: map[engine.Type]int{"fire": 8192},
		NoContact:          true,
		Breakable:          true,
	}
	if !reflect.DeepEqual(a.Effect, want) {
		t.Errorf("Effect = %+v, want %+v", a.Effect, want)
	}
}

func TestLookupReturnsCopiesOfStage2AbilityEffects(t *testing.T) {
	store := newStore(t, stage2Export(t))
	a, ok := store.Ability("teststage2")
	if !ok || a.Effect == nil || a.Effect.FlagTypeConvert == nil || len(a.Effect.PostAuraPowerMods) == 0 || len(a.Effect.DefImmuneFlags) == 0 {
		t.Fatalf("teststage2 の段階2の項目が引けない: %+v", a.Effect)
	}
	a.Effect.PostAuraPowerMods[0].Modifier = 9999
	a.Effect.FlagTypeConvert.To = "fire"
	a.Effect.DefImmuneFlags[0] = engine.MoveFlagSound
	a.Effect.DefFinalModsByFlag[engine.MoveFlagContact] = 1
	a.Effect.DefFinalModsByType["fire"] = 1

	again, _ := store.Ability("teststage2")
	if got := again.Effect.PostAuraPowerMods[0].Modifier; got != 5325 {
		t.Errorf("PostAuraPowerMods の書き換えが Store に漏れた: %d", got)
	}
	if got := again.Effect.FlagTypeConvert.To; got != "water" {
		t.Errorf("FlagTypeConvert の書き換えが Store に漏れた: %q", got)
	}
	if got := again.Effect.DefImmuneFlags[0]; got != engine.MoveFlagBullet {
		t.Errorf("DefImmuneFlags の書き換えが Store に漏れた: %q", got)
	}
	if got := again.Effect.DefFinalModsByFlag[engine.MoveFlagContact]; got != 2048 {
		t.Errorf("DefFinalModsByFlag の書き換えが Store に漏れた: %d", got)
	}
	if got := again.Effect.DefFinalModsByType["fire"]; got != 8192 {
		t.Errorf("DefFinalModsByType の書き換えが Store に漏れた: %d", got)
	}
}
