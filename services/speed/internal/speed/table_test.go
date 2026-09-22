package speed

import (
	"errors"
	"reflect"
	"testing"
)

// 期待値は ADR-0601 §2 の表と ADR-0600 §3 の式から手で導いた値(実装の写しではない)。
//
//	実数値 = 種族値 + 20 + SP(補正なし)、上昇は floor(× 11/10)
//	ランク +n: floor(v × (2+n) / 2)
//	スカーフ(ランクの後): floor((v × 6144 + 2047) / 4096)

func TestPresetsDefinition(t *testing.T) {
	t.Parallel()

	// ADR-0601 §2 の表をそのまま書き写した期待値(順序も表のとおり)。SP 32 は engine.MaxSPPerStat。
	want := []Preset{
		{ID: "uninvested", SP: 0, Nature: NatureNeutral, Rank: 0, Scarf: false},
		{ID: "neutral-max", SP: 32, Nature: NatureNeutral, Rank: 0, Scarf: false},
		{ID: "max", SP: 32, Nature: NaturePlus, Rank: 0, Scarf: false},
		{ID: "max-scarf", SP: 32, Nature: NaturePlus, Rank: 0, Scarf: true},
		{ID: "max-plus1", SP: 32, Nature: NaturePlus, Rank: 1, Scarf: false},
		{ID: "max-plus2", SP: 32, Nature: NaturePlus, Rank: 2, Scarf: false},
	}
	if got := Presets(); !reflect.DeepEqual(got, want) {
		t.Errorf("Presets() = %+v, want %+v", got, want)
	}

	// 名前付き定数が表の ID と一致する(API の enum と同じ文字列)。
	constants := []PresetID{PresetUninvested, PresetNeutralMax, PresetMax, PresetMaxScarf, PresetMaxPlus1, PresetMaxPlus2}
	for i, id := range constants {
		if id != want[i].ID {
			t.Errorf("constant #%d = %q, want %q", i, id, want[i].ID)
		}
	}
}

func TestPresetsReturnsCopy(t *testing.T) {
	t.Parallel()

	// 呼び出し側が変更しても、次の呼び出しの定義は変わらない(可変のグローバルを公開しない)。
	first := Presets()
	first[0].SP = 99
	first[0].ID = "changed"
	if second := Presets(); second[0].ID != PresetUninvested || second[0].SP != 0 {
		t.Errorf("Presets()[0] after mutation = %+v, want the uninvested definition", second[0])
	}
}

func TestNormalizePresets(t *testing.T) {
	t.Parallel()

	all := []PresetID{"uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"}
	tests := []struct {
		name string
		in   []PresetID
		want []PresetID
	}{
		{"表の順のまま", all, all},
		{"逆順は表の順に並べ直す", []PresetID{"max-plus2", "max-plus1", "max-scarf", "max", "neutral-max", "uninvested"}, all},
		{"1 つだけ", []PresetID{"max-scarf"}, []PresetID{"max-scarf"}},
		{"部分集合の順不同", []PresetID{"max-plus1", "uninvested", "max"}, []PresetID{"uninvested", "max", "max-plus1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizePresets(tt.in)
			if err != nil {
				t.Fatalf("NormalizePresets(%v) error = %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizePresets(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizePresetsRejectsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []PresetID
		want error
	}{
		{"nil", nil, ErrNoPresets},
		{"空の slice", []PresetID{}, ErrNoPresets},
		{"未知の ID", []PresetID{"max-plus3"}, ErrUnknownPreset},
		{"空文字の ID(presets= に相当)", []PresetID{""}, ErrUnknownPreset},
		{"既知の中に未知", []PresetID{"max", "fastest"}, ErrUnknownPreset},
		{"大文字は未知", []PresetID{"MAX"}, ErrUnknownPreset},
		{"重複", []PresetID{"max", "max"}, ErrDuplicatePreset},
		{"離れた重複", []PresetID{"max-scarf", "uninvested", "max-scarf"}, ErrDuplicatePreset},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NormalizePresets(tt.in); !errors.Is(err, tt.want) {
				t.Errorf("NormalizePresets(%v) error = %v, want %v", tt.in, err, tt.want)
			}
		})
	}
}

// tableRoster は順不同の架空の roster。9001 は種族値 68 で、準速(120)が 9002 の無振り(120)と同速になる
// (別のポケモン・別のプリセットの同速。段の中で pokemonId の順がプリセットの順より優先されることも確かめる)。
func tableRoster() Roster {
	return Roster{RegulationID: "example", Pokemon: []Pokemon{
		{PokemonID: "9003-000", NameJa: "テストサンバンメ", Types: []string{"water"}, BaseSpeed: 81},
		{PokemonID: "9001-000", NameJa: "テストイチバンメ", Types: []string{"rock", "ground"}, BaseSpeed: 68},
		{PokemonID: "9002-000", NameJa: "テストニバンメ", Types: []string{"fire", "flying"}, BaseSpeed: 100},
	}}
}

func entry(p Pokemon, preset PresetID) TableEntry {
	return TableEntry{Pokemon: p, Preset: preset}
}

// TestBuildTableAllPresets は 3 体 × 6 行 = 18 行の全段を手計算した完全一致のテスト。
//
//	9001 種族値 68 : 無振り 88 / 準速 120 / 最速 floor(120×1.1)=132 / スカーフ 132×1.5=198
//	                 / +1 floor(132×3/2)=198 / +2 132×2=264
//	9002 種族値 100: 無振り 120 / 準速 152 / 最速 floor(152×1.1)=167 / スカーフ floor(250.5 の五捨五超入)=250
//	                 / +1 floor(167×3/2)=250 / +2 334
//	9003 種族値 81 : 無振り 101 / 準速 133 / 最速 floor(133×1.1)=146 / スカーフ 146×1.5=219
//	                 / +1 floor(146×3/2)=219 / +2 292
func TestBuildTableAllPresets(t *testing.T) {
	t.Parallel()

	roster := tableRoster()
	p9001, p9002, p9003 := roster.Pokemon[1], roster.Pokemon[2], roster.Pokemon[0]
	all := []PresetID{"uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"}

	got, err := BuildTable(roster, all)
	if err != nil {
		t.Fatalf("BuildTable error = %v", err)
	}
	want := Table{RegulationID: "example", Presets: all, Tiers: []Tier{
		{Speed: 334, Entries: []TableEntry{entry(p9002, "max-plus2")}},
		{Speed: 292, Entries: []TableEntry{entry(p9003, "max-plus2")}},
		{Speed: 264, Entries: []TableEntry{entry(p9001, "max-plus2")}},
		// 同じポケモンの同速はプリセットの表の順(スカーフ → +1)。
		{Speed: 250, Entries: []TableEntry{entry(p9002, "max-scarf"), entry(p9002, "max-plus1")}},
		{Speed: 219, Entries: []TableEntry{entry(p9003, "max-scarf"), entry(p9003, "max-plus1")}},
		{Speed: 198, Entries: []TableEntry{entry(p9001, "max-scarf"), entry(p9001, "max-plus1")}},
		{Speed: 167, Entries: []TableEntry{entry(p9002, "max")}},
		{Speed: 152, Entries: []TableEntry{entry(p9002, "neutral-max")}},
		{Speed: 146, Entries: []TableEntry{entry(p9003, "max")}},
		{Speed: 133, Entries: []TableEntry{entry(p9003, "neutral-max")}},
		{Speed: 132, Entries: []TableEntry{entry(p9001, "max")}},
		// 別のポケモンの同速は pokemonId の昇順(9001 の準速がプリセットの順では後でも先)。
		{Speed: 120, Entries: []TableEntry{entry(p9001, "neutral-max"), entry(p9002, "uninvested")}},
		{Speed: 101, Entries: []TableEntry{entry(p9003, "uninvested")}},
		{Speed: 88, Entries: []TableEntry{entry(p9001, "uninvested")}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildTable =\n%+v\nwant\n%+v", got, want)
	}
}

// TestBuildTableSameBaseSpeed: 種族値が同じ別のポケモンは全行が同じ段に入り、段の中は pokemonId の昇順
// (testdata の 9002-000 と 9005-000 と同じ種族値 81)。
//
//	種族値 81: 無振り 101 / 準速 133 / 最速 146 / スカーフ 219 / +1 219 / +2 292(TestBuildTableAllPresets と同じ)
func TestBuildTableSameBaseSpeed(t *testing.T) {
	t.Parallel()

	later := Pokemon{PokemonID: "9005-000", NameJa: "テストゴバンメ", Types: []string{"grass", "poison"}, BaseSpeed: 81}
	earlier := Pokemon{PokemonID: "9002-000", NameJa: "テストニバンメ", Types: []string{"water"}, BaseSpeed: 81}
	roster := Roster{RegulationID: "example", Pokemon: []Pokemon{later, earlier}}

	got, err := BuildTable(roster, []PresetID{"max", "uninvested", "max-scarf", "max-plus1"})
	if err != nil {
		t.Fatalf("BuildTable error = %v", err)
	}
	want := Table{RegulationID: "example", Presets: []PresetID{"uninvested", "max", "max-scarf", "max-plus1"}, Tiers: []Tier{
		// 219: pokemonId の昇順が先、同じポケモンの中はプリセットの順。
		{Speed: 219, Entries: []TableEntry{
			entry(earlier, "max-scarf"), entry(earlier, "max-plus1"),
			entry(later, "max-scarf"), entry(later, "max-plus1"),
		}},
		{Speed: 146, Entries: []TableEntry{entry(earlier, "max"), entry(later, "max")}},
		{Speed: 101, Entries: []TableEntry{entry(earlier, "uninvested"), entry(later, "uninvested")}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("BuildTable =\n%+v\nwant\n%+v", got, want)
	}
}

// TestBuildTableSubset: 指定しなかったプリセットの行は出ない。値は TestBuildTableAllPresets の手計算。
func TestBuildTableSubset(t *testing.T) {
	t.Parallel()

	roster := tableRoster()
	p9001, p9002, p9003 := roster.Pokemon[1], roster.Pokemon[2], roster.Pokemon[0]

	tests := []struct {
		name    string
		presets []PresetID
		want    Table
	}{
		{
			name:    "スカーフだけ",
			presets: []PresetID{"max-scarf"},
			want: Table{RegulationID: "example", Presets: []PresetID{"max-scarf"}, Tiers: []Tier{
				{Speed: 250, Entries: []TableEntry{entry(p9002, "max-scarf")}},
				{Speed: 219, Entries: []TableEntry{entry(p9003, "max-scarf")}},
				{Speed: 198, Entries: []TableEntry{entry(p9001, "max-scarf")}},
			}},
		},
		{
			// 準速と無振りだけでも、9001 の準速と 9002 の無振りは同じ段(120)。
			name:    "準速と無振り(逆順の指定)",
			presets: []PresetID{"neutral-max", "uninvested"},
			want: Table{RegulationID: "example", Presets: []PresetID{"uninvested", "neutral-max"}, Tiers: []Tier{
				{Speed: 152, Entries: []TableEntry{entry(p9002, "neutral-max")}},
				{Speed: 133, Entries: []TableEntry{entry(p9003, "neutral-max")}},
				{Speed: 120, Entries: []TableEntry{entry(p9001, "neutral-max"), entry(p9002, "uninvested")}},
				{Speed: 101, Entries: []TableEntry{entry(p9003, "uninvested")}},
				{Speed: 88, Entries: []TableEntry{entry(p9001, "uninvested")}},
			}},
		},
		{
			name:    "ランクなし(ADR-0601 §4 の例)",
			presets: []PresetID{"uninvested", "neutral-max", "max", "max-scarf"},
			want: Table{RegulationID: "example", Presets: []PresetID{"uninvested", "neutral-max", "max", "max-scarf"}, Tiers: []Tier{
				{Speed: 250, Entries: []TableEntry{entry(p9002, "max-scarf")}},
				{Speed: 219, Entries: []TableEntry{entry(p9003, "max-scarf")}},
				{Speed: 198, Entries: []TableEntry{entry(p9001, "max-scarf")}},
				{Speed: 167, Entries: []TableEntry{entry(p9002, "max")}},
				{Speed: 152, Entries: []TableEntry{entry(p9002, "neutral-max")}},
				{Speed: 146, Entries: []TableEntry{entry(p9003, "max")}},
				{Speed: 133, Entries: []TableEntry{entry(p9003, "neutral-max")}},
				{Speed: 132, Entries: []TableEntry{entry(p9001, "max")}},
				{Speed: 120, Entries: []TableEntry{entry(p9001, "neutral-max"), entry(p9002, "uninvested")}},
				{Speed: 101, Entries: []TableEntry{entry(p9003, "uninvested")}},
				{Speed: 88, Entries: []TableEntry{entry(p9001, "uninvested")}},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := BuildTable(tableRoster(), tt.presets)
			if err != nil {
				t.Fatalf("BuildTable error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildTable =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

// TestBuildTableIgnoresPresetOrder: presets の指定の順は結果に影響しない(ADR-0601 §4)。
func TestBuildTableIgnoresPresetOrder(t *testing.T) {
	t.Parallel()

	forward, err := BuildTable(tableRoster(), []PresetID{"uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"})
	if err != nil {
		t.Fatalf("BuildTable(forward) error = %v", err)
	}
	shuffled, err := BuildTable(tableRoster(), []PresetID{"max-plus1", "uninvested", "max-plus2", "max", "max-scarf", "neutral-max"})
	if err != nil {
		t.Fatalf("BuildTable(shuffled) error = %v", err)
	}
	if !reflect.DeepEqual(forward, shuffled) {
		t.Errorf("BuildTable depends on the preset order:\nforward  %+v\nshuffled %+v", forward, shuffled)
	}
}

// TestBuildTableTiersInvariant: 段は素早さの厳密な降順(同じ値の段が 2 つに分かれない)で、各段は 1 件以上。
func TestBuildTableTiersInvariant(t *testing.T) {
	t.Parallel()

	got, err := BuildTable(tableRoster(), []PresetID{"uninvested", "neutral-max", "max", "max-scarf", "max-plus1", "max-plus2"})
	if err != nil {
		t.Fatalf("BuildTable error = %v", err)
	}
	entries := 0
	for i, tier := range got.Tiers {
		if len(tier.Entries) == 0 {
			t.Errorf("tier #%d (speed %d) has no entries", i, tier.Speed)
		}
		if i > 0 && tier.Speed >= got.Tiers[i-1].Speed {
			t.Errorf("tier #%d speed %d is not below the previous %d", i, tier.Speed, got.Tiers[i-1].Speed)
		}
		entries += len(tier.Entries)
	}
	if entries != 18 { // 3 体 × 6 行
		t.Errorf("entries = %d, want 18", entries)
	}
}

func TestBuildTableRejectsInvalidPresets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		presets []PresetID
		want    error
	}{
		{"nil", nil, ErrNoPresets},
		{"空", []PresetID{}, ErrNoPresets},
		{"未知", []PresetID{"max", "max-plus6"}, ErrUnknownPreset},
		{"重複", []PresetID{"uninvested", "uninvested"}, ErrDuplicatePreset},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := BuildTable(tableRoster(), tt.presets); !errors.Is(err, tt.want) {
				t.Errorf("BuildTable(%v) error = %v, want %v", tt.presets, err, tt.want)
			}
		})
	}
}

// TestBuildTableWrapsSpeedError: roster の種族値が不正なら Speed の sentinel エラーを包んで返す
// (read model は検証済みなので通常は起きない。ADR-0601 §5)。
func TestBuildTableWrapsSpeedError(t *testing.T) {
	t.Parallel()

	for _, baseSpeed := range []int{0, 256} {
		roster := tableRoster()
		roster.Pokemon = append(roster.Pokemon, Pokemon{PokemonID: "9009-000", NameJa: "テストフセイ", Types: []string{"normal"}, BaseSpeed: baseSpeed})
		if _, err := BuildTable(roster, []PresetID{"max"}); !errors.Is(err, ErrInvalidBaseSpeed) {
			t.Errorf("BuildTable with base speed %d error = %v, want %v", baseSpeed, err, ErrInvalidBaseSpeed)
		}
	}
}
