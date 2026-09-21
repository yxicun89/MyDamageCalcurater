package engine

// 逆算(P1-8)の受け入れ条件をテストで固定する。ADR-0010 が定義の正。
//
// 方針:
//   - CalcReverse は CalcDamage の合成にすぎない。候補の想定ダメージ幅は CalcDamage と一致すること。
//   - 「正解」は SP の完全一致ではなく型(Archetype)の一致。分母は ArchetypeOf が定義する。
//   - 順序は全順序・決定的(ADR-0010 §6.3)。同じ入力なら常に同じ並び。
//
// Recall@5 の合格基準そのものは engine/reverse_recall_test.go(allspecies タグ)にある。
// 本ファイルの TestReverseRecallSmoke はその代わりではなく、小標本の早期検知用。

import (
	"errors"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// フィクスチャ
// ---------------------------------------------------------------------------

// revDefenderSpecies は逆算される側の試験用種族(HP100 / B80 / D80 のエスパー単)。
func revDefenderSpecies() Species {
	return Species{
		Key:       "0997-000",
		NameJa:    "テストぎゃくさんぼう",
		Types:     []Type{TypePsychic},
		BaseStats: Stats{HP: 100, Atk: 50, Def: 80, SpA: 50, SpD: 80, Spe: 50},
	}
}

// revAttackerSpecies は攻撃側の試験用種族。
func revAttackerSpecies() Species {
	return Species{
		Key:       "0996-000",
		NameJa:    "テストぎゃくさんこう",
		Types:     []Type{TypeWater},
		BaseStats: Stats{HP: 100, Atk: 120, Def: 70, SpA: 120, SpD: 70, Spe: 100},
	}
}

// revKnownAttacker は既知側(自分)の攻撃個体。A/C に振らない素の個体。
func revKnownAttacker() Individual {
	return Individual{
		Species: revAttackerSpecies(),
		Level:   DefaultLevel,
		Nature:  NatureNeutral,
		Status:  StatusNone,
	}
}

// revMove は威力100の水技(防御側エスパー単に対して等倍)。
func revMove(cat MoveCategory) Move {
	return Move{ID: "testmove", NameJa: "テストわざ", Type: TypeWater, Category: cat, Power: 100}
}

// revEviolite / revVest は持ち物候補の試験用(効果はマスタから解決済みの体で直接与える)。
func revEviolite() *Item {
	return &Item{ID: "eviolite", NameJa: "しんかのきせき",
		Effect: &ItemEffect{StatMods: map[StatKey]int{StatDef: 6144, StatSpD: 6144}}}
}

func revVest() *Item {
	return &Item{ID: "assaultvest", NameJa: "とつげきチョッキ",
		Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}
}

// revDefender は逆算対象の防御側個体を組み立てる(CalcReverse が内部で作るものと同じ形)。
func revDefender(sp Stats, n Nature, item *Item) Individual {
	return Individual{
		Species: revDefenderSpecies(),
		Level:   DefaultLevel,
		Nature:  n,
		SP:      sp,
		Item:    item,
		Status:  StatusNone,
	}
}

// revObserve は真値の個体に technique をぶつけ、指定ロールの表示%を観測値にする。
func revObserve(t *testing.T, in DamageInput, rollIdx int) (Observation, DamageResult) {
	t.Helper()
	res, err := CalcDamage(in)
	if err != nil {
		t.Fatalf("真値の CalcDamage が失敗した: %v", err)
	}
	hp := res.DefenderHP
	pct := DisplayPercent(res.Rolls[rollIdx], hp)
	if pct <= 0 {
		t.Fatalf("観測%%が 0 以下(DisplayPercent 未実装?): damage=%d hp=%d pct=%d",
			res.Rolls[rollIdx], hp, pct)
	}
	return Observation{Percent: pct}, res
}

// findCandidate は候補列から型キーと持ち物 ID が一致するものを探し、その順位を返す。
func findCandidate(cands []ReverseCandidate, key ArchetypeKey, itemID string) (int, *ReverseCandidate) {
	for i := range cands {
		if cands[i].Archetype.Key == key && cands[i].ItemID == itemID {
			return i, &cands[i]
		}
	}
	return -1, nil
}

// ---------------------------------------------------------------------------
// 受け入れ条件 1: 表示%の丸め(ADR-0010 §3)
// ---------------------------------------------------------------------------

func TestDisplayPercentRounding(t *testing.T) {
	// round-half-up = floor(100*damage/maxHP + 0.5)。float を使わない整数演算。
	tests := []struct {
		name   string
		damage int
		maxHP  int
		want   int
	}{
		{"ちょうど半分は切り上げ(round-half-up)", 1, 200, 1}, // 0.5% -> 1
		{"ちょうど 50%", 100, 200, 50},
		{"ちょうど 100%", 200, 200, 100},
		{"31.5% は切り上げ", 63, 200, 32},          // 31.5 -> 32
		{"31.0% はそのまま", 62, 200, 31},          // 31.0 -> 31
		{"30.5% は切り上げ(half-up)", 61, 200, 31}, // 30.5 -> 31
		{"0 ダメージは 0%", 0, 175, 0},
		{"1 ダメージ", 1, 175, 1},
		{"HP175 の 87 ダメージ", 87, 175, 50},   // 49.71 -> 50
		{"HP175 の 86 ダメージ", 86, 175, 49},   // 49.14 -> 49
		{"HP207 の 100 ダメージ", 100, 207, 48}, // 48.31 -> 48
		{"HP207 の 104 ダメージ", 104, 207, 50}, // 50.24 -> 50
		{"maxHP が 0 なら 0(ゼロ除算しない)", 10, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DisplayPercent(tt.damage, tt.maxHP); got != tt.want {
				t.Errorf("DisplayPercent(%d, %d) = %d, want %d", tt.damage, tt.maxHP, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 受け入れ条件 2: 型カタログの順序・キー・ラベル(ADR-0010 §5.2, §5.3)
// ---------------------------------------------------------------------------

func TestReverseArchetypeCatalogOrder(t *testing.T) {
	tests := []struct {
		name     string
		side     ReverseSide
		category MoveCategory
		want     []ArchetypeKey
		// wantLabelHead は先頭8件(attacker は4件)の表示名。requirements の「名前付きの型」。
		wantLabelHead []string
	}{
		{
			name: "防御側・物理(B)", side: SideDefender, category: CategoryPhysical,
			want: []ArchetypeKey{
				"hnone-bnone-neutral", "hfull-bnone-neutral", "hfull-bnone-plus",
				"hfull-bfull-plus", "hfull-bfull-neutral", "hnone-bfull-plus",
				"hnone-bfull-neutral", "hnone-bnone-plus",
				"hfull-bfull-minus", "hfull-bnone-minus",
				"hfull-bmid-plus", "hfull-bmid-neutral", "hfull-bmid-minus",
				"hnone-bfull-minus", "hnone-bnone-minus",
				"hnone-bmid-plus", "hnone-bmid-neutral", "hnone-bmid-minus",
				"hmid-bfull-plus", "hmid-bfull-neutral", "hmid-bfull-minus",
				"hmid-bnone-plus", "hmid-bnone-neutral", "hmid-bnone-minus",
				"hmid-bmid-plus", "hmid-bmid-neutral", "hmid-bmid-minus",
			},
			wantLabelHead: []string{
				"無振り", "H振り", "H振り+B補正", "HB特化",
				"HB振り(無補正)", "B特化(H無振り)", "B振り(無補正)", "B補正のみ",
			},
		},
		{
			name: "防御側・特殊(D)", side: SideDefender, category: CategorySpecial,
			want: []ArchetypeKey{
				"hnone-dnone-neutral", "hfull-dnone-neutral", "hfull-dnone-plus",
				"hfull-dfull-plus", "hfull-dfull-neutral", "hnone-dfull-plus",
				"hnone-dfull-neutral", "hnone-dnone-plus",
				"hfull-dfull-minus", "hfull-dnone-minus",
				"hfull-dmid-plus", "hfull-dmid-neutral", "hfull-dmid-minus",
				"hnone-dfull-minus", "hnone-dnone-minus",
				"hnone-dmid-plus", "hnone-dmid-neutral", "hnone-dmid-minus",
				"hmid-dfull-plus", "hmid-dfull-neutral", "hmid-dfull-minus",
				"hmid-dnone-plus", "hmid-dnone-neutral", "hmid-dnone-minus",
				"hmid-dmid-plus", "hmid-dmid-neutral", "hmid-dmid-minus",
			},
			wantLabelHead: []string{
				"無振り", "H振り", "H振り+D補正", "HD特化",
				"HD振り(無補正)", "D特化(H無振り)", "D振り(無補正)", "D補正のみ",
			},
		},
		{
			name: "攻撃側・物理(A)", side: SideAttacker, category: CategoryPhysical,
			want: []ArchetypeKey{
				"anone-neutral", "afull-plus", "afull-neutral", "anone-plus",
				"afull-minus", "anone-minus", "amid-plus", "amid-neutral", "amid-minus",
			},
			wantLabelHead: []string{"無振り", "A特化", "A振り(無補正)", "A補正のみ"},
		},
		{
			name: "攻撃側・特殊(C)", side: SideAttacker, category: CategorySpecial,
			want: []ArchetypeKey{
				"cnone-neutral", "cfull-plus", "cfull-neutral", "cnone-plus",
				"cfull-minus", "cnone-minus", "cmid-plus", "cmid-neutral", "cmid-minus",
			},
			wantLabelHead: []string{"無振り", "C特化", "C振り(無補正)", "C補正のみ"},
		},
		{
			// 変化技は関連ステータスを物理と同じ扱いにする(ADR-0010 §2)。
			name: "防御側・変化技は物理と同じカタログ", side: SideDefender, category: CategoryStatus,
			want: []ArchetypeKey{
				"hnone-bnone-neutral", "hfull-bnone-neutral", "hfull-bnone-plus",
				"hfull-bfull-plus", "hfull-bfull-neutral", "hnone-bfull-plus",
				"hnone-bfull-neutral", "hnone-bnone-plus",
				"hfull-bfull-minus", "hfull-bnone-minus",
				"hfull-bmid-plus", "hfull-bmid-neutral", "hfull-bmid-minus",
				"hnone-bfull-minus", "hnone-bnone-minus",
				"hnone-bmid-plus", "hnone-bmid-neutral", "hnone-bmid-minus",
				"hmid-bfull-plus", "hmid-bfull-neutral", "hmid-bfull-minus",
				"hmid-bnone-plus", "hmid-bnone-neutral", "hmid-bnone-minus",
				"hmid-bmid-plus", "hmid-bmid-neutral", "hmid-bmid-minus",
			},
			wantLabelHead: []string{
				"無振り", "H振り", "H振り+B補正", "HB特化",
				"HB振り(無補正)", "B特化(H無振り)", "B振り(無補正)", "B補正のみ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReverseArchetypes(tt.side, tt.category)
			if len(got) != len(tt.want) {
				t.Fatalf("型カタログの件数 = %d, want %d", len(got), len(tt.want))
			}
			for i, w := range tt.want {
				if got[i].Key != w {
					t.Errorf("事前順位 %d のキー = %q, want %q", i, got[i].Key, w)
				}
				if got[i].Side != tt.side {
					t.Errorf("事前順位 %d の Side = %q, want %q", i, got[i].Side, tt.side)
				}
			}
			for i, w := range tt.wantLabelHead {
				if got[i].Label != w {
					t.Errorf("事前順位 %d のラベル = %q, want %q", i, got[i].Label, w)
				}
			}
			// キーは重複しない(候補の同一性が壊れる)。
			seen := map[ArchetypeKey]bool{}
			for _, a := range got {
				if seen[a.Key] {
					t.Errorf("型キーが重複: %q", a.Key)
				}
				seen[a.Key] = true
			}
			// 呼び出しごとに新しいスライスを返し、書き換えが次回に漏れないこと。
			got[0].Label = "書き換え"
			if again := ReverseArchetypes(tt.side, tt.category); len(again) > 0 && again[0].Label == "書き換え" {
				t.Error("ReverseArchetypes が内部スライスを共有している")
			}
		})
	}
}

// TestReverseArchetypeGeneratedLabels は生成規則のラベル(ADR-0010 §5.2)を固定する。
func TestReverseArchetypeGeneratedLabels(t *testing.T) {
	want := map[ArchetypeKey]string{
		"hfull-bmid-plus":    "H振り+B中間+B補正",
		"hfull-bnone-minus":  "H振り+B下降",
		"hfull-bfull-minus":  "H振り+B振り+B下降",
		"hnone-bnone-minus":  "B下降",
		"hmid-bmid-neutral":  "H中間+B中間",
		"hmid-bnone-neutral": "H中間",
		"hnone-bmid-neutral": "B中間",
	}
	byKey := map[ArchetypeKey]Archetype{}
	for _, a := range ReverseArchetypes(SideDefender, CategoryPhysical) {
		byKey[a.Key] = a
	}
	for k, w := range want {
		a, ok := byKey[k]
		if !ok {
			t.Errorf("型 %q がカタログに無い", k)
			continue
		}
		if a.Label != w {
			t.Errorf("型 %q のラベル = %q, want %q", k, a.Label, w)
		}
	}

	wantAtk := map[ArchetypeKey]string{
		"afull-minus":  "A振り+A下降",
		"anone-minus":  "A下降",
		"amid-plus":    "A中間+A補正",
		"amid-neutral": "A中間",
	}
	byKeyAtk := map[ArchetypeKey]Archetype{}
	for _, a := range ReverseArchetypes(SideAttacker, CategoryPhysical) {
		byKeyAtk[a.Key] = a
	}
	for k, w := range wantAtk {
		a, ok := byKeyAtk[k]
		if !ok {
			t.Errorf("型 %q がカタログに無い", k)
			continue
		}
		if a.Label != w {
			t.Errorf("型 %q のラベル = %q, want %q", k, a.Label, w)
		}
	}
}

// ---------------------------------------------------------------------------
// 受け入れ条件 3: ArchetypeOf は格子を漏れなく・重複なく型に写す(Recall の分母の定義)
// ---------------------------------------------------------------------------

func TestArchetypeOfCoversGrid(t *testing.T) {
	// 防御側: H 0..32 × B 0..32 × 性格3クラス。どの点もカタログのどれか1つに属する。
	catalog := ReverseArchetypes(SideDefender, CategoryPhysical)
	if len(catalog) == 0 {
		t.Fatal("型カタログが空(ReverseArchetypes 未実装?)")
	}
	known := map[ArchetypeKey]bool{}
	for _, a := range catalog {
		known[a.Key] = true
	}
	natures := []Nature{
		{Plus: StatDef, Minus: StatAtk}, // plus
		NatureNeutral,                   // neutral
		{Plus: StatAtk, Minus: StatDef}, // minus
	}
	wantClass := []NatureClass{NatureClassPlus, NatureClassNeutral, NatureClassMinus}
	hit := map[ArchetypeKey]int{}
	for hp := 0; hp <= MaxSPPerStat; hp++ {
		for def := 0; def <= MaxSPPerStat; def++ {
			for ni, n := range natures {
				sp := Stats{HP: hp, Def: def}
				a, ok := ArchetypeOf(SideDefender, CategoryPhysical, sp, n)
				if !ok {
					t.Fatalf("ArchetypeOf が hp=%d def=%d nature=%+v を型にできない", hp, def, n)
				}
				if !known[a.Key] {
					t.Fatalf("ArchetypeOf が返した型 %q がカタログに無い", a.Key)
				}
				if a.NatureClass != wantClass[ni] {
					t.Fatalf("hp=%d def=%d nature=%+v の性格クラス = %q, want %q",
						hp, def, n, a.NatureClass, wantClass[ni])
				}
				hit[a.Key]++
			}
		}
	}
	if len(hit) != len(catalog) {
		t.Errorf("格子が到達した型 = %d 種, カタログ = %d 種(空の型がある)", len(hit), len(catalog))
	}
	// バケット境界: 0 は none、32 は full、その間は mid。
	boundary := []struct {
		sp   int
		want SPBucket
	}{{0, SPBucketNone}, {1, SPBucketMid}, {31, SPBucketMid}, {32, SPBucketFull}}
	for _, b := range boundary {
		a, ok := ArchetypeOf(SideDefender, CategoryPhysical, Stats{Def: b.sp}, NatureNeutral)
		if !ok {
			t.Fatalf("ArchetypeOf(def=%d) が失敗", b.sp)
		}
		if a.StatBucket != b.want {
			t.Errorf("def SP=%d のバケット = %q, want %q", b.sp, a.StatBucket, b.want)
		}
	}
	// 範囲外は型にできない。
	if _, ok := ArchetypeOf(SideDefender, CategoryPhysical, Stats{Def: MaxSPPerStat + 1}, NatureNeutral); ok {
		t.Error("SP が上限超えでも型を返した")
	}
	// 攻撃側は H の次元を持たない(常に SPBucketNone)。
	a, ok := ArchetypeOf(SideAttacker, CategoryPhysical, Stats{HP: 32, Atk: 32}, Nature{Plus: StatAtk, Minus: StatSpA})
	if !ok {
		t.Fatal("攻撃側の ArchetypeOf が失敗")
	}
	if a.Key != "afull-plus" {
		t.Errorf("攻撃側の型キー = %q, want %q(H の振りは型に影響しない)", a.Key, "afull-plus")
	}
	if a.HPBucket != SPBucketNone {
		t.Errorf("攻撃側の HPBucket = %q, want %q", a.HPBucket, SPBucketNone)
	}
}

// ---------------------------------------------------------------------------
// 受け入れ条件 4: 単一観測で真値の型が「完全一致」かつ上位5件に入る
// ---------------------------------------------------------------------------

func TestReverseSingleObservationFindsTruth(t *testing.T) {
	items := []*Item{nil, revEviolite()}
	tests := []struct {
		name     string
		category MoveCategory
		sp       Stats
		nature   Nature
		itemIdx  int
		rollIdx  int
	}{
		{"無振り・持ち物なし", CategoryPhysical, Stats{}, NatureNeutral, 0, 7},
		{"H振り・持ち物なし", CategoryPhysical, Stats{HP: 32}, NatureNeutral, 0, 0},
		{"HB特化・持ち物なし", CategoryPhysical, Stats{HP: 32, Def: 32}, Nature{Plus: StatDef, Minus: StatAtk}, 0, 15},
		{"H振り+B補正・持ち物なし", CategoryPhysical, Stats{HP: 32}, Nature{Plus: StatDef, Minus: StatAtk}, 0, 3},
		{"B振り・持ち物あり", CategoryPhysical, Stats{Def: 32}, NatureNeutral, 1, 11},
		{"HB特化・持ち物あり", CategoryPhysical, Stats{HP: 32, Def: 32}, Nature{Plus: StatDef, Minus: StatAtk}, 1, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			move := revMove(tt.category)
			truth := revDefender(tt.sp, tt.nature, items[tt.itemIdx])
			obs, _ := revObserve(t, DamageInput{
				Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truth, Move: move,
			}, tt.rollIdx)

			res, err := CalcReverse(ReverseInput{
				Format:         FormatSingle,
				Side:           SideDefender,
				Known:          revKnownAttacker(),
				UnknownSpecies: revDefenderSpecies(),
				Move:           move,
				ItemCandidates: items,
				Observations:   []Observation{obs},
			})
			if err != nil {
				t.Fatalf("CalcReverse: %v", err)
			}
			wantArch, ok := ArchetypeOf(SideDefender, tt.category, tt.sp, tt.nature)
			if !ok {
				t.Fatal("真値を型にできない")
			}
			wantItemID := ""
			if items[tt.itemIdx] != nil {
				wantItemID = items[tt.itemIdx].ID
			}
			rank, cand := findCandidate(res.Candidates, wantArch.Key, wantItemID)
			if cand == nil {
				t.Fatalf("真値の型 %q(持ち物 %q)が候補に無い。候補数=%d",
					wantArch.Key, wantItemID, len(res.Candidates))
			}
			if !cand.Exact {
				t.Errorf("真値の型 %q が完全一致になっていない: MatchScore=%v", wantArch.Key, cand.MatchScore)
			}
			if cand.MatchScore != 1.0 {
				t.Errorf("完全一致の MatchScore = %v, want 1.0(ADR-0010 §6.2)", cand.MatchScore)
			}
			if rank >= 5 {
				t.Errorf("真値の型 %q の順位 = %d, want < 5", wantArch.Key, rank)
			}
			if res.Side != SideDefender {
				t.Errorf("Side = %q, want %q", res.Side, SideDefender)
			}
			if res.Stat != StatDef {
				t.Errorf("Stat = %q, want %q(物理技の防御側は def)", res.Stat, StatDef)
			}
			if res.ExactCount < 1 {
				t.Errorf("ExactCount = %d, want >= 1", res.ExactCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 受け入れ条件 5: 観測を足すと完全一致候補が単調に非増加(絞り込み)
// ---------------------------------------------------------------------------

func TestReverseMultipleObservationsNarrow(t *testing.T) {
	move := revMove(CategoryPhysical)
	items := []*Item{nil}
	truthSP := Stats{HP: 32, Def: 32}
	truthNature := Nature{Plus: StatDef, Minus: StatAtk}
	truth := revDefender(truthSP, truthNature, nil)
	in := DamageInput{Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truth, Move: move}

	// 同じ相手への別々の1発。乱数は観測ごとに独立に選ばれてよい(ADR-0010 §6.1)。
	o1, _ := revObserve(t, in, 0)
	o2, _ := revObserve(t, in, 15)
	o3, _ := revObserve(t, in, 8)

	call := func(obs []Observation) ReverseResult {
		t.Helper()
		res, err := CalcReverse(ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move,
			ItemCandidates: items, Observations: obs,
		})
		if err != nil {
			t.Fatalf("CalcReverse: %v", err)
		}
		return res
	}

	exactSet := func(res ReverseResult) map[ArchetypeKey]bool {
		s := map[ArchetypeKey]bool{}
		for _, c := range res.Candidates {
			if c.Exact {
				s[c.Archetype.Key] = true
			}
		}
		return s
	}

	r1 := call([]Observation{o1})
	r2 := call([]Observation{o1, o2})
	r3 := call([]Observation{o1, o2, o3})

	s1, s2, s3 := exactSet(r1), exactSet(r2), exactSet(r3)

	// 観測を足しても真値は完全一致のまま残る。
	wantArch, ok := ArchetypeOf(SideDefender, CategoryPhysical, truthSP, truthNature)
	if !ok {
		t.Fatal("真値を型にできない")
	}
	for i, s := range []map[ArchetypeKey]bool{s1, s2, s3} {
		if !s[wantArch.Key] {
			t.Errorf("観測 %d 件で真値の型 %q が完全一致から消えた", i+1, wantArch.Key)
		}
	}

	// 単調に非増加(部分集合であること)。観測を足して候補が増えてはいけない。
	for k := range s2 {
		if !s1[k] {
			t.Errorf("観測2件目で新しい完全一致 %q が増えた(部分集合でない)", k)
		}
	}
	for k := range s3 {
		if !s2[k] {
			t.Errorf("観測3件目で新しい完全一致 %q が増えた(部分集合でない)", k)
		}
	}
	if !(r1.ExactCount >= r2.ExactCount && r2.ExactCount >= r3.ExactCount) {
		t.Errorf("ExactCount が単調非増加でない: %d, %d, %d", r1.ExactCount, r2.ExactCount, r3.ExactCount)
	}
	// 候補そのものは絞られても消えない(近い候補は常に返す)。
	if len(r3.Candidates) != len(r1.Candidates) {
		t.Errorf("候補の総数が観測数で変わった: %d -> %d(型の数は観測に依らない)",
			len(r1.Candidates), len(r3.Candidates))
	}
}

// 到達不能な2件目の観測を足すと、1件目で完全一致していた型が完全一致でなくなる。
// TestReverseMultipleObservationsNarrow は実測では観測を足しても完全一致の集合が変わらないため、
// 「2件目以降の観測を無視する」実装を検出できない。その対照(ADR-0010 §6.1)。
func TestReverseUnreachableSecondObservationRemovesExact(t *testing.T) {
	move := revMove(CategoryPhysical)
	truth := revDefender(Stats{HP: 32, Def: 32}, Nature{Plus: StatDef, Minus: StatAtk}, nil)
	in := DamageInput{Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truth, Move: move}
	o1, _ := revObserve(t, in, 0)

	call := func(obs []Observation) ReverseResult {
		t.Helper()
		res, err := CalcReverse(ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move,
			ItemCandidates: []*Item{nil}, Observations: obs,
		})
		if err != nil {
			t.Fatalf("CalcReverse: %v", err)
		}
		return res
	}

	one := call([]Observation{o1})
	if one.ExactCount == 0 || one.Candidates[0].MatchScore != 1.0 {
		t.Fatalf("1件目だけなら完全一致がある: ExactCount=%d 先頭 MatchScore=%v",
			one.ExactCount, one.Candidates[0].MatchScore)
	}

	// 1% はこの技のどの調整でも到達しない(TestReverseNoExactStillReturnsCandidates と同じ前提)。
	two := call([]Observation{o1, {Percent: 1}})
	if two.ExactCount != 0 {
		t.Errorf("到達不能な2件目を足したのに ExactCount = %d, want 0", two.ExactCount)
	}
	if two.Candidates[0].Exact || two.Candidates[0].MatchScore >= 1.0 {
		t.Errorf("到達不能な2件目を足したのに先頭が完全一致のまま: MatchScore=%v", two.Candidates[0].MatchScore)
	}
	if two.Candidates[0].MatchScore <= 0 {
		t.Errorf("先頭の MatchScore = %v, want > 0(近い候補は返す)", two.Candidates[0].MatchScore)
	}
}

// ---------------------------------------------------------------------------
// 受け入れ条件 6: Side=attacker(受けたダメージから相手の A/C を逆算)
// ---------------------------------------------------------------------------

func TestReverseAttackerSide(t *testing.T) {
	move := revMove(CategoryPhysical)
	items := []*Item{nil}
	// 自分(既知側)= 防御側。相手(未知側)= 攻撃側。
	known := revDefender(Stats{HP: 32, Def: 32}, Nature{Plus: StatDef, Minus: StatAtk}, nil)
	truthSP := Stats{Atk: 32}
	truthNature := Nature{Plus: StatAtk, Minus: StatSpA}
	truth := Individual{
		Species: revAttackerSpecies(), Level: DefaultLevel,
		Nature: truthNature, SP: truthSP, Status: StatusNone,
	}
	obs, _ := revObserve(t, DamageInput{
		Format: FormatSingle, Attacker: truth, Defender: known, Move: move,
	}, 9)

	res, err := CalcReverse(ReverseInput{
		Format: FormatSingle, Side: SideAttacker, Known: known,
		UnknownSpecies: revAttackerSpecies(), Move: move,
		ItemCandidates: items, Observations: []Observation{obs},
	})
	if err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	if res.Side != SideAttacker {
		t.Errorf("Side = %q, want %q", res.Side, SideAttacker)
	}
	if res.Stat != StatAtk {
		t.Errorf("Stat = %q, want %q(物理技の攻撃側は atk)", res.Stat, StatAtk)
	}
	wantArch, ok := ArchetypeOf(SideAttacker, CategoryPhysical, truthSP, truthNature)
	if !ok {
		t.Fatal("真値を型にできない")
	}
	rank, cand := findCandidate(res.Candidates, wantArch.Key, "")
	if cand == nil {
		t.Fatalf("真値の型 %q が候補に無い。候補数=%d", wantArch.Key, len(res.Candidates))
	}
	if !cand.Exact {
		t.Errorf("真値の型 %q が完全一致になっていない: MatchScore=%v", wantArch.Key, cand.MatchScore)
	}
	if rank >= 5 {
		t.Errorf("真値の型 %q の順位 = %d, want < 5", wantArch.Key, rank)
	}
	// 攻撃側の型は H の次元を持たないので 9 種 × 持ち物1 = 9 件。
	if len(res.Candidates) != 9 {
		t.Errorf("攻撃側の候補数 = %d, want 9", len(res.Candidates))
	}
}

// ---------------------------------------------------------------------------
// 受け入れ条件 7: Damage 観測(HP の実点数)
// ---------------------------------------------------------------------------

func TestReverseDamageObservation(t *testing.T) {
	move := revMove(CategorySpecial)
	items := []*Item{nil}
	known := revDefender(Stats{HP: 32}, NatureNeutral, nil)
	truthSP := Stats{SpA: 32}
	truthNature := Nature{Plus: StatSpA, Minus: StatAtk}
	truth := Individual{
		Species: revAttackerSpecies(), Level: DefaultLevel,
		Nature: truthNature, SP: truthSP, Status: StatusNone,
	}
	base, err := CalcDamage(DamageInput{Format: FormatSingle, Attacker: truth, Defender: known, Move: move})
	if err != nil {
		t.Fatalf("真値の CalcDamage: %v", err)
	}

	res, err := CalcReverse(ReverseInput{
		Format: FormatSingle, Side: SideAttacker, Known: known,
		UnknownSpecies: revAttackerSpecies(), Move: move,
		ItemCandidates: items,
		// 実点数の観測。完全一致で判定する(ADR-0010 §3)。
		Observations: []Observation{{Damage: base.Rolls[6]}},
	})
	if err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	wantArch, ok := ArchetypeOf(SideAttacker, CategorySpecial, truthSP, truthNature)
	if !ok {
		t.Fatal("真値を型にできない")
	}
	rank, cand := findCandidate(res.Candidates, wantArch.Key, "")
	if cand == nil {
		t.Fatalf("真値の型 %q が候補に無い", wantArch.Key)
	}
	if !cand.Exact {
		t.Errorf("Damage 観測で真値の型 %q が完全一致になっていない: MatchScore=%v",
			wantArch.Key, cand.MatchScore)
	}
	if rank >= 5 {
		t.Errorf("真値の型 %q の順位 = %d, want < 5", wantArch.Key, rank)
	}
}

// ---------------------------------------------------------------------------
// 受け入れ条件 8: 完全一致が無くても近い候補を返す(正確さより候補の提示を優先)
// ---------------------------------------------------------------------------

func TestReverseNoExactStillReturnsCandidates(t *testing.T) {
	move := revMove(CategoryPhysical)
	items := []*Item{nil}
	// どの調整でも到達しない観測(1% と 100%)を与える。
	for _, pct := range []int{1, 100} {
		res, err := CalcReverse(ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move,
			ItemCandidates: items, Observations: []Observation{{Percent: pct}},
		})
		if err != nil {
			t.Fatalf("observed=%d%%: CalcReverse: %v", pct, err)
		}
		if len(res.Candidates) == 0 {
			t.Fatalf("observed=%d%%: 完全一致が無くても候補を返すこと(要件: 候補の提示を優先)", pct)
		}
		for i, c := range res.Candidates {
			if c.MatchScore <= 0 || c.MatchScore > 1 {
				t.Errorf("observed=%d%%: 候補 %d の MatchScore = %v, want (0,1]", pct, i, c.MatchScore)
			}
			if c.Exact != (c.MatchScore == 1.0) {
				t.Errorf("observed=%d%%: 候補 %d の Exact=%v と MatchScore=%v が矛盾",
					pct, i, c.Exact, c.MatchScore)
			}
		}
		// スコアは降順。
		for i := 1; i < len(res.Candidates); i++ {
			if res.Candidates[i-1].MatchScore < res.Candidates[i].MatchScore {
				t.Fatalf("observed=%d%%: MatchScore が降順でない(%d 番目)", pct, i)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 受け入れ条件 9: 順序は決定的な全順序。同点は事前順位 → 持ち物添字
// ---------------------------------------------------------------------------

func TestReverseOrderDeterministic(t *testing.T) {
	move := revMove(CategoryPhysical)
	items := []*Item{nil, revEviolite()}
	obs := []Observation{{Percent: 42}, {Percent: 38}}
	build := func() ReverseInput {
		return ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move,
			ItemCandidates: items, Observations: obs,
		}
	}
	first, err := CalcReverse(build())
	if err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := CalcReverse(build())
		if err != nil {
			t.Fatalf("CalcReverse(%d): %v", i, err)
		}
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("同じ入力で結果が変わった(%d 回目)", i)
		}
	}

	// 全順序: 同じ (型キー, 持ち物ID) は1件だけ。
	seen := map[string]bool{}
	for _, c := range first.Candidates {
		k := string(c.Archetype.Key) + "|" + c.ItemID
		if seen[k] {
			t.Errorf("候補が重複: %s", k)
		}
		seen[k] = true
	}

	// 同点の並びは 事前順位 昇順 → 持ち物候補の添字 昇順(ADR-0010 §6.3)。
	prio := map[ArchetypeKey]int{}
	for i, a := range ReverseArchetypes(SideDefender, CategoryPhysical) {
		prio[a.Key] = i
	}
	itemIdx := map[string]int{}
	for i, it := range items {
		id := ""
		if it != nil {
			id = it.ID
		}
		itemIdx[id] = i
	}
	for i := 1; i < len(first.Candidates); i++ {
		a, b := first.Candidates[i-1], first.Candidates[i]
		if a.MatchScore != b.MatchScore {
			continue
		}
		pa, pb := prio[a.Archetype.Key], prio[b.Archetype.Key]
		if pa > pb {
			t.Errorf("同点の並びが事前順位の昇順でない: %q(%d) の後に %q(%d)",
				a.Archetype.Key, pa, b.Archetype.Key, pb)
		}
		if pa == pb && itemIdx[a.ItemID] > itemIdx[b.ItemID] {
			t.Errorf("同点・同型の並びが持ち物添字の昇順でない: %q の後に %q", a.ItemID, b.ItemID)
		}
	}

	// MaxCandidates は上位から切り取るだけで、順序を変えない。
	in := build()
	in.MaxCandidates = 3
	limited, err := CalcReverse(in)
	if err != nil {
		t.Fatalf("CalcReverse(MaxCandidates=3): %v", err)
	}
	if len(limited.Candidates) != 3 {
		t.Fatalf("MaxCandidates=3 のとき候補数 = %d, want 3", len(limited.Candidates))
	}
	for i := range limited.Candidates {
		if !reflect.DeepEqual(limited.Candidates[i], first.Candidates[i]) {
			t.Errorf("MaxCandidates で %d 番目の候補が変わった", i)
		}
	}
}

// ---------------------------------------------------------------------------
// 受け入れ条件 10: 候補は CalcDamage の合成(独自のダメージ式を書かない)
// ---------------------------------------------------------------------------

func TestReverseCandidateRangeMatchesCalcDamage(t *testing.T) {
	move := revMove(CategoryPhysical)
	items := []*Item{nil, revEviolite()}
	truth := revDefender(Stats{HP: 32, Def: 32}, Nature{Plus: StatDef, Minus: StatAtk}, nil)
	obs, _ := revObserve(t, DamageInput{
		Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truth, Move: move,
	}, 4)

	res, err := CalcReverse(ReverseInput{
		Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
		UnknownSpecies: revDefenderSpecies(), Move: move,
		ItemCandidates: items, Observations: []Observation{obs},
	})
	if err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	if len(res.Candidates) == 0 {
		t.Fatal("候補が空")
	}
	for _, c := range res.Candidates {
		def := revDefender(c.SP, c.Nature, c.Item)
		want, err := CalcDamage(DamageInput{
			Format: FormatSingle, Attacker: revKnownAttacker(), Defender: def, Move: move,
		})
		if err != nil {
			t.Fatalf("代表点 %q の CalcDamage: %v", c.Archetype.Key, err)
		}
		wantMin := DisplayPercent(want.Rolls[0], want.DefenderHP)
		wantMax := DisplayPercent(want.Rolls[15], want.DefenderHP)
		if c.MinPercent != wantMin || c.MaxPercent != wantMax {
			t.Errorf("型 %q(持ち物 %q)の想定ダメージ幅 = [%d,%d], want [%d,%d]",
				c.Archetype.Key, c.ItemID, c.MinPercent, c.MaxPercent, wantMin, wantMax)
		}
		// 代表 SP / 性格は、その型のバケット規則に合っていること。
		got, ok := ArchetypeOf(SideDefender, CategoryPhysical, c.SP, c.Nature)
		if !ok || got.Key != c.Archetype.Key {
			t.Errorf("型 %q の代表 SP=%+v nature=%+v が別の型 %q に属している",
				c.Archetype.Key, c.SP, c.Nature, got.Key)
		}
		// 完全一致の候補は、代表点のロールのどれかが観測とぴったり一致する。
		if c.Exact {
			hit := false
			for _, r := range want.Rolls {
				if DisplayPercent(r, want.DefenderHP) == obs.Percent {
					hit = true
					break
				}
			}
			if !hit {
				t.Errorf("型 %q は完全一致なのに代表点が観測 %d%% を再現しない",
					c.Archetype.Key, obs.Percent)
			}
		}
		if c.Points <= 0 {
			t.Errorf("型 %q の Points = %d, want > 0", c.Archetype.Key, c.Points)
		}
		if c.ExactPoints > c.Points {
			t.Errorf("型 %q の ExactPoints=%d > Points=%d", c.Archetype.Key, c.ExactPoints, c.Points)
		}
		if c.Exact != (c.ExactPoints > 0) {
			t.Errorf("型 %q の Exact=%v と ExactPoints=%d が矛盾", c.Archetype.Key, c.Exact, c.ExactPoints)
		}
	}
}

// ---------------------------------------------------------------------------
// 受け入れ条件 11: 持ち物候補が結果に効く
// ---------------------------------------------------------------------------

func TestReverseItemCandidates(t *testing.T) {
	move := revMove(CategorySpecial)
	vest := revVest()
	truthSP := Stats{HP: 32}
	truthNature := NatureNeutral
	truth := revDefender(truthSP, truthNature, vest)
	obs, _ := revObserve(t, DamageInput{
		Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truth, Move: move,
	}, 7)
	wantArch, ok := ArchetypeOf(SideDefender, CategorySpecial, truthSP, truthNature)
	if !ok {
		t.Fatal("真値を型にできない")
	}

	t.Run("候補に含めれば持ち物付きで完全一致する", func(t *testing.T) {
		res, err := CalcReverse(ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move,
			ItemCandidates: []*Item{nil, vest}, Observations: []Observation{obs},
		})
		if err != nil {
			t.Fatalf("CalcReverse: %v", err)
		}
		// 型 × 持ち物 = 27 × 2。
		if len(res.Candidates) != 54 {
			t.Errorf("候補数 = %d, want 54(27型 × 持ち物2)", len(res.Candidates))
		}
		_, cand := findCandidate(res.Candidates, wantArch.Key, vest.ID)
		if cand == nil {
			t.Fatalf("型 %q × 持ち物 %q の候補が無い", wantArch.Key, vest.ID)
		}
		if !cand.Exact {
			t.Errorf("持ち物付きの真値が完全一致になっていない: MatchScore=%v", cand.MatchScore)
		}
		if cand.Item != vest {
			t.Error("候補は渡された *Item をそのまま保持すること")
		}
	})

	t.Run("持ち物候補を省略すると持ち物なしの1通りになる", func(t *testing.T) {
		res, err := CalcReverse(ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move,
			Observations: []Observation{obs},
		})
		if err != nil {
			t.Fatalf("CalcReverse: %v", err)
		}
		if len(res.Candidates) != 27 {
			t.Errorf("候補数 = %d, want 27(持ち物なしの1通り)", len(res.Candidates))
		}
		for _, c := range res.Candidates {
			if c.Item != nil || c.ItemID != "" {
				t.Fatalf("持ち物候補を省略したのに持ち物付きの候補が出た: %q", c.ItemID)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// 受け入れ条件 12: 入力検証(errors.Is で判別できる sentinel)
// ---------------------------------------------------------------------------

func TestReverseValidationErrors(t *testing.T) {
	valid := func() ReverseInput {
		return ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: revMove(CategoryPhysical),
			Observations: []Observation{{Percent: 40}},
		}
	}
	tests := []struct {
		name    string
		mutate  func(*ReverseInput)
		wantErr error
	}{
		{"Side が空", func(in *ReverseInput) { in.Side = "" }, ErrInvalidReverseSide},
		{"Side が未知の値", func(in *ReverseInput) { in.Side = "both" }, ErrInvalidReverseSide},
		{"観測が nil", func(in *ReverseInput) { in.Observations = nil }, ErrNoObservation},
		{"観測が空", func(in *ReverseInput) { in.Observations = []Observation{} }, ErrNoObservation},
		{"Percent と Damage の同時指定", func(in *ReverseInput) {
			in.Observations = []Observation{{Percent: 40, Damage: 80}}
		}, ErrInvalidObservation},
		{"Percent も Damage も無い", func(in *ReverseInput) {
			in.Observations = []Observation{{Note: "めも"}}
		}, ErrInvalidObservation},
		{"Percent=0 は未指定扱い(Percent も Damage も 0)", func(in *ReverseInput) {
			in.Observations = []Observation{{Percent: 0}}
		}, ErrInvalidObservation},
		{"Percent が負", func(in *ReverseInput) {
			in.Observations = []Observation{{Percent: -1}}
		}, ErrInvalidObservation},
		{"Percent が 100 超", func(in *ReverseInput) {
			in.Observations = []Observation{{Percent: 101}}
		}, ErrInvalidObservation},
		{"Damage が負", func(in *ReverseInput) {
			in.Observations = []Observation{{Damage: -5}}
		}, ErrInvalidObservation},
		{"2件目だけ不正", func(in *ReverseInput) {
			in.Observations = []Observation{{Percent: 40}, {Percent: 0}}
		}, ErrInvalidObservation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := valid()
			tt.mutate(&in)
			_, err := CalcReverse(in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want errors.Is(..., %v)", err, tt.wantErr)
			}
		})
	}

	t.Run("既知側の個体が不正なら CalcDamage と同じエラー", func(t *testing.T) {
		in := valid()
		in.Known.SP = Stats{Atk: MaxSPPerStat + 1}
		if _, err := CalcReverse(in); err == nil {
			t.Fatal("SP 上限超えの既知側を受け入れてしまった")
		}
	})

	t.Run("未知側の種族が不正ならエラー", func(t *testing.T) {
		in := valid()
		in.UnknownSpecies.Types = nil // タイプは1〜2個
		if _, err := CalcReverse(in); err == nil {
			t.Fatal("タイプなしの種族を受け入れてしまった")
		}
	})
}

// ---------------------------------------------------------------------------
// 受け入れ条件 13: 純粋性(入力を変更しない)
// ---------------------------------------------------------------------------

func TestReverseDoesNotMutateInput(t *testing.T) {
	vest := revVest()
	items := []*Item{nil, vest}
	obs := []Observation{{Percent: 40}, {Percent: 45}}
	in := ReverseInput{
		Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
		UnknownSpecies: revDefenderSpecies(), Move: revMove(CategoryPhysical),
		ItemCandidates: items, Observations: obs,
	}
	before := ReverseInput{
		Format: in.Format, Side: in.Side, Known: in.Known,
		UnknownSpecies: in.UnknownSpecies, Move: in.Move,
		ItemCandidates: append([]*Item(nil), items...),
		Observations:   append([]Observation(nil), obs...),
	}
	beforeVest := *vest

	if _, err := CalcReverse(in); err != nil {
		t.Fatalf("CalcReverse: %v", err)
	}
	if !reflect.DeepEqual(in.Known, before.Known) {
		t.Error("CalcReverse が Known を変更した")
	}
	if !reflect.DeepEqual(in.UnknownSpecies, before.UnknownSpecies) {
		t.Error("CalcReverse が UnknownSpecies を変更した")
	}
	if !reflect.DeepEqual(in.ItemCandidates, before.ItemCandidates) {
		t.Error("CalcReverse が ItemCandidates を変更した")
	}
	if !reflect.DeepEqual(in.Observations, before.Observations) {
		t.Error("CalcReverse が Observations を変更した")
	}
	if !reflect.DeepEqual(*vest, beforeVest) {
		t.Error("CalcReverse が渡された *Item の中身を変更した")
	}
}

// ---------------------------------------------------------------------------
// 強化テスト(critic 指摘)。既存の受け入れ条件は弱めず、取りこぼしていた振る舞いを固定する。
// ---------------------------------------------------------------------------

// revBigHPSpecies は HP が大きい既知側の試験用種族。
// 点数差 1〜2 が「差*100/HP」で 0 に落ちる(整数除算)ことを利用して、
// 距離の下限 1 が効いているかを検出する。
func revBigHPSpecies() Species {
	return Species{
		Key:       "0995-000",
		NameJa:    "テストだいたいりょく",
		Types:     []Type{TypeNormal},
		BaseStats: Stats{HP: 255, Atk: 10, Def: 60, SpA: 10, SpD: 60, Spe: 55},
	}
}

// ロールに存在しない近傍の Damage 観測は Exact にならず、MatchScore も 1.0 未満になる
// (ADR-0010 §3, §6.1「dist=0 のときだけ 1.0」「Exact ⇔ MatchScore==1.0」)。
func TestReverseDamageObservationNearMissIsNotExact(t *testing.T) {
	move := revMove(CategoryPhysical)
	known := Individual{Species: revBigHPSpecies(), Level: DefaultLevel, Nature: NatureNeutral, Status: StatusNone}
	hp := RealStats(known).HP
	if hp <= 200 {
		t.Fatalf("フィクスチャ不正: HP=%d。差2でも 差*100/HP が 0 に落ちる HP>200 が必要", hp)
	}

	// 攻撃側の全格子(99点)のロールの集合を、テスト側で独立に作る。
	rolls := map[int]bool{}
	maxRoll, minRoll := 0, 1<<30
	for x := 0; x <= MaxSPPerStat; x++ {
		for _, c := range []NatureClass{NatureClassPlus, NatureClassNeutral, NatureClassMinus} {
			atk := Individual{
				Species: revAttackerSpecies(), Level: DefaultLevel,
				Nature: revNatureFor(StatAtk, c), SP: Stats{Atk: x}, Status: StatusNone,
			}
			res, err := CalcDamage(DamageInput{Format: FormatSingle, Attacker: atk, Defender: known, Move: move})
			if err != nil {
				t.Fatalf("格子点の CalcDamage: %v", err)
			}
			for _, r := range res.Rolls {
				rolls[r] = true
				maxRoll = max(maxRoll, r)
				minRoll = min(minRoll, r)
			}
		}
	}

	call := func(d int) ReverseResult {
		t.Helper()
		res, err := CalcReverse(ReverseInput{
			Format: FormatSingle, Side: SideAttacker, Known: known,
			UnknownSpecies: revAttackerSpecies(), Move: move,
			Observations: []Observation{{Damage: d}},
		})
		if err != nil {
			t.Fatalf("Damage=%d: CalcReverse: %v", d, err)
		}
		return res
	}

	// 対照: 実在するロールは完全一致する。
	if res := call(maxRoll); res.ExactCount == 0 {
		t.Fatalf("Damage=%d(実在するロール)が完全一致しない", maxRoll)
	}

	misses := []int{maxRoll + 1, maxRoll + 2}
	if minRoll-2 >= 1 {
		misses = append(misses, minRoll-1, minRoll-2)
	}
	for _, d := range misses {
		if rolls[d] {
			t.Fatalf("フィクスチャ不正: Damage=%d は実在するロール", d)
		}
		res := call(d)
		if res.ExactCount != 0 {
			t.Errorf("Damage=%d(実在しない近傍)で ExactCount = %d, want 0", d, res.ExactCount)
		}
		for _, c := range res.Candidates {
			if c.Exact || c.MatchScore >= 1.0 || c.ExactPoints != 0 {
				t.Errorf("Damage=%d(実在しない近傍)で型 %q が完全一致扱い: Exact=%v MatchScore=%v ExactPoints=%d",
					d, c.Archetype.Key, c.Exact, c.MatchScore, c.ExactPoints)
				break
			}
		}
	}
}

// MaxCandidates で切り取っても ExactCount は切り取る前の値(全候補中の Exact 数)。
// 負値は無制限。
func TestReverseMaxCandidatesKeepsExactCount(t *testing.T) {
	move := revMove(CategoryPhysical)
	items := []*Item{nil, revEviolite()}
	truth := revDefender(Stats{HP: 32, Def: 32}, Nature{Plus: StatDef, Minus: StatAtk}, nil)
	obs, _ := revObserve(t, DamageInput{
		Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truth, Move: move,
	}, 4)
	call := func(limit int) ReverseResult {
		t.Helper()
		res, err := CalcReverse(ReverseInput{
			Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: move, ItemCandidates: items,
			Observations: []Observation{obs}, MaxCandidates: limit,
		})
		if err != nil {
			t.Fatalf("CalcReverse(MaxCandidates=%d): %v", limit, err)
		}
		return res
	}

	full := call(0)
	if full.ExactCount < 2 {
		t.Fatalf("フィクスチャ不正: 切り取り前の ExactCount = %d, want >= 2", full.ExactCount)
	}
	limited := call(1)
	if len(limited.Candidates) != 1 {
		t.Fatalf("MaxCandidates=1 のとき候補数 = %d, want 1", len(limited.Candidates))
	}
	if limited.ExactCount != full.ExactCount {
		t.Errorf("MaxCandidates=1 の ExactCount = %d, want %d(切り取り前の値)", limited.ExactCount, full.ExactCount)
	}
	if neg := call(-1); len(neg.Candidates) != len(full.Candidates) {
		t.Errorf("MaxCandidates=-1 の候補数 = %d, want %d(負値は無制限)", len(neg.Candidates), len(full.Candidates))
	}
}

// 観測の生成時と同じ Critical / Field を渡したときだけ、真値の型が完全一致になる。
func TestReverseCriticalAndFieldAreApplied(t *testing.T) {
	move := revMove(CategoryPhysical) // 水技
	truthSP := Stats{HP: 32, Def: 32}
	truthNature := Nature{Plus: StatDef, Minus: StatAtk}
	truth := revDefender(truthSP, truthNature, nil)
	wantArch, ok := ArchetypeOf(SideDefender, CategoryPhysical, truthSP, truthNature)
	if !ok {
		t.Fatal("真値を型にできない")
	}

	tests := []struct {
		name     string
		critical bool
		field    Field
	}{
		{"急所", true, Field{}},
		{"雨 + 水技", false, Field{Weather: WeatherRain}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs, _ := revObserve(t, DamageInput{
				Format: FormatSingle, Attacker: revKnownAttacker(), Defender: truth, Move: move,
				Field: tt.field, Critical: tt.critical,
			}, 8)
			call := func(critical bool, field Field) *ReverseCandidate {
				t.Helper()
				res, err := CalcReverse(ReverseInput{
					Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
					UnknownSpecies: revDefenderSpecies(), Move: move,
					Field: field, Critical: critical, Observations: []Observation{obs},
				})
				if err != nil {
					t.Fatalf("CalcReverse: %v", err)
				}
				_, cand := findCandidate(res.Candidates, wantArch.Key, "")
				if cand == nil {
					t.Fatalf("真値の型 %q が候補に無い", wantArch.Key)
				}
				return cand
			}

			if c := call(tt.critical, tt.field); !c.Exact || c.MatchScore != 1.0 {
				t.Errorf("同じ Critical/Field を渡したのに真値が完全一致しない: MatchScore=%v", c.MatchScore)
			}
			// 渡さなければ、真値の型(格子点1つ)は観測を再現できない。
			if c := call(false, Field{}); c.Exact || c.MatchScore >= 1.0 {
				t.Errorf("Critical/Field を渡さないのに真値が完全一致した: MatchScore=%v", c.MatchScore)
			}
		})
	}
}

// 変化技・無効相性はダメージ 0 なので、エラーにせず全候補が同点になる。
// 同点の並びは「事前順位 → 持ち物添字」(ADR-0010 §2, §6.3)。
func TestReverseZeroDamageMovesKeepPriorityOrder(t *testing.T) {
	items := []*Item{nil, revEviolite(), revVest()}
	tests := []struct {
		name    string
		species Species
		move    Move
	}{
		{"変化技", revDefenderSpecies(), Move{ID: "status", NameJa: "テストへんか", Type: TypeWater, Category: CategoryStatus}},
		// 未知側はノーマル単。ゴーストタイプの技は無効。
		{"無効相性", revBigHPSpecies(), Move{ID: "immune", NameJa: "テストむこう", Type: TypeGhost, Category: CategoryPhysical, Power: 100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := CalcReverse(ReverseInput{
				Format: FormatSingle, Side: SideDefender, Known: revKnownAttacker(),
				UnknownSpecies: tt.species, Move: tt.move,
				ItemCandidates: items, Observations: []Observation{{Percent: 30}},
			})
			if err != nil {
				t.Fatalf("ダメージ 0 の技でエラーになった: %v", err)
			}
			catalog := ReverseArchetypes(SideDefender, tt.move.Category)
			if len(res.Candidates) != len(catalog)*len(items) {
				t.Fatalf("候補数 = %d, want %d", len(res.Candidates), len(catalog)*len(items))
			}
			if res.ExactCount != 0 {
				t.Errorf("ExactCount = %d, want 0", res.ExactCount)
			}
			first := res.Candidates[0].MatchScore
			if first <= 0 || first >= 1 {
				t.Fatalf("MatchScore = %v, want (0,1)", first)
			}
			for i, c := range res.Candidates {
				if c.MatchScore != first {
					t.Errorf("候補 %d の MatchScore = %v, want 全候補同点 %v", i, c.MatchScore, first)
				}
				wantKey := catalog[i/len(items)].Key
				wantItem := ""
				if it := items[i%len(items)]; it != nil {
					wantItem = it.ID
				}
				if c.Archetype.Key != wantKey || c.ItemID != wantItem {
					t.Fatalf("候補 %d = (%q, %q), want (%q, %q)(事前順位 → 持ち物添字の順)",
						i, c.Archetype.Key, c.ItemID, wantKey, wantItem)
				}
				// 全格子点が同点なので、代表点は走査順(H 昇順 → X 昇順)で最初の点になる(ADR-0010 §6.2)。
				if c.Archetype.Key == "hmid-bmid-plus" && c.SP != (Stats{HP: 1, Def: 1}) {
					t.Errorf("hmid-bmid-plus の代表 SP = %+v, want {HP:1 Def:1}(全点同点なら走査順で最初)", c.SP)
				}
			}
		})
	}
}

// 代表点・格子点数・性格の Minus の置き場所(ADR-0010 §4, §5.1, §6.2)。
func TestReverseRepresentativePointsAndNatures(t *testing.T) {
	call := func(side ReverseSide, cat MoveCategory) ReverseResult {
		t.Helper()
		in := ReverseInput{
			Format: FormatSingle, Side: side, Known: revKnownAttacker(),
			UnknownSpecies: revDefenderSpecies(), Move: revMove(cat),
			Observations: []Observation{{Percent: 40}},
		}
		if side == SideAttacker {
			in.Known = revDefender(Stats{HP: 32, Def: 32}, NatureNeutral, nil)
			in.UnknownSpecies = revAttackerSpecies()
		}
		res, err := CalcReverse(in)
		if err != nil {
			t.Fatalf("CalcReverse: %v", err)
		}
		return res
	}
	sumPoints := func(res ReverseResult) int {
		n := 0
		for _, c := range res.Candidates {
			n += c.Points
		}
		return n
	}
	get := func(res ReverseResult, key ArchetypeKey) ReverseCandidate {
		t.Helper()
		_, c := findCandidate(res.Candidates, key, "")
		if c == nil {
			t.Fatalf("型 %q が候補に無い", key)
		}
		return *c
	}

	t.Run("防御側・物理", func(t *testing.T) {
		res := call(SideDefender, CategoryPhysical)
		if n := sumPoints(res); n != 33*33*3 {
			t.Errorf("Points の総和 = %d, want %d(格子全体)", n, 33*33*3)
		}
		c := get(res, "hfull-bfull-plus")
		if c.SP != (Stats{HP: 32, Def: 32}) || c.Nature != (Nature{Plus: StatDef, Minus: StatAtk}) || c.Points != 1 {
			t.Errorf("hfull-bfull-plus: SP=%+v Nature=%+v Points=%d", c.SP, c.Nature, c.Points)
		}
		c = get(res, "hnone-bnone-minus")
		if c.SP != (Stats{}) || c.Nature != (Nature{Plus: StatAtk, Minus: StatDef}) || c.Points != 1 {
			t.Errorf("hnone-bnone-minus: SP=%+v Nature=%+v Points=%d", c.SP, c.Nature, c.Points)
		}
		c = get(res, "hnone-bnone-neutral")
		if c.SP != (Stats{}) || c.Nature != NatureNeutral || c.Points != 1 {
			t.Errorf("hnone-bnone-neutral: SP=%+v Nature=%+v Points=%d", c.SP, c.Nature, c.Points)
		}
		c = get(res, "hmid-bmid-plus")
		if c.Points != 31*31 || c.SP.HP < 1 || c.SP.HP > 31 || c.SP.Def < 1 || c.SP.Def > 31 {
			t.Errorf("hmid-bmid-plus: SP=%+v Points=%d, want Points=%d で代表 SP は 1..31", c.SP, c.Points, 31*31)
		}
	})

	t.Run("防御側・特殊", func(t *testing.T) {
		res := call(SideDefender, CategorySpecial)
		if c := get(res, "hfull-dfull-plus"); c.SP != (Stats{HP: 32, SpD: 32}) ||
			c.Nature != (Nature{Plus: StatSpD, Minus: StatAtk}) {
			t.Errorf("hfull-dfull-plus: SP=%+v Nature=%+v", c.SP, c.Nature)
		}
		if c := get(res, "hnone-dnone-minus"); c.Nature != (Nature{Plus: StatAtk, Minus: StatSpD}) {
			t.Errorf("hnone-dnone-minus: Nature=%+v", c.Nature)
		}
	})

	t.Run("攻撃側・物理", func(t *testing.T) {
		res := call(SideAttacker, CategoryPhysical)
		if n := sumPoints(res); n != 33*3 {
			t.Errorf("Points の総和 = %d, want %d", n, 33*3)
		}
		if c := get(res, "afull-plus"); c.SP != (Stats{Atk: 32}) || c.Nature != (Nature{Plus: StatAtk, Minus: StatSpA}) {
			t.Errorf("afull-plus: SP=%+v Nature=%+v", c.SP, c.Nature)
		}
		// 関連ステータスが atk のときだけ、相手方は spa に置く。
		if c := get(res, "afull-minus"); c.Nature != (Nature{Plus: StatSpA, Minus: StatAtk}) {
			t.Errorf("afull-minus: Nature=%+v", c.Nature)
		}
	})

	t.Run("攻撃側・特殊", func(t *testing.T) {
		res := call(SideAttacker, CategorySpecial)
		if c := get(res, "cfull-plus"); c.SP != (Stats{SpA: 32}) || c.Nature != (Nature{Plus: StatSpA, Minus: StatAtk}) {
			t.Errorf("cfull-plus: SP=%+v Nature=%+v", c.SP, c.Nature)
		}
		if c := get(res, "cfull-minus"); c.Nature != (Nature{Plus: StatAtk, Minus: StatSpA}) {
			t.Errorf("cfull-minus: Nature=%+v", c.Nature)
		}
	})
}

// Percent 観測と Damage 観測の混在は許容され、真値は完全一致のまま残る。
func TestReverseMixedObservationKinds(t *testing.T) {
	move := revMove(CategoryPhysical)
	known := revDefender(Stats{HP: 32, Def: 32}, Nature{Plus: StatDef, Minus: StatAtk}, nil)
	truthSP := Stats{Atk: 32}
	truthNature := Nature{Plus: StatAtk, Minus: StatSpA}
	truth := Individual{
		Species: revAttackerSpecies(), Level: DefaultLevel,
		Nature: truthNature, SP: truthSP, Status: StatusNone,
	}
	base, err := CalcDamage(DamageInput{Format: FormatSingle, Attacker: truth, Defender: known, Move: move})
	if err != nil {
		t.Fatalf("真値の CalcDamage: %v", err)
	}
	res, err := CalcReverse(ReverseInput{
		Format: FormatSingle, Side: SideAttacker, Known: known,
		UnknownSpecies: revAttackerSpecies(), Move: move,
		Observations: []Observation{
			{Percent: DisplayPercent(base.Rolls[3], base.DefenderHP)},
			{Damage: base.Rolls[9]},
		},
	})
	if err != nil {
		t.Fatalf("Percent と Damage の混在でエラー: %v", err)
	}
	wantArch, _ := ArchetypeOf(SideAttacker, CategoryPhysical, truthSP, truthNature)
	if _, c := findCandidate(res.Candidates, wantArch.Key, ""); c == nil || !c.Exact {
		t.Errorf("混在観測で真値の型 %q が完全一致にならない: %+v", wantArch.Key, c)
	}
}

// ---------------------------------------------------------------------------
// 受け入れ条件 14: Recall@5(小標本の早期検知)
//
// ★ 合格基準そのものではない。合格基準は docs/test-strategy.md の
//    「1回観測 Recall@5 >= 80%、2回観測 >= 95%、全ポケモン 1,000 ケース」であり、
//    engine/reverse_recall_test.go の TestAllSpeciesReverseRecall(allspecies タグ、
//    make test-all-species)で判定する。
//    ここは make test の速度を保ったままの回帰検知用で、標本が小さいぶん
//    標本誤差を見込んだ下限にしてある(ADR-0010 §10)。この下限を理由に
//    本来の基準を下げてはならない(CLAUDE.md 絶対ルール6)。
// ---------------------------------------------------------------------------

func TestReverseRecallSmoke(t *testing.T) {
	const (
		cases      = 60
		minRecall1 = 75         // 1回観測。全種族 1,000 ケースの実測は defender 92.9% / attacker 95.0%
		minRecall2 = 88         // 2回観測。全種族 1,000 ケースの実測は defender 95.6% / attacker 95.5%
		fixedSeed  = 0x50314238 // "P1B8"
	)
	for _, side := range []ReverseSide{SideDefender, SideAttacker} {
		for _, nObs := range []int{1, 2} {
			want := minRecall1
			if nObs == 2 {
				want = minRecall2
			}
			hit, total := reverseRecall(t, side, smokeSpecies(), cases, nObs, fixedSeed)
			got := 100 * hit / total
			t.Logf("side=%s 観測 %d 件: Recall@5 = %d%% (%d/%d)", side, nObs, got, hit, total)
			if got < want {
				t.Errorf("side=%s 観測 %d 件の Recall@5 = %d%% (%d/%d), want >= %d%%",
					side, nObs, got, hit, total, want)
			}
		}
	}
}

// smokeSpecies は小標本テスト用の種族(耐久・タイプを散らした12件)。
// 全種族での判定は TestAllSpeciesReverseRecall が testdata/golden から読む。
func smokeSpecies() []Species {
	return []Species{
		{Key: "s01", Types: []Type{TypeNormal}, BaseStats: Stats{HP: 55, Atk: 81, Def: 60, SpA: 50, SpD: 70, Spe: 97}},
		{Key: "s02", Types: []Type{TypeWater, TypeGround}, BaseStats: Stats{HP: 100, Atk: 100, Def: 90, SpA: 85, SpD: 90, Spe: 60}},
		{Key: "s03", Types: []Type{TypeSteel, TypeFairy}, BaseStats: Stats{HP: 80, Atk: 50, Def: 115, SpA: 80, SpD: 95, Spe: 30}},
		{Key: "s04", Types: []Type{TypeFire}, BaseStats: Stats{HP: 78, Atk: 84, Def: 78, SpA: 109, SpD: 85, Spe: 100}},
		{Key: "s05", Types: []Type{TypeDragon, TypeGround}, BaseStats: Stats{HP: 108, Atk: 130, Def: 95, SpA: 80, SpD: 85, Spe: 102}},
		{Key: "s06", Types: []Type{TypeGhost}, BaseStats: Stats{HP: 45, Atk: 30, Def: 35, SpA: 20, SpD: 20, Spe: 45}},
		{Key: "s07", Types: []Type{TypeRock, TypeIce}, BaseStats: Stats{HP: 95, Atk: 95, Def: 180, SpA: 95, SpD: 45, Spe: 70}},
		{Key: "s08", Types: []Type{TypePsychic}, BaseStats: Stats{HP: 190, Atk: 33, Def: 58, SpA: 33, SpD: 58, Spe: 33}},
		{Key: "s09", Types: []Type{TypeDark, TypeFlying}, BaseStats: Stats{HP: 100, Atk: 125, Def: 52, SpA: 105, SpD: 52, Spe: 71}},
		{Key: "s10", Types: []Type{TypeGrass, TypePoison}, BaseStats: Stats{HP: 80, Atk: 82, Def: 83, SpA: 100, SpD: 100, Spe: 80}},
		{Key: "s11", Types: []Type{TypeElectric}, BaseStats: Stats{HP: 35, Atk: 55, Def: 40, SpA: 50, SpD: 50, Spe: 90}},
		{Key: "s12", Types: []Type{TypeFighting, TypeBug}, BaseStats: Stats{HP: 70, Atk: 110, Def: 80, SpA: 55, SpD: 80, Spe: 105}},
	}
}

// ---------------------------------------------------------------------------
// Recall テストの共通ヘルパー(ADR-0010 §7)
//
// TestReverseRecallSmoke(小標本・タグなし)と TestAllSpeciesReverseRecall
// (全種族・allspecies タグ)が同じ手順を共有するため、タグの付いていない
// このファイルに置く。真値の型は ArchetypeOf が決める = Recall の分母の定義そのもの。
// ---------------------------------------------------------------------------

// revRNG は xorshift64。固定シードの再現性を Go の math/rand の実装に依存させない。
type revRNG struct{ s uint64 }

func newRevRNG(seed uint64) *revRNG {
	if seed == 0 {
		seed = 0x9E3779B97F4A7C15
	}
	return &revRNG{s: seed}
}

func (r *revRNG) next() uint64 {
	r.s ^= r.s << 13
	r.s ^= r.s >> 7
	r.s ^= r.s << 17
	return r.s
}

func (r *revRNG) intn(n int) int { return int(r.next() % uint64(n)) }

// revRecallTypes は代表技のタイプ18種。
var revRecallTypes = []Type{
	TypeNormal, TypeFire, TypeWater, TypeElectric, TypeGrass, TypeIce,
	TypeFighting, TypePoison, TypeGround, TypeFlying, TypePsychic, TypeBug,
	TypeRock, TypeGhost, TypeDragon, TypeDark, TypeSteel, TypeFairy,
}

// revRecallPowers は代表技の威力。
var revRecallPowers = []int{60, 80, 90, 100, 120}

// revNatureFor は関連ステータスに対する性格クラスの代表 Nature を返す(ADR-0010 §4)。
// 下降側は結果に影響しないステータスに置く。
func revNatureFor(stat StatKey, c NatureClass) Nature {
	other := StatAtk
	if stat == StatAtk {
		other = StatSpA
	}
	switch c {
	case NatureClassPlus:
		return Nature{Plus: stat, Minus: other}
	case NatureClassMinus:
		return Nature{Plus: other, Minus: stat}
	}
	return NatureNeutral
}

// revRecallItems は逆算に渡す持ち物候補。真値はこの中から選ぶ。
// engine に持ち物一覧を持ち込まない方針なので、テスト側で効果を組み立てる(ADR-0005)。
func revRecallItems(side ReverseSide, cat MoveCategory) []*Item {
	if side == SideDefender {
		if cat == CategorySpecial {
			return []*Item{nil, revVest()}
		}
		return []*Item{nil, revEviolite()}
	}
	if cat == CategorySpecial {
		return []*Item{nil, {ID: "choicespecs", NameJa: "こだわりメガネ",
			Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpA: 6144}}}}
	}
	return []*Item{nil, {ID: "choiceband", NameJa: "こだわりハチマキ",
		Effect: &ItemEffect{StatMods: map[StatKey]int{StatAtk: 6144}}}}
}

// reverseRecall は「真値の型が上位5件に入った割合」を数える(ADR-0010 §7)。
//
// 真値の作り方:
//  1. 種族は species から一様に2体(既知側・未知側)
//  2. 技は タイプ18 × 分類2 × 威力5 の代表技から一様に
//  3. 既知側は現実的な調整(無振り / 関連ステータス32 + 上昇補正)
//  4. 真値(未知側)は SP が 0 か 32、性格は無補正か上昇補正、持ち物は候補集合から
//     → ADR-0010 §5.3 の上位8型(attacker は上位4型)のいずれかに必ず属する
//  5. ダメージ0・無効相性・表示0% は観測にならないので引き直す。
//     表示が 100% を超える観測(瀕死)は 100% に丸めて分母に残す
//  6. 16ロールから一様に nObs 段階を独立に選び、DisplayPercent で観測化する
func reverseRecall(t *testing.T, side ReverseSide, species []Species, cases, nObs int, seed uint64) (hit, total int) {
	t.Helper()
	if len(species) == 0 {
		t.Fatal("種族集合が空")
	}
	r := newRevRNG(seed)
	// スタブ段階で無限ループにならないよう、引き直しの回数に上限を置く。
	const maxAttempts = 1 << 16
	attempts := 0

	for total < cases {
		attempts++
		if attempts > maxAttempts {
			t.Fatalf("観測を作れるケースが %d 回引いても %d 件しか集まらない"+
				"(DisplayPercent / CalcDamage を確認)", maxAttempts, total)
		}

		knownSpecies := species[r.intn(len(species))]
		unknownSpecies := species[r.intn(len(species))]

		cat := CategoryPhysical
		if r.intn(2) == 0 {
			cat = CategorySpecial
		}
		move := Move{
			ID: "recall", Type: revRecallTypes[r.intn(len(revRecallTypes))],
			Category: cat, Power: revRecallPowers[r.intn(len(revRecallPowers))],
		}
		atkStat, defStat := StatAtk, StatDef
		if cat == CategorySpecial {
			atkStat, defStat = StatSpA, StatSpD
		}

		items := revRecallItems(side, cat)

		// --- 真値(未知側): 現実的な調整 ---
		xStat := defStat
		if side == SideAttacker {
			xStat = atkStat
		}
		hpSP := 0
		if side == SideDefender {
			hpSP = []int{0, MaxSPPerStat}[r.intn(2)]
		}
		xSP := []int{0, MaxSPPerStat}[r.intn(2)]
		natClass := []NatureClass{NatureClassNeutral, NatureClassPlus}[r.intn(2)]
		itemIdx := r.intn(len(items))

		truthSP := Stats{HP: hpSP}.WithStat(xStat, xSP)
		truthNature := revNatureFor(xStat, natClass)
		truth := Individual{
			Species: unknownSpecies, Level: DefaultLevel, Nature: truthNature,
			SP: truthSP, Item: items[itemIdx], Status: StatusNone,
		}

		// --- 既知側(自分): 現実的な調整 ---
		known := Individual{Species: knownSpecies, Level: DefaultLevel, Status: StatusNone}
		if r.intn(2) == 0 {
			if side == SideDefender {
				// 自分は攻撃側。関連する攻撃を振り切る。
				known.SP = Stats{}.WithStat(atkStat, MaxSPPerStat)
				known.Nature = revNatureFor(atkStat, NatureClassPlus)
			} else {
				// 自分は防御側。H と関連する防御を振り切る。
				known.SP = Stats{HP: MaxSPPerStat}.WithStat(defStat, MaxSPPerStat)
				known.Nature = revNatureFor(defStat, NatureClassPlus)
			}
		}

		// --- 観測を作る ---
		dmgIn := DamageInput{Format: FormatSingle, Move: move}
		var refHP int
		if side == SideDefender {
			dmgIn.Attacker, dmgIn.Defender = known, truth
		} else {
			dmgIn.Attacker, dmgIn.Defender = truth, known
		}
		res, err := CalcDamage(dmgIn)
		if err != nil {
			continue // SP 合計超過などは引き直す(現実的な調整では起きない)
		}
		refHP = res.DefenderHP
		if res.Effectiveness == 0 || res.MinDamage() == 0 || refHP <= 0 {
			continue // 無効相性・ダメージ0は観測にならない
		}
		obs := make([]Observation, 0, nObs)
		ok := true
		for k := 0; k < nObs; k++ {
			pct := DisplayPercent(res.Rolls[r.intn(16)], refHP)
			if pct <= 0 {
				ok = false
				break
			}
			if pct > 100 {
				// ゲームの表示は 100% で頭打ち。相手が瀕死になる観測も分母に残す(ADR-0010 §7)。
				pct = 100
			}
			obs = append(obs, Observation{Percent: pct})
		}
		if !ok {
			continue // 表示 0% は観測にならない
		}

		total++

		// --- 逆算して真値の型の順位を見る ---
		out, err := CalcReverse(ReverseInput{
			Format: FormatSingle, Side: side, Known: known,
			UnknownSpecies: unknownSpecies, Move: move,
			ItemCandidates: items, Observations: obs,
		})
		if err != nil {
			t.Fatalf("CalcReverse: %v", err)
		}
		wantArch, found := ArchetypeOf(side, cat, truthSP, truthNature)
		if !found {
			t.Fatalf("真値 SP=%+v nature=%+v を型にできない", truthSP, truthNature)
		}
		wantItemID := ""
		if items[itemIdx] != nil {
			wantItemID = items[itemIdx].ID
		}
		if rank, cand := findCandidate(out.Candidates, wantArch.Key, wantItemID); cand != nil && rank < 5 {
			hit++
		}
	}
	return hit, total
}
