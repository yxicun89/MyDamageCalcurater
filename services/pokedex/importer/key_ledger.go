package importer

// 種族 key の台帳と、ID の消滅・消滅後の再利用の検出(issue #277・ADR-0131)。
// この判定は DB を使わない純粋な関数。DB との受け渡しは Apply が行う。

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrKeyRemoved は、既存の DB にある種族 key・技/持ち物/特性の ID が新しい出力から消える投入で、
// 人が承認していないこと(ErrKeyChanged と区別する)。詳細は *RemovedIDsError。
var ErrKeyRemoved = errors.New("importer: 既存の ID が消える投入")

// IDKind は消滅を扱う ID の種類。
type IDKind string

const (
	IDKindSpecies IDKind = "species"
	IDKindMove    IDKind = "move"
	IDKindItem    IDKind = "item"
	IDKindAbility IDKind = "ability"
)

// valid は既知の種類か。
func (k IDKind) valid() bool {
	switch k {
	case IDKindSpecies, IDKindMove, IDKindItem, IDKindAbility:
		return true
	}
	return false
}

// RemovedID は消える(または消えることを承認する)ID 1件。
type RemovedID struct {
	Kind IDKind
	ID   string
}

// String は "<種類>:<ID>"(-allow-removed にそのまま写せる形)。
func (r RemovedID) String() string { return string(r.Kind) + ":" + r.ID }

// RemovedIDsError は ErrKeyRemoved を包み、承認されていない消滅の一覧を持つ。
type RemovedIDsError struct{ IDs []RemovedID }

func (e *RemovedIDsError) Error() string {
	return fmt.Sprintf("%v: %s(内容を確かめたうえで -allow-removed で承認する)", ErrKeyRemoved, joinRemoved(e.IDs))
}

func (e *RemovedIDsError) Unwrap() error { return ErrKeyRemoved }

func joinRemoved(ids []RemovedID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = id.String()
	}
	return strings.Join(parts, ",")
}

// ApplyOptions は Apply の人による承認。ゼロ値は何も許さない。
type ApplyOptions struct {
	AllowRemoved []RemovedID
}

// ParseAllowRemoved は -allow-removed の値(<種類>:<ID> のカンマ区切り)を読む。
// 空文字は何も許さない(nil)。形式の誤り・重複は ErrInvalidInput。
func ParseAllowRemoved(s string) ([]RemovedID, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var out []RemovedID
	seen := map[RemovedID]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		kind, id, ok := strings.Cut(part, ":")
		r := RemovedID{Kind: IDKind(strings.TrimSpace(kind)), ID: strings.TrimSpace(id)}
		if !ok || !r.Kind.valid() || r.ID == "" || strings.Contains(r.ID, ":") {
			return nil, fmt.Errorf("%w: -allow-removed の要素 %q が <種類>:<ID>(種類は species/move/item/ability)の形でない", ErrInvalidInput, part)
		}
		if seen[r] {
			return nil, fmt.Errorf("%w: -allow-removed に %s が重複している", ErrInvalidInput, r)
		}
		seen[r] = true
		out = append(out, r)
	}
	return out, nil
}

// ExistingIDs は投入前の DB にある ID。
type ExistingIDs struct {
	SpeciesKeys, MoveIDs, ItemIDs, AbilityIDs []string
}

// RemovedIDs は existing にあって out に無い ID を、種類・ID の順に並べて返す。
func RemovedIDs(existing ExistingIDs, out Output) []RemovedID {
	var removed []RemovedID
	add := func(kind IDKind, old []string, now map[string]bool) {
		for _, id := range old {
			if !now[id] {
				removed = append(removed, RemovedID{Kind: kind, ID: id})
			}
		}
	}
	species := map[string]bool{}
	for _, s := range out.Species {
		species[s.Key] = true
	}
	moves := map[string]bool{}
	for _, m := range out.Moves {
		moves[m.ID] = true
	}
	items := map[string]bool{}
	for _, it := range out.Items {
		items[it.ID] = true
	}
	abilities := map[string]bool{}
	for _, a := range out.Abilities {
		abilities[a.ID] = true
	}
	add(IDKindSpecies, existing.SpeciesKeys, species)
	add(IDKindMove, existing.MoveIDs, moves)
	add(IDKindItem, existing.ItemIDs, items)
	add(IDKindAbility, existing.AbilityIDs, abilities)
	sort.Slice(removed, func(i, j int) bool {
		if removed[i].Kind != removed[j].Kind {
			return removed[i].Kind < removed[j].Kind
		}
		return removed[i].ID < removed[j].ID
	})
	return removed
}

// CheckRemovals は removed(実際の消滅)を allowed(人の承認)と突き合わせる。
// 承認の無い消滅があれば *RemovedIDsError(ErrKeyRemoved)、実際には消えない ID を承認していれば
// ErrInvalidInput(打ち間違い)。
func CheckRemovals(removed, allowed []RemovedID) error {
	actual := make(map[RemovedID]bool, len(removed))
	for _, r := range removed {
		actual[r] = true
	}
	var typos []string
	approved := make(map[RemovedID]bool, len(allowed))
	for _, a := range allowed {
		approved[a] = true
		if !actual[a] {
			typos = append(typos, a.String())
		}
	}
	if len(typos) > 0 {
		return fmt.Errorf("%w: -allow-removed の %s は消える ID ではない(打ち間違い)", ErrInvalidInput, strings.Join(typos, ","))
	}
	var unapproved []RemovedID
	for _, r := range removed {
		if !approved[r] {
			unapproved = append(unapproved, r)
		}
	}
	if len(unapproved) > 0 {
		return &RemovedIDsError{IDs: unapproved}
	}
	return nil
}

// LedgerEntry は台帳の1行(過去に配った key と showdown_id の対応)。
type LedgerEntry struct{ Key, ShowdownID string }

// CheckLedger は out の各種族の key↔showdown_id が台帳と食い違えば ErrKeyChanged を返す。
// 台帳にある組がそのまま戻る(復活)ことと、台帳に無い組の追加は通す。
func CheckLedger(ledger []LedgerEntry, out Output) error {
	keyByShowdownID := make(map[string]string, len(ledger))
	showdownIDByKey := make(map[string]string, len(ledger))
	for _, e := range ledger {
		keyByShowdownID[e.ShowdownID] = e.Key
		showdownIDByKey[e.Key] = e.ShowdownID
	}
	for _, sp := range out.Species {
		if oldKey, ok := keyByShowdownID[sp.ShowdownID]; ok && oldKey != sp.Key {
			return fmt.Errorf("%w: showdown_id %q の key が(台帳では)%q → %q", ErrKeyChanged, sp.ShowdownID, oldKey, sp.Key)
		}
		if oldID, ok := showdownIDByKey[sp.Key]; ok && oldID != sp.ShowdownID {
			return fmt.Errorf("%w: key %q の showdown_id が(台帳では)%q → %q", ErrKeyChanged, sp.Key, oldID, sp.ShowdownID)
		}
	}
	return nil
}
