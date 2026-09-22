package readmodel_test

// 素早さレーンの依頼(2026-09-22): export した speed-pokemon.json が services/speed/internal/master.LoadPokemon
// (services/speed は別 module で、internal パッケージのため import できない。README・ADR-0600 §4 の規則を
// このテストに写して同等の検査を行う)の受け付ける形であることを確かめる。
//
// services/speed/internal/master.LoadPokemon(ADR-0600 §4)の規則:
//   - 最上位は {schemaVersion, regulationId, pokemon} だけ(未知のフィールドを拒否。JSON は1つだけ)
//   - schemaVersion は 1
//   - regulationId は ^[a-z0-9]+(-[a-z0-9]+)*$
//   - pokemon は1件以上。各要素は {pokemonId, nameJa, types, baseSpeed} だけで、pokemonId は
//     ^\d{4}-\d{3}$ かつ重複なし、nameJa は前後の空白を除いて空でない、types は1〜2個・重複なし・^[a-z]+$、
//     baseSpeed は 1〜255 の整数

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/readmodel"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

var (
	speedPokemonIDPattern    = regexp.MustCompile(`^\d{4}-\d{3}$`)
	speedRegulationIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	speedTypeIDPattern       = regexp.MustCompile(`^[a-z]+$`)
)

type speedPokemonFile struct {
	SchemaVersion int                 `json:"schemaVersion"`
	RegulationID  string              `json:"regulationId"`
	Pokemon       []speedPokemonEntry `json:"pokemon"`
}

type speedPokemonEntry struct {
	PokemonID string   `json:"pokemonId"`
	NameJa    string   `json:"nameJa"`
	Types     []string `json:"types"`
	BaseSpeed int      `json:"baseSpeed"`
}

// assertLoadableBySpeed は services/speed/internal/master.LoadPokemon が受け付ける形であることを確かめる
// (同じ検査をここに複製している。speed 側の規則が変わったらこのテストも合わせて直すこと)。
func assertLoadableBySpeed(t *testing.T, doc []byte) {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.DisallowUnknownFields()
	var f speedPokemonFile
	if err := dec.Decode(&f); err != nil {
		t.Fatalf("speed の loader が読める形でない: %v\n%s", err, doc)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Fatalf("JSON が1つの値でない(speed の loader は単一ドキュメントだけを受け付ける)")
	}

	if f.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", f.SchemaVersion)
	}
	if !speedRegulationIDPattern.MatchString(f.RegulationID) {
		t.Errorf("regulationId %q が形式に合わない", f.RegulationID)
	}
	if len(f.Pokemon) == 0 {
		t.Fatal("pokemon が空(speed の loader は1件以上を要求する)")
	}
	seenIDs := map[string]bool{}
	for _, p := range f.Pokemon {
		if !speedPokemonIDPattern.MatchString(p.PokemonID) {
			t.Errorf("pokemonId %q が形式に合わない", p.PokemonID)
		}
		if seenIDs[p.PokemonID] {
			t.Errorf("pokemonId %q が重複している", p.PokemonID)
		}
		seenIDs[p.PokemonID] = true
		if strings.TrimSpace(p.NameJa) == "" {
			t.Errorf("%s の nameJa が空", p.PokemonID)
		}
		if len(p.Types) < 1 || len(p.Types) > 2 {
			t.Errorf("%s の types が1〜2個でない: %v", p.PokemonID, p.Types)
		}
		seenTypes := map[string]bool{}
		for _, ty := range p.Types {
			if !speedTypeIDPattern.MatchString(ty) {
				t.Errorf("%s のタイプ %q が形式に合わない", p.PokemonID, ty)
			}
			if seenTypes[ty] {
				t.Errorf("%s のタイプ %q が重複している", p.PokemonID, ty)
			}
			seenTypes[ty] = true
		}
		if p.BaseSpeed < 1 || p.BaseSpeed > 255 {
			t.Errorf("%s の baseSpeed %d が1〜255でない", p.PokemonID, p.BaseSpeed)
		}
	}
}

// AC-E6 追加確認(2026-09-22 素早さレーンの依頼): export した speed-pokemon.json が
// services/speed の loader の全条件を満たす。
func TestSpeedPokemonIsLoadableBySpeedService(t *testing.T) {
	files, _, err := readmodel.Export(t.Context(), storetest.New())
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	assertLoadableBySpeed(t, files.SpeedPokemon)
}
