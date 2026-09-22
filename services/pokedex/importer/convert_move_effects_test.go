package importer_test

// 技の追加効果(命中時のランク変化)の変換(ADR-0107 決定6)。
// 取得元は Showdown だけ(@smogon/calc は `secondaries: true` の真偽値しか持たず、
// Close Combat のように欠落もあるため突き合わせない。ADR-0107「調査」)。
//
// 架空データ(testdata/fictional)の Showdown スナップショットが持つ形:
//   testflame   secondary {chance 10, self.boosts {spa +1}}      → 確率つきの自分のランク変化
//   teststrike  self.boosts {def -1, spe +1}                     → 必ず発動(確率 100 に正規化)
//   testrevived secondary {chance 20, boosts {def -1}}           → 相手のランク変化
//   testsplash  secondary {chance 30, boosts {accuracy -1}}      → engine に持ち場が無い → 落として警告
//   testbanned  self.boosts {atk +1}                             → moves 表に採らない技 → 行を作らない
//   testglare / testold                                          → 追加効果なし

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/importer"
)

func moveEffectsByID(out importer.Output) map[string]string {
	m := map[string]string{}
	for _, r := range out.MoveEffects {
		m[r.ID] = string(r.Effect)
	}
	return m
}

func TestConvertMoveEffectsFromShowdown(t *testing.T) {
	out, rep := convertOK(t, loadFixture(t))

	// 値は master.EncodeMoveEffect の正準形(ADR-0107 決定4)。
	want := map[string]string{
		"testflame":   `{"Chance":10,"Target":"self","Stages":{"spa":1}}`,
		"teststrike":  `{"Chance":100,"Target":"self","Stages":{"def":-1,"spe":1}}`,
		"testrevived": `{"Chance":20,"Target":"target","Stages":{"def":-1}}`,
	}
	if got := moveEffectsByID(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("move_effects = %v,\nwant %v", got, want)
	}

	// accuracy / evasion しか動かさない追加効果は落とす(engine の Ranks に持ち場が無い)。
	// 黙って落とさず、報告に残す。
	if !hasFinding(rep.Warnings, importer.KindMoveEffectUnsupportedStat, "testsplash") {
		t.Errorf("accuracy だけの追加効果(testsplash)が Warnings(%s)に無い", importer.KindMoveEffectUnsupportedStat)
	}
}

// TestConvertMoveEffectsAreSortedByID は行が ID 昇順であること(投入の決定性。ADR-0101 §8)。
func TestConvertMoveEffectsAreSortedByID(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	ids := make([]string, 0, len(out.MoveEffects))
	for _, r := range out.MoveEffects {
		ids = append(ids, r.ID)
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatalf("move_effects が ID 昇順でない: %v", ids)
	}
}

// TestConvertMoveEffectsOnlyForIncludedMoves は、moves 表に採らなかった技の効果を作らないこと。
// testbanned は isNonstandard が null でないため技そのものが除外される(ADR-0101 §4 規則1)。
func TestConvertMoveEffectsOnlyForIncludedMoves(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	moves := movesByID(out)
	for _, r := range out.MoveEffects {
		if _, ok := moves[r.ID]; !ok {
			t.Errorf("moves 表に無い技 %q の効果を作っている(外部キーが張れない)", r.ID)
		}
	}
	if _, ok := moveEffectsByID(out)["testbanned"]; ok {
		t.Errorf("取り込まない技 testbanned の効果が作られている")
	}
}

// TestConvertMoveEffectsAbsentWhenSourceHasNone は、取得元に追加効果の情報が無い技は
// 行を作らない(空の {} の行を作らない)こと。
func TestConvertMoveEffectsAbsentWhenSourceHasNone(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	got := moveEffectsByID(out)
	for _, id := range []string{"testglare", "testsplash"} {
		if raw, ok := got[id]; ok {
			t.Errorf("追加効果を持たない技 %q の行がある: %s", id, raw)
		}
	}
}

// TestConvertBlocksOnAmbiguousMoveEffect は、1つの技が2つ以上のランク変化エントリを持つときに止めること。
// ADR-0107 決定3 は「取得元に2エントリ以上の技は 1 件も無い」という実測に依っている。
// その前提が崩れたら、黙って片方を落とさずに人に知らせる。
func TestConvertBlocksOnAmbiguousMoveEffect(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, in *importer.Input)
	}{
		{
			"トップレベルの self.boosts と secondary の両方がある",
			func(t *testing.T, in *importer.Input) {
				m := showdownMove(t, in, "testrevived")
				m.Self = &importer.ShowdownBoosts{Boosts: map[string]int{"atk": 1}}
			},
		},
		{
			"secondary が自分と相手の両方を上げ下げする",
			func(t *testing.T, in *importer.Input) {
				m := showdownMove(t, in, "testflame")
				m.Secondary.Boosts = map[string]int{"def": -1}
			},
		},
		{
			"secondaries が2件以上ある(取得元が配列で複数持っている)",
			func(t *testing.T, in *importer.Input) {
				showdownMove(t, in, "testflame").Secondaries = []importer.ShowdownSecondary{
					{Chance: 10, Boosts: map[string]int{"def": -1}},
					{Chance: 10, Boosts: map[string]int{"spd": -1}},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := loadFixture(t)
			tt.mutate(t, &in)
			_, rep, err := importer.Convert(in)
			if err != nil {
				return // 変換自体を止めるのも「黙って落とさない」を満たす
			}
			found := false
			for _, f := range rep.Blockers {
				if f.Kind == importer.KindMoveEffectAmbiguous {
					found = true
				}
			}
			if !found {
				t.Fatalf("2エントリ以上の追加効果を素通りさせた。Blockers に %s が無い: %+v",
					importer.KindMoveEffectAmbiguous, rep.Blockers)
			}
		})
	}
}

// TestConvertDoesNotBlockOnNonBoostSecondaries は、secondaries 配列が2件以上あっても、
// どちらも boosts(自分・相手とも)を持たなければ blocker にしないこと。実データの
// firefang/icefang/thunderfang/triplearrows は secondaries を2件持つが、中身は火傷/氷結/麻痺/
// ひるみ等でランク変化を伴わない(ADR-0107 2026-09-23 追記)。決定3 の前提が脅かされるのは
// 「ランク変化エントリ」が2件以上のときだけで、boosts の無い副次効果はその対象外。
func TestConvertDoesNotBlockOnNonBoostSecondaries(t *testing.T) {
	in := loadFixture(t)
	// 状態異常・ひるみ等は boosts を持たない(Showdown の secondary は chance だけで表現される)。
	showdownMove(t, &in, "testflame").Secondaries = []importer.ShowdownSecondary{
		{Chance: 10},
		{Chance: 10},
	}
	_, rep, err := importer.Convert(in)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if hasFinding(rep.Blockers, importer.KindMoveEffectAmbiguous, "testflame") {
		t.Fatalf("boosts を持たない secondaries を blocker にした: %+v", rep.Blockers)
	}
}

// TestConvertWarnsOnUnsupportedStatMixedWithSupported は、accuracy/evasion が atk 等の
// engine が持つステータスと同じエントリに混ざっていても、警告 KindMoveEffectUnsupportedStat を
// 必ず残すこと(黙って落とさない。engine が持つステータスの行は作る。2026-09-23 追記)。
func TestConvertWarnsOnUnsupportedStatMixedWithSupported(t *testing.T) {
	in := loadFixture(t)
	showdownMove(t, &in, "testrevived").Secondary.Boosts = map[string]int{"def": -1, "accuracy": -1}
	out, rep, err := importer.Convert(in)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !hasFinding(rep.Warnings, importer.KindMoveEffectUnsupportedStat, "testrevived") {
		t.Errorf("accuracy と atk 等が混ざった追加効果(testrevived)が Warnings(%s)に無い", importer.KindMoveEffectUnsupportedStat)
	}
	want := `{"Chance":20,"Target":"target","Stages":{"def":-1}}`
	if got := moveEffectsByID(out)["testrevived"]; got != want {
		t.Errorf("testrevived の move_effects = %q, want %q(accuracy だけ落として def は残す)", got, want)
	}
}

// TestConvertRejectsOutOfRangeMoveEffect は、取得元の値が engine の範囲に収まらないときに
// 変換を止めること(master.ErrInvalidEffect。持ち物・特性の効果と同じ扱い)。
func TestConvertRejectsOutOfRangeMoveEffect(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, in *importer.Input)
	}{
		{"確率が 0", func(t *testing.T, in *importer.Input) { showdownMove(t, in, "testflame").Secondary.Chance = 0 }},
		{"確率が 101", func(t *testing.T, in *importer.Input) { showdownMove(t, in, "testflame").Secondary.Chance = 101 }},
		{"変化量が +7", func(t *testing.T, in *importer.Input) {
			showdownMove(t, in, "testflame").Secondary.Self.Boosts = map[string]int{"spa": 7}
		}},
		{"変化量が 0", func(t *testing.T, in *importer.Input) {
			showdownMove(t, in, "testflame").Secondary.Self.Boosts = map[string]int{"spa": 0}
		}},
		{"未知のステータス", func(t *testing.T, in *importer.Input) {
			showdownMove(t, in, "teststrike").Self.Boosts = map[string]int{"hp": 1}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := loadFixture(t)
			tt.mutate(t, &in)
			if _, _, err := importer.Convert(in); !errors.Is(err, master.ErrInvalidEffect) {
				t.Fatalf("err = %v, want master.ErrInvalidEffect", err)
			}
		})
	}
}

// masterMoveRow は importer の行(+ 別表の効果)を共通マスタの行に組み立てる。
// importer.MoveRow 自体には効果を持たせない(DB も move_effects 別表。ADR-0107 決定5)。
func masterMoveRow(r importer.MoveRow, effect []byte) master.MoveRow {
	return master.MoveRow{
		ID: r.ID, NameJa: r.NameJa, Type: r.Type, Category: r.Category,
		Power: r.Power, Priority: r.Priority, Effect: effect,
	}
}

// TestConvertMoveEffectsPassMasterMapping は、作った行が共通マスタの写像を通ること
// (TestConvertOutputPassesMasterMapping の技版。ADR-0100 §6)。
func TestConvertMoveEffectsPassMasterMapping(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	chart := engineChart(t, out)
	effects := map[string][]byte{}
	for _, r := range out.MoveEffects {
		effects[r.ID] = r.Effect
	}
	seen := 0
	for _, r := range out.Moves {
		mv, err := master.Move(masterMoveRow(r, effects[r.ID]), chart)
		if err != nil {
			t.Fatalf("master.Move(%s): %v", r.ID, err)
		}
		if _, hasEffect := effects[r.ID]; hasEffect {
			seen++
			if mv.Effect == nil {
				t.Errorf("技 %s の追加効果が engine.Move に載っていない", r.ID)
				continue
			}
			if err := mv.Effect.Validate(); err != nil {
				t.Errorf("技 %s の追加効果が engine の検証を通らない: %v", r.ID, err)
			}
		} else if mv.Effect != nil {
			t.Errorf("技 %s は追加効果を持たないのに Effect がある: %+v", r.ID, *mv.Effect)
		}
	}
	if seen != 3 {
		t.Fatalf("追加効果を持つ技の件数 = %d, want 3", seen)
	}
}

// TestConvertMoveEffectsAreDeterministic は同じ入力から同じバイト列が出ること
// (TestConvertIsDeterministic の技の効果版。map の反復順に依らない)。
func TestConvertMoveEffectsAreDeterministic(t *testing.T) {
	first, _ := convertOK(t, loadFixture(t))
	for i := 0; i < 10; i++ {
		out, _ := convertOK(t, loadFixture(t))
		if !reflect.DeepEqual(out.MoveEffects, first.MoveEffects) {
			t.Fatalf("%d 回目で move_effects が変わった:\n%v\n%v", i, out.MoveEffects, first.MoveEffects)
		}
	}
}

// 追加効果はダメージ計算に混ざらない(ADR-0107 決定2)ことを、取り込んだ実データの経路でも確かめる。
func TestImportedMoveEffectDoesNotChangeDamage(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	chart := engineChart(t, out)
	effects := map[string][]byte{}
	for _, r := range out.MoveEffects {
		effects[r.ID] = r.Effect
	}

	var withEffect, withoutEffect engine.Move
	for _, r := range out.Moves {
		if r.ID != "teststrike" {
			continue
		}
		plain, err := master.Move(masterMoveRow(r, nil), chart)
		if err != nil {
			t.Fatalf("master.Move: %v", err)
		}
		withoutEffect = plain
		boosted, err := master.Move(masterMoveRow(r, effects[r.ID]), chart)
		if err != nil {
			t.Fatalf("master.Move(効果つき): %v", err)
		}
		withEffect = boosted
	}
	if withEffect.Effect == nil {
		t.Fatal("teststrike の追加効果が取り込まれていない(fixture か変換の誤り)")
	}

	types := chart.Types()
	if len(types) == 0 {
		t.Fatal("相性表が空")
	}
	mk := func(mv engine.Move) engine.DamageInput {
		ind := engine.Individual{
			Species: engine.Species{Types: []engine.Type{types[0]}, BaseStats: engine.Stats{HP: 100, Atk: 100, Def: 100, SpA: 100, SpD: 100, Spe: 100}},
			Nature:  engine.NatureNeutral,
		}
		return engine.DamageInput{Format: engine.FormatSingle, Attacker: ind, Defender: ind, Move: mv, TypeChart: chart}
	}
	want, err := engine.CalcDamage(mk(withoutEffect))
	if err != nil {
		t.Fatalf("CalcDamage(効果なし): %v", err)
	}
	got, err := engine.CalcDamage(mk(withEffect))
	if err != nil {
		t.Fatalf("CalcDamage(効果つき): %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("追加効果でダメージが変わった\n got = %+v\nwant = %+v", got, want)
	}
}
