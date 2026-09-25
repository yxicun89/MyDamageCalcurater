package engine

// 特性の候補(issue #272。ADR-0126)。一括計算の防御側・逆算の相手側に、種族が持ちうる特性を渡す。
//
//   - 候補は解決済みの Ability(効果はマスタから)として受け取る。engine に特性の一覧は持ち込まない(ADR-0005)。
//   - 空は「特性なし」の1通り(ゼロ値の Ability)。issue #272 以前と同じ結果になる(後方互換)。
//   - 結果が完全に同じになる特性は1つにまとめる(代表 = 先に渡したもの)。まとめかたは効果の定義を
//     読んで推測せず、計算結果そのものの一致で決める(特性の効果の見落としで黙って誤らないため)。

import (
	"errors"
	"fmt"
	"slices"
)

// MaxAbilityCandidates は特性の候補の件数上限。種族が持つ特性(通常2つ + 隠れ特性1つ)の最大と同じ。
const MaxAbilityCandidates = 3

// ErrInvalidAbilityCandidates は特性の候補が不正(件数超過・ID の重複・空の ID・種族が持たない特性)。
var ErrInvalidAbilityCandidates = errors.New("特性の候補が不正")

// abilityCandidates は候補を検証し、計算に使う特性の列を返す。空なら特性なしの1通り。
// species.Abilities が空(種族の特性が不明)のときは所属を検査しない。
func abilityCandidates(field string, species Species, abilities []Ability) ([]Ability, error) {
	if len(abilities) == 0 {
		return []Ability{{}}, nil
	}
	if len(abilities) > MaxAbilityCandidates {
		return nil, fmt.Errorf("%w: %s は %d 件以下(%d 件)", ErrInvalidAbilityCandidates, field, MaxAbilityCandidates, len(abilities))
	}
	seen := make(map[string]bool, len(abilities))
	for i, a := range abilities {
		switch {
		case a.ID == "":
			return nil, fmt.Errorf("%w: %s[%d] の ID が空", ErrInvalidAbilityCandidates, field, i)
		case seen[a.ID]:
			return nil, fmt.Errorf("%w: %s に %q が重複", ErrInvalidAbilityCandidates, field, a.ID)
		case len(species.Abilities) > 0 && !slices.Contains(species.Abilities, a.ID):
			return nil, fmt.Errorf("%w: 種族 %q は特性 %q を持たない", ErrInvalidAbilityCandidates, species.Key, a.ID)
		}
		seen[a.ID] = true
	}
	return abilities, nil
}

// groupAbilities は特性の添字を「結果が同じもの」でまとめる。各グループの先頭が代表で、渡した順を保つ。
// same(i, j) は特性 i と j の計算結果がすべて一致するかを返す。
func groupAbilities(n int, same func(i, j int) bool) [][]int {
	var groups [][]int
	for i := range n {
		placed := false
		for g := range groups {
			if same(groups[g][0], i) {
				groups[g] = append(groups[g], i)
				placed = true
				break
			}
		}
		if !placed {
			groups = append(groups, []int{i})
		}
	}
	return groups
}

// groupAbilityIDs はグループの特性 ID の列を返す。特性なし(ID 空)の1通りなら nil(従来の出力と同じ)。
func groupAbilityIDs(abilities []Ability, group []int) []string {
	if len(abilities) == 1 && abilities[0].ID == "" {
		return nil
	}
	ids := make([]string, 0, len(group))
	for _, i := range group {
		ids = append(ids, abilities[i].ID)
	}
	return ids
}
