package readmodel_test

// 持ち物の役割・メガストーンの判定(ADR-0175)は公開 API(searchItems)だけに出し、balance・speed の read model には出さない
// (ADR-0175 §5)。read model の6ファイルは持ち物を持たず、loader は未知のフィールドを拒否するので、形を変えない。
// メガストーン・効果を持つ持ち物がマスタにあっても、どのファイルにも持ち物の ID・役割の欄が現れないことを確かめる。

import (
	"bytes"
	"testing"

	"example.com/pokecalc/services/pokedex/internal/storetest"
)

func TestExportDoesNotCarryItemRoles(t *testing.T) {
	// storetest.New() は メガストーン teststone(メガ種族 9001-001 の requiredItemId)・testorb・testberry(効果あり)を持つ。
	files, _ := export(t, storetest.New())
	outputs := map[string][]byte{
		"pokemon-types.json": files.PokemonTypes,
		"moves.json":         files.Moves,
		"abilities.json":     files.Abilities,
		"speed-pokemon.json": files.SpeedPokemon,
		"type-chart.json":    files.TypeChart,
		"metadata.json":      files.Metadata,
	}
	for name, data := range outputs {
		for _, needle := range []string{`"roles"`, `"isMegaStone"`, `"teststone"`, `"testorb"`, `"testberry"`} {
			if bytes.Contains(data, []byte(needle)) {
				t.Errorf("%s に %s が出た(read model は持ち物を持たない。ADR-0175 §5)", name, needle)
			}
		}
	}
}

// 役割を足しても read model の出力はバイト単位で変わらない(同じマスタなら同じ出力。dataVersion・checksum も同じ)。
func TestExportIsIndependentOfItemData(t *testing.T) {
	base, _ := export(t, storetest.New())
	q := storetest.New()
	q.ItemEffects = nil // 持ち物の効果を消しても read model は変わらない(持ち物に依存しない)
	changed, _ := export(t, q)
	if !bytes.Equal(base.PokemonTypes, changed.PokemonTypes) || !bytes.Equal(base.Moves, changed.Moves) ||
		!bytes.Equal(base.Abilities, changed.Abilities) || !bytes.Equal(base.SpeedPokemon, changed.SpeedPokemon) ||
		!bytes.Equal(base.TypeChart, changed.TypeChart) || !bytes.Equal(base.Metadata, changed.Metadata) {
		t.Error("持ち物の効果を変えたら read model が変わった(read model は持ち物に依存しない)")
	}
}
