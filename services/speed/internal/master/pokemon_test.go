package master

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/speed/internal/speed"
)

// examplePokemonPath は架空データの正(ADR-0600 §4)。
const examplePokemonPath = "../../testdata/pokemon.example.json"

// overlayPokemonPath は local overlay が ConfigMap でマウントするコピー。Kustomize の load restrictor が
// overlay の外のファイルを参照させないため複製し、このテストで同期を検査する(balance と同じ)。
const overlayPokemonPath = "../../deploy/k8s/overlays/local/pokemon.example.json"

func TestOverlayExampleMatchesTestdataExample(t *testing.T) {
	t.Parallel()

	want, err := os.ReadFile(examplePokemonPath)
	if err != nil {
		t.Fatalf("read %s: %v", examplePokemonPath, err)
	}
	got, err := os.ReadFile(overlayPokemonPath)
	if err != nil {
		t.Fatalf("read %s: %v", overlayPokemonPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is out of sync with %s (the source of truth); copy it over", overlayPokemonPath, examplePokemonPath)
	}
}

func TestLoadPokemonFileReadsExample(t *testing.T) {
	t.Parallel()

	model, err := LoadPokemonFile(examplePokemonPath)
	if err != nil {
		t.Fatalf("LoadPokemonFile(%s) error = %v", examplePokemonPath, err)
	}
	roster, err := model.Roster()
	if err != nil {
		t.Fatalf("Roster() error = %v", err)
	}
	if roster.RegulationID != "example" {
		t.Errorf("RegulationID = %q, want example", roster.RegulationID)
	}
	if len(roster.Pokemon) < 2 {
		t.Fatalf("len(Pokemon) = %d, want at least 2", len(roster.Pokemon))
	}
	want := speed.Pokemon{PokemonID: "9001-000", NameJa: "テストカソウドリ", Types: []string{"fire", "flying"}, BaseSpeed: 100}
	if !reflect.DeepEqual(roster.Pokemon[0], want) {
		t.Errorf("Pokemon[0] = %+v, want %+v", roster.Pokemon[0], want)
	}
	// SP1 の同速のまとめを試すため、例には同じ素早さ種族値のポケモンが 2 体以上いること。
	seen := map[int]string{}
	hasTie := false
	for _, p := range roster.Pokemon {
		if !strings.HasPrefix(p.PokemonID, "9") {
			t.Errorf("pokemonId %q must be a fictional 9xxx ID (ADR-0002)", p.PokemonID)
		}
		if _, ok := seen[p.BaseSpeed]; ok {
			hasTie = true
		}
		seen[p.BaseSpeed] = p.PokemonID
	}
	if !hasTie {
		t.Error("the example must contain at least two pokemon with the same baseSpeed (for the SP1 tie test)")
	}
}

func TestLoadPokemonPreservesFileOrder(t *testing.T) {
	t.Parallel()

	doc := `{"schemaVersion":1,"regulationId":"m-c","pokemon":[
		{"pokemonId":"9002-000","nameJa":"テストニバンメ","types":["water"],"baseSpeed":81},
		{"pokemonId":"9001-000","nameJa":"テストイチバンメ","types":["fire","flying"],"baseSpeed":100}]}`
	model, err := LoadPokemon(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("LoadPokemon error = %v", err)
	}
	roster, err := model.Roster()
	if err != nil {
		t.Fatalf("Roster() error = %v", err)
	}
	want := speed.Roster{RegulationID: "m-c", Pokemon: []speed.Pokemon{
		{PokemonID: "9002-000", NameJa: "テストニバンメ", Types: []string{"water"}, BaseSpeed: 81},
		{PokemonID: "9001-000", NameJa: "テストイチバンメ", Types: []string{"fire", "flying"}, BaseSpeed: 100},
	}}
	if !reflect.DeepEqual(roster, want) {
		t.Errorf("Roster() = %+v, want %+v", roster, want)
	}
}

// TestRosterReturnsCopy は呼び出し側の変更が read model に残らないことを確かめる(PokemonProvider の約束)。
func TestRosterReturnsCopy(t *testing.T) {
	t.Parallel()

	model, err := LoadPokemon(strings.NewReader(validDoc(validEntry)))
	if err != nil {
		t.Fatalf("LoadPokemon error = %v", err)
	}
	first, err := model.Roster()
	if err != nil {
		t.Fatalf("Roster() error = %v", err)
	}
	first.Pokemon[0].Types[0] = "changed"
	first.Pokemon[0].BaseSpeed = 1
	first.Pokemon = append(first.Pokemon, speed.Pokemon{PokemonID: "9999-999"})

	second, err := model.Roster()
	if err != nil {
		t.Fatalf("Roster() error = %v", err)
	}
	want := []speed.Pokemon{{PokemonID: "9001-000", NameJa: "テストカソウドリ", Types: []string{"fire", "flying"}, BaseSpeed: 100}}
	if !reflect.DeepEqual(second.Pokemon, want) {
		t.Errorf("Roster() after mutation = %+v, want %+v", second.Pokemon, want)
	}
}

const validEntry = `{"pokemonId":"9001-000","nameJa":"テストカソウドリ","types":["fire","flying"],"baseSpeed":100}`

func validDoc(entries ...string) string {
	return `{"schemaVersion":1,"regulationId":"example","pokemon":[` + strings.Join(entries, ",") + `]}`
}

func entryWith(pokemonID, nameJa, types, baseSpeed string) string {
	return `{"pokemonId":` + pokemonID + `,"nameJa":` + nameJa + `,"types":` + types + `,"baseSpeed":` + baseSpeed + `}`
}

func TestLoadPokemonAcceptsBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		doc  string
	}{
		{"baseSpeed 1", validDoc(entryWith(`"9001-000"`, `"ア"`, `["fire"]`, `1`))},
		{"baseSpeed 255", validDoc(entryWith(`"9001-000"`, `"ア"`, `["fire"]`, `255`))},
		{"types 1 個", validDoc(entryWith(`"9001-000"`, `"ア"`, `["normal"]`, `50`))},
		{"regulationId にハイフン", `{"schemaVersion":1,"regulationId":"m-c-2","pokemon":[` + validEntry + `]}`},
		{"regulationId に数字", `{"schemaVersion":1,"regulationId":"reg2026","pokemon":[` + validEntry + `]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := LoadPokemon(strings.NewReader(tt.doc)); err != nil {
				t.Errorf("LoadPokemon error = %v, want nil", err)
			}
		})
	}
}

func TestLoadPokemonRejectsInvalidReadModel(t *testing.T) {
	t.Parallel()

	withRegulation := func(regulationID string) string {
		return `{"schemaVersion":1,"regulationId":` + regulationID + `,"pokemon":[` + validEntry + `]}`
	}
	tests := []struct {
		name string
		doc  string
	}{
		// schemaVersion
		{"schemaVersion 0", `{"schemaVersion":0,"regulationId":"example","pokemon":[` + validEntry + `]}`},
		{"schemaVersion 2", `{"schemaVersion":2,"regulationId":"example","pokemon":[` + validEntry + `]}`},
		{"schemaVersion 欠落", `{"regulationId":"example","pokemon":[` + validEntry + `]}`},
		// regulationId: ^[a-z0-9]+(-[a-z0-9]+)*$
		{"regulationId 欠落", `{"schemaVersion":1,"pokemon":[` + validEntry + `]}`},
		{"regulationId 空", withRegulation(`""`)},
		{"regulationId 大文字", withRegulation(`"M-C"`)},
		{"regulationId 先頭ハイフン", withRegulation(`"-mc"`)},
		{"regulationId 末尾ハイフン", withRegulation(`"mc-"`)},
		{"regulationId 連続ハイフン", withRegulation(`"m--c"`)},
		{"regulationId 下線", withRegulation(`"m_c"`)},
		// pokemon
		{"pokemon 空", validDoc()},
		{"pokemon 欠落", `{"schemaVersion":1,"regulationId":"example"}`},
		// pokemonId: NNNN-NNN、重複不可
		{"pokemonId 空", validDoc(entryWith(`""`, `"ア"`, `["fire"]`, `100`))},
		{"pokemonId 桁不足", validDoc(entryWith(`"9001-00"`, `"ア"`, `["fire"]`, `100`))},
		{"pokemonId 桁超過", validDoc(entryWith(`"9001-0000"`, `"ア"`, `["fire"]`, `100`))},
		{"pokemonId 区切りなし", validDoc(entryWith(`"9001000"`, `"ア"`, `["fire"]`, `100`))},
		{"pokemonId 英字", validDoc(entryWith(`"a001-000"`, `"ア"`, `["fire"]`, `100`))},
		{"pokemonId 重複", validDoc(validEntry, entryWith(`"9001-000"`, `"イ"`, `["water"]`, `50`))},
		// nameJa
		{"nameJa 空", validDoc(entryWith(`"9001-000"`, `""`, `["fire"]`, `100`))},
		{"nameJa 空白だけ", validDoc(entryWith(`"9001-000"`, `"　 "`, `["fire"]`, `100`))},
		{"nameJa 欠落", validDoc(`{"pokemonId":"9001-000","types":["fire"],"baseSpeed":100}`)},
		// types: 1〜2 個・重複なし・英小文字
		{"types 0 個", validDoc(entryWith(`"9001-000"`, `"ア"`, `[]`, `100`))},
		{"types 欠落", validDoc(`{"pokemonId":"9001-000","nameJa":"テストア","baseSpeed":100}`)},
		{"types 3 個", validDoc(entryWith(`"9001-000"`, `"ア"`, `["fire","water","grass"]`, `100`))},
		{"types 重複", validDoc(entryWith(`"9001-000"`, `"ア"`, `["fire","fire"]`, `100`))},
		{"types 空文字", validDoc(entryWith(`"9001-000"`, `"ア"`, `[""]`, `100`))},
		{"types 大文字", validDoc(entryWith(`"9001-000"`, `"ア"`, `["Fire"]`, `100`))},
		{"types 数字", validDoc(entryWith(`"9001-000"`, `"ア"`, `["type1"]`, `100`))},
		{"types 日本語", validDoc(entryWith(`"9001-000"`, `"ア"`, `["ほのお"]`, `100`))},
		// baseSpeed: 1〜255
		{"baseSpeed 0", validDoc(entryWith(`"9001-000"`, `"ア"`, `["fire"]`, `0`))},
		{"baseSpeed 256", validDoc(entryWith(`"9001-000"`, `"ア"`, `["fire"]`, `256`))},
		{"baseSpeed 負", validDoc(entryWith(`"9001-000"`, `"ア"`, `["fire"]`, `-1`))},
		{"baseSpeed 欠落", validDoc(`{"pokemonId":"9001-000","nameJa":"テストア","types":["fire"]}`)},
		{"baseSpeed 小数", validDoc(entryWith(`"9001-000"`, `"ア"`, `["fire"]`, `100.5`))},
		// JSON の形
		{"未知のフィールド(最上位)", `{"schemaVersion":1,"regulationId":"example","extra":true,"pokemon":[` + validEntry + `]}`},
		{"未知のフィールド(ポケモン)", validDoc(`{"pokemonId":"9001-000","nameJa":"テストア","types":["fire"],"baseSpeed":100,"baseAttack":80}`)},
		{"末尾の余計な JSON", validDoc(validEntry) + `{}`},
		{"JSON でない", `not json`},
		{"空の入力", ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			model, err := LoadPokemon(strings.NewReader(tt.doc))
			if !errors.Is(err, ErrInvalidPokemon) {
				t.Fatalf("LoadPokemon error = %v, want errors.Is(_, ErrInvalidPokemon)", err)
			}
			if model != nil {
				t.Errorf("model = %+v, want nil on error (a broken file is never partially used)", model)
			}
		})
	}
}

func TestLoadPokemonFileErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalid, []byte(validDoc()), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPokemonFile(invalid); !errors.Is(err, ErrInvalidPokemon) {
		t.Errorf("LoadPokemonFile(invalid) error = %v, want ErrInvalidPokemon", err)
	}
	if _, err := LoadPokemonFile(filepath.Join(dir, "missing.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("LoadPokemonFile(missing) error = %v, want os.ErrNotExist", err)
	}
}
