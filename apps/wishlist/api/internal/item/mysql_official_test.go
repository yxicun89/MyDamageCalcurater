//go:build mysql

package item_test

import (
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/item/itemtest"
)

// フェーズ4-3(docs/phase4-spec.md AC-O4〜O8)。migration 000008 が要る。
func TestMySQLOfficialRepositoryContract(t *testing.T) {
	itemtest.RunOfficialRepositoryContract(t, func(t *testing.T) itemtest.OfficialFullRepository {
		return newMySQLRepo(t).(*item.MySQLRepository)
	})
}
