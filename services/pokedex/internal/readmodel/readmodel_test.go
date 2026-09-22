package readmodel_test

// pokedex export(balance・speed 向けの read model。ADR-0100 §8・ADR-0105 §5)のテスト。
// 出力が services/balance/schema/ の JSON Schema(ADR-0402)と speed の read model の形(ADR-0600 §4)に合うこと、
// 既定のレギュレーションの使用可能集合に絞ること、特性の効果を ADR-0017 の正規化された形にすることを確かめる。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"example.com/pokecalc/services/pokedex/internal/readmodel"
	"example.com/pokecalc/services/pokedex/internal/store"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

// balance の JSON Schema(タイプバランスレーンの持ち物。読むだけ)。
const balanceSchemaDir = "../../../balance/schema"

func export(t *testing.T, q *storetest.Querier) (readmodel.Files, readmodel.Report) {
	t.Helper()
	files, rep, err := readmodel.Export(context.Background(), q)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	return files, rep
}

func compileSchema(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	sch, err := c.Compile(filepath.Join(balanceSchemaDir, name))
	if err != nil {
		t.Fatalf("schema %s を読めない: %v", name, err)
	}
	return sch
}

func validate(t *testing.T, sch *jsonschema.Schema, doc []byte) error {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		t.Fatalf("JSON として読めない: %v\n%s", err, doc)
	}
	return sch.Validate(inst)
}

// AC-E1: balance 向けの3ファイルが services/balance/schema/ の JSON Schema に合う。
func TestExportSatisfiesBalanceSchemas(t *testing.T) {
	files, _ := export(t, storetest.New())
	tests := []struct {
		schema string
		doc    []byte
	}{
		{"pokemon-types.schema.json", files.PokemonTypes},
		{"moves.schema.json", files.Moves},
		{"abilities.schema.json", files.Abilities},
	}
	for _, tt := range tests {
		t.Run(tt.schema, func(t *testing.T) {
			if len(tt.doc) == 0 {
				t.Fatal("出力が空")
			}
			if err := validate(t, compileSchema(t, tt.schema), tt.doc); err != nil {
				t.Fatalf("%s に合わない: %v\n%s", tt.schema, err, tt.doc)
			}
		})
	}
}

// pokemon-types.schema.json の abilityIds.maxItems と、readmodel の切り詰め件数(maxCatalogAbilityCount。
// 未export のためテストからは直接読めず、切り詰めが起きる4件目入りの fixture で振る舞いを確認する)が
// ずれていないことを確かめる。balance が上限を上げても、こちらが黙って古い上限のまま切り詰め続けないように
// する(critic の軽微指摘)。schema 側の値が変わったらこのテストを直す。
func TestBalanceAbilityIdsMaxItemsMatchesReadmodel(t *testing.T) {
	sch := compileSchema(t, "pokemon-types.schema.json")
	props, ok := sch.Properties["pokemon"]
	if !ok {
		t.Fatal("pokemon-types.schema.json に pokemon が無い")
	}
	items := props.Items2020
	if items == nil {
		t.Fatal("pokemon.items が無い")
	}
	if items.Ref != nil { // "items": {"$ref": "#/$defs/entry"} を解決する
		items = items.Ref
	}
	abilityIds, ok := items.Properties["abilityIds"]
	if !ok {
		t.Fatal("pokemon-types.schema.json の pokemon[].abilityIds が無い")
	}
	if abilityIds.MaxItems == nil {
		t.Fatal("abilityIds.maxItems が無い")
	}
	const wantMaxItems = 4 // readmodel.go の maxCatalogAbilityCount と同じ値を書く(2箇所で手動同期)
	if got := *abilityIds.MaxItems; got != wantMaxItems {
		t.Fatalf("schema の abilityIds.maxItems=%d だが readmodel の maxCatalogAbilityCount は %d のまま。"+
			"readmodel.go の maxCatalogAbilityCount を %d に合わせる", got, wantMaxItems, got)
	}
}

// AC-E1: 検証が空振りしていない(schema が実際に不正を拒否する)ことを、出力を壊して確かめる。
func TestBalanceSchemaCheckIsNotVacuous(t *testing.T) {
	files, _ := export(t, storetest.New())
	broken := bytes.Replace(files.PokemonTypes, []byte(`"schemaVersion":1`), []byte(`"schemaVersion":2`), 1)
	if bytes.Equal(broken, files.PokemonTypes) {
		t.Fatalf("pokemon-types の出力に \"schemaVersion\":1 が無い(空白を入れない正準形で出すこと)\n%s", files.PokemonTypes)
	}
	if err := validate(t, compileSchema(t, "pokemon-types.schema.json"), broken); err == nil {
		t.Fatal("schemaVersion 2 が schema を通った(検証が空振りしている)")
	}
}

// AC-E2: 数値はすべて整数の字面(`2`。`2.0`・`2e0` は不可。balance の loader が拒否する。ADR-0402 §3)。
func TestExportWritesPlainIntegers(t *testing.T) {
	files, _ := export(t, storetest.New())
	plain := regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	for name, doc := range map[string][]byte{
		readmodel.FilePokemonTypes: files.PokemonTypes,
		readmodel.FileMoves:        files.Moves,
		readmodel.FileAbilities:    files.Abilities,
		readmodel.FileSpeedPokemon: files.SpeedPokemon,
	} {
		dec := json.NewDecoder(bytes.NewReader(doc))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		walkNumbers(v, func(n json.Number) {
			if !plain.MatchString(n.String()) {
				t.Errorf("%s に整数の字面でない数値 %q がある", name, n)
			}
		})
	}
}

func walkNumbers(v any, f func(json.Number)) {
	switch x := v.(type) {
	case json.Number:
		f(x)
	case []any:
		for _, e := range x {
			walkNumbers(e, f)
		}
	case map[string]any:
		for _, e := range x {
			walkNumbers(e, f)
		}
	}
}

type pokemonTypesFile struct {
	SchemaVersion int `json:"schemaVersion"`
	Pokemon       []struct {
		PokemonID  string   `json:"pokemonId"`
		NameJa     *string  `json:"nameJa"`
		Types      []string `json:"types"`
		AbilityIDs []string `json:"abilityIds"`
	} `json:"pokemon"`
}

type movesFile struct {
	SchemaVersion int `json:"schemaVersion"`
	Moves         []struct {
		MoveID   string `json:"moveId"`
		Type     string `json:"type"`
		Category string `json:"category"`
	} `json:"moves"`
}

type abilityEffect struct {
	Kind        string `json:"kind"`
	AttackType  string `json:"attackType,omitempty"`
	Numerator   int    `json:"numerator,omitempty"`
	Denominator int    `json:"denominator,omitempty"`
}

type abilitiesFile struct {
	SchemaVersion int `json:"schemaVersion"`
	Abilities     []struct {
		AbilityID string          `json:"abilityId"`
		Effects   []abilityEffect `json:"effects"`
	} `json:"abilities"`
}

// speedFile は speed の read model(ADR-0600 §4。services/speed/internal/master が読む形)。未知のフィールドは拒否する。
type speedFile struct {
	SchemaVersion int    `json:"schemaVersion"`
	RegulationID  string `json:"regulationId"`
	Pokemon       []struct {
		PokemonID string   `json:"pokemonId"`
		NameJa    string   `json:"nameJa"`
		Types     []string `json:"types"`
		BaseSpeed int      `json:"baseSpeed"`
	} `json:"pokemon"`
}

func strict(t *testing.T, doc []byte, v any) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		t.Fatalf("形が違う: %v\n%s", err, doc)
	}
	if dec.More() {
		t.Fatalf("後続のデータがある")
	}
}

// AC-E3: ポケモンの read model は既定のレギュレーションの使用可能集合だけ(pokemonId 昇順)。各ポケモンに nameJa・
// types([type1] か [type1, type2])・abilityIds(slot 昇順。隠れ特性 slot 3 を含む)。
// 4つ目(slot 4 = Showdown の特殊枠)は balance の上限(3件)を超えるので落とし、Report に残す。
func TestExportPokemonTypes(t *testing.T) {
	files, rep := export(t, storetest.New())
	var f pokemonTypesFile
	strict(t, files.PokemonTypes, &f)
	if f.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d", f.SchemaVersion)
	}
	var ids []string
	for _, p := range f.Pokemon {
		ids = append(ids, p.PokemonID)
	}
	if want := []string{"9001-000", "9001-001", "9002-000"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("pokemonId = %v, want %v(既定のレギュレーションの使用可能集合だけ・昇順)", ids, want)
	}
	byID := map[string]int{}
	for i, p := range f.Pokemon {
		byID[p.PokemonID] = i
	}
	mon := f.Pokemon[byID["9001-000"]]
	if mon.NameJa == nil || *mon.NameJa != "テストモン" || !reflect.DeepEqual(mon.Types, []string{"fire"}) ||
		!reflect.DeepEqual(mon.AbilityIDs, []string{"testblaze", "testguard"}) {
		t.Errorf("9001-000 = %+v(abilityIds は slot 順で隠れ特性を含む)", mon)
	}
	mega := f.Pokemon[byID["9001-001"]]
	if !reflect.DeepEqual(mega.Types, []string{"fire", "water"}) || !reflect.DeepEqual(mega.AbilityIDs, []string{"teststance"}) {
		t.Errorf("9001-001 = %+v", mega)
	}
	// 9002-000 は slot 1〜4 の4つを持つ。balance の abilityIds.maxItems が 4 に上がったため
	// (feat/tb-readmodel-wiring。TestBalanceAbilityIdsMaxItemsMatchesReadmodel が追従を検出する)、
	// もう slot 4 は落とさない。TruncatedAbilities の切り詰めそのもの(5件目以降)は
	// storetest の固定データに5件目を作れないため、この fixture では確認できない。
	leaf := f.Pokemon[byID["9002-000"]]
	if !reflect.DeepEqual(leaf.AbilityIDs, []string{"testleafy", "testguard", "testhidden", "testspecial"}) {
		t.Errorf("9002-000 の abilityIds = %v, want slot 1〜4(すべて含む)", leaf.AbilityIDs)
	}
	if len(rep.TruncatedAbilities) != 0 {
		t.Errorf("Report.TruncatedAbilities = %+v, want 空(上限4に収まる)", rep.TruncatedAbilities)
	}
}

// AC-E4: 技の read model は使用可能な技だけ(moveId 昇順)。
func TestExportMoves(t *testing.T) {
	files, _ := export(t, storetest.New())
	var f movesFile
	strict(t, files.Moves, &f)
	var got []string
	for _, m := range f.Moves {
		got = append(got, m.MoveID+":"+m.Type+":"+m.Category)
	}
	want := []string{"testflame:fire:special", "testglare:normal:status", "teststrike:normal:physical"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("moves = %v, want %v", got, want)
	}
}

// AC-E5: 特性の read model(ADR-0017 §2)は使用可能な特性だけ(abilityId 昇順)。ability_effects から防御側のタイプ相性に
// 関わるものだけを正規化する: DefResistType{t: m} → type_multiplier(attackType t、m/4096 を約分。攻撃タイプ ID 昇順)、
// ReduceSuperEffective m → super_effective_multiplier(m/4096 を約分)。攻撃側の効果だけの特性・効果の行が無い特性は effects []。
// ポケモンの read model の abilityIds はすべて特性の read model にある。
func TestExportAbilities(t *testing.T) {
	files, _ := export(t, storetest.New())
	var f abilitiesFile
	strict(t, files.Abilities, &f)
	got := map[string][]abilityEffect{}
	var ids []string
	for _, a := range f.Abilities {
		got[a.AbilityID] = a.Effects
		ids = append(ids, a.AbilityID)
	}
	if want := []string{"testblaze", "testguard", "testhidden", "testleafy", "testspecial", "teststance"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("abilityId = %v, want %v", ids, want)
	}
	want := map[string][]abilityEffect{
		"testblaze": {},
		"testguard": {
			{Kind: "type_multiplier", AttackType: "fire", Numerator: 1, Denominator: 2},
			{Kind: "type_multiplier", AttackType: "water", Numerator: 1, Denominator: 2},
		},
		"testhidden":  {},
		"testleafy":   {{Kind: "super_effective_multiplier", Numerator: 3, Denominator: 4}},
		"testspecial": {},
		"teststance":  {},
	}
	for id, w := range want {
		g := got[id]
		if g == nil {
			g = []abilityEffect{}
		}
		if !reflect.DeepEqual(g, w) {
			t.Errorf("%s の effects = %+v, want %+v", id, g, w)
		}
	}
	if bytes.Contains(files.Abilities, []byte(`"effects":null`)) {
		t.Errorf("effects が null(空配列 [] で出すこと)")
	}

	var p pokemonTypesFile
	strict(t, files.PokemonTypes, &p)
	for _, pk := range p.Pokemon {
		for _, a := range pk.AbilityIDs {
			if _, ok := got[a]; !ok {
				t.Errorf("%s の abilityId %s が特性の read model に無い", pk.PokemonID, a)
			}
		}
	}
}

// AC-E6: speed 向けの read model(ADR-0600 §4 の形。speed はファイルの差し替えだけで切り替えられる)。
// regulationId は既定のレギュレーションの ID(DB の値)、pokemon は使用可能集合だけ(pokemonId 昇順)、baseSpeed は素早さの種族値。
func TestExportSpeedPokemon(t *testing.T) {
	files, _ := export(t, storetest.New())
	var f speedFile
	strict(t, files.SpeedPokemon, &f)
	if f.SchemaVersion != 1 || f.RegulationID != storetest.DefaultRegulationID {
		t.Errorf("schemaVersion=%d regulationId=%q", f.SchemaVersion, f.RegulationID)
	}
	var got []string
	for _, p := range f.Pokemon {
		got = append(got, p.PokemonID+":"+p.NameJa+":"+strings.Join(p.Types, "/")+":"+itoa(p.BaseSpeed))
	}
	want := []string{"9001-000:テストモン:fire:85", "9001-001:テストメガモン:fire/water:105", "9002-000:テストリーフ:grass:60"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("speed pokemon = %v, want %v", got, want)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// AC-E7: 出力は決定的(DB の行の順によらずバイト単位で同じ)。
func TestExportIsDeterministic(t *testing.T) {
	a, _ := export(t, storetest.New())
	q := storetest.New()
	reverse(q.Species)
	reverse(q.SpeciesAbilities)
	reverse(q.Moves)
	reverse(q.Abilities)
	reverse(q.AbilityEffects)
	for k := range q.RegulationSpecies {
		reverse(q.RegulationSpecies[k])
	}
	for k := range q.RegulationMoves {
		reverse(q.RegulationMoves[k])
	}
	for k := range q.RegulationAbilities {
		reverse(q.RegulationAbilities[k])
	}
	b, _ := export(t, q)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("行の順を変えると出力が変わる")
	}
}

func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// AC-E8: 出力できないときは失敗し、部分的なファイルを返さない。
func TestExportFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(q *storetest.Querier)
		want   error
	}{
		{"既定のレギュレーションが無い", func(q *storetest.Querier) { q.DefaultRegulation = nil }, readmodel.ErrNoDefaultRegulation},
		{"使用可能な種族が0件(schema の minItems 1)", func(q *storetest.Querier) { q.RegulationSpecies = map[string][]string{} }, readmodel.ErrInvalidExport},
		{"使用可能な技が0件", func(q *storetest.Querier) { q.RegulationMoves = map[string][]string{} }, readmodel.ErrInvalidExport},
		{"使用可能な特性が0件", func(q *storetest.Querier) { q.RegulationAbilities = map[string][]string{} }, readmodel.ErrInvalidExport},
		{"係数が 1〜16 の比にならない(4915/4096)", func(q *storetest.Querier) {
			q.AbilityEffects = append(q.AbilityEffects, store.AbilityEffect{AbilityID: "testhidden", Effect: json.RawMessage(`{"DefResistType": {"fire": 4915}}`)})
		}, readmodel.ErrInvalidExport},
		{"効果の JSON が共通マスタの検査を通らない", func(q *storetest.Querier) {
			q.AbilityEffects = append(q.AbilityEffects, store.AbilityEffect{AbilityID: "testhidden", Effect: json.RawMessage(`{"Unknown": 1}`)})
		}, readmodel.ErrInvalidExport},
		{"DB の失敗はそのまま包む", func(q *storetest.Querier) { q.Err = storetest.ErrDB }, storetest.ErrDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			tt.mutate(q)
			files, _, err := readmodel.Export(context.Background(), q)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if !reflect.DeepEqual(files, readmodel.Files{}) {
				t.Errorf("失敗なのに Files がゼロ値でない")
			}
		})
	}
}

// AC-E9: WriteDir は4つのファイル名で書く。無いディレクトリは作り、既存のファイルは置き換え、一時ファイルを残さない。
func TestWriteDir(t *testing.T) {
	files, _ := export(t, storetest.New())
	dir := filepath.Join(t.TempDir(), "readmodel")
	if err := files.WriteDir(dir); err != nil {
		t.Fatalf("WriteDir: %v", err)
	}
	// 2回目(置き換え)。
	if err := os.WriteFile(filepath.Join(dir, readmodel.FileMoves), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteDir(dir); err != nil {
		t.Fatalf("2回目の WriteDir: %v", err)
	}
	want := map[string][]byte{
		readmodel.FilePokemonTypes: files.PokemonTypes,
		readmodel.FileMoves:        files.Moves,
		readmodel.FileAbilities:    files.Abilities,
		readmodel.FileSpeedPokemon: files.SpeedPokemon,
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	var wantNames []string
	for n := range want {
		wantNames = append(wantNames, n)
	}
	sort.Strings(wantNames)
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("書いたファイル = %v, want %v(一時ファイルを残さない)", names, wantNames)
	}
	for name, body := range want {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, body) {
			t.Errorf("%s の中身が Files と違う", name)
		}
	}

	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := files.WriteDir(notDir); err == nil {
		t.Errorf("出力先がファイルなのに WriteDir が成功した")
	}
}

// AC-E10: ファイル名は balance・speed の設定(BALANCE_*_PATH・SPEED_POKEMON_PATH)で指す名前として固定する。
func TestFileNames(t *testing.T) {
	got := []string{readmodel.FilePokemonTypes, readmodel.FileMoves, readmodel.FileAbilities, readmodel.FileSpeedPokemon}
	want := []string{"pokemon-types.json", "moves.json", "abilities.json", "speed-pokemon.json"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ファイル名 = %v, want %v", got, want)
	}
}
