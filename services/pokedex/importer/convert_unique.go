package importer

// 取得元の ID の重複検査(#311・ADR-0115)。変換は ID で引く map を作るので、同じ ID の行が
// 2つあると後の行が黙って勝つ(calc の技は後の行だけが Showdown と比べられ、PokeAPI は後の
// slug の名前が採られる)。どちらを採るべきかは人が見るべきなので、変換の前に止める。

import "fmt"

// checkUniqueSourceIDs は calc(toID(名前))・Showdown(id)・PokeAPI(toID(slug))の各一覧に
// 同じ ID が2回現れないことを確かめる。重複は ErrInvalidData。
func checkUniqueSourceIDs(in Input) error {
	lists := []struct {
		label string
		ids   []string
	}{
		{"calc の種族名", mapNames(in.Calc.Species, func(s CalcSpecies) string { return toID(s.Name) })},
		{"calc の技名", mapNames(in.Calc.Moves, func(m CalcMove) string { return toID(m.Name) })},
		{"calc の持ち物名", mapNames(in.Calc.Items, toID)},
		{"calc の特性名", mapNames(in.Calc.Abilities, toID)},
		{"calc の性格名", mapNames(in.Calc.Natures, func(n CalcNature) string { return toID(n.Name) })},
		{"Showdown の種族 id", mapNames(in.Showdown.Species, func(s ShowdownSpecies) string { return s.ID })},
		{"Showdown の技 id", mapNames(in.Showdown.Moves, func(m ShowdownMove) string { return m.ID })},
		{"Showdown の持ち物 id", mapNames(in.Showdown.Items, func(it ShowdownItem) string { return it.ID })},
		{"Showdown の特性 id", mapNames(in.Showdown.Abilities, func(a ShowdownAbility) string { return a.ID })},
		{"Showdown の性格 id", mapNames(in.Showdown.Natures, func(n ShowdownNature) string { return n.ID })},
		{"PokeAPI の種族 slug", pokeAPIIDs(in.PokeAPI.Species)},
		{"PokeAPI のフォーム slug", pokeAPIIDs(in.PokeAPI.Forms)},
		{"PokeAPI の技 slug", pokeAPIIDs(in.PokeAPI.Moves)},
		{"PokeAPI の持ち物 slug", pokeAPIIDs(in.PokeAPI.Items)},
		{"PokeAPI の特性 slug", pokeAPIIDs(in.PokeAPI.Abilities)},
		{"PokeAPI のタイプ slug", pokeAPIIDs(in.PokeAPI.Types)},
		{"PokeAPI の性格 slug", pokeAPIIDs(in.PokeAPI.Natures)},
	}
	for _, l := range lists {
		seen := make(map[string]bool, len(l.ids))
		for _, id := range l.ids {
			if seen[id] {
				return fmt.Errorf("%w: %s が重複している: %q(どちらを採るかは人が決める)", ErrInvalidData, l.label, id)
			}
			seen[id] = true
		}
	}
	return nil
}

func mapNames[T any](xs []T, f func(T) string) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func pokeAPIIDs(entries []PokeAPIName) []string {
	return mapNames(entries, func(e PokeAPIName) string { return toID(e.Slug) })
}
