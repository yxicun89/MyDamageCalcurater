package importer

// 性格(natures)の変換(ADR-0105 §4)。補正の正は Showdown。calc の natures とは突き合わせるだけ
// (calc は無補正を「同じ能力の上昇と下降」で表し、ID を持たないため正にしない)。

import (
	"fmt"
	"regexp"
)

// NatureRow は natures テーブルの行(ADR-0105 §4)。Plus/Minus の空文字は NULL(無補正)。
type NatureRow struct {
	ID           string
	NameJa       string
	NameJaSource string
	NameEn       string
	Plus         string
	Minus        string
}

// CalcNature は calc の性格1件(突き合わせにだけ使う。ID を持たない)。無補正は Plus==Minus。
type CalcNature struct {
	Name  string `json:"name"`
	Plus  string `json:"plus"`
	Minus string `json:"minus"`
}

// ShowdownNature は Showdown の性格1件(補正の正)。無補正は Plus/Minus とも省略(空文字)。
type ShowdownNature struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Plus  string `json:"plus,omitempty"`
	Minus string `json:"minus,omitempty"`
}

// natureMismatchRecovery は nature-mismatch の Blocker の Detail に載せる復旧案内(#74)。
// 片方だけ古いスナップショットが原因のことが多いので、まず両方を取り直す。
const natureMismatchRecovery = "make import-fetch で calc と Showdown のスナップショットを取り直して再実行する。" +
	"取り直しても残るなら data/importer/config.json の sources の版の組み合わせを確かめ、ADR-0105 §4 に沿って人間が裁定する"

// natureIDPattern は natures.id の形式(migration 000006 の chk_natures_id と同じ)。
var natureIDPattern = regexp.MustCompile(`^[a-z0-9]+$`)

// natureStatKeys は plus/minus に許す能力(HP を含まない。migration の chk_natures_plus/minus と同じ)。
var natureStatKeys = map[string]bool{"atk": true, "def": true, "spa": true, "spd": true, "spe": true}

// validateShowdownNatures は Showdown の natures 単体の妥当性(DB の CHECK・UNIQUE と同じ規則)を
// 投入前に検証する。空(古いスナップショット。natures キーが無い)は ErrInvalidData。
func validateShowdownNatures(natures []ShowdownNature) error {
	if len(natures) == 0 {
		return fmt.Errorf("%w: Showdown の natures が空(古いスナップショット。make import-fetch で取り直すこと)", ErrInvalidData)
	}
	seenID := map[string]bool{}
	seenPair := map[[2]string]bool{}
	for _, n := range natures {
		if !natureIDPattern.MatchString(n.ID) {
			return fmt.Errorf("%w: 性格 ID の形式が不正: %q", ErrInvalidData, n.ID)
		}
		if seenID[n.ID] {
			return fmt.Errorf("%w: 性格 ID が重複: %q", ErrInvalidData, n.ID)
		}
		seenID[n.ID] = true
		if (n.Plus == "") != (n.Minus == "") {
			return fmt.Errorf("%w: 性格 %q の plus/minus は組で指定する(片方だけは不可)", ErrInvalidData, n.ID)
		}
		if n.Plus != "" {
			if !natureStatKeys[n.Plus] {
				return fmt.Errorf("%w: 性格 %q の plus が不正: %q", ErrInvalidData, n.ID, n.Plus)
			}
			if !natureStatKeys[n.Minus] {
				return fmt.Errorf("%w: 性格 %q の minus が不正: %q", ErrInvalidData, n.ID, n.Minus)
			}
			if n.Plus == n.Minus {
				return fmt.Errorf("%w: 性格 %q の plus と minus が同じ: %q", ErrInvalidData, n.ID, n.Plus)
			}
			pair := [2]string{n.Plus, n.Minus}
			if seenPair[pair] {
				return fmt.Errorf("%w: 性格の補正の組 (%s, %s) が重複(同じ組は1つだけ)", ErrInvalidData, n.Plus, n.Minus)
			}
			seenPair[pair] = true
		}
	}
	return nil
}

// convertNatures は Showdown の natures(正)を calc の natures と突き合わせ、日本語名を解決する。
// 補正の食い違い・片方にだけある性格は Blocker(KindNatureMismatch)で返す(rows は nil)。
func convertNatures(in Input, usedOverride map[string]bool) ([]NatureRow, []Finding, []Finding, error) {
	if err := validateShowdownNatures(in.Showdown.Natures); err != nil {
		return nil, nil, nil, err
	}

	showdownByID := make(map[string]ShowdownNature, len(in.Showdown.Natures))
	for _, n := range in.Showdown.Natures {
		showdownByID[n.ID] = n
	}
	// calc は無補正を「同じ能力の上昇と下降」で表す(ID を持たない。名前を toID したものを ID とみなす)。
	calcByID := make(map[string]CalcNature, len(in.Calc.Natures))
	for _, n := range in.Calc.Natures {
		calcByID[toID(n.Name)] = n
	}

	ids := map[string]bool{}
	for id := range showdownByID {
		ids[id] = true
	}
	for id := range calcByID {
		ids[id] = true
	}

	var blockers []Finding
	mismatch := func(id, cause string) {
		blockers = append(blockers, Finding{Kind: KindNatureMismatch, ID: id, Detail: cause + ": " + natureMismatchRecovery})
	}
	for _, id := range sortedKeysRaw(ids) {
		sw, hasSW := showdownByID[id]
		calc, hasCalc := calcByID[id]
		switch {
		case !hasSW:
			mismatch(id, "calc-only")
		case !hasCalc:
			mismatch(id, "showdown-only")
		case (sw.Plus == "" && sw.Minus == "") != (calc.Plus == calc.Minus):
			mismatch(id, "modifier")
		case sw.Plus != "" && (sw.Plus != calc.Plus || sw.Minus != calc.Minus):
			mismatch(id, "modifier")
		}
	}
	if len(blockers) > 0 {
		return nil, nil, blockers, nil
	}

	pokeAPINatures := newPokeAPILookup(in.PokeAPI.Natures)
	var warnings []Finding
	rows := make([]NatureRow, 0, len(in.Showdown.Natures))
	for _, id := range sortedKeysRaw(showdownByID) {
		sw := showdownByID[id]
		res := resolveJaName(id, in.Overrides.Natures, pokeAPINatures[id], in.Config.NameJaLanguages, sw.Name, usedOverride)
		if res.Source == "fallback_en" {
			warnings = append(warnings, Finding{Kind: KindNameFallback, ID: id})
		}
		rows = append(rows, NatureRow{
			ID: id, NameJa: res.NameJa, NameJaSource: res.Source, NameEn: sw.Name,
			Plus: sw.Plus, Minus: sw.Minus,
		})
	}
	return rows, warnings, nil, nil
}
