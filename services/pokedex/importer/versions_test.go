package importer_test

// 取得元の版に変化が無ければ投入をスキップする判定(ADR-0101 §9。DECISIONS 2026-09-21 の CronJob 運用の前提)。

import (
	"errors"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

func TestChecksumIsSHA256Hex(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	}
	for _, tt := range tests {
		if got := importer.Checksum([]byte(tt.in)); got != tt.want {
			t.Errorf("Checksum(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func sv(source, version, fill string) importer.SourceVersion {
	return importer.SourceVersion{Source: source, Version: version, Checksum: strings.Repeat(fill, 64)}
}

func TestNeedsImport(t *testing.T) {
	applied := []importer.SourceVersion{sv("calc", "v1", "a"), sv("showdown", "c1", "b")}
	tests := []struct {
		name     string
		applied  []importer.SourceVersion
		incoming []importer.SourceVersion
		want     bool
	}{
		{"初回(DB に版が無い)", nil, applied, true},
		{"全取得元で版とチェックサムが同じ", applied, []importer.SourceVersion{sv("showdown", "c1", "b"), sv("calc", "v1", "a")}, false},
		{"チェックサムが違う", applied, []importer.SourceVersion{sv("calc", "v1", "c"), sv("showdown", "c1", "b")}, true},
		{"版が違う(チェックサムは同じ)", applied, []importer.SourceVersion{sv("calc", "v2", "a"), sv("showdown", "c1", "b")}, true},
		{"新しい取得元が増えた", applied, []importer.SourceVersion{sv("calc", "v1", "a"), sv("showdown", "c1", "b"), sv("pokeapi", "p1", "d")}, true},
		{"取得元が減った", applied, []importer.SourceVersion{sv("calc", "v1", "a")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := importer.NeedsImport(tt.applied, tt.incoming)
			if err != nil {
				t.Fatalf("NeedsImport: %v", err)
			}
			if got != tt.want {
				t.Errorf("NeedsImport = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeedsImportRejectsInvalidVersions(t *testing.T) {
	tests := []struct {
		name     string
		incoming []importer.SourceVersion
	}{
		{"取得元の重複", []importer.SourceVersion{sv("calc", "v1", "a"), sv("calc", "v2", "a")}},
		{"source の形式(data_versions の CHECK と同じ)", []importer.SourceVersion{sv("Calc", "v1", "a")}},
		{"version が空", []importer.SourceVersion{sv("calc", "", "a")}},
		{"checksum が sha256 の16進でない", []importer.SourceVersion{{Source: "calc", Version: "v1", Checksum: "xyz"}}},
		{"空(何も取り込まない)", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := importer.NeedsImport(nil, tt.incoming); !errors.Is(err, importer.ErrInvalidInput) {
				t.Errorf("err = %v, want ErrInvalidInput", err)
			}
		})
	}
}
