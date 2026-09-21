package importer

// 入力ディレクトリの読み込み(ADR-0101 §1・§9・§12)。
//
//	root/importer/config.json
//	root/importer/effects.json
//	root/importer/regulations.json
//	root/local/name_ja_overrides.json (任意)
//	root/generated/<source>/<version>/snapshot.json (calc / showdown / pokeapi)

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// versionPattern は取得元の版としてパスに使える文字だけを許す(パスの外に出さない)。
var versionPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func readRequired(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %s を読めない: %v", ErrInvalidInput, path, err)
	}
	return raw, nil
}

// LoadInput は root(本番は data/)から入力一式を読み、取得元ごとの版一覧(source 順)を返す。
func LoadInput(root string) (Input, []SourceVersion, error) {
	configRaw, err := readRequired(filepath.Join(root, "importer", "config.json"))
	if err != nil {
		return Input{}, nil, err
	}
	cfg, err := DecodeConfig(configRaw)
	if err != nil {
		return Input{}, nil, err
	}

	effectsRaw, err := readRequired(filepath.Join(root, "importer", "effects.json"))
	if err != nil {
		return Input{}, nil, err
	}
	effects, err := DecodeEffectsFile(effectsRaw)
	if err != nil {
		return Input{}, nil, err
	}

	regulationsRaw, err := readRequired(filepath.Join(root, "importer", "regulations.json"))
	if err != nil {
		return Input{}, nil, err
	}
	regulations, err := DecodeRegulationsFile(regulationsRaw)
	if err != nil {
		return Input{}, nil, err
	}

	overridesPath := filepath.Join(root, "local", "name_ja_overrides.json")
	var overridesRaw []byte
	var overrides NameOverrides
	overridesVersion := "none"
	if raw, err := os.ReadFile(overridesPath); err == nil {
		overridesRaw = raw
		overridesVersion = "local"
		overrides, err = DecodeNameOverrides(raw)
		if err != nil {
			return Input{}, nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Input{}, nil, fmt.Errorf("%w: %s を読めない: %v", ErrInvalidInput, overridesPath, err)
	}

	calcRaw, calcVersion, err := readSnapshot(root, "calc", cfg)
	if err != nil {
		return Input{}, nil, err
	}
	calc, err := DecodeCalcSnapshot(calcRaw)
	if err != nil {
		return Input{}, nil, err
	}
	if calc.Version != calcVersion {
		return Input{}, nil, fmt.Errorf("%w: calc スナップショットの version %q が設定の版 %q と違う", ErrInvalidInput, calc.Version, calcVersion)
	}

	showdownRaw, showdownVersion, err := readSnapshot(root, "showdown", cfg)
	if err != nil {
		return Input{}, nil, err
	}
	showdown, err := DecodeShowdownSnapshot(showdownRaw)
	if err != nil {
		return Input{}, nil, err
	}
	if showdown.Version != showdownVersion {
		return Input{}, nil, fmt.Errorf("%w: showdown スナップショットの version %q が設定の版 %q と違う", ErrInvalidInput, showdown.Version, showdownVersion)
	}

	pokeapiRaw, pokeapiVersion, err := readSnapshot(root, "pokeapi", cfg)
	if err != nil {
		return Input{}, nil, err
	}
	pokeapi, err := DecodePokeAPISnapshot(pokeapiRaw)
	if err != nil {
		return Input{}, nil, err
	}
	if pokeapi.Version != pokeapiVersion {
		return Input{}, nil, fmt.Errorf("%w: pokeapi スナップショットの version %q が設定の版 %q と違う", ErrInvalidInput, pokeapi.Version, pokeapiVersion)
	}

	versions := []SourceVersion{
		{Source: "calc", Version: calcVersion, Checksum: Checksum(calcRaw)},
		{Source: "showdown", Version: showdownVersion, Checksum: Checksum(showdownRaw)},
		{Source: "pokeapi", Version: pokeapiVersion, Checksum: Checksum(pokeapiRaw)},
		{Source: "importer-config", Version: "local", Checksum: Checksum(configRaw)},
		{Source: "effects", Version: "local", Checksum: Checksum(effectsRaw)},
		{Source: "regulations", Version: "local", Checksum: Checksum(regulationsRaw)},
		{Source: "name-overrides", Version: overridesVersion, Checksum: Checksum(overridesRaw)},
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].Source < versions[j].Source })

	in := Input{
		Calc:        calc,
		Showdown:    showdown,
		PokeAPI:     pokeapi,
		Overrides:   overrides,
		Effects:     effects,
		Regulations: regulations,
		Config:      cfg,
	}
	return in, versions, nil
}

// readSnapshot は config.json の sources[source] を版として、
// root/generated/<source>/<version>/snapshot.json を読む。
func readSnapshot(root, source string, cfg Config) (raw []byte, version string, err error) {
	version = cfg.Sources[source]
	if !versionPattern.MatchString(version) {
		return nil, "", fmt.Errorf("%w: %s の版の形式が不正: %q", ErrInvalidInput, source, version)
	}
	path := filepath.Join(root, "generated", source, version, "snapshot.json")
	raw, err = readRequired(path)
	if err != nil {
		return nil, "", err
	}
	return raw, version, nil
}
