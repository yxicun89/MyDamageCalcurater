package domain

// engine を呼ぶ前の入力の検証(観測・対戦の状態・メガ持ち物・個体)の境界値テスト。

import (
	"testing"

	"example.com/pokecalc/engine"
)

// 観測: 0 件は no_observation。各要素は 3 つのうちちょうど 1 つで、値域の外は invalid_observation。
func TestConvertObservationsBoundaries(t *testing.T) {
	if _, err := ConvertObservations(nil); codeOf(t, err) != CodeNoObservation {
		t.Errorf("0 件: %v", err)
	}
	note := "メモ"
	ok := []Observation{
		{Percent: intp(1)}, {Percent: intp(100)},
		{PercentTenths: intp(1)}, {PercentTenths: intp(1000)},
		{Damage: intp(1), Note: &note},
	}
	got, err := ConvertObservations(ok)
	if err != nil {
		t.Fatalf("境界の内側: %v", err)
	}
	want := []engine.Observation{
		{Percent: 1}, {Percent: 100}, {PercentTenths: 1}, {PercentTenths: 1000}, {Damage: 1, Note: note},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("観測 %d = %+v, want %+v", i+1, got[i], want[i])
		}
	}
	for name, o := range map[string]Observation{
		"指定なし":               {},
		"2つ指定":               {Percent: intp(10), Damage: intp(10)},
		"percent 0":          {Percent: intp(0)},
		"percent 101":        {Percent: intp(101)},
		"percentTenths 0":    {PercentTenths: intp(0)},
		"percentTenths 1001": {PercentTenths: intp(1001)},
		"damage 0":           {Damage: intp(0)},
	} {
		if _, err := ConvertObservations([]Observation{{Damage: intp(5)}, o}); codeOf(t, err) != CodeInvalidObservation {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// 対戦の状態: nil はゼロ値、指定した 0・負は invalid_input、1 以上はそのまま。
func TestBattleStateFromBoundaries(t *testing.T) {
	if got, err := BattleStateFrom(nil); err != nil || got != (engine.BattleState{}) {
		t.Errorf("nil = (%+v, %v)", got, err)
	}
	got, err := BattleStateFrom(&BattleState{AttackerCurrentHP: intp(1), Hits: intp(2)})
	if err != nil || got != (engine.BattleState{AttackerCurrentHP: 1, Hits: 2}) {
		t.Errorf("= (%+v, %v)", got, err)
	}
	for _, b := range []*BattleState{{AttackerCurrentHP: intp(0)}, {DefenderCurrentHP: intp(-1)}, {Hits: intp(0)}} {
		if _, err := BattleStateFrom(b); codeOf(t, err) != CodeInvalidInput {
			t.Errorf("%+v: %v", b, err)
		}
	}
}

// fakeMegaRules は "mega-x" だけがメガ種族(必要な持ち物 "stone-x")、"mega-none" は持ち物を持てないメガ種族。
type fakeMegaRules struct{}

func (fakeMegaRules) MegaRequiredItem(key string) (string, bool) {
	switch key {
	case "mega-x":
		return "stone-x", true
	case "mega-none":
		return "", true
	}
	return "", false
}

func TestCheckMegaItem(t *testing.T) {
	item := func(id string) *engine.Item { return &engine.Item{ID: id} }
	tests := []struct {
		name    string
		species string
		item    *engine.Item
		ok      bool
	}{
		{"持ち物なし", "mega-x", nil, true},
		{"メガストーン", "mega-x", item("stone-x"), true},
		{"別の持ち物", "mega-x", item("other"), false},
		{"持てないメガ", "mega-none", item("stone-x"), false},
		{"メガでない", "plain", item("other"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckMegaItem(fakeMegaRules{}, "攻撃側", engine.Species{Key: tt.species}, tt.item)
			if tt.ok != (err == nil) || (!tt.ok && codeOf(t, err) != CodeInvalidInput) {
				t.Errorf("err = %v, want ok=%v", err, tt.ok)
			}
		})
	}
	// 候補は添字付きで最初の失敗を返す。
	err := CheckMegaItems(fakeMegaRules{}, "itemCandidates", engine.Species{Key: "mega-x"},
		[]*engine.Item{nil, item("stone-x"), item("a"), item("b")})
	if err == nil || err.Error() != `itemCandidates[2]: メガシンカ後の種族 "mega-x" には持ち物 "a" を持たせられない(持てるのはメガストーン "stone-x" だけ)` {
		t.Errorf("CheckMegaItems = %v", err)
	}
}

func TestValidateIndividual(t *testing.T) {
	base := engine.Individual{Species: engine.Species{Types: []engine.Type{engine.TypeNormal}, BaseStats: engine.Stats{HP: 80, Atk: 80, Def: 80, SpA: 80, SpD: 80, Spe: 80}}}
	if err := ValidateIndividual("攻撃側", base); err != nil {
		t.Errorf("妥当な個体: %v", err)
	}
	bad := base
	bad.SP = engine.Stats{Atk: engine.MaxSPPerStat + 1}
	if err := ValidateIndividual("攻撃側", bad); codeOf(t, err) != CodeInvalidInput {
		t.Errorf("SP の範囲外: %v", err)
	}
}
