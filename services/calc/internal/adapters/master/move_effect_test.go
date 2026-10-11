package master

// MasterExport の MasterMove.effect(技の追加効果)が engine.Move.Effect まで通ること(ADR-0107 決定7)。
// 検証は共通マスタ(services/internal/master の DecodeMoveEffect)に委ねる。ここで見るのは配線。

import (
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
	sharedmaster "example.com/pokecalc/services/internal/master"
)

// withMoveEffects は baseExport の技に追加効果を足す(testbeam のみ。testwave は効果なしのまま)。
func withMoveEffects(t *testing.T) api.MasterExport {
	t.Helper()
	e := baseExport(t)
	for i := range e.Moves {
		if e.Moves[i].Id == "testbeam" {
			e.Moves[i].Effect = effect(map[string]any{
				"Chance": 100, "Target": "self", "Stages": map[string]any{"spe": 1},
			})
		}
	}
	return e
}

func TestFromExportCarriesMoveEffect(t *testing.T) {
	store := newStore(t, withMoveEffects(t))

	mv, ok := store.Move("testbeam")
	if !ok {
		t.Fatal("Move(testbeam) が引けない")
	}
	if mv.Effect == nil {
		t.Fatal("Move(testbeam).Effect = nil, want 追加効果")
	}
	want := engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatSpe: 1}}
	if !reflect.DeepEqual(*mv.Effect, want) {
		t.Fatalf("Effect = %+v, want %+v", *mv.Effect, want)
	}

	// effect が無い技は nil(空の MoveEffect を作らない)。
	plain, ok := store.Move("testwave")
	if !ok {
		t.Fatal("Move(testwave) が引けない")
	}
	if plain.Effect != nil {
		t.Fatalf("effect の無い技の Effect = %+v, want nil", *plain.Effect)
	}
}

// TestFromExportRejectsInvalidMoveEffect は不正な効果定義でマスタごと読み込みを失敗させること
// (持ち物・特性の効果と同じ扱い。ADR-0204 §2)。
func TestFromExportRejectsInvalidMoveEffect(t *testing.T) {
	tests := []struct {
		name string
		eff  map[string]any
	}{
		{"空", map[string]any{}},
		{"未知のフィールド", map[string]any{"Chance": 100, "Target": "self", "Stages": map[string]any{"spe": 1}, "Note": "x"}},
		{"Target が未知", map[string]any{"Chance": 100, "Target": "ally", "Stages": map[string]any{"spe": 1}}},
		{"Chance が範囲外", map[string]any{"Chance": 0, "Target": "self", "Stages": map[string]any{"spe": 1}}},
		{"Stages が空", map[string]any{"Chance": 100, "Target": "self", "Stages": map[string]any{}}},
		{"Stages のキーが accuracy", map[string]any{"Chance": 100, "Target": "target", "Stages": map[string]any{"accuracy": -1}}},
		{"変化量が範囲外", map[string]any{"Chance": 100, "Target": "self", "Stages": map[string]any{"spe": 7}}},
		{"持ち物の効果定義を入れている", map[string]any{"DamageMod": 5324}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := baseExport(t)
			for i := range e.Moves {
				if e.Moves[i].Id == "testbeam" {
					e.Moves[i].Effect = effect(tt.eff)
				}
			}
			_, err := FromExport(e)
			if err == nil {
				t.Fatal("FromExport = nil, want error")
			}
			if !errors.Is(err, ErrInvalidMaster) {
				t.Fatalf("err = %v, want ErrInvalidMaster でラップされた誤り", err)
			}
			if !errors.Is(err, sharedmaster.ErrInvalidEffect) {
				t.Fatalf("err = %v, want 共通マスタの ErrInvalidEffect も連ねる", err)
			}
		})
	}
}

// TestStoreMoveDoesNotAliasEffect は Store が返す Move の Effect を呼び出し側が書き換えても
// Store の中身が変わらないこと(Item/Ability と同じ約束。master.go の doc)。
func TestStoreMoveDoesNotAliasEffect(t *testing.T) {
	store := newStore(t, withMoveEffects(t))
	first, _ := store.Move("testbeam")
	if first.Effect == nil {
		t.Fatal("Effect = nil")
	}
	first.Effect.Chance = 1
	first.Effect.Stages[engine.StatSpe] = -6

	second, _ := store.Move("testbeam")
	want := engine.MoveEffect{Chance: 100, Target: engine.RankTargetSelf, Stages: map[engine.StatKey]int{engine.StatSpe: 1}}
	if !reflect.DeepEqual(*second.Effect, want) {
		t.Fatalf("呼び出し側の書き換えが Store に漏れた: %+v, want %+v", *second.Effect, want)
	}
}
