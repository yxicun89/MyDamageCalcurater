package importer

// 各入力ファイルの厳格なデコード(ADR-0101 §3・§12)。未知のフィールド・schemaVersion/source の
// 食い違い・後続データを拒否する(フィールドの改名で値が黙ってゼロ値にならないように)。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const currentSchemaVersion = 1

// regulationIDPattern / datePattern は regulations.json の値の形式(ADR-0101 §7)。
// commitPattern は Showdown・PokeAPI の版(ADR-0101 §3: どちらもリポジトリの commit)の形式。
var (
	regulationIDPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	datePattern         = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
	commitPattern       = regexp.MustCompile(`^[0-9a-f]{40}$`)
	sha256HexPattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	effectHookPattern   = regexp.MustCompile(`^on[A-Z][A-Za-z]*$`)
)

// pendingPrefix は「版・mod がまだ固定されていない」プレースホルダの目印(取り違えたまま
// 実行しないよう明示的に止める。大文字小文字を区別しない)。
const pendingPrefix = "PENDING"

func isPendingPlaceholder(s string) bool {
	return len(s) >= len(pendingPrefix) && strings.EqualFold(s[:len(pendingPrefix)], pendingPrefix)
}

// strictDecode は raw を v に厳格デコードする: 未知のフィールドを拒否し(ネストした構造体にも
// 及ぶ)、単一の JSON 値であること(後続データを許さない)を確認する。
func strictDecode(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if dec.More() {
		return fmt.Errorf("%w: 後続のデータがある", ErrInvalidInput)
	}
	return nil
}

func checkSchemaVersion(v int) error {
	if v != currentSchemaVersion {
		return fmt.Errorf("%w: schemaVersion = %d, want %d", ErrInvalidInput, v, currentSchemaVersion)
	}
	return nil
}

func checkSource(got, want string) error {
	if got != want {
		return fmt.Errorf("%w: source = %q, want %q", ErrInvalidInput, got, want)
	}
	return nil
}

// DecodeCalcSnapshot は data/generated/calc/<version>/snapshot.json をデコードする。
func DecodeCalcSnapshot(raw []byte) (CalcSnapshot, error) {
	var s CalcSnapshot
	if err := strictDecode(raw, &s); err != nil {
		return CalcSnapshot{}, err
	}
	if err := checkSchemaVersion(s.SchemaVersion); err != nil {
		return CalcSnapshot{}, err
	}
	if err := checkSource(s.Source, "calc"); err != nil {
		return CalcSnapshot{}, err
	}
	return s, nil
}

// DecodeShowdownSnapshot は data/generated/showdown/<version>/snapshot.json をデコードする。
func DecodeShowdownSnapshot(raw []byte) (ShowdownSnapshot, error) {
	var s ShowdownSnapshot
	if err := strictDecode(raw, &s); err != nil {
		return ShowdownSnapshot{}, err
	}
	if err := checkSchemaVersion(s.SchemaVersion); err != nil {
		return ShowdownSnapshot{}, err
	}
	if err := checkSource(s.Source, "showdown"); err != nil {
		return ShowdownSnapshot{}, err
	}
	return s, nil
}

// DecodePokeAPISnapshot は data/generated/pokeapi/<version>/snapshot.json をデコードする。
func DecodePokeAPISnapshot(raw []byte) (PokeAPISnapshot, error) {
	var s PokeAPISnapshot
	if err := strictDecode(raw, &s); err != nil {
		return PokeAPISnapshot{}, err
	}
	if err := checkSchemaVersion(s.SchemaVersion); err != nil {
		return PokeAPISnapshot{}, err
	}
	if err := checkSource(s.Source, "pokeapi"); err != nil {
		return PokeAPISnapshot{}, err
	}
	return s, nil
}

// DecodeNameOverrides は data/local/name_ja_overrides.json をデコードする。値の空文字は不正
// (override の意味を持たない)。
func DecodeNameOverrides(raw []byte) (NameOverrides, error) {
	var o NameOverrides
	if err := strictDecode(raw, &o); err != nil {
		return NameOverrides{}, err
	}
	if err := checkSchemaVersion(o.SchemaVersion); err != nil {
		return NameOverrides{}, err
	}
	for _, m := range []map[string]string{o.Species, o.Moves, o.Items, o.Abilities, o.Types} {
		for k, v := range m {
			if v == "" {
				return NameOverrides{}, fmt.Errorf("%w: override %q の値が空", ErrInvalidInput, k)
			}
		}
	}
	return o, nil
}

// DecodeEffectsFile は data/importer/effects.json をデコードする(値そのものの検証は Convert 時に
// master.DecodeItemEffect/DecodeAbilityEffect で行う)。
func DecodeEffectsFile(raw []byte) (EffectsFile, error) {
	var f EffectsFile
	if err := strictDecode(raw, &f); err != nil {
		return EffectsFile{}, err
	}
	if err := checkSchemaVersion(f.SchemaVersion); err != nil {
		return EffectsFile{}, err
	}
	return f, nil
}

// DecodeRegulationsFile は data/importer/regulations.json をデコードする。
func DecodeRegulationsFile(raw []byte) (RegulationsFile, error) {
	var f RegulationsFile
	if err := strictDecode(raw, &f); err != nil {
		return RegulationsFile{}, err
	}
	if err := checkSchemaVersion(f.SchemaVersion); err != nil {
		return RegulationsFile{}, err
	}
	for _, r := range f.Regulations {
		if !regulationIDPattern.MatchString(r.ID) {
			return RegulationsFile{}, fmt.Errorf("%w: レギュレーション ID の形式が不正: %q", ErrInvalidInput, r.ID)
		}
		if r.NameJa == "" {
			return RegulationsFile{}, fmt.Errorf("%w: レギュレーション %q の日本語名が空", ErrInvalidInput, r.ID)
		}
		if r.StartsOn != "" && !datePattern.MatchString(r.StartsOn) {
			return RegulationsFile{}, fmt.Errorf("%w: レギュレーション %q の startsOn の形式が不正: %q", ErrInvalidInput, r.ID, r.StartsOn)
		}
		if r.EndsOn != "" && !datePattern.MatchString(r.EndsOn) {
			return RegulationsFile{}, fmt.Errorf("%w: レギュレーション %q の endsOn の形式が不正: %q", ErrInvalidInput, r.ID, r.EndsOn)
		}
		if r.StartsOn != "" && r.EndsOn != "" && r.StartsOn > r.EndsOn {
			return RegulationsFile{}, fmt.Errorf("%w: レギュレーション %q の開始が終了より後", ErrInvalidInput, r.ID)
		}
		if r.ShowdownMod == "" {
			return RegulationsFile{}, fmt.Errorf("%w: レギュレーション %q の showdownMod が空", ErrInvalidInput, r.ID)
		}
		if isPendingPlaceholder(r.ShowdownMod) {
			return RegulationsFile{}, fmt.Errorf("%w: レギュレーション %q の showdownMod が未固定のプレースホルダのまま: %q", ErrInvalidInput, r.ID, r.ShowdownMod)
		}
		if r.MinSourceGen < 1 {
			return RegulationsFile{}, fmt.Errorf("%w: レギュレーション %q の minSourceGen が不正(1以上であること): %d", ErrInvalidInput, r.ID, r.MinSourceGen)
		}
	}
	return f, nil
}

// DecodeConfig は data/importer/config.json をデコードする。
func DecodeConfig(raw []byte) (Config, error) {
	var c Config
	if err := strictDecode(raw, &c); err != nil {
		return Config{}, err
	}
	if err := checkSchemaVersion(c.SchemaVersion); err != nil {
		return Config{}, err
	}
	if len(c.NameJaLanguages) == 0 {
		return Config{}, fmt.Errorf("%w: nameJaLanguages が空", ErrInvalidInput)
	}
	for _, source := range sortedKeysRaw(c.Sources) {
		version := c.Sources[source]
		if isPendingPlaceholder(version) {
			return Config{}, fmt.Errorf("%w: sources.%s の版が未固定のプレースホルダのまま: %q", ErrInvalidInput, source, version)
		}
	}
	// showdown・pokeapi は ADR-0101 §3 の定義どおり、どちらもリポジトリの commit(40桁の16進)。
	for _, source := range []string{"showdown", "pokeapi"} {
		version, ok := c.Sources[source]
		if !ok || version == "" {
			continue
		}
		if !commitPattern.MatchString(version) {
			return Config{}, fmt.Errorf("%w: sources.%s の版が commit(40桁の16進)の形式でない: %q", ErrInvalidInput, source, version)
		}
	}
	if c.Reconcile != nil {
		if err := validateReconcileConfig(c.Reconcile, c.Sources); err != nil {
			return Config{}, err
		}
	}
	return c, nil
}

// validateReconcileConfig は Config.Reconcile を厳格に検証する(ADR-0103 §5・§9)。
func validateReconcileConfig(r *ReconcileConfig, sources map[string]string) error {
	if len(r.EffectHooks) == 0 {
		return fmt.Errorf("%w: reconcile.effectHooks が空", ErrInvalidInput)
	}
	for _, h := range r.EffectHooks {
		if !effectHookPattern.MatchString(h) {
			return fmt.Errorf("%w: reconcile.effectHooks の形式が不正: %q", ErrInvalidInput, h)
		}
	}
	for _, key := range sortedKeysRaw(r.Verdicts.Basis) {
		if _, ok := sources[key]; !ok {
			return fmt.Errorf("%w: reconcile.verdicts.basis のキー %q が sources に無い", ErrInvalidInput, key)
		}
		version := r.Verdicts.Basis[key]
		if version == "" {
			return fmt.Errorf("%w: reconcile.verdicts.basis.%s の版が空", ErrInvalidInput, key)
		}
		if isPendingPlaceholder(version) {
			return fmt.Errorf("%w: reconcile.verdicts.basis.%s の版が未固定のプレースホルダのまま: %q", ErrInvalidInput, key, version)
		}
	}
	for _, vc := range []struct {
		name  string
		count VerdictCount
	}{
		{"calcOnlyExcluded", r.Verdicts.Moves.CalcOnlyExcluded},
		{"showdownOnlyIncluded", r.Verdicts.Moves.ShowdownOnlyIncluded},
		{"statusTypeMismatch", r.Verdicts.Moves.StatusTypeMismatch},
	} {
		if vc.count.Count < 0 {
			return fmt.Errorf("%w: reconcile.verdicts.moves.%s.count が負: %d", ErrInvalidInput, vc.name, vc.count.Count)
		}
		if !sha256HexPattern.MatchString(vc.count.IDsSHA256) {
			return fmt.Errorf("%w: reconcile.verdicts.moves.%s.idsSha256 の形式が不正(64桁の小文字16進であること): %q", ErrInvalidInput, vc.name, vc.count.IDsSHA256)
		}
	}
	return nil
}
