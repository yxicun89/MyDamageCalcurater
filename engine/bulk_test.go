package engine

// 一括計算(P1-7)の受け入れ条件をテストで固定する。ADR-0009 が定義の正。
// 方針: CalcBulk は CalcDamage の合成にすぎない。各行は同じ入力の CalcDamage と完全一致すること。

import (
	"errors"
	"reflect"
	"testing"
)

// bulkDefenderSpecies は防御側の試験用種族(HP100 / B80 / D80 のエスパー単)。
func bulkDefenderSpecies() Species {
	return Species{
		Key:       "0999-000",
		NameJa:    "テストぼうぎょ",
		Types:     []Type{TypePsychic},
		BaseStats: Stats{HP: 100, Atk: 50, Def: 80, SpA: 50, SpD: 80, Spe: 50},
	}
}

// bulkAttacker は攻撃側の試験用個体(A/C ともに実数値 200)。
func bulkAttacker() Individual {
	return Individual{
		Species: Species{
			Key:       "0998-000",
			NameJa:    "テストこうげき",
			Types:     []Type{TypeWater},
			BaseStats: Stats{HP: 100, Atk: 180, Def: 50, SpA: 180, SpD: 50, Spe: 100},
		},
		Nature: NatureNeutral,
	}
}

// bulkInput は分類と技タイプを指定した一括計算の入力を作る(毎回同じ値)。
func bulkInput(cat MoveCategory, moveType Type) BulkInput {
	return BulkInput{
		Format:          FormatSingle,
		Attacker:        bulkAttacker(),
		DefenderSpecies: bulkDefenderSpecies(),
		Move:            Move{ID: "testmove", NameJa: "テストわざ", Type: moveType, Category: cat, Power: 100},
	}
}

// wantDefender は ADR-0009 が定めるプリセットからの防御側個体の組み立て。
func wantDefender(p DefenderPreset, species Species, item *Item) Individual {
	return Individual{
		Species: species,
		Level:   DefaultLevel,
		Nature:  p.Nature,
		SP:      p.SP,
		Item:    item,
		Status:  StatusNone,
	}
}

// wantResult は行の期待値を CalcDamage から直接作る。
func wantResult(t *testing.T, in BulkInput, def Individual) DamageResult {
	t.Helper()
	r, err := calcDamage(DamageInput{
		Format:   in.Format,
		Attacker: in.Attacker,
		Defender: def,
		Move:     in.Move,
		Field:    in.Field,
		Critical: in.Critical,
	})
	if err != nil {
		t.Fatalf("CalcDamage: %v", err)
	}
	return r
}

func presetKeys(ps []DefenderPreset) []PresetKey {
	keys := make([]PresetKey, 0, len(ps))
	for _, p := range ps {
		keys = append(keys, p.Key)
	}
	return keys
}

func rowKeys(rows []BulkRow) []PresetKey {
	keys := make([]PresetKey, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, r.Preset)
	}
	return keys
}

// --- プリセット定義 -------------------------------------------------------

// ADR-0009 §1(2026-09-21 改訂)の既定カタログ8件。
//
// 期待値を変えた理由: ユーザー決定(docs/ai-shared/DECISIONS.md 2026-09-21 最終エントリ、
// docs/requirements.md「相手側の一括表示」)で防御プリセットを再定義した。
//   - hb / hd は「H32・B(D)32・**性格補正なし**」= HB振り / HD振り に変わった(初版は上昇性格込み)
//   - 初版の hb / hd(上昇性格込みの最大耐久)は hb_full / hd_full として新設したキーへ移った
//   - hb_boost / hd_boost は初版の仮定どおりで確定(人間の確認待ちは解消)
//
// ラベルは要件書の表記に合わせる。ADR-0010 §5.3 の型ラベルとの対応は ADR-0009 §1 の表を参照。
func TestDefenderPresetCatalogDefinitions(t *testing.T) {
	want := []DefenderPreset{
		{Key: PresetNone, Label: "無振り", SP: Stats{}, Nature: NatureNeutral, Applies: ""},
		{Key: PresetHP, Label: "H振り", SP: Stats{HP: 32}, Nature: NatureNeutral, Applies: ""},
		{Key: PresetHBBoost, Label: "H振り+B補正", SP: Stats{HP: 32}, Nature: Nature{Plus: StatDef, Minus: StatAtk}, Applies: CategoryPhysical},
		{Key: PresetHB, Label: "HB振り", SP: Stats{HP: 32, Def: 32}, Nature: NatureNeutral, Applies: CategoryPhysical},
		{Key: PresetHBFull, Label: "HB特化", SP: Stats{HP: 32, Def: 32}, Nature: Nature{Plus: StatDef, Minus: StatAtk}, Applies: CategoryPhysical},
		{Key: PresetHDBoost, Label: "H振り+D補正", SP: Stats{HP: 32}, Nature: Nature{Plus: StatSpD, Minus: StatAtk}, Applies: CategorySpecial},
		{Key: PresetHD, Label: "HD振り", SP: Stats{HP: 32, SpD: 32}, Nature: NatureNeutral, Applies: CategorySpecial},
		{Key: PresetHDFull, Label: "HD特化", SP: Stats{HP: 32, SpD: 32}, Nature: Nature{Plus: StatSpD, Minus: StatAtk}, Applies: CategorySpecial},
	}
	got := DefenderPresetCatalog()
	if len(got) != len(want) {
		t.Fatalf("カタログ件数 %d、期待 %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("catalog[%d]=%+v want %+v", i, got[i], want[i])
		}
	}
	// SP はドメイン規約の範囲内(1ステータス 32 以下・合計 66 以下)。
	for _, p := range got {
		for _, k := range AllStatKeys() {
			if v := p.SP.Get(k); v < 0 || v > MaxSPPerStat {
				t.Errorf("%s の SP %s=%d が範囲外", p.Key, k, v)
			}
		}
		if p.SP.Sum() > MaxSPTotal {
			t.Errorf("%s の SP 合計 %d が上限 %d 超過", p.Key, p.SP.Sum(), MaxSPTotal)
		}
	}
}

// カタログは呼び出しごとに独立(呼び出し側の変更が次回に漏れない)。
func TestDefenderPresetCatalogIsIndependentPerCall(t *testing.T) {
	first := DefenderPresetCatalog()
	if len(first) == 0 {
		t.Fatal("カタログが空")
	}
	first[0].Label = "書き換え"
	first[0].SP = Stats{HP: 7}
	second := DefenderPresetCatalog()
	if second[0].Label == "書き換え" || second[0].SP.HP == 7 {
		t.Errorf("カタログが共有されている: %+v", second[0])
	}
}

// ADR-0009 §3: presets 省略時の既定セットは技の分類で切り替わる。
func TestDefaultDefenderPresetsByCategory(t *testing.T) {
	tests := []struct {
		name     string
		category MoveCategory
		want     []PresetKey
	}{
		// ADR-0009 §3(改訂): 物理・特殊の既定セットは 4 件から 5 件になった(hb_full / hd_full の新設分)。
		{"物理はB系", CategoryPhysical, []PresetKey{PresetNone, PresetHP, PresetHBBoost, PresetHB, PresetHBFull}},
		{"特殊はD系", CategorySpecial, []PresetKey{PresetNone, PresetHP, PresetHDBoost, PresetHD, PresetHDFull}},
		{"変化技は none と hp のみ", CategoryStatus, []PresetKey{PresetNone, PresetHP}},
		{"分類なしも none と hp のみ", MoveCategory(""), []PresetKey{PresetNone, PresetHP}},
		{"未知の分類も none と hp のみ", MoveCategory("unknown"), []PresetKey{PresetNone, PresetHP}},
	}
	catalog := DefenderPresetCatalog()
	byKey := map[PresetKey]DefenderPreset{}
	for _, p := range catalog {
		byKey[p.Key] = p
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultDefenderPresets(tt.category)
			if !reflect.DeepEqual(presetKeys(got), tt.want) {
				t.Fatalf("keys=%v want %v", presetKeys(got), tt.want)
			}
			// 既定セットの中身はカタログと同じ定義であること(別の SP を持たない)。
			for _, p := range got {
				if !reflect.DeepEqual(p, byKey[p.Key]) {
					t.Errorf("%s の定義がカタログと異なる: %+v want %+v", p.Key, p, byKey[p.Key])
				}
			}
		})
	}
}

// ADR-0009 §1 と ADR-0010 §R1(P1-12 改訂)の対応。
// 逆算は型(Archetype)を持たなくなり、防御側は「H32 前提 × B(D) SP 0..32 × 性格{補正なし, 上昇}」を
// 探索する。H32 の防御プリセットはすべてこの探索空間の点なので、そのプリセットで作った観測を逆算すると、
// プリセットの性格クラスの候補が説明可能になり、プリセットの B(D) SP がその範囲に入らなければならない。
// 無振り(none = H0)だけは H32 前提の外にある(ユーザー決定の前提。ADR-0010 §R7)。
// (旧 TestDefenderPresetsMatchReverseArchetypes の置き換え。プリセットのラベルは
// TestDefenderPresetCatalog の表が固定している。)
func TestDefenderPresetsInsideReverseSpace(t *testing.T) {
	outside := map[PresetKey]bool{PresetNone: true}
	species := Species{Key: "0990-000", NameJa: "テストぼうぎょ", Types: []Type{TypePsychic},
		BaseStats: Stats{HP: 100, Atk: 50, Def: 80, SpA: 50, SpD: 80, Spe: 50}}
	attacker := Individual{Species: Species{Key: "0989-000", NameJa: "テストこうげき", Types: []Type{TypeWater},
		BaseStats: Stats{HP: 100, Atk: 120, Def: 70, SpA: 120, SpD: 70, Spe: 100}}, Level: DefaultLevel}
	for _, p := range DefenderPresetCatalog() {
		t.Run(string(p.Key), func(t *testing.T) {
			if p.SP.HP != MaxSPPerStat {
				if !outside[p.Key] {
					t.Fatalf("%q は H%d(H32 前提の外)。想定外のプリセットが増えた", p.Key, p.SP.HP)
				}
				return
			}
			if outside[p.Key] {
				t.Fatalf("%q は H32 なのに探索空間の外として扱われている", p.Key)
			}
			for _, cat := range []MoveCategory{CategoryPhysical, CategorySpecial} {
				if p.Applies != "" && p.Applies != cat {
					continue
				}
				stat := StatDef
				if cat == CategorySpecial {
					stat = StatSpD
				}
				class := NatureClassNeutral
				if p.Nature.Plus == stat {
					class = NatureClassPlus
				} else if p.Nature != NatureNeutral {
					t.Fatalf("%q の性格 %+v は 補正なし/%s上昇 のどちらでもない", p.Key, p.Nature, stat)
				}
				move := Move{ID: "m", NameJa: "テストわざ", Type: TypeWater, Category: cat, Power: 100}
				res, err := calcDamage(DamageInput{Format: FormatSingle, Attacker: attacker, Defender: p.Defender(species, nil), Move: move})
				if err != nil {
					t.Fatal(err)
				}
				pct := (res.Rolls[5]*100 + res.DefenderHP/2) / res.DefenderHP
				out, err := calcReverse(ReverseInput{Format: FormatSingle, Side: SideDefender, Known: attacker,
					UnknownSpecies: species, Move: move, Observations: []Observation{{Percent: pct}}})
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, c := range out.Candidates {
					if c.NatureClass != class || c.ItemID != "" {
						continue
					}
					found = true
					in := false
					for _, r := range c.Ranges {
						if r.Min <= p.SP.Get(stat) && p.SP.Get(stat) <= r.Max {
							in = true
						}
					}
					if !c.Exact || !in {
						t.Errorf("%q/%s: 候補(%s)に SP %d が入らない: Exact=%v Ranges=%+v", p.Key, cat, class, p.SP.Get(stat), c.Exact, c.Ranges)
					}
				}
				if !found {
					t.Errorf("%q/%s: 性格クラス %s の候補が無い", p.Key, cat, class)
				}
			}
		})
	}
}

// プリセットから組み立てる防御側個体の形(ADR-0009 §1)。
func TestDefenderPresetBuildsIndividual(t *testing.T) {
	species := bulkDefenderSpecies()
	item := &Item{ID: "assaultvest", NameJa: "テストもちもの2", Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}
	p := DefenderPreset{Key: PresetHD, Label: "HD特化", SP: Stats{HP: 32, SpD: 32}, Nature: Nature{Plus: StatSpD, Minus: StatAtk}}
	got := p.Defender(species, item)
	if !reflect.DeepEqual(got, wantDefender(p, species, item)) {
		t.Fatalf("Defender=%+v want %+v", got, wantDefender(p, species, item))
	}
	if err := got.Validate(); err != nil {
		t.Errorf("組み立てた個体が不正: %v", err)
	}
	if none := p.Defender(species, nil); none.Item != nil {
		t.Errorf("持ち物なしは nil のままであること: %+v", none.Item)
	}
}

// --- 行が CalcDamage の合成であること -------------------------------------

// 受け入れ条件 a: 各行は手組みした防御側個体に対する CalcDamage と完全一致する。
func TestCalcBulkRowsMatchCalcDamage(t *testing.T) {
	tests := []struct {
		name     string
		category MoveCategory
		moveType Type
	}{
		{"物理・不一致", CategoryPhysical, TypeNormal},
		{"物理・一致", CategoryPhysical, TypeWater},
		{"特殊・一致", CategorySpecial, TypeWater},
		{"特殊・抜群", CategorySpecial, TypeDark},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := bulkInput(tt.category, tt.moveType)
			presets := DefaultDefenderPresets(tt.category)
			// ADR-0009 §3(改訂): 物理・特殊の既定セットは 5 件(none, hp, *_boost, *, *_full)。
			if len(presets) != 5 {
				t.Fatalf("既定セットは5件のはず: %v", presetKeys(presets))
			}
			res, err := calcBulk(in)
			if err != nil {
				t.Fatalf("CalcBulk: %v", err)
			}
			if res.DefenderSpeciesKey != in.DefenderSpecies.Key {
				t.Errorf("DefenderSpeciesKey=%q want %q", res.DefenderSpeciesKey, in.DefenderSpecies.Key)
			}
			if len(res.Rows) != len(presets) {
				t.Fatalf("行数=%d want %d (rows=%+v)", len(res.Rows), len(presets), rowKeys(res.Rows))
			}
			for i, p := range presets {
				row := res.Rows[i]
				if row.Preset != p.Key {
					t.Errorf("rows[%d].Preset=%q want %q", i, row.Preset, p.Key)
				}
				if row.PresetLabel != p.Label {
					t.Errorf("rows[%d].PresetLabel=%q want %q", i, row.PresetLabel, p.Label)
				}
				def := wantDefender(p, in.DefenderSpecies, nil)
				if !reflect.DeepEqual(row.Defender, def) {
					t.Errorf("rows[%d].Defender=%+v want %+v", i, row.Defender, def)
				}
				if want := wantResult(t, in, def); !reflect.DeepEqual(row.Result, want) {
					t.Errorf("rows[%d].Result=%+v want %+v", i, row.Result, want)
				}
			}
			// 耐久が上がる順(ADR-0009 §1): none >= hp >= *_boost >= * >= *_full のダメージ。
			// 厳密な大小関係は TestCalcBulkStrictDurabilityOrder が固定する。
			for i := 1; i < len(res.Rows); i++ {
				if res.Rows[i].Result.MaxDamage() > res.Rows[i-1].Result.MaxDamage() {
					t.Errorf("行 %d のダメージが前行より大きい: %d > %d", i, res.Rows[i].Result.MaxDamage(), res.Rows[i-1].Result.MaxDamage())
				}
			}
		})
	}
}

// 受け入れ条件 g: 行数・ラベルは渡した定義から出る(engine が勝手な表示名を作らない)。
func TestCalcBulkUsesGivenPresetDefinitions(t *testing.T) {
	in := bulkInput(CategoryPhysical, TypeWater)
	custom := []DefenderPreset{
		{Key: "custom_a", Label: "自作A", SP: Stats{HP: 32, Def: 32, SpD: 2}, Nature: Nature{Plus: StatDef, Minus: StatSpe}},
		{Key: "custom_b", Label: "", SP: Stats{Def: 32}, Nature: NatureNeutral},
	}
	in.Presets = custom
	res, err := calcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("行数=%d want 2", len(res.Rows))
	}
	if res.Rows[0].Preset != "custom_a" || res.Rows[0].PresetLabel != "自作A" {
		t.Errorf("rows[0]=%q/%q want custom_a/自作A", res.Rows[0].Preset, res.Rows[0].PresetLabel)
	}
	// Label 空はエラーにせず Key を表示名に使う(ADR-0009 §5)
	if res.Rows[1].Preset != "custom_b" || res.Rows[1].PresetLabel != "custom_b" {
		t.Errorf("rows[1]=%q/%q want custom_b/custom_b", res.Rows[1].Preset, res.Rows[1].PresetLabel)
	}
	for i, p := range custom {
		def := wantDefender(p, in.DefenderSpecies, nil)
		if want := wantResult(t, in, def); !reflect.DeepEqual(res.Rows[i].Result, want) {
			t.Errorf("rows[%d].Result が CalcDamage と不一致", i)
		}
	}
}

// PresetKeys はカタログからの選別で、指定した順序がそのまま行順になる。
func TestCalcBulkPresetKeysSelectOrder(t *testing.T) {
	in := bulkInput(CategoryPhysical, TypeWater)
	in.PresetKeys = []PresetKey{PresetHD, PresetNone, PresetHBBoost}
	res, err := calcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if !reflect.DeepEqual(rowKeys(res.Rows), in.PresetKeys) {
		t.Fatalf("keys=%v want %v", rowKeys(res.Rows), in.PresetKeys)
	}
}

// --- 持ち物バリアント -----------------------------------------------------

// 受け入れ条件 d: nil は素の1通り。複数指定は プリセット×持ち物 の順で並ぶ。
func TestCalcBulkItemVariants(t *testing.T) {
	// 特防1.5倍の持ち物相当(D×1.5)。engine に持ち物一覧は持たせず、テストが定義を渡す。
	vest := &Item{ID: "assaultvest", NameJa: "テストもちもの2", Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}
	shield := &Item{ID: "defboost", NameJa: "テストぼうぐ", Effect: &ItemEffect{StatMods: map[StatKey]int{StatDef: 6144}}}

	t.Run("nilは素の1通り", func(t *testing.T) {
		in := bulkInput(CategorySpecial, TypeWater)
		in.PresetKeys = []PresetKey{PresetNone, PresetHP}
		res, err := calcBulk(in)
		if err != nil {
			t.Fatalf("CalcBulk: %v", err)
		}
		if len(res.Rows) != 2 {
			t.Fatalf("行数=%d want 2", len(res.Rows))
		}
		for i, row := range res.Rows {
			if row.Item != nil || row.ItemID != "" {
				t.Errorf("rows[%d] は持ち物なしのはず: item=%+v id=%q", i, row.Item, row.ItemID)
			}
		}
	})

	t.Run("プリセット×持ち物の順序と件数", func(t *testing.T) {
		in := bulkInput(CategorySpecial, TypeWater)
		in.PresetKeys = []PresetKey{PresetNone, PresetHD}
		in.ItemVariants = []*Item{nil, vest, shield}
		res, err := calcBulk(in)
		if err != nil {
			t.Fatalf("CalcBulk: %v", err)
		}
		if len(res.Rows) != 6 {
			t.Fatalf("行数=%d want 6 (2プリセット×3持ち物)", len(res.Rows))
		}
		wantOrder := []struct {
			preset PresetKey
			item   *Item
		}{
			{PresetNone, nil}, {PresetNone, vest}, {PresetNone, shield},
			{PresetHD, nil}, {PresetHD, vest}, {PresetHD, shield},
		}
		catalog := map[PresetKey]DefenderPreset{}
		for _, p := range DefenderPresetCatalog() {
			catalog[p.Key] = p
		}
		for i, w := range wantOrder {
			row := res.Rows[i]
			if row.Preset != w.preset {
				t.Errorf("rows[%d].Preset=%q want %q", i, row.Preset, w.preset)
			}
			if row.Item != w.item {
				t.Errorf("rows[%d].Item=%+v want %+v(渡したポインタをそのまま保持する)", i, row.Item, w.item)
			}
			wantID := ""
			if w.item != nil {
				wantID = w.item.ID
			}
			if row.ItemID != wantID {
				t.Errorf("rows[%d].ItemID=%q want %q", i, row.ItemID, wantID)
			}
			def := wantDefender(catalog[w.preset], in.DefenderSpecies, w.item)
			if !reflect.DeepEqual(row.Defender, def) {
				t.Errorf("rows[%d].Defender=%+v want %+v", i, row.Defender, def)
			}
			if want := wantResult(t, in, def); !reflect.DeepEqual(row.Result, want) {
				t.Errorf("rows[%d].Result が CalcDamage と不一致", i)
			}
		}
	})

	t.Run("持ち物で結果が変わる", func(t *testing.T) {
		in := bulkInput(CategorySpecial, TypeWater)
		in.PresetKeys = []PresetKey{PresetNone}
		in.ItemVariants = []*Item{nil, vest, shield}
		res, err := calcBulk(in)
		if err != nil {
			t.Fatalf("CalcBulk: %v", err)
		}
		if len(res.Rows) != 3 {
			t.Fatalf("行数=%d want 3", len(res.Rows))
		}
		bare, withVest, withShield := res.Rows[0].Result, res.Rows[1].Result, res.Rows[2].Result
		if withVest.MaxDamage() >= bare.MaxDamage() {
			t.Errorf("特殊技に特防1.5倍の持ち物でダメージが減っていない: %d >= %d", withVest.MaxDamage(), bare.MaxDamage())
		}
		if withShield.MaxDamage() != bare.MaxDamage() {
			t.Errorf("特殊技に防御補正の持ち物は影響しないはず: %d != %d", withShield.MaxDamage(), bare.MaxDamage())
		}
	})

	// 先頭要素が non-nil でも上書きされず、渡した順序・ポインタのまま行になること
	// (先頭が nil の上のケースだけでは「先頭要素の上書き」を検出できない)。
	t.Run("先頭がnon-nilの順序とポインタ保持", func(t *testing.T) {
		in := bulkInput(CategorySpecial, TypeWater)
		in.PresetKeys = []PresetKey{PresetNone, PresetHD}
		in.ItemVariants = []*Item{vest, nil, shield}
		res, err := calcBulk(in)
		if err != nil {
			t.Fatalf("CalcBulk: %v", err)
		}
		if len(res.Rows) != 6 {
			t.Fatalf("行数=%d want 6 (2プリセット×3持ち物)", len(res.Rows))
		}
		catalog := map[PresetKey]DefenderPreset{}
		for _, p := range DefenderPresetCatalog() {
			catalog[p.Key] = p
		}
		wantOrder := []struct {
			preset PresetKey
			item   *Item
		}{
			{PresetNone, vest}, {PresetNone, nil}, {PresetNone, shield},
			{PresetHD, vest}, {PresetHD, nil}, {PresetHD, shield},
		}
		for i, w := range wantOrder {
			row := res.Rows[i]
			if row.Preset != w.preset {
				t.Errorf("rows[%d].Preset=%q want %q", i, row.Preset, w.preset)
			}
			if row.Item != w.item {
				t.Errorf("rows[%d].Item=%+v want %+v(渡したポインタをそのまま保持する)", i, row.Item, w.item)
			}
			wantID := ""
			if w.item != nil {
				wantID = w.item.ID
			}
			if row.ItemID != wantID {
				t.Errorf("rows[%d].ItemID=%q want %q", i, row.ItemID, wantID)
			}
			def := wantDefender(catalog[w.preset], in.DefenderSpecies, w.item)
			if !reflect.DeepEqual(row.Defender, def) {
				t.Errorf("rows[%d].Defender=%+v want %+v", i, row.Defender, def)
			}
			if want := wantResult(t, in, def); !reflect.DeepEqual(row.Result, want) {
				t.Errorf("rows[%d].Result が CalcDamage と不一致", i)
			}
		}
	})
}

// --- 入力の非破壊・決定性 -------------------------------------------------

// 受け入れ条件 e: 渡した入力(個体・持ち物・スライス)を変更しない。
func TestCalcBulkDoesNotMutateInput(t *testing.T) {
	build := func() BulkInput {
		in := bulkInput(CategoryPhysical, TypeWater)
		in.Attacker.Item = &Item{ID: "choiceband", Effect: &ItemEffect{StatMods: map[StatKey]int{StatAtk: 6144}}}
		in.Presets = []DefenderPreset{
			{Key: PresetNone, Label: "無振り", SP: Stats{}, Nature: NatureNeutral},
			{Key: PresetHB, Label: "HB特化", SP: Stats{HP: 32, Def: 32}, Nature: Nature{Plus: StatDef, Minus: StatAtk}},
		}
		in.ItemVariants = []*Item{nil, {ID: "assaultvest", Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}}
		return in
	}
	in := build()
	if _, err := calcBulk(in); err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if !reflect.DeepEqual(in, build()) {
		t.Errorf("入力が変更された: %+v", in)
	}
}

// 受け入れ条件 e: 同じ入力は常に同じ順序・同じ結果を返す。
func TestCalcBulkDeterministic(t *testing.T) {
	in := bulkInput(CategoryPhysical, TypeWater)
	in.ItemVariants = []*Item{nil, {ID: "assaultvest", Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}}
	first, err := calcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if len(first.Rows) == 0 {
		t.Fatal("行が空")
	}
	for i := 0; i < 3; i++ {
		again, err := calcBulk(in)
		if err != nil {
			t.Fatalf("CalcBulk(%d): %v", i, err)
		}
		if !reflect.DeepEqual(first, again) {
			t.Fatalf("結果が呼び出しごとに異なる: %+v vs %+v", rowKeys(first.Rows), rowKeys(again.Rows))
		}
	}
}

// --- 境界値 ---------------------------------------------------------------

// 受け入れ条件 f: 無効相性は全行ダメージ0・倒せない。
func TestCalcBulkImmuneAllRowsZero(t *testing.T) {
	in := bulkInput(CategoryPhysical, TypeNormal)
	in.DefenderSpecies.Types = []Type{TypeGhost}
	res, err := calcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if len(res.Rows) != 5 {
		t.Fatalf("行数=%d want 5(ADR-0009 §3 改訂: 物理の既定セットは5件)", len(res.Rows))
	}
	for i, row := range res.Rows {
		if row.Result.Effectiveness != 0 {
			t.Errorf("rows[%d].Effectiveness=%v want 0", i, row.Result.Effectiveness)
		}
		for _, d := range row.Result.Rolls {
			if d != 0 {
				t.Fatalf("rows[%d] 無効相性なのにダメージ %v", i, row.Result.Rolls)
			}
		}
		if row.Result.KO.Hits != 0 {
			t.Errorf("rows[%d].KO.Hits=%d want 0", i, row.Result.KO.Hits)
		}
	}
}

// 受け入れ条件 f: 変化技は none/hp の2行、全行0ダメージ(ADR-0009 §3)。
func TestCalcBulkStatusMove(t *testing.T) {
	in := bulkInput(CategoryStatus, TypeNormal)
	in.Move.Power = 0
	res, err := calcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if !reflect.DeepEqual(rowKeys(res.Rows), []PresetKey{PresetNone, PresetHP}) {
		t.Fatalf("keys=%v want [none hp]", rowKeys(res.Rows))
	}
	for i, row := range res.Rows {
		if row.Result.MaxDamage() != 0 {
			t.Errorf("rows[%d] 変化技のダメージ=%d want 0", i, row.Result.MaxDamage())
		}
		if row.Result.DefenderHP <= 0 {
			t.Errorf("rows[%d].DefenderHP=%d、変化技でも実数値は返すこと", i, row.Result.DefenderHP)
		}
	}
}

// 受け入れ条件 f: 最低耐久(HP種族値1 = チャンピオンズの実数値 76)でも破綻しない。
func TestCalcBulkLowHPDefender(t *testing.T) {
	in := bulkInput(CategoryPhysical, TypeWater)
	in.DefenderSpecies.BaseStats = Stats{HP: 1, Atk: 1, Def: 1, SpA: 1, SpD: 1, Spe: 1}
	res, err := calcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if len(res.Rows) != 5 {
		t.Fatalf("行数=%d want 5(ADR-0009 §3 改訂: 物理の既定セットは5件)", len(res.Rows))
	}
	for i, row := range res.Rows {
		wantHP := 1 + 75 + row.Defender.SP.HP
		if row.Result.DefenderHP != wantHP {
			t.Errorf("rows[%d].DefenderHP=%d want %d", i, row.Result.DefenderHP, wantHP)
		}
		if row.Result.KO.Hits != 1 || !row.Result.KO.Guaranteed {
			t.Errorf("rows[%d].KO=%+v want 確定1発", i, row.Result.KO)
		}
	}
}

// 受け入れ条件 f: format=double でも行の構成は single と同じで、各行は Format=double の CalcDamage と一致する。
// 実態: CalcDamage は現状 Format を一切参照しない(ダブル固有補正は ADR-0005 の未対応範囲)ため、
// このテストが固定するのは「行構成が Format に依存しないこと」だけで、Format が CalcDamage へ
// 素通しされていること自体は検証できない(bulk.go が Format を FormatSingle に固定しても通る = 等価変異)。
// CalcDamage がダブル補正を持ったら、single/double で結果が変わる入力を足して素通しを検証すること。
func TestCalcBulkFormatDouble(t *testing.T) {
	single := bulkInput(CategoryPhysical, TypeWater)
	double := bulkInput(CategoryPhysical, TypeWater)
	double.Format = FormatDouble
	sres, err := calcBulk(single)
	if err != nil {
		t.Fatalf("CalcBulk(single): %v", err)
	}
	dres, err := calcBulk(double)
	if err != nil {
		t.Fatalf("CalcBulk(double): %v", err)
	}
	if !reflect.DeepEqual(rowKeys(sres.Rows), rowKeys(dres.Rows)) || len(dres.Rows) != 5 {
		t.Fatalf("double の行構成が single と異なる: %v vs %v", rowKeys(sres.Rows), rowKeys(dres.Rows))
	}
	for i, row := range dres.Rows {
		if want := wantResult(t, double, row.Defender); !reflect.DeepEqual(row.Result, want) {
			t.Errorf("rows[%d] が Format=double の CalcDamage と不一致", i)
		}
	}
}

// SP の上限ちょうど(1ステータス32・合計66)は通る。
func TestCalcBulkSPBoundaryAccepted(t *testing.T) {
	in := bulkInput(CategoryPhysical, TypeWater)
	in.Presets = []DefenderPreset{
		{Key: "max", Label: "上限", SP: Stats{HP: MaxSPPerStat, Def: MaxSPPerStat, SpD: MaxSPTotal - 2*MaxSPPerStat}, Nature: Nature{Plus: StatDef, Minus: StatAtk}},
	}
	if in.Presets[0].SP.Sum() != MaxSPTotal {
		t.Fatalf("テストの前提が壊れている: SP 合計=%d", in.Presets[0].SP.Sum())
	}
	res, err := calcBulk(in)
	if err != nil {
		t.Fatalf("上限ちょうどは受け付けること: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("行数=%d want 1", len(res.Rows))
	}
}

// --- エラー系(ADR-0009 §5)------------------------------------------------

func TestCalcBulkErrors(t *testing.T) {
	valid := DefenderPreset{Key: PresetNone, Label: "無振り"}
	tests := []struct {
		name string
		mod  func(*BulkInput)
		want error
	}{
		{"未知のプリセットキー", func(in *BulkInput) {
			in.PresetKeys = []PresetKey{PresetNone, "hb_super_special"}
		}, ErrUnknownPreset},
		{"PresetKeys の重複", func(in *BulkInput) {
			in.PresetKeys = []PresetKey{PresetNone, PresetHP, PresetNone}
		}, ErrDuplicatePreset},
		{"Presets のキー重複", func(in *BulkInput) {
			in.Presets = []DefenderPreset{valid, {Key: PresetNone, Label: "無振り(別定義)", SP: Stats{HP: 4}}}
		}, ErrDuplicatePreset},
		{"SPが1ステータス上限超過", func(in *BulkInput) {
			in.Presets = []DefenderPreset{{Key: "over", Label: "超過", SP: Stats{Def: MaxSPPerStat + 1}}}
		}, ErrInvalidPreset},
		{"SP合計が上限超過", func(in *BulkInput) {
			in.Presets = []DefenderPreset{{Key: "over", Label: "超過", SP: Stats{HP: 32, Def: 32, SpD: 32}}}
		}, ErrInvalidPreset},
		{"SPが負", func(in *BulkInput) {
			in.Presets = []DefenderPreset{{Key: "neg", Label: "負", SP: Stats{HP: -1}}}
		}, ErrInvalidPreset},
		{"性格補正がHPを指す", func(in *BulkInput) {
			in.Presets = []DefenderPreset{{Key: "hpnature", Label: "HP補正", Nature: Nature{Plus: StatHP, Minus: StatAtk}}}
		}, ErrInvalidPreset},
		{"キーが空", func(in *BulkInput) {
			in.Presets = []DefenderPreset{{Key: "", Label: "名無し"}}
		}, ErrInvalidPreset},
		{"攻撃側の個体が不正", func(in *BulkInput) {
			in.Attacker.SP = Stats{HP: 32, Atk: 32, Def: 32}
		}, nil},
		{"防御側種族のタイプが無い", func(in *BulkInput) {
			in.DefenderSpecies.Types = nil
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := bulkInput(CategoryPhysical, TypeWater)
			tt.mod(&in)
			res, err := calcBulk(in)
			if err == nil {
				t.Fatalf("エラーになるべき入力が成功した: rows=%v", rowKeys(res.Rows))
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("err=%v、errors.Is(%v) を満たすこと", err, tt.want)
			}
			// 注意: この検査は現状ほぼ vacuous。上の各ケースはいずれも選択段階・事前検証・最初の行で
			// 失敗し、「途中の行まで計算してから失敗」する経路は通らない。後続行だけが失敗する入力は、
			// CalcBulk が SP・性格を計算前に全プリセットで事前検証し、防御側種族・攻撃側の不正は
			// 全行に共通(=最初の行で失敗)なので到達不能。将来 CalcDamage が行ごとに失敗しうるように
			// なったら、後続行だけ失敗するケースを足すこと。
			if len(res.Rows) != 0 {
				t.Errorf("エラー時は部分的な行を返さないこと: %v", rowKeys(res.Rows))
			}
		})
	}
}

// --- PresetKeys × Presets の同時指定(ADR-0009 §3)--------------------------

// PresetKeys と Presets が両方あるときは、カタログではなく in.Presets から選ぶ。
func TestCalcBulkPresetKeysSelectFromGivenPresets(t *testing.T) {
	customA := DefenderPreset{Key: "custom_a", Label: "自作A", SP: Stats{HP: 32, Def: 32, SpD: 2}, Nature: Nature{Plus: StatDef, Minus: StatSpe}}
	customB := DefenderPreset{Key: "custom_b", Label: "自作B", SP: Stats{Def: 20}, Nature: NatureNeutral}
	customC := DefenderPreset{Key: "custom_c", Label: "自作C(選ばれない)", SP: Stats{SpD: 10}, Nature: NatureNeutral}
	// カタログと同じキーで定義が違うもの。カタログではなくこちらが使われること。
	overrideHP := DefenderPreset{Key: PresetHP, Label: "H振り(上書き)", SP: Stats{Def: 20, SpD: 20}, Nature: Nature{Plus: StatSpD, Minus: StatAtk}}

	t.Run("既定カタログに無いキーを in.Presets から選び、行順は PresetKeys の順", func(t *testing.T) {
		in := bulkInput(CategoryPhysical, TypeWater)
		in.Presets = []DefenderPreset{customA, customB, customC, overrideHP}
		in.PresetKeys = []PresetKey{PresetHP, "custom_b", "custom_a"}
		res, err := calcBulk(in)
		if err != nil {
			t.Fatalf("CalcBulk: %v", err)
		}
		if !reflect.DeepEqual(rowKeys(res.Rows), in.PresetKeys) {
			t.Fatalf("keys=%v want %v(PresetKeys の順。custom_c は選ばれない)", rowKeys(res.Rows), in.PresetKeys)
		}
		wantPresets := []DefenderPreset{overrideHP, customB, customA}
		for i, p := range wantPresets {
			row := res.Rows[i]
			if row.PresetLabel != p.Label {
				t.Errorf("rows[%d].PresetLabel=%q want %q(カタログ定義ではなく in.Presets の定義)", i, row.PresetLabel, p.Label)
			}
			def := wantDefender(p, in.DefenderSpecies, nil)
			if !reflect.DeepEqual(row.Defender, def) {
				t.Errorf("rows[%d].Defender=%+v want %+v", i, row.Defender, def)
			}
			if want := wantResult(t, in, def); !reflect.DeepEqual(row.Result, want) {
				t.Errorf("rows[%d].Result が CalcDamage と不一致", i)
			}
		}
	})

	t.Run("in.Presets に無いキーはカタログにあっても未知", func(t *testing.T) {
		in := bulkInput(CategoryPhysical, TypeWater)
		in.Presets = []DefenderPreset{customA}
		in.PresetKeys = []PresetKey{"custom_a", PresetNone} // none はカタログにだけある
		res, err := calcBulk(in)
		if !errors.Is(err, ErrUnknownPreset) {
			t.Fatalf("err=%v、ErrUnknownPreset を期待(カタログへフォールバックしない)", err)
		}
		if len(res.Rows) != 0 {
			t.Errorf("エラー時は行を返さないこと: %v", rowKeys(res.Rows))
		}
	})

	t.Run("検索元 Presets 内のキー重複は PresetKeys 指定でも ErrDuplicatePreset", func(t *testing.T) {
		in := bulkInput(CategoryPhysical, TypeWater)
		dup := customA
		dup.Label = "自作A(重複定義)"
		in.Presets = []DefenderPreset{customA, customB, dup}
		in.PresetKeys = []PresetKey{"custom_b"} // 重複していないキーだけを選んでも検索元の重複はエラー
		res, err := calcBulk(in)
		if !errors.Is(err, ErrDuplicatePreset) {
			t.Fatalf("err=%v、ErrDuplicatePreset を期待", err)
		}
		if len(res.Rows) != 0 {
			t.Errorf("エラー時は行を返さないこと: %v", rowKeys(res.Rows))
		}
	})
}

// --- 耐久の厳密な単調性(ADR-0009 §1 の「耐久が上がる順」)---------------------

// 既存の緩い単調性検査(>=)に加え、既定セット内の大小が厳密に成り立つこと:
//
//	none == hp > *_boost > * > *_full
//
// 被ダメージ = 最大ダメージなので、防御実数値が上がるほど最大ダメージは厳密に減る(HP はダメージに効かない)。
// none == hp を等号で固定するのは、H振りが防御実数値を動かさないことの確認。
// プリセットの性格・SP の取り違え(例: hb に上昇性格を残す = hb_full と同値になる)は、
// この厳密不等号でのみ検出できる。ADR-0009 §2「耐久の単調性」が根拠。
func TestCalcBulkStrictDurabilityOrder(t *testing.T) {
	tests := []struct {
		name     string
		category MoveCategory
		moveType Type
		boost    PresetKey
		plain    PresetKey
		full     PresetKey
	}{
		{"物理は hp > hb_boost > hb > hb_full", CategoryPhysical, TypeWater, PresetHBBoost, PresetHB, PresetHBFull},
		{"物理・不一致技でも同じ", CategoryPhysical, TypeNormal, PresetHBBoost, PresetHB, PresetHBFull},
		{"特殊は hp > hd_boost > hd > hd_full", CategorySpecial, TypeWater, PresetHDBoost, PresetHD, PresetHDFull},
		{"特殊・抜群でも同じ", CategorySpecial, TypeDark, PresetHDBoost, PresetHD, PresetHDFull},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := calcBulk(bulkInput(tt.category, tt.moveType))
			if err != nil {
				t.Fatalf("CalcBulk: %v", err)
			}
			max := map[PresetKey]int{}
			for _, row := range res.Rows {
				max[row.Preset] = row.Result.MaxDamage()
			}
			for _, k := range []PresetKey{PresetNone, PresetHP, tt.boost, tt.plain, tt.full} {
				if _, ok := max[k]; !ok {
					t.Fatalf("行 %q が無い: %v", k, rowKeys(res.Rows))
				}
			}
			// H振りは HP しか変えず、ダメージ(=防御実数値で決まる)は無振りと同じ。
			if max[PresetNone] != max[PresetHP] {
				t.Errorf("none(%d) == hp(%d) であること(HP はダメージに影響しない)", max[PresetNone], max[PresetHP])
			}
			steps := []struct{ hi, lo PresetKey }{
				{PresetHP, tt.boost},
				{tt.boost, tt.plain},
				{tt.plain, tt.full},
			}
			for _, s := range steps {
				if !(max[s.hi] > max[s.lo]) {
					t.Errorf("%s(%d) > %s(%d) であること", s.hi, max[s.hi], s.lo, max[s.lo])
				}
			}
		})
	}
}

// ADR-0009 §2「耐久の単調性」: 上の厳密順序が特定の種族値に依存しないこと。
//
// 防御実数値は floor((種族値 + 20 + SP) × 性格補正)、上昇補正は ×11/10(engine/stats.go)。
// D0 = 種族値 + 20 と置くと boost = floor(11·D0/10)、plain = D0 + 32、full = floor(11·(D0+32)/10)。
// boost < plain は floor(D0/10) < 32 ⟺ D0 < 320 ⟺ 種族値 <= 299 のときに成り立つ。
// 種族値は仕様上 255 が上限(実在最大は B/D = 230 のツボツボ)なので、全種族で常に真。
// ここでは 1..255 を総当たりして「ありうる種族値すべてで厳密順序が成り立つ」ことを固定する。
// 成立条件を見失わないよう、閾値 299/300 の挙動も併せて検査する
// (300 以上で boost と plain が逆転するのは算術上の事実であり、実在しない入力なので
//
//	カタログ順の反例にはならない。ここでは「なぜ 255 上限なら安全か」の根拠として記録する)。
func TestDefenderPresetOrderHoldsForAllBaseStats(t *testing.T) {
	defStat := func(base, sp int, n Nature, k StatKey) int {
		in := Individual{
			Species: Species{
				Key:       "sweep",
				Types:     []Type{TypeNormal},
				BaseStats: Stats{HP: 100, Atk: 100, Def: base, SpA: 100, SpD: base, Spe: 100},
			},
			Level:  DefaultLevel,
			Nature: n,
			SP:     Stats{HP: 32, Def: 0, SpD: 0},
			Status: StatusNone,
		}
		switch k {
		case StatDef:
			in.SP.Def = sp
		case StatSpD:
			in.SP.SpD = sp
		}
		return RealStats(in).Get(k)
	}

	catalog := map[PresetKey]DefenderPreset{}
	for _, p := range DefenderPresetCatalog() {
		catalog[p.Key] = p
	}
	groups := []struct {
		name                   string
		stat                   StatKey
		boost, plain, full, hp PresetKey
		maxBase, breakEvenBase int
	}{
		{"B系", StatDef, PresetHBBoost, PresetHB, PresetHBFull, PresetHP, 255, 299},
		{"D系", StatSpD, PresetHDBoost, PresetHD, PresetHDFull, PresetHP, 255, 299},
	}
	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			for _, k := range []PresetKey{g.hp, g.boost, g.plain, g.full} {
				if _, ok := catalog[k]; !ok {
					t.Fatalf("カタログに %q が無い", k)
				}
			}
			stat := func(base int, key PresetKey) int {
				p := catalog[key]
				return defStat(base, p.SP.Get(g.stat), p.Nature, g.stat)
			}
			for base := 1; base <= g.maxBase; base++ {
				hp, boost, plain, full := stat(base, g.hp), stat(base, g.boost), stat(base, g.plain), stat(base, g.full)
				if !(hp < boost && boost < plain && plain < full) {
					t.Fatalf("種族値 %d で防御実数値の厳密順序が崩れた: hp=%d boost=%d plain=%d full=%d", base, hp, boost, plain, full)
				}
			}
			// 閾値の根拠。299 までは成り立ち、300 で boost >= plain に反転する。
			if b, p := stat(g.breakEvenBase, g.boost), stat(g.breakEvenBase, g.plain); !(b < p) {
				t.Errorf("種族値 %d では boost(%d) < plain(%d) のはず", g.breakEvenBase, b, p)
			}
			if b, p := stat(g.breakEvenBase+1, g.boost), stat(g.breakEvenBase+1, g.plain); b < p {
				t.Errorf("種族値 %d で boost(%d) < plain(%d) のまま。反転の閾値が動いた(ADR-0009 §2 の算術を見直すこと)", g.breakEvenBase+1, b, p)
			}
		})
	}
}

// --- 攻撃側・場・急所の素通し(ADR-0009「CalcBulk は CalcDamage の合成」の担保)---
//
// 他のテストの入力は Field ゼロ値・Critical=false・攻撃側オプション無しのため、
// bulk.go がそれらを落としても通ってしまう。ここでは非ゼロ値を載せ、
//  1. 各行が同入力の CalcDamage と完全一致すること(素通し)
//  2. その値が結果に効いていること(両側で同じ値を落とす退行の検出)
// を検証する。

// checkedBulk は CalcBulk を実行し、既定セット × ItemVariants の全行が
// 手組みの防御側に対する CalcDamage と完全一致することを検証して結果を返す。
func checkedBulk(t *testing.T, in BulkInput) BulkResult {
	t.Helper()
	if len(in.Presets) != 0 || len(in.PresetKeys) != 0 {
		t.Fatal("checkedBulk は既定セット専用")
	}
	presets := DefaultDefenderPresets(in.Move.Category)
	items := in.ItemVariants
	if len(items) == 0 {
		items = []*Item{nil}
	}
	res, err := calcBulk(in)
	if err != nil {
		t.Fatalf("CalcBulk: %v", err)
	}
	if len(res.Rows) != len(presets)*len(items) {
		t.Fatalf("行数=%d want %d", len(res.Rows), len(presets)*len(items))
	}
	i := 0
	for _, p := range presets {
		for _, item := range items {
			row := res.Rows[i]
			if row.Preset != p.Key || row.Item != item {
				t.Errorf("rows[%d]=%q/%p want %q/%p", i, row.Preset, row.Item, p.Key, item)
			}
			def := wantDefender(p, in.DefenderSpecies, item)
			if !reflect.DeepEqual(row.Defender, def) {
				t.Errorf("rows[%d].Defender=%+v want %+v", i, row.Defender, def)
			}
			if want := wantResult(t, in, def); !reflect.DeepEqual(row.Result, want) {
				t.Errorf("rows[%d](%s).Result=%+v want %+v(CalcDamage と不一致)", i, p.Key, row.Result, want)
			}
			i++
		}
	}
	return res
}

// maxDamageCompare は全行について a と b の最大ダメージが dir の関係にあることを検証する。
// dir: +1 = a が b より大きい / -1 = 小さい / 0 = 等しい。
func maxDamageCompare(t *testing.T, label string, a, b BulkResult, dir int) {
	t.Helper()
	if len(a.Rows) != len(b.Rows) {
		t.Fatalf("%s: 行数が違う %d vs %d", label, len(a.Rows), len(b.Rows))
	}
	for i := range a.Rows {
		x, y := a.Rows[i].Result.MaxDamage(), b.Rows[i].Result.MaxDamage()
		ok := (dir > 0 && x > y) || (dir < 0 && x < y) || (dir == 0 && x == y)
		if !ok {
			t.Errorf("%s: rows[%d](%s) 最大ダメージ %d vs %d、期待 dir=%+d", label, i, a.Rows[i].Preset, x, y, dir)
		}
	}
}

func testChoiceBand() *Item {
	return &Item{ID: "choiceband", NameJa: "テストもちもの3", Effect: &ItemEffect{StatMods: map[StatKey]int{StatAtk: 6144}}}
}

func testAdaptability() Ability {
	return Ability{ID: "adaptability", NameJa: "テストとくせい", Effect: &AbilityEffect{StabMod: 8192}}
}

// 攻撃側オプション・場・急所を1つずつ載せる。各行は CalcDamage と一致し、かつ
// 何も載せない場合に対して dir の方向へ最大ダメージが動く(=値が結果に効いている)。
func TestCalcBulkPassesThroughFieldCriticalAndAttackerOptions(t *testing.T) {
	tests := []struct {
		name     string
		category MoveCategory
		moveType Type
		apply    func(*BulkInput)
		dir      int
	}{
		// 天候(水技: 雨で 1.5 倍・晴れで 0.5 倍)
		{"雨×水技(物理)", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Field.Weather = WeatherRain }, +1},
		{"雨×水技(特殊)", CategorySpecial, TypeWater, func(in *BulkInput) { in.Field.Weather = WeatherRain }, +1},
		{"晴れ×水技", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Field.Weather = WeatherSun }, -1},
		// フィールド(威力補正: エレキで電気 1.3 倍・ミストでドラゴン 0.5 倍)
		{"エレキフィールド×電気技", CategoryPhysical, TypeElectric, func(in *BulkInput) { in.Field.Terrain = TerrainElectric }, +1},
		{"ミストフィールド×ドラゴン技", CategoryPhysical, TypeDragon, func(in *BulkInput) { in.Field.Terrain = TerrainMisty }, -1},
		// 防御側の壁(分類が合うときだけ半減)
		{"リフレクター×物理", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Field.DefenderScreens.Reflect = true }, -1},
		{"ひかりのかべ×特殊", CategorySpecial, TypeWater, func(in *BulkInput) { in.Field.DefenderScreens.LightScreen = true }, -1},
		{"オーロラベール×物理", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Field.DefenderScreens.AuroraVeil = true }, -1},
		{"リフレクター×特殊は無関係", CategorySpecial, TypeWater, func(in *BulkInput) { in.Field.DefenderScreens.Reflect = true }, 0},
		// 攻撃側の壁は防御に無関係
		{"攻撃側のリフレクターは無関係", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Field.AttackerScreens.Reflect = true }, 0},
		// 急所(1.5 倍)
		{"急所(物理)", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Critical = true }, +1},
		{"急所(特殊)", CategorySpecial, TypeWater, func(in *BulkInput) { in.Critical = true }, +1},
		// 攻撃側ランク
		{"攻撃ランク+2", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Attacker.Ranks.Atk = 2 }, +1},
		{"攻撃ランク+6(境界)", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Attacker.Ranks.Atk = 6 }, +1},
		{"攻撃ランク-6(境界)", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Attacker.Ranks.Atk = -6 }, -1},
		{"特攻ランク+2×特殊", CategorySpecial, TypeWater, func(in *BulkInput) { in.Attacker.Ranks.SpA = 2 }, +1},
		{"特攻ランク+2×物理は無関係", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Attacker.Ranks.SpA = 2 }, 0},
		// 攻撃側の状態異常(やけどは物理のみ半減)
		{"やけど×物理", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Attacker.Status = StatusBurn }, -1},
		{"やけど×特殊は無関係", CategorySpecial, TypeWater, func(in *BulkInput) { in.Attacker.Status = StatusBurn }, 0},
		// 攻撃側の持ち物・特性
		{"攻撃側の攻撃1.5倍持ち物×物理", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Attacker.Item = testChoiceBand() }, +1},
		{"攻撃側のタイプ一致強化特性×タイプ一致技", CategoryPhysical, TypeWater, func(in *BulkInput) { in.Attacker.Ability = testAdaptability() }, +1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := checkedBulk(t, bulkInput(tt.category, tt.moveType))
			in := bulkInput(tt.category, tt.moveType)
			tt.apply(&in)
			got := checkedBulk(t, in)
			maxDamageCompare(t, tt.name, got, base, tt.dir)
		})
	}
}

// 攻撃側ランクは -6 < 0 < +2 < +6 の順に最大ダメージが厳密に増える(全行)。
func TestCalcBulkAttackerRankBoundaryOrder(t *testing.T) {
	results := map[int]BulkResult{}
	for _, rank := range []int{-6, 0, 2, 6} {
		in := bulkInput(CategoryPhysical, TypeWater)
		in.Attacker.Ranks.Atk = rank
		results[rank] = checkedBulk(t, in)
	}
	maxDamageCompare(t, "0 > -6", results[0], results[-6], +1)
	maxDamageCompare(t, "+2 > 0", results[2], results[0], +1)
	maxDamageCompare(t, "+6 > +2", results[6], results[2], +1)
}

// 急所は壁を貫通し、攻撃側の不利なランクを無視する(CalcDamage の規則が Bulk 経由でも保たれる)。
func TestCalcBulkCriticalInteractions(t *testing.T) {
	crit := func(mod func(*BulkInput)) BulkResult {
		in := bulkInput(CategoryPhysical, TypeWater)
		in.Critical = true
		if mod != nil {
			mod(&in)
		}
		return checkedBulk(t, in)
	}
	plain := crit(nil)

	withReflect := crit(func(in *BulkInput) { in.Field.DefenderScreens.Reflect = true })
	maxDamageCompare(t, "急所はリフレクターを貫通", withReflect, plain, 0)

	minus6 := crit(func(in *BulkInput) { in.Attacker.Ranks.Atk = -6 })
	maxDamageCompare(t, "急所は攻撃側の負のランクを無視", minus6, plain, 0)

	// 非急所なら壁・負のランクは効く(急所との差が Critical の素通しに由来することの裏付け)。
	noCrit := checkedBulk(t, bulkInput(CategoryPhysical, TypeWater))
	maxDamageCompare(t, "急所 > 非急所", plain, noCrit, +1)
}

// 全部載せ。値がすべて CalcDamage へ素通しされ、かつ各値が結果に効いていること。
// 1つ剥がすごとに(全行の)結果が変わることで、「両側で同じ値を落とす」退行も検出する。
func TestCalcBulkPassesThroughAllOptionsCombined(t *testing.T) {
	sink := func(critical bool) BulkInput {
		in := bulkInput(CategoryPhysical, TypeWater) // 水攻撃側 × 水技 = タイプ一致
		in.Field = Field{
			Weather:         WeatherRain,
			Terrain:         TerrainElectric,
			AttackerScreens: Screens{Reflect: true},
			DefenderScreens: Screens{Reflect: true},
		}
		in.Critical = critical
		in.Attacker.Ranks = Ranks{Atk: 2, Def: -1}
		in.Attacker.Status = StatusBurn
		in.Attacker.Item = testChoiceBand()
		in.Attacker.Ability = testAdaptability()
		in.Attacker.TeraType = TypeWater
		in.ItemVariants = []*Item{nil, {ID: "assaultvest", NameJa: "テストもちもの2", Effect: &ItemEffect{StatMods: map[StatKey]int{StatSpD: 6144}}}}
		return in
	}
	strips := []struct {
		name string
		mod  func(*BulkInput)
		// 急所時は壁が無視されるので、壁を剥がしても変わらない
		observableUnderCrit bool
	}{
		{"天候", func(in *BulkInput) { in.Field.Weather = WeatherNone }, true},
		{"防御側の壁", func(in *BulkInput) { in.Field.DefenderScreens = Screens{} }, false},
		{"攻撃ランク", func(in *BulkInput) { in.Attacker.Ranks.Atk = 0 }, true},
		{"やけど", func(in *BulkInput) { in.Attacker.Status = StatusNone }, true},
		{"攻撃側の持ち物", func(in *BulkInput) { in.Attacker.Item = nil }, true},
		{"攻撃側の特性", func(in *BulkInput) { in.Attacker.Ability = Ability{} }, true},
	}
	for _, critical := range []bool{false, true} {
		name := "非急所"
		if critical {
			name = "急所"
		}
		t.Run(name, func(t *testing.T) {
			in := sink(critical)
			full := checkedBulk(t, in)
			for _, s := range strips {
				if critical && !s.observableUnderCrit {
					continue
				}
				stripped := sink(critical)
				s.mod(&stripped)
				got := checkedBulk(t, stripped)
				for i := range full.Rows {
					if reflect.DeepEqual(full.Rows[i].Result.Rolls, got.Rows[i].Result.Rolls) {
						t.Errorf("%s を剥がしても rows[%d](%s) のダメージが変わらない: %v(値が結果に効いていない)", s.name, i, full.Rows[i].Preset, full.Rows[i].Result.Rolls)
					}
				}
			}
			// 急所フラグ自体。
			flipped := sink(!critical)
			got := checkedBulk(t, flipped)
			for i := range full.Rows {
				if reflect.DeepEqual(full.Rows[i].Result.Rolls, got.Rows[i].Result.Rolls) {
					t.Errorf("急所フラグを反転しても rows[%d](%s) のダメージが変わらない", i, full.Rows[i].Preset)
				}
			}
			// 場全体をゼロ値にしても変わる(Field 丸ごと落とす退行)。
			noField := sink(critical)
			noField.Field = Field{}
			got = checkedBulk(t, noField)
			for i := range full.Rows {
				if reflect.DeepEqual(full.Rows[i].Result.Rolls, got.Rows[i].Result.Rolls) {
					t.Errorf("Field をゼロ値にしても rows[%d](%s) が変わらない", i, full.Rows[i].Preset)
				}
			}
		})
	}
	// TeraType は CalcDamage が参照しない(未実装)ため、結果には現れない。
	// フィクスチャには載せてあり、CalcDamage が使うようになれば checkedBulk が素通しを検証する。
	withTera := checkedBulk(t, sink(false))
	in := sink(false)
	in.Attacker.TeraType = TypeNone
	withoutTera := checkedBulk(t, in)
	maxDamageCompare(t, "TeraType は現状ダメージに影響しない", withTera, withoutTera, 0)
}
